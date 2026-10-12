package internalhttp

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type mockLogger struct{}

func (mockLogger) Info(string, ...any)  {}
func (mockLogger) Error(string, ...any) {}

func TestServerRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "hello",
			method:     http.MethodGet,
			path:       "/hello",
			wantStatus: http.StatusOK,
			wantBody:   "Hello, world!\n",
		},
		{
			name:       "not found",
			method:     http.MethodGet,
			path:       "/unknown",
			wantStatus: http.StatusNotFound,
			wantBody:   "404 page not found\n",
		},
		{
			name:       "method not allowed",
			method:     http.MethodPost,
			path:       "/hello",
			wantStatus: http.StatusMethodNotAllowed,
			wantBody:   "Method Not Allowed\n",
		},
	}

	handler := newHandler(mockLogger{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			require.Equal(t, tt.wantStatus, recorder.Code)
			require.Equal(t, tt.wantBody, recorder.Body.String())
		})
	}
}

func TestServerStartStop(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	server := NewServer(mockLogger{}, nil, "127.0.0.1", 0)

	done := make(chan error, 1)

	go func() {
		done <- server.Serve(listener)
	}()

	client := &http.Client{
		Timeout: time.Second,
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		"http://"+listener.Addr().String()+"/hello",
		nil,
	)

	require.NoError(t, err)

	response, err := client.Do(req)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, response.Body.Close())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	require.NoError(t, server.Stop(ctx))
	require.Equal(t, http.StatusOK, response.StatusCode)

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestServerGracefulShutdown(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)

		<-release

		w.WriteHeader(http.StatusOK)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := NewServer(mockLogger{}, nil, "127.0.0.1", 0)
	server.server.Handler = handler

	done := make(chan error, 1)

	go func() {
		done <- server.Serve(listener)
	}()

	requestDone := make(chan error, 1)

	go func() {
		client := &http.Client{
			Timeout: 5 * time.Second,
		}

		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"http://"+listener.Addr().String(),
			nil,
		)
		if err != nil {
			requestDone <- err
			return
		}

		response, err := client.Do(req)
		if err != nil {
			requestDone <- err
			return
		}

		_ = response.Body.Close()
		requestDone <- nil
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}

	shutdownDone := make(chan error, 1)

	go func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancel()

		shutdownDone <- server.Stop(ctx)
	}()

	select {
	case err := <-shutdownDone:
		t.Fatalf("shutdown completed before request: %v", err)

	case <-time.After(100 * time.Millisecond):
		// Ожидаем, что активный запрос ещё выполняется.
	}

	close(release)

	select {
	case err := <-shutdownDone:
		require.NoError(t, err)

	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not finish")
	}

	require.NoError(t, <-requestDone)
	require.NoError(t, <-done)
}
