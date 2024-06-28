//go:build integration_tests

package rpc

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	pb "github.com/teleport-jobworker/pkg/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGRPCServerCanStartGetStatusAndStopJobs(t *testing.T) {
	// Run grpc server and shutdown after test
	s := NewServer()
	go startServer(s)
	defer s.GracefulStop()
	// Create grpc client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, client, err := newClient(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Start a job with a long running process
	var jobId string
	if jobId, err = Start(ctx, client, []string{"", "", "bash", "-c", "while true; do echo hello; sleep 1; done"}, 100, 100, "100M"); err != nil {
		t.Errorf("expected start job to return non nil error: actual error %v", err)
	}
	// Assert the status show it's running
	var status *pb.JobStatus
	if status, err = Status(ctx, client, []string{"", "", jobId}); err != nil {
		t.Errorf("expected status to return non nil error: actual error %v", err)
	}
	if !status.Running {
		t.Error("expected job to be running and it isn't")
	}
	// Stop the job and assert no errors and process isn't running
	if err = Stop(ctx, client, []string{"", "", jobId}); err != nil {
		t.Errorf("expected stop to return non nil error: actual error %v", err)
	}
	_, err = os.FindProcess(int(status.Pid))
	if err != nil {
		t.Errorf("expected process not to be running")
		return
	}
}

// todo fix
func _TestGRPCServerCanHandleConcurrentReaders(t *testing.T) {
	// Run grpc server and shutdown after test
	s := NewServer()
	go startServer(s)
	defer s.GracefulStop()
	// Create grpc client
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, client, err := newClient(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Start a job with a long running process
	var jobId string
	if jobId, err = Start(ctx, client, []string{"", "", "bash", "-c", "while true; do echo hello; sleep 0.2; done"}, 100, 100, "100M"); err != nil {
		t.Errorf("expected start job to return non nil error: actual error %v", err)
	}

	// Start 5 concurrent readers and stream the output for 2 seconds asserting the log output and no errors
	for i := 0; i < 5; i++ {
		go func(id string) {
			// Get stream to log output
			conn, c, err := newClient(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			req := &pb.OutputRequest{Id: id}
			stream, err := c.Output(ctx, req)
			if err != nil {
				t.Errorf("expected output not to return an error, actual error: %v", err)
				return
			}
			defer stream.CloseSend()
			// Process logs for 2 seconds and assert the output with no errors
			start := time.Now()
			for {
				if start.Add(2*time.Second).Unix() < time.Now().Unix() {
					return
				}
				data, err := stream.Recv()
				if err != nil {
					t.Errorf("expected nil error receiving logs, actual error: %v", err)
					return
				}
				line := string(data.GetBytes())
				fmt.Println(line)
				if line != "hello" {
					t.Errorf("expected log output to be hello but was actually: %s", line)
					break
				}
			}
		}(jobId)
	}
	time.Sleep(2 * time.Second)
	// Clean the job up
	if err = Stop(ctx, client, []string{"", "", jobId}); err != nil {
		t.Errorf("expected stop to return non nil error: actual error %v", err)
	}
}

// TestGrpcServerAuthz ensures a client (client2) using a different tls cert and common name cannot execute commands on a job
// owned by another client (localhost)
func TestGrpcServerAuthz(t *testing.T) {
	// Run grpc server and shutdown after test
	s := NewServer()
	go startServer(s)
	defer s.GracefulStop()
	// Create grpc client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, client, err := newClient(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Start a job with a long running process
	var jobId string
	if jobId, err = Start(ctx, client, []string{"", "", "bash", "-c", "while true; do echo hello; sleep 1; done"}, 100, 100, "100M"); err != nil {
		t.Errorf("expected start job to return non nil error: actual error %v", err)
	}

	// Create grpc client2
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()
	conn2, client2, err := newClient2(ctx2)
	if err != nil {
		t.Fatal(err)
	}
	defer conn2.Close()

	// Stop the job using client 2 and assert unauth error
	if err = Stop(ctx, client2, []string{"", "", jobId}); err != nil {
		st, ok := status.FromError(err)
		if !ok {
			t.Errorf("expected return error to be status")
		}
		if st.Code() != codes.Unauthenticated {
			t.Errorf("expected return code to be unauthenticated, actual: %d", st.Code())
		}
	}

	// Status the job and assert no errors and process isn't running
	if _, err = Status(ctx, client2, []string{"", "", jobId}); err != nil {
		st, ok := status.FromError(err)
		if !ok {
			t.Errorf("expected return error to be status")
		}
		if st.Code() != codes.Unauthenticated {
			t.Errorf("expected return code to be unauthenticated, actual: %d", st.Code())
		}
	}

	// Stop the job and assert no errors and process isn't running
	if err = Stop(ctx, client, []string{"", "", jobId}); err != nil {
		t.Errorf("expected stop to not return an error %v", err)
	}
}
