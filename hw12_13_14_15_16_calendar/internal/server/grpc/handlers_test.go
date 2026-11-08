package internalgrpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	pb "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/pkg/calendarpb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockApplication struct {
	Application

	createdEvent storage.Event
	createCalled bool
	createErr    error

	updatedEvent storage.Event
	updateCalled bool
	updateErr    error

	deletedID    uuid.UUID
	deleteCalled bool
	deleteErr    error

	listMethod string
	listUserID int64
	listDate   time.Time
	listCalls  int

	listResult []storage.Event
	listErr    error
}

func (m *mockApplication) CreateEvent(
	_ context.Context,
	event storage.Event,
) error {
	m.createCalled = true
	m.createdEvent = event
	return m.createErr
}

func (m *mockApplication) UpdateEvent(
	_ context.Context,
	event storage.Event,
) error {
	m.updateCalled = true
	m.updatedEvent = event
	return m.updateErr
}

func (m *mockApplication) DeleteEvent(
	_ context.Context,
	id uuid.UUID,
) error {
	m.deleteCalled = true
	m.deletedID = id
	return m.deleteErr
}

func (m *mockApplication) recordList(
	method string,
	userID int64,
	date time.Time,
) ([]storage.Event, error) {
	m.listMethod = method
	m.listUserID = userID
	m.listDate = date
	m.listCalls++

	return m.listResult, m.listErr
}

func (m *mockApplication) ListEventsForDay(
	_ context.Context,
	userID int64,
	date time.Time,
) ([]storage.Event, error) {
	return m.recordList("day", userID, date)
}

func (m *mockApplication) ListEventsForWeek(
	_ context.Context,
	userID int64,
	date time.Time,
) ([]storage.Event, error) {
	return m.recordList("week", userID, date)
}

func (m *mockApplication) ListEventsForMonth(
	_ context.Context,
	userID int64,
	date time.Time,
) ([]storage.Event, error) {
	return m.recordList("month", userID, date)
}

func TestCreateEvent(t *testing.T) {
	id := uuid.New()
	start := time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)

	app := &mockApplication{}

	server := &Server{
		app: app,
	}

	resp, err := server.CreateEvent(
		context.Background(),
		&pb.CreateEventRequest{
			Event: &pb.Event{
				Id:           id.String(),
				UserId:       42,
				Title:        "Созвон",
				Description:  "Обсуждение проекта",
				StartsAt:     timestamppb.New(start),
				EndsAt:       timestamppb.New(start.Add(time.Hour)),
				NotifyBefore: durationpb.New(15 * time.Minute),
			},
		},
	)

	require.NoError(t, err)
	require.Equal(t, id.String(), resp.GetId())

	require.True(t, app.createCalled)
	require.Equal(t, id, app.createdEvent.ID)
	require.Equal(t, int64(42), app.createdEvent.UserID)
	require.Equal(t, "Созвон", app.createdEvent.Title)
	require.Equal(t, 15*time.Minute, app.createdEvent.NotifyBefore)
}

func TestUpdateEvent(t *testing.T) {
	id := uuid.New()
	start := time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC)

	app := &mockApplication{}
	server := &Server{app: app}

	resp, err := server.UpdateEvent(
		context.Background(),
		&pb.UpdateEventRequest{
			Event: &pb.Event{
				Id:       id.String(),
				UserId:   42,
				Title:    "Перенесённый созвон",
				StartsAt: timestamppb.New(start),
				EndsAt:   timestamppb.New(start.Add(time.Hour)),
			},
		},
	)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, app.updateCalled)
	require.Equal(t, id, app.updatedEvent.ID)
	require.Equal(t, "Перенесённый созвон", app.updatedEvent.Title)
}

func TestDeleteEvent(t *testing.T) {
	id := uuid.New()

	app := &mockApplication{}
	server := &Server{app: app}

	resp, err := server.DeleteEvent(
		context.Background(),
		&pb.DeleteEventRequest{Id: id.String()},
	)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, app.deleteCalled)
	require.Equal(t, id, app.deletedID)
}

func TestDeleteEventErrors(t *testing.T) {
	tests := []struct {
		name     string
		request  *pb.DeleteEventRequest
		appErr   error
		wantCode codes.Code
		called   bool
	}{
		{
			name:     "nil request",
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "invalid UUID",
			request:  &pb.DeleteEventRequest{Id: "invalid"},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "nil UUID",
			request:  &pb.DeleteEventRequest{Id: uuid.Nil.String()},
			wantCode: codes.InvalidArgument,
		},
		{
			name:     "event not found",
			request:  &pb.DeleteEventRequest{Id: uuid.NewString()},
			appErr:   storage.ErrEventNotFound,
			wantCode: codes.NotFound,
			called:   true,
		},
		{
			name:     "internal error",
			request:  &pb.DeleteEventRequest{Id: uuid.NewString()},
			appErr:   errors.New("database unavailable"),
			wantCode: codes.Internal,
			called:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &mockApplication{deleteErr: tt.appErr}
			server := &Server{app: app}

			_, err := server.DeleteEvent(context.Background(), tt.request)

			require.Equal(t, tt.wantCode, status.Code(err))
			require.Equal(t, tt.called, app.deleteCalled)
		})
	}
}

func TestListEvents(t *testing.T) {
	id := uuid.MustParse(
		"a60baf46-3f79-4f0e-9d33-4a57c4b1a890",
	)

	date := time.Date(
		2026, 11, 9, 0, 0, 0, 0, time.UTC,
	)

	event := storage.Event{
		ID:           id,
		UserID:       42,
		Title:        "Созвон",
		Description:  "Обсуждение проекта",
		StartsAt:     date.Add(14 * time.Hour),
		EndsAt:       date.Add(15 * time.Hour),
		NotifyBefore: 15 * time.Minute,
	}

	type listMethod func(
		*Server,
		context.Context,
		*pb.ListEventsRequest,
	) (*pb.ListEventsResponse, error)

	tests := []struct {
		name       string
		method     listMethod
		wantMethod string
		events     []storage.Event
	}{
		{
			name:       "day",
			method:     (*Server).ListEventsForDay,
			wantMethod: "day",
			events:     []storage.Event{event},
		},
		{
			name:       "week",
			method:     (*Server).ListEventsForWeek,
			wantMethod: "week",
			events:     []storage.Event{event},
		},
		{
			name:       "month",
			method:     (*Server).ListEventsForMonth,
			wantMethod: "month",
			events:     []storage.Event{event},
		},
		{
			name:       "empty list",
			method:     (*Server).ListEventsForDay,
			wantMethod: "day",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &mockApplication{
				listResult: tt.events,
			}
			server := &Server{app: app}

			resp, err := tt.method(
				server,
				context.Background(),
				&pb.ListEventsRequest{
					UserId: 42,
					Date:   timestamppb.New(date),
				},
			)

			require.NoError(t, err)
			require.NotNil(t, resp)

			require.Equal(t, 1, app.listCalls)
			require.Equal(t, tt.wantMethod, app.listMethod)
			require.Equal(t, int64(42), app.listUserID)
			require.True(t, date.Equal(app.listDate))

			require.Len(t, resp.GetEvents(), len(tt.events))

			if len(tt.events) == 0 {
				return
			}

			got := resp.GetEvents()[0]

			require.Equal(t, id.String(), got.GetId())
			require.Equal(t, int64(42), got.GetUserId())
			require.Equal(t, "Созвон", got.GetTitle())
			require.Equal(t, "Обсуждение проекта", got.GetDescription())

			require.True(
				t,
				event.StartsAt.Equal(got.GetStartsAt().AsTime()),
			)
			require.True(
				t,
				event.EndsAt.Equal(got.GetEndsAt().AsTime()),
			)
			require.Equal(
				t,
				15*time.Minute,
				got.GetNotifyBefore().AsDuration(),
			)
		})
	}
}

func TestListEventsErrors(t *testing.T) {
	date := timestamppb.New(
		time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
	)

	type listMethod func(
		*Server,
		context.Context,
		*pb.ListEventsRequest,
	) (*pb.ListEventsResponse, error)

	methods := []struct {
		name string
		call listMethod
	}{
		{"day", (*Server).ListEventsForDay},
		{"week", (*Server).ListEventsForWeek},
		{"month", (*Server).ListEventsForMonth},
	}

	tests := []struct {
		name     string
		req      *pb.ListEventsRequest
		appErr   error
		wantCode codes.Code
		called   bool
	}{
		{
			name:     "nil request",
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing user ID",
			req: &pb.ListEventsRequest{
				Date: date,
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "negative user ID",
			req: &pb.ListEventsRequest{
				UserId: -1,
				Date:   date,
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing date",
			req: &pb.ListEventsRequest{
				UserId: 42,
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid timestamp",
			req: &pb.ListEventsRequest{
				UserId: 42,
				Date: &timestamppb.Timestamp{
					Seconds: 253402300800,
				},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "internal error",
			req: &pb.ListEventsRequest{
				UserId: 42,
				Date:   date,
			},
			appErr:   errors.New("database unavailable"),
			wantCode: codes.Internal,
			called:   true,
		},
	}

	for _, method := range methods {
		t.Run(method.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					app := &mockApplication{
						listErr: tt.appErr,
					}
					server := &Server{app: app}

					_, err := method.call(
						server,
						context.Background(),
						tt.req,
					)

					require.Equal(t, tt.wantCode, status.Code(err))

					if tt.called {
						require.Equal(t, 1, app.listCalls)
					} else {
						require.Zero(t, app.listCalls)
					}
				})
			}
		})
	}
}
