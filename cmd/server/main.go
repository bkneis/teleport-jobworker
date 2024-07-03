package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/teleport-jobworker/pkg/rpc"
)

var port = flag.Int("port", 50051, "the port to serve on")

func main() {
	// Parse CLI args
	flag.Parse()
	log.Printf("server starting on port %d...\n", *port)
	// Setup and run gRPC server
	s := rpc.NewServer()
	// TODO in production we would use an actual DNS to provide additional host verification in the TLS
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
