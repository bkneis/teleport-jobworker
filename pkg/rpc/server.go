package rpc

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	"github.com/teleport-jobworker/pkg/jobworker"
	pb "github.com/teleport-jobworker/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// ErrNotFound is returned when a job was not found using the UUID
type ErrNotFound struct {
	id string
}

func (err ErrNotFound) Error() string {
	return fmt.Sprintf("could not find job %s", err.id)
}

// DB defines how to persist jobs across rpc requests, including ownership for authz
type DB interface {
	Get(string, string) *jobworker.Job
	Update(string, *jobworker.Job)
	Remove(string, string)
}

// Server implements the grpc service Worker
type Server struct {
	pb.UnimplementedWorkerServer
	db DB
}

// newServer returns an initialized Server with in memory DB of jobs
func newServer(db DB) *Server {
	return &Server{
		db: db,
	}
}

// NewServer returns a grpc.Server set up with mtls
func NewServer() *grpc.Server {
	// todo fix file paths
	cert, err := tls.LoadX509KeyPair("/home/arthur/go/src/github.com/teleport-jobworker/certs/server.pem", "/home/arthur/go/src/github.com/teleport-jobworker/certs/server-key.pem")
	if err != nil {
		log.Fatalf("failed to load key pair: %s", err)
	}

	ca := x509.NewCertPool()
	caFilePath := "/home/arthur/go/src/github.com/teleport-jobworker/certs/root.pem"
	caBytes, err := os.ReadFile(caFilePath)
	if err != nil {
		log.Fatalf("failed to read ca cert %q: %v", caFilePath, err)
	}
	if ok := ca.AppendCertsFromPEM(caBytes); !ok {
		log.Fatalf("failed to parse %q", caFilePath)
	}

	tlsConfig := &tls.Config{
		ClientAuth:         tls.RequireAndVerifyClientCert,
		Certificates:       []tls.Certificate{cert},
		ClientCAs:          ca,
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: false,
	}

	db := &JobsDB{jobs: map[string]jobList{}}
	m := Middleware{db}

	s := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)), grpc.UnaryInterceptor(m.Unary), grpc.StreamInterceptor(m.Stream))
	pb.RegisterWorkerServer(s, newServer(db))
	return s
}

// getOwner extracts the owner from a request's context metadata
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
		err = stream.Send(&pb.Data{Bytes: scanner.Bytes()})
		if err != nil {
			return err
		}
	}
	return nil
}
