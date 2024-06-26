package rpc

import (
	"bufio"
	"context"
	"fmt"
	"log"

	"github.com/teleport-jobworker/pkg/jobworker"
	pb "github.com/teleport-jobworker/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

type ErrNotFound struct {
	id string
}

func (err ErrNotFound) Error() string {
	return fmt.Sprintf("could not find job %s", err.id)
}

type JobsDB interface {
	Get(string, string) *jobworker.Job
	Update(string, *jobworker.Job)
	Remove(string, string)
}

// Server implements the grpc service Worker
type Server struct {
	pb.UnimplementedWorkerServer
	db JobsDB
}

// NewServer returns an initialized Server with in memory DB of jobs
func NewServer() *Server {
	return &Server{
		db: &InMemoryJobsDB{jobs: map[string]jobList{}},
	}
}

func getOwner(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("could not get metadata from context")
	}
	owner, ok := md["owner"]
	if !ok {
		return "", fmt.Errorf("could not find owner")
	}
	if len(md["owner"]) < 1 {
		return "", fmt.Errorf("could not find at least 1 owner")
	}
	fmt.Println("owner")
	fmt.Println(owner[0])
	return owner[0], nil
}

func (s *Server) Start(ctx context.Context, req *pb.StartRequest) (*pb.StartResponse, error) {
	owner, err := getOwner(ctx)
	if err != nil {
		return nil, err
	}
	// Define job's command and options
	opts := jobworker.NewOpts(req.Opts.CpuWeight, req.Opts.IoWeight, jobworker.ParseCgroupByte(req.Opts.MemLimit))
	// Run the job
	job, err := jobworker.Start(opts, req.Command, req.Args...)
	if err != nil {
		fmt.Print("failed to start command")
		fmt.Print(err)
		return nil, err
	}
	s.db.Update(owner, job)
	return &pb.StartResponse{Id: job.ID}, nil
}

func (s *Server) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	owner, err := getOwner(ctx)
	if err != nil {
		return nil, err
	}
	job := s.db.Get(owner, req.Id)
	if job == nil {
		return nil, &ErrNotFound{req.Id}
	}
	err = job.Stop()
	if err != nil {
		return nil, err
	}
	s.db.Remove(owner, job.ID)
	return &pb.StopResponse{}, nil
}

func (s *Server) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	owner, err := getOwner(ctx)
	if err != nil {
		return nil, err
	}
	job := s.db.Get(owner, req.Id)
	if job == nil {
		return nil, &ErrNotFound{req.Id}
	}
	status := job.Status()
	return &pb.StatusResponse{JobStatus: &pb.JobStatus{
		Id:       job.ID,
		Pid:      int64(status.PID), // todo fix unneeded casts
		Running:  status.Running,
		ExitCode: int32(status.ExitCode),
	}}, nil
}

func (s *Server) Output(req *pb.OutputRequest, stream pb.Worker_OutputServer) error {
	ctx := stream.Context()
	owner, err := getOwner(ctx)
	if err != nil {
		return err
	}
	job := s.db.Get(owner, req.Id)
	if job == nil {
		return &ErrNotFound{req.Id}
	}
	reader, err := job.Output()
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		// line := scanner.Text()
		err = stream.Send(&pb.Data{Bytes: scanner.Bytes()})
		if err != nil {
			return err
		}
		// fmt.Printf("%s\n", line)
	}
	return nil
}

// MiddlewareHandler performs authz by checking the job ID belongs to the owner (subject name)
func MiddlewareHandler(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	if p, ok := peer.FromContext(ctx); ok {
		if mtls, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			for _, item := range mtls.State.PeerCertificates {
				log.Println("client certificate subject: ", item.Subject.CommonName)

				md, ok := metadata.FromIncomingContext(ctx)
				if ok {
					md.Append("owner", item.Subject.CommonName)
				}
				newCtx := metadata.NewIncomingContext(ctx, md)

				// Allow any client after authentication to start a job
				if info.FullMethod != "/JobWorker.Worker/Start" {
					// TODO check client_id exists and owns the desired job, how to parse req?
				}
				return handler(newCtx, req)
			}
		}
	}
	return handler(ctx, req)
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

func newWrappedStream(s grpc.ServerStream, ctx context.Context) grpc.ServerStream {
	return &wrappedStream{s, ctx}
}

// todo extract common logic from this and unary operator
func StreamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	if p, ok := peer.FromContext(ctx); ok {
		if mtls, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			for _, item := range mtls.State.PeerCertificates {
				log.Println("client certificate subject: ", item.Subject.CommonName)

				md, ok := metadata.FromIncomingContext(ctx)
				if ok {
					md.Append("owner", item.Subject.CommonName)
				}
				newCtx := metadata.NewIncomingContext(ctx, md)
				// TODO check client_id exists and owns the desired job, how to parse req?
				return handler(srv, newWrappedStream(ss, newCtx))
			}
		}
	}
	return handler(srv, ss)
}
