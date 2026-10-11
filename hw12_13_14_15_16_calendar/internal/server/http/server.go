package internalhttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

type Logger interface {
	Info(msg string)
	Error(msg string)
}

type Application interface{}

type Server struct {
	server *http.Server
	logger Logger
	app    Application
}

func NewServer(
	logger Logger,
	app Application,
	host string,
	port int,
) *Server {
	address := net.JoinHostPort(host, strconv.Itoa(port))

	return &Server{
		server: &http.Server{
			Addr:              address,
			Handler:           newHandler(logger),
			ReadHeaderTimeout: 5 * time.Second,
		},
		logger: logger,
		app:    app,
	}
}

func newHandler(logger Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Hello, world!\n"))
	})

	return loggingMiddleware(logger, mux)
}

func (s *Server) Start(_ context.Context) error {
	s.logger.Info("starting HTTP server on " + s.server.Addr)

	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}

	return s.Serve(listener)
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("stopping HTTP server")

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	return nil
}

func (s *Server) Serve(listener net.Listener) error {
	err := s.server.Serve(listener)

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}
