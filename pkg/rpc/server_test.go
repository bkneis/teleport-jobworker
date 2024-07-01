//go:build integration_tests

package rpc

import (
	"context"
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
