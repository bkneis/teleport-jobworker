package rpc

import (
	"context"
	"fmt"

	pb "github.com/teleport-jobworker/pkg/proto"
)

func Start(ctx context.Context, client pb.WorkerClient, args []string, cpuWeight, ioWeight int32, memLimit string) error {
	req := &pb.StartRequest{
		Command: args[2],
		Args:    args[3:],
		Opts:    &pb.JobOpts{CpuWeight: cpuWeight, MemLimit: memLimit, IoWeight: ioWeight},
	}
	resp, err := client.Start(ctx, req)
	if err != nil {
		return err
	}
	fmt.Printf("Start Job %s\n", resp.GetId())
	return nil
}

func Stop(ctx context.Context, client pb.WorkerClient, args []string) error {
	req := &pb.StopRequest{Id: args[2]}
	_, err := client.Stop(ctx, req)
	if err != nil {
		return err
	}
	fmt.Printf("Stopped job %s\n", req.Id)
	return nil
}

func Status(ctx context.Context, client pb.WorkerClient, args []string) error {
	req := &pb.StatusRequest{Id: args[2]}
	resp, err := client.Status(ctx, req)
	if err != nil {
		return err
	}
	fmt.Println("Job Status")
	fmt.Println("ID: ", resp.JobStatus.Id)
	fmt.Println("PID: ", resp.JobStatus.Pid)
	fmt.Println("Running: ", resp.JobStatus.Running)
	fmt.Println("Exit Code: ", resp.JobStatus.ExitCode)
	return nil
}

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
