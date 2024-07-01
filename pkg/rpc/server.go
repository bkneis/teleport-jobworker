package rpc

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	"github.com/teleport-jobworker/certs"
	"github.com/teleport-jobworker/pkg/jobworker"
	pb "github.com/teleport-jobworker/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ErrNotFound is returned when a job was not found using the UUID
var ErrNotFound = status.Errorf(codes.Unauthenticated, "invalid job UUID")

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

// NewServer returns a grpc.Server that implements WorkerServer set up with mtls and authz middleware
func NewServer() *grpc.Server {
	// Load TLS certs
	cert, err := tls.LoadX509KeyPair(certs.Path("./server.pem"), certs.Path("./server-key.pem"))
	if err != nil {
		log.Fatalf("failed to load key pair: %s", err)
	}
	ca := x509.NewCertPool()
	caFilePath := certs.Path("./root.pem")
	caBytes, err := os.ReadFile(caFilePath)
	if err != nil {
		log.Fatalf("failed to read ca cert %q: %v", caFilePath, err)
	}
	if ok := ca.AppendCertsFromPEM(caBytes); !ok {
		log.Fatalf("failed to parse %q", caFilePath)
	}
	// Initialise TLS config with min version 1.3 and mtls
	tlsConfig := &tls.Config{
		ClientAuth:         tls.RequireAndVerifyClientCert,
		Certificates:       []tls.Certificate{cert},
		ClientCAs:          ca,
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: false,
	}
	// Initialise jobs database and pass a reference to grpc server and middleware
	db := &JobsDB{jobs: map[string]jobList{}}
	m := Middleware{db}
	// Set up gRPC server
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

// Start runs a command as a job
func (s *Server) Start(ctx context.Context, req *pb.StartRequest) (*pb.StartResponse, error) {
	owner, err := getOwner(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	// Define job's command and options
	memLimit, err := jobworker.ParseCgroupByte(req.Opts.MemLimit)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "mem limit job option was not valid")
	}
	opts := jobworker.NewOpts(req.Opts.CpuWeight, req.Opts.IoWeight, memLimit)
	// Run the job
	job, err := jobworker.Start(opts, req.Command, req.Args...)
	if err != nil {
		fmt.Print("failed to start command")
		fmt.Print(err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	s.db.Update(owner, job)
	return &pb.StartResponse{Id: job.ID}, nil
}

// Stop kills a job's process and cleans up it's environment
func (s *Server) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	owner, err := getOwner(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	// Get Job from DB
	job := s.db.Get(owner, req.Id)
	if job == nil {
		fmt.Printf("Job not found using id=%s\n", req.Id)
		return nil, ErrNotFound
	}
	// Stop the job from executing
	err = job.Stop()
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	// TODO in production we would need to "clean" up these jobs by running some go routine to remove old terminated jobs
	// Remove job from DB
	// s.db.Remove(owner, job.ID)
	return &pb.StopResponse{}, nil
}

// Status returns the status of a running job
func (s *Server) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	owner, err := getOwner(ctx)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	// Get Job from DB
	job := s.db.Get(owner, req.Id)
	if job == nil {
		fmt.Printf("Job not found using id=%s\n", req.Id)
		return nil, ErrNotFound
	}
	// Convert job status to pb.JobStatus
	status := job.Status()
	return &pb.StatusResponse{JobStatus: &pb.JobStatus{
		Id:       job.ID,
		Pid:      status.PID,
		Running:  status.Running,
		ExitCode: int32(status.ExitCode),
	}}, nil
}

// Output pipes the STDOUT and STDERR of a job to a gRPC stream
func (s *Server) Output(req *pb.OutputRequest, stream pb.Worker_OutputServer) error {
	ctx := stream.Context()
	owner, err := getOwner(ctx)
	if err != nil {
		return status.Error(codes.Unauthenticated, err.Error())
	}
	job := s.db.Get(owner, req.Id)
	if job == nil {
		fmt.Printf("Job not found using id=%s\n", req.Id)
		return ErrNotFound
	}
	// Get io.ReadCloser to job's output and pipe over gRPC stream
	reader, err := job.Output()
	if err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		err = stream.Send(&pb.Data{Bytes: scanner.Bytes()})
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	}
	return nil
}
