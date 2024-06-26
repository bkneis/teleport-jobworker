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

	pb "github.com/teleport-jobworker/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func TestGRPCServer(t *testing.T) {
	s := NewServer()
	go startServer(s)
	defer s.GracefulStop()

	tlsConfig, err := loadTLS("/home/arthur/go/src/github.com/teleport-jobworker/certs/client.pem", "/home/arthur/go/src/github.com/teleport-jobworker/certs/client-key.pem", "/home/arthur/go/src/github.com/teleport-jobworker/certs/root.pem")
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "localhost:50051", grpc.WithTransportCredentials(tlsConfig))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := pb.NewWorkerClient(conn)

	if err = Start(ctx, client, []string{"", "", "bash", "-c", "echo test"}, 100, 100, "100M"); err != nil {
		t.Errorf("expected start job to return non nil: actual error %v", err)
	}
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
