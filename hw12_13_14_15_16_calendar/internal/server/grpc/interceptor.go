package internalgrpc

import (
	"context"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

func loggingInterceptor(logger Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		var clientIP string

		if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
			host, _, err := net.SplitHostPort(p.Addr.String())
			if err == nil {
				clientIP = host
			} else {
				clientIP = p.Addr.String()
			}
		}

		code := status.Code(err)

		logger.Info(
			"grpc request",
			"method", info.FullMethod,
			"client_ip", clientIP,
			"status", code.String(),
			"latency_ms", float64(time.Since(start).Microseconds())/1000,
		)

		return resp, err
	}
}
