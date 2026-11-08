package internalgrpc

import (
	"context"
	"net"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	pb "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/pkg/calendarpb"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

type Application interface {
	CreateEvent(ctx context.Context, event storage.Event) error
	UpdateEvent(ctx context.Context, event storage.Event) error
	DeleteEvent(ctx context.Context, id uuid.UUID) error

	ListEventsForDay(ctx context.Context, userID int64, date time.Time) ([]storage.Event, error)
	ListEventsForWeek(ctx context.Context, userID int64, date time.Time) ([]storage.Event, error)
	ListEventsForMonth(ctx context.Context, userID int64, date time.Time) ([]storage.Event, error)
}

type Server struct {
	pb.UnimplementedCalendarServiceServer

	server *grpc.Server
	app    Application
}

func NewServer(app Application) *Server {
	s := &Server{
		server: grpc.NewServer(),
		app:    app,
	}

	pb.RegisterCalendarServiceServer(s.server, s)
	return s
}

func (s *Server) Serve(listener net.Listener) error {
	return s.server.Serve(listener)
}

func (s *Server) Stop() {
	s.server.GracefulStop()
}
