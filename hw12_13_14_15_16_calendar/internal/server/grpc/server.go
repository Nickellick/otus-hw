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

func NewServer(logger Logger, app Application) *Server {
	s := &Server{
		server: grpc.NewServer(
			grpc.UnaryInterceptor(loggingInterceptor(logger)),
		),
		app: app,
	}

	pb.RegisterCalendarServiceServer(s.server, s)

	return s
}

func (s *Server) Serve(listener net.Listener) error {
	return s.server.Serve(listener)
}

func (s *Server) Stop(ctx context.Context) error {
	done := make(chan struct{})

	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil

	case <-ctx.Done():
		s.server.Stop()
		<-done

		return ctx.Err()
	}
}
