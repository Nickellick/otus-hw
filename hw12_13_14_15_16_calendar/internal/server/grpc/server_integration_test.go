package internalgrpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	pb "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/pkg/calendarpb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGRPCServerIntegration(t *testing.T) {
	const bufferSize = 1024 * 1024

	listener := bufconn.Listen(bufferSize)
	defer listener.Close()

	app := &mockApplication{}
	logg := &captureLogger{}

	server := NewServer(logg, app)

	serveDone := make(chan error, 1)

	go func() {
		serveDone <- server.Serve(listener)
	}()

	defer func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancel()

		require.NoError(t, server.Stop(ctx))

		select {
		case err := <-serveDone:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Error("gRPC server did not stop")
		}
	}()

	dialer := func(
		_ context.Context,
		_ string,
	) (net.Conn, error) {
		return listener.Dial()
	}

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewCalendarServiceClient(conn)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)
	defer cancel()

	eventID := uuid.New()
	start := time.Date(
		2026, 11, 9, 14, 0, 0, 0, time.UTC,
	)

	response, err := client.CreateEvent(
		ctx,
		&pb.CreateEventRequest{
			Event: &pb.Event{
				Id:           eventID.String(),
				UserId:       42,
				Title:        "Тестовый созвон",
				Description:  "Проверка gRPC",
				StartsAt:     timestamppb.New(start),
				EndsAt:       timestamppb.New(start.Add(time.Hour)),
				NotifyBefore: durationpb.New(15 * time.Minute),
			},
		},
	)

	require.NoError(t, err)
	require.Equal(t, eventID.String(), response.GetId())

	require.True(t, app.createCalled)
	require.Equal(t, eventID, app.createdEvent.ID)
	require.Equal(t, int64(42), app.createdEvent.UserID)
	require.Equal(t, "Тестовый созвон", app.createdEvent.Title)
	require.Equal(t, "Проверка gRPC", app.createdEvent.Description)
	require.True(t, start.Equal(app.createdEvent.StartsAt))
	require.Equal(
		t,
		15*time.Minute,
		app.createdEvent.NotifyBefore,
	)

	require.Len(t, logg.entries, 1)
	require.Equal(t, "grpc request", logg.entries[0].message)

	attrs := make(map[string]any)
	entry := logg.entries[0]

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
	require.Equal(t, "OK", attrs["status"])
}

func TestGRPCErrorMapping(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	defer listener.Close()

	app := &mockApplication{
		createErr: storage.ErrDateBusy,
	}

	server := NewServer(&captureLogger{}, app)

	done := make(chan error, 1)
	go func() {
		done <- server.Serve(listener)
	}()

	defer func() {
		server.server.Stop()

		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Error("gRPC server did not stop")
		}
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufconn",
		grpc.WithContextDialer(func(
			_ context.Context,
			_ string,
		) (net.Conn, error) {
			return listener.Dial()
		}), grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	client := pb.NewCalendarServiceClient(conn)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)
	defer cancel()

	start := time.Date(
		2026, 11, 9, 14, 0, 0, 0, time.UTC,
	)

	_, err = client.CreateEvent(ctx, &pb.CreateEventRequest{
		Event: &pb.Event{
			Id:       uuid.NewString(),
			UserId:   42,
			Title:    "Конфликтующее событие",
			StartsAt: timestamppb.New(start),
			EndsAt:   timestamppb.New(start.Add(time.Hour)),
		},
	})

	require.Error(t, err)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	require.True(t, app.createCalled)
}
