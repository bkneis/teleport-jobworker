package rpc

import (
	"context"
	"fmt"

	pb "github.com/teleport-jobworker/pkg/proto"
)

// Start sends a Start request to the gRPC server given a client and returns it's ID
func Start(
	ctx context.Context,
	client pb.WorkerClient,
	command string,
	args []string,
	cpuWeight, ioWeight int32,
	memLimit string) (string, error) {
	req := &pb.StartRequest{
		Command: command,
		Args:    args,
		Opts:    &pb.JobOpts{CpuWeight: cpuWeight, MemLimit: memLimit, IoWeight: ioWeight},
	}
	resp, err := client.Start(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.GetId(), nil
}

// Stop sends a Stop request to the gRPC server given a client and checks for errors
func Stop(ctx context.Context, client pb.WorkerClient, id string) error {
	req := &pb.StopRequest{Id: id}
	_, err := client.Stop(ctx, req)
	if err != nil {
		return err
	}
	fmt.Printf("Stopped job %s\n", req.Id)
	return nil
}

// Status sends a Status request to the gRPC server given a client and returns a JobStatus
func Status(ctx context.Context, client pb.WorkerClient, id string) (*pb.JobStatus, error) {
	req := &pb.StatusRequest{Id: id}
	resp, err := client.Status(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.JobStatus, nil
}

// Logs sends a Output request to the gRPC server and logs the output stream
func Logs(ctx context.Context, client pb.WorkerClient, id string, follow bool) error {
	req := &pb.OutputRequest{Id: id, Follow: follow}
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
