package rpc

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// Middleware implements the unary and stream interceptors on the gRPC server for authorization
type Middleware struct {
	db DB
}

// authz returns true if the jobId exists under the given owner
func authz(db DB, owner, jobId string) bool {
	return db.Get(owner, jobId) == nil
}

// addOwnerMetadata extracts the common name from the tls context and appends it to the grpc's context metadata
func (m *Middleware) addOwnerMetadata(ctx context.Context) (string, context.Context) {
	if p, ok := peer.FromContext(ctx); ok {
		if mtls, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			if len(mtls.State.PeerCertificates) < 1 {
				return "", nil
			}
			// Append common name to metadata
			cn := mtls.State.PeerCertificates[0].Subject.CommonName
			md, ok := metadata.FromIncomingContext(ctx)
			if ok {
				md.Append("owner", cn)
			}
			return cn, metadata.NewIncomingContext(ctx, md)
		}
	}
	return "", nil
}

// GenericRequest wraps a generic interface around the protobuf requests so we can call GetId()
type GenericRequest interface {
	GetId() string
}

// Unary implements the Start, Stop and Status authz scheme using a grpc interceptor
func (m *Middleware) Unary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	owner, newCtx := m.addOwnerMetadata(ctx)
	if newCtx == nil {
		return nil, status.Errorf(codes.Unauthenticated, "no common name available in client cert")
	}
	// Allow any client to start a job, check ownership for status, logs and stop
	if info.FullMethod != "/JobWorker.Worker/Start" {
		r, ok := req.(GenericRequest)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "could not parse request")
		}
		fmt.Printf("%s request from owner: %s, job UUID: %s\n", info.FullMethod, owner, r.GetId())
		if authz(m.db, owner, r.GetId()) {
			return nil, status.Errorf(codes.Unauthenticated, "invalid job UUID")
		}
	}
	return handler(newCtx, req)
}

// wrappedStream wraps the ServerStream with a context to pass metadata to the stream handler and a RecvMsg to authorize log requests
type wrappedStream struct {
	grpc.ServerStream
	ctx   context.Context
	db    DB
	owner string
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

// RecvMsg intercepts a stream request for authorization by checking the ownership of the job
func (s *wrappedStream) RecvMsg(m interface{}) error {
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return err
	}
	req, ok := m.(GenericRequest)
	if !ok {
		return status.Errorf(codes.InvalidArgument, "could not parse logs request")
	}
	fmt.Printf("/JobWorker.Worker/Logs stream request from owner: %s, job UUID: %s\n", s.owner, req.GetId())
	if authz(s.db, s.owner, req.GetId()) {
		return status.Errorf(codes.Unauthenticated, "invalid job UUID")
	}
	return nil
}

// Stream implements the Logs authz scheme using a grpc interceptor
func (m *Middleware) Stream(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	owner, newCtx := m.addOwnerMetadata(ctx)
	if newCtx == nil {
		return status.Errorf(codes.Unauthenticated, "no common name available in client cert")
	}
	return handler(srv, &wrappedStream{ss, newCtx, m.db, owner})
}
