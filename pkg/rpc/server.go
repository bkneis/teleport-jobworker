package rpc

import (
	"context"
	"log"

	"github.com/google/uuid"
	pb "github.com/teleport-jobworker/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedWorkerServer
}

func (s *Server) Start(ctx context.Context, req *pb.StartRequest) (*pb.StartResponse, error) {
	return &pb.StartResponse{Id: uuid.New().String()}, nil
}
func (s *Server) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	return &pb.StopResponse{}, nil
}
func (s *Server) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	return &pb.StatusResponse{JobStatus: &pb.JobStatus{
		Id:       req.Id,
		Pid:      0,
		Running:  false,
		ExitCode: -1,
	}}, nil
}
func (s *Server) Output(req *pb.OutputRequest, stream pb.Worker_OutputServer) error {
	return status.Errorf(codes.Unimplemented, "method Output not implemented")
}

func MiddlewareHandler(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	// Allow any client after authentication to start a job
	if info.FullMethod == "/JobWorker.Worker/Start" {
		return handler(ctx, req)
	}
	if p, ok := peer.FromContext(ctx); ok {
		if mtls, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			for _, item := range mtls.State.PeerCertificates {
				log.Println("client certificate subject: ", item.Subject)
				// todo check client_id exists and owns the desired job
				ctx = context.WithValue(ctx, "client_id", item.Subject)
			}
		}
	}
	return handler(ctx, req)
}
