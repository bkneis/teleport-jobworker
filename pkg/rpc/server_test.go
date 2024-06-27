//go:build integration_tests

package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/teleport-jobworker/certs"
	pb "github.com/teleport-jobworker/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func TestGRPCServer(t *testing.T) {
	// Run grpc server and shutdown after test
	s := NewServer()
	go startServer(s)
	defer s.GracefulStop()
	// Create grpc client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, client := newClient(ctx)
	defer conn.Close()
	// Start a job with a long running process
	var err error
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
	conn, client := newClient(ctx)
	defer conn.Close()
	// Start a job with a long running process
	var err error
	var jobId string
	if jobId, err = Start(ctx, client, []string{"", "", "bash", "-c", "while true; do echo hello; sleep 0.2; done"}, 100, 100, "100M"); err != nil {
		t.Errorf("expected start job to return non nil error: actual error %v", err)
	}

	// Start 5 concurrent readers and stream the output for 2 seconds asserting the log output and no errors
	for i := 0; i < 5; i++ {
		go func(id string) {
			// Get stream to log output
			conn, c := newClient(ctx)
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

// todo test authz with owner check and using other cert to query job status

func newClient(ctx context.Context) (*grpc.ClientConn, pb.WorkerClient) {
	tlsConfig, err := loadTLS(certs.Path("./client.pem"), certs.Path("./client-key.pem"), certs.Path("./root.pem"))
	if err != nil {
		panic(err)
	}
	conn, err := grpc.DialContext(ctx, "localhost:50051", grpc.WithTransportCredentials(tlsConfig))
	if err != nil {
		panic(err)
	}
	return conn, pb.NewWorkerClient(conn)
}

func loadTLS(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certification: %w", err)
	}

	ca, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("faild to read CA certificate: %w", err)
	}

	capool := x509.NewCertPool()
	if !capool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("faild to append the CA certificate to CA pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      capool,
	}

	return credentials.NewTLS(tlsConfig), nil
}
