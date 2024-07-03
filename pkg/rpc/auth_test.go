package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/teleport-jobworker/certs"
	pb "github.com/teleport-jobworker/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func TestMtlsRejectsLowTlsVersion(t *testing.T) {
	s := NewServer()
	go startServer(s)
	defer s.GracefulStop()

	tlsConfig, err := loadTLSLowVersion(certs.Path("./client.pem"), certs.Path("./client-key.pem"), certs.Path("./root.pem"))
	if err != nil {
		t.Fatal(err)
	}
	// Connect client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "localhost:50051", grpc.WithTransportCredentials(tlsConfig))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewWorkerClient(conn)

	// Try to call gRPC and assert TLS connection was rejected
	if _, err = Status(ctx, client, ""); err != nil {
		if !strings.Contains(err.Error(), "tls: no supported versions satisfy MinVersion") && !strings.Contains(err.Error(), "tls: protocol version not supported") {
			t.Errorf("expected connection to be rejected for low tls version: actual error %v", err)
		}
	}
}

func TestMtlsChecksClientCert(t *testing.T) {
	s := NewServer()
	go startServer(s)
	defer s.GracefulStop()

	tlsConfig, err := loadTLSWithoutClientCert(certs.Path("./root.pem"))
	if err != nil {
		t.Fatal(err)
	}
	// Connect client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "localhost:50051", grpc.WithTransportCredentials(tlsConfig))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewWorkerClient(conn)

	// Assert error is either tls cert required or the connection was already torn down
	if _, err = Start(ctx, client, "bash", []string{"-c", "echo test"}, 100, 100, "100M"); err != nil {
		if !strings.Contains(err.Error(), "tls: certificate required") && !strings.Contains(err.Error(), "write: broken pipe") {
			t.Errorf("expected connection to be rejected for no client cert: actual error %v", err)
		}
	}
}

// startServer runs the gRPC JobWorker service blocking call Serve
func startServer(s *grpc.Server) {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", 50051))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// loadTLSLowVersion provides a client TLS config that sets the max tls version to 1.2
func loadTLSLowVersion(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
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
		MaxVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// loadTLSWithoutClientCert returns a client TLS config that doesn't provide client cert
func loadTLSWithoutClientCert(caFile string) (credentials.TransportCredentials, error) {
	ca, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("faild to read CA certificate: %w", err)
	}

	capool := x509.NewCertPool()
	if !capool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("faild to append the CA certificate to CA pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{},
		RootCAs:      capool,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// newClient initiates a gRPC client connected to the gRPC server
func newClient(ctx context.Context) (*grpc.ClientConn, pb.WorkerClient, error) {
	tlsConfig, err := loadTLS(certs.Path("./client.pem"), certs.Path("./client-key.pem"), certs.Path("./root.pem"))
	if err != nil {
		return nil, nil, err
	}
	conn, err := grpc.DialContext(ctx, "localhost:50051", grpc.WithTransportCredentials(tlsConfig))
	if err != nil {
		return nil, nil, err
	}
	return conn, pb.NewWorkerClient(conn), nil
}

// newClient2 does the same as newClient but using the client2 cert/key pair with different common name
func newClient2(ctx context.Context) (*grpc.ClientConn, pb.WorkerClient, error) {
	tlsConfig, err := loadTLS(certs.Path("./client2.pem"), certs.Path("./client2-key.pem"), certs.Path("./root.pem"))
	if err != nil {
		return nil, nil, err
	}
	conn, err := grpc.DialContext(ctx, "localhost:50051", grpc.WithTransportCredentials(tlsConfig))
	if err != nil {
		return nil, nil, err
	}
	return conn, pb.NewWorkerClient(conn), nil
}

// loadTLS returns the good TLS config
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
