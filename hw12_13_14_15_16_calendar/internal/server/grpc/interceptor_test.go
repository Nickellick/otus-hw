package internalgrpc

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type capturedLog struct {
	message string
	args    []any
}

type captureLogger struct {
	entries []capturedLog
}

func (l *captureLogger) Info(msg string, args ...any) {
	l.entries = append(l.entries, capturedLog{
		message: msg,
		args:    args,
	})
}

func (l *captureLogger) Error(string, ...any) {}

func TestLoggingInterceptor(t *testing.T) {
	tests := []struct {
		name       string
		handlerErr error
		wantStatus string
	}{
		{
			name:       "success",
			wantStatus: codes.OK.String(),
		},
		{
			name:       "not found",
			handlerErr: status.Error(codes.NotFound, "event not found"),
			wantStatus: codes.NotFound.String(),
		},
		{
			name:       "internal error",
			handlerErr: errors.New("database unavailable"),
			wantStatus: codes.Unknown.String(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logg := &captureLogger{}

			ctx := peer.NewContext(
				context.Background(),
				&peer.Peer{
					Addr: &net.TCPAddr{
						IP:   net.ParseIP("192.0.2.10"),
						Port: 12345,
					},
				},
			)

			request := &struct{ Title string }{Title: "Созвон"}
			response := &struct{ ID string }{ID: "123"}

			handlerCalled := 0

			handler := func(
				_ context.Context,
				req any,
			) (any, error) {
				handlerCalled++
				require.Same(t, request, req)

				return response, tt.handlerErr
			}

			interceptor := loggingInterceptor(logg)

			got, err := interceptor(
				ctx,
				request,
				&grpc.UnaryServerInfo{
					FullMethod: "/event.CalendarService/CreateEvent",
				},
				handler,
			)

			require.Equal(t, tt.handlerErr, err)
			require.Same(t, response, got)
			require.Equal(t, 1, handlerCalled)

			require.Len(t, logg.entries, 1)

			entry := logg.entries[0]
			require.Equal(t, "grpc request", entry.message)

			attrs := make(map[string]any)

			require.Zero(t, len(entry.args)%2)

			for i := 0; i < len(entry.args); i += 2 {
				key, ok := entry.args[i].(string)
				require.True(t, ok)

				attrs[key] = entry.args[i+1]
			}

			require.Equal(
				t,
				"/event.CalendarService/CreateEvent",
				attrs["method"],
			)
			require.Equal(t, "192.0.2.10", attrs["client_ip"])
			require.Equal(t, tt.wantStatus, attrs["status"])

			latency, ok := attrs["latency_ms"].(float64)
			require.True(t, ok)
			require.GreaterOrEqual(t, latency, 0.0)
		})
	}
}
