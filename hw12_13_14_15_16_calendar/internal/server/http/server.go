package internalhttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
)

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type Application interface {
	CreateEvent(ctx context.Context, event storage.Event) error
	UpdateEvent(ctx context.Context, event storage.Event) error
	DeleteEvent(ctx context.Context, id uuid.UUID) error
	ListEventsForDay(ctx context.Context, userID int64, date time.Time) ([]storage.Event, error)
	ListEventsForWeek(ctx context.Context, userID int64, date time.Time) ([]storage.Event, error)
	ListEventsForMonth(ctx context.Context, userID int64, date time.Time) ([]storage.Event, error)
}
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
			Handler:           newHandler(logger, app),
			ReadHeaderTimeout: 5 * time.Second,
		},
		logger: logger,
		app:    app,
	}
}

func newHandler(logger Logger, app Application) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Hello, world!\n"))
	})
	mux.Handle("POST /events", createEventHandler(app))
	mux.Handle("PUT /events/{id}", updateEventHandler(app))
	mux.Handle("DELETE /events/{id}", deleteEventHandler(app))
	mux.Handle(
		"GET /events/day",
		listEventsHandler(app.ListEventsForDay),
	)

	mux.Handle(
		"GET /events/week",
		listEventsHandler(app.ListEventsForWeek),
	)

	mux.Handle(
		"GET /events/month",
		listEventsHandler(app.ListEventsForMonth),
	)
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
