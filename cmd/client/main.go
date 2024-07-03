package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	pb "github.com/teleport-jobworker/pkg/proto"
	"github.com/teleport-jobworker/pkg/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	port       = flag.Int("port", 50051, "the port to serve on")
	cpuWeight  = flag.Int("cpu", 100, "CPU weight as defined y cgroups v2 `cpu.weight` interface file")
	memLimit   = flag.String("mem", "100M", "Memory limit as defined y cgroups v2 `mem.high` interface file")
	ioWeight   = flag.Int("io", 50, "IO weight as defined y cgroups v2 `io.weight` interface file")
	followLogs = flag.Bool("f", false, "Follows the job's logs, similiar to tail -f")
)

func help() {
	fmt.Println("not enough arguments! usage:")
	fmt.Println(`./client start bash -c "echo hello"`)
	fmt.Println(`or ./client status {uuid}`)
	fmt.Println(`or ./client stop {uuid}`)
	fmt.Println(`or ./client logs {uuid}`)
}

func main() {
	// Parse CLI args
	if len(os.Args) < 3 {
		help()
		return
	}
	flag.Parse()
	args := flag.Args()
	// Set up gRPC client
	tlsConfig, err := loadTLSConfig("certs/client.pem", "certs/client-key.pem", "certs/root.pem")
	if err != nil {
		fmt.Printf("error loading tls config: %v", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// TODO in production we would use an actual DNS to provide additional host verification in the TLS
	conn, err := grpc.DialContext(ctx, fmt.Sprintf("localhost:%d", *port), grpc.WithTransportCredentials(tlsConfig))
	if err != nil {
		fmt.Printf("error loading tls config: %v", err)
		return
	}
	defer conn.Close()
	client := pb.NewWorkerClient(conn)

	// Decide which action to execute
	switch args[0] {
	case "start":
		id, err := rpc.Start(ctx, client, args[1], args[2:], int32(*cpuWeight), int32(*ioWeight), *memLimit)
		if err != nil {
			fmt.Printf("error starting job: %v\n", err)
		} else {
			fmt.Printf("Started Job %s\n", id)
			fmt.Printf("View the logs: ./worker logs %s\n", id)
			fmt.Printf("Check the status: ./worker status %s\n", id)
			fmt.Printf("Stop the job: ./worker stop %s\n", id)
		}
		break
	case "stop":
		if err = rpc.Stop(ctx, client, args[1]); err != nil {
			fmt.Printf("error stopping job: %v\n", err)
		}
		break
	case "status":
		if status, err := rpc.Status(ctx, client, args[1]); err != nil {
			fmt.Printf("error getting status for job: %v\n", err)
		} else {
			fmt.Println("Job Status")
			fmt.Println("ID: ", status.Id)
			fmt.Println("PID: ", status.Pid)
			fmt.Println("Running: ", status.Running)
			fmt.Println("Exit Code: ", status.ExitCode)
		}
		break
	case "logs":
		streamCtx, streamCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer streamCancel()
		if err = rpc.Logs(streamCtx, client, args[1], *followLogs); err != nil && err != io.EOF {
			fmt.Printf("error getting job logs: %v\n", err)
		}
		break
	default:
		fmt.Printf("%s action not supported, try start, status, stop or logs", os.Args[1])
		help()
		break
	}
}

// loadTLSConfig returns a mtls configured TransportCredentials for the gRPC connection
func loadTLSConfig(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
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
