package rpc

import (
	"context"
	"fmt"

	pb "github.com/teleport-jobworker/pkg/proto"
)

// Start sends a Start request to the gRPC server given a client and returns it's ID
func Start(ctx context.Context, client pb.WorkerClient, args []string, cpuWeight, ioWeight int32, memLimit string) (string, error) {
	req := &pb.StartRequest{
		Command: args[2],
		Args:    args[3:],
		Opts:    &pb.JobOpts{CpuWeight: cpuWeight, MemLimit: memLimit, IoWeight: ioWeight},
	}
	resp, err := client.Start(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.GetId(), nil
}

// Stop sends a Stop request to the gRPC server given a client and checks for errors
func Stop(ctx context.Context, client pb.WorkerClient, args []string) error {
	req := &pb.StopRequest{Id: args[2]}
	_, err := client.Stop(ctx, req)
	if err != nil {
		return err
	}
	fmt.Printf("Stopped job %s\n", req.Id)
	return nil
}

// Status sends a Status request to the gRPC server given a client and returns a JobStatus
func Status(ctx context.Context, client pb.WorkerClient, args []string) (*pb.JobStatus, error) {
	req := &pb.StatusRequest{Id: args[2]}
	resp, err := client.Status(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.JobStatus, nil
}

// Logs sends a Output request to the gRPC server and logs the output stream
func Logs(ctx context.Context, client pb.WorkerClient, args []string) error {
	req := &pb.OutputRequest{Id: args[2]}
	stream, err := client.Output(ctx, req)
	if err != nil {
		return err
	}
	for {
		data, err := stream.Recv()
		if err != nil {
			return err
		}
		fmt.Println(string(data.GetBytes()))
	}
}
