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

	pb "github.com/teleport-jobworker/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func startServer(s *grpc.Server) {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", 50051))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func TestMtlsRejectsLowTlsVersion(t *testing.T) {
	s := NewServer()
	go startServer(s)
	defer s.GracefulStop()

	tlsConfig, err := loadTLSLowVersion("/home/arthur/go/src/github.com/teleport-jobworker/certs/client.pem", "/home/arthur/go/src/github.com/teleport-jobworker/certs/client-key.pem", "/home/arthur/go/src/github.com/teleport-jobworker/certs/root.pem")
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
	if err = Status(ctx, client, os.Args); err != nil {
		if !strings.Contains(err.Error(), "tls: no supported versions satisfy MinVersion") {
			t.Errorf("expected connection to be rejected for low tls version: actual error %v", err)
		}
	}
}

func TestMtlsChecksClientCert(t *testing.T) {
	s := NewServer()
	go startServer(s)
	defer s.GracefulStop()

	tlsConfig, err := loadTLSWithoutClientCert("/home/arthur/go/src/github.com/teleport-jobworker/certs/root.pem")
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
		// Check error is either tls cert required or the connection was already torn down
		if !strings.Contains(err.Error(), "tls: certificate required") && !strings.Contains(err.Error(), "write: broken pipe") {
			t.Errorf("expected connection to be rejected for no client cert: actual error %v", err)
		}
	}
}

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
		MaxVersion:   tls.VersionTLS10,
	}

	return credentials.NewTLS(tlsConfig), nil
}

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
