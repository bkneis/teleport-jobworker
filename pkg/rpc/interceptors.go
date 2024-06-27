package rpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type Middleware struct {
	db DB
}

// addOwnerMetadata extracts the common name from the tls context and appends it to the grpc's context metadata
func (m *Middleware) addOwnerMetadata(ctx context.Context) (string, context.Context) {
	if p, ok := peer.FromContext(ctx); ok {
		if mtls, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			if len(mtls.State.PeerCertificates) < 1 {
				return "", nil
			}
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

// Unary implements the Start, Stop and Status authz scheme using a grpc interceptor
func (m *Middleware) Unary(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	_, newCtx := m.addOwnerMetadata(ctx)
	if newCtx == nil {
		return nil, status.Errorf(codes.Unauthenticated, "no common name available in client cert")
	}
	// Allow any client to start a job, check ownership for status, logs and stop
	// if info.FullMethod != "/JobWorker.Worker/Start" {
	// 	if j := m.db.Get(owner, "job id"); j == nil {
	// 		return nil, status.Errorf(codes.Unauthenticated, "invalid job UUID")
	// 	}
	// }
	return handler(newCtx, req)
}

// wrappedStream wraps the grpc.ServerStream with a context to pass metadata to the stream handler
type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context {
	return w.ctx
}

// Stream implements the Logs authz scheme using a grpc interceptor
func (m *Middleware) Stream(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := ss.Context()
	_, newCtx := m.addOwnerMetadata(ctx)
	if newCtx == nil {
		return status.Errorf(codes.Unauthenticated, "no common name available in client cert")
	}
	// todo check client has ownership for job, need to find out how to get job ID from req
	// if j := m.db.Get(owner, "job id"); j == nil {
	// 	return status.Errorf(codes.Unauthenticated, "invalid job UUID")
	// }
	return handler(srv, &wrappedStream{ss, newCtx})
}
