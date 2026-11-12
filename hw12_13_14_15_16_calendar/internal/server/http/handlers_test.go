package internalhttp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type mockApplication struct {
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

func TestCreateEventHandler(t *testing.T) {
	body := []byte(`{
		"id": "a60baf46-3f79-4f0e-9d33-4a57c4b1a890",
		"userId": 42,
		"title": "Созвон",
		"description": "Обсуждение проекта",
		"startsAt": "2026-11-09T14:00:00Z",
		"endsAt": "2026-11-09T15:00:00Z",
		"notifyBeforeSeconds": 900
	}`)

	app := &mockApplication{}

	req := httptest.NewRequest(
		http.MethodPost,
		"/events",
		bytes.NewReader(body),
	)

	rec := httptest.NewRecorder()

	createEventHandler(app).ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.True(t, app.createCalled)
	require.Equal(t, int64(42), app.createdEvent.UserID)
	require.Equal(t, "Созвон", app.createdEvent.Title)
	require.Equal(t, 15, int(app.createdEvent.NotifyBefore.Minutes()))
	require.JSONEq(
		t,
		`{"id":"a60baf46-3f79-4f0e-9d33-4a57c4b1a890"}`,
		rec.Body.String(),
	)
}

func TestCreateEventHandlerErrors(t *testing.T) {
	validJSON := `{
		"id": "a60baf46-3f79-4f0e-9d33-4a57c4b1a890",
		"userId": 42,
		"title": "Созвон",
		"startsAt": "2026-11-09T14:00:00Z",
		"endsAt": "2026-11-09T15:00:00Z",
		"notifyBeforeSeconds": 900
	}`

	tests := []struct {
		name       string
		body       string
		createErr  error
		wantStatus int
		wantCalled bool
	}{
		{
			name:       "invalid JSON",
			body:       `{invalid`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown field",
			body:       `{"user_id":42}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid event",
			body:       validJSON,
			createErr:  storage.ErrInvalidEvent,
			wantStatus: http.StatusBadRequest,
			wantCalled: true,
		},
		{
			name:       "event already exists",
			body:       validJSON,
			createErr:  storage.ErrEventExists,
			wantStatus: http.StatusConflict,
			wantCalled: true,
		},
		{
			name:       "date busy",
			body:       validJSON,
			createErr:  storage.ErrDateBusy,
			wantStatus: http.StatusConflict,
			wantCalled: true,
		},
		{
			name:       "wrapped date busy",
			body:       validJSON,
			createErr:  fmt.Errorf("create event: %w", storage.ErrDateBusy),
			wantStatus: http.StatusConflict,
			wantCalled: true,
		},
		{
			name:       "unexpected error",
			body:       validJSON,
			createErr:  errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &mockApplication{
				createErr: tt.createErr,
			}

			req := httptest.NewRequest(
				http.MethodPost,
				"/events",
				strings.NewReader(tt.body),
			)

			rec := httptest.NewRecorder()

			createEventHandler(app).ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, tt.wantCalled, app.createCalled)
		})
	}
}

func TestUpdateEventHandler(t *testing.T) {
	const eventID = "a60baf46-3f79-4f0e-9d33-4a57c4b1a890"

	body := `{
		"userId": 42,
		"title": "Перенесённый созвон",
		"description": "Обсуждение проекта",
		"startsAt": "2026-11-09T15:00:00Z",
		"endsAt": "2026-11-09T16:00:00Z",
		"notifyBeforeSeconds": 600
	}`

	app := &mockApplication{}
	handler := newHandler(mockLogger{}, app)

	req := httptest.NewRequest(
		http.MethodPut,
		"/events/"+eventID,
		strings.NewReader(body),
	)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String())
	require.True(t, app.updateCalled)

	require.Equal(t, uuid.MustParse(eventID), app.updatedEvent.ID)
	require.Equal(t, int64(42), app.updatedEvent.UserID)
	require.Equal(t, "Перенесённый созвон", app.updatedEvent.Title)
	require.Equal(t, 10*time.Minute, app.updatedEvent.NotifyBefore)
}

func TestUpdateEventHandlerErrors(t *testing.T) {
	const validBody = `{
		"userId": 42,
		"title": "Созвон",
		"startsAt": "2026-11-09T15:00:00Z",
		"endsAt": "2026-11-09T16:00:00Z",
		"notifyBeforeSeconds": 600
	}`

	tests := []struct {
		name       string
		id         string
		body       string
		updateErr  error
		wantStatus int
		wantCalled bool
	}{
		{
			name:       "invalid UUID",
			id:         "not-a-uuid",
			body:       validBody,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "nil UUID",
			id:         uuid.Nil.String(),
			body:       validBody,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON",
			id:         uuid.NewString(),
			body:       `{invalid`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown field",
			id:         uuid.NewString(),
			body:       `{"user_id":42}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative notification interval",
			id:         uuid.NewString(),
			body:       `{"notifyBeforeSeconds":-1}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "event not found",
			id:         uuid.NewString(),
			body:       validBody,
			updateErr:  storage.ErrEventNotFound,
			wantStatus: http.StatusNotFound,
			wantCalled: true,
		},
		{
			name:       "invalid event",
			id:         uuid.NewString(),
			body:       validBody,
			updateErr:  storage.ErrInvalidEvent,
			wantStatus: http.StatusBadRequest,
			wantCalled: true,
		},
		{
			name:       "overlapping event",
			id:         uuid.NewString(),
			body:       validBody,
			updateErr:  storage.ErrDateBusy,
			wantStatus: http.StatusConflict,
			wantCalled: true,
		},
		{
			name:       "unexpected error",
			id:         uuid.NewString(),
			body:       validBody,
			updateErr:  errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &mockApplication{updateErr: tt.updateErr}
			handler := newHandler(mockLogger{}, app)

			req := httptest.NewRequest(
				http.MethodPut,
				"/events/"+tt.id,
				strings.NewReader(tt.body),
			)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, tt.wantCalled, app.updateCalled)
		})
	}
}

func TestDeleteEventHandler(t *testing.T) {
	eventID := uuid.MustParse("a60baf46-3f79-4f0e-9d33-4a57c4b1a890")

	tests := []struct {
		name       string
		id         string
		deleteErr  error
		wantStatus int
		wantCalled bool
	}{
		{
			name:       "success",
			id:         eventID.String(),
			wantStatus: http.StatusNoContent,
			wantCalled: true,
		},
		{
			name:       "invalid UUID",
			id:         "invalid",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "nil UUID",
			id:         uuid.Nil.String(),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "event not found",
			id:         eventID.String(),
			deleteErr:  storage.ErrEventNotFound,
			wantStatus: http.StatusNotFound,
			wantCalled: true,
		},
		{
			name:       "unexpected error",
			id:         eventID.String(),
			deleteErr:  errors.New("database unavailable"),
			wantStatus: http.StatusInternalServerError,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &mockApplication{
				deleteErr: tt.deleteErr,
			}
			handler := newHandler(mockLogger{}, app)

			req := httptest.NewRequest(
				http.MethodDelete,
				"/events/"+tt.id,
				nil,
			)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			require.Equal(t, tt.wantStatus, rec.Code)
			require.Equal(t, tt.wantCalled, app.deleteCalled)

			if tt.wantCalled {
				require.Equal(t, eventID, app.deletedID)
			}

			if tt.wantStatus == http.StatusNoContent {
				require.Empty(t, rec.Body.String())
			}
		})
	}
}

func TestListEventsHandler(t *testing.T) {
	eventID := uuid.MustParse(
		"a60baf46-3f79-4f0e-9d33-4a57c4b1a890",
	)

	event := storage.Event{
		ID:           eventID,
		UserID:       42,
		Title:        "Созвон",
		Description:  "Обсуждение проекта",
		StartsAt:     time.Date(2026, 11, 9, 14, 0, 0, 0, time.UTC),
		EndsAt:       time.Date(2026, 11, 9, 15, 0, 0, 0, time.UTC),
		NotifyBefore: 15 * time.Minute,
	}

	const expectedJSON = `{
		"events": [{
			"id": "a60baf46-3f79-4f0e-9d33-4a57c4b1a890",
			"userId": 42,
			"title": "Созвон",
			"description": "Обсуждение проекта",
			"startsAt": "2026-11-09T14:00:00Z",
			"endsAt": "2026-11-09T15:00:00Z",
			"notifyBeforeSeconds": 900
		}]
	}`

	tests := []struct {
		name     string
		path     string
		method   string
		events   []storage.Event
		wantJSON string
	}{
		{
			name:     "day",
			path:     "/events/day?userId=42&date=2026-11-09",
			method:   "day",
			events:   []storage.Event{event},
			wantJSON: expectedJSON,
		},
		{
			name:     "week",
			path:     "/events/week?userId=42&date=2026-11-09",
			method:   "week",
			events:   []storage.Event{event},
			wantJSON: expectedJSON,
		},
		{
			name:     "month",
			path:     "/events/month?userId=42&date=2026-11-09",
			method:   "month",
			events:   []storage.Event{event},
			wantJSON: expectedJSON,
		},
		{
			name:     "empty list",
			path:     "/events/day?userId=42&date=2026-11-09",
			method:   "day",
			wantJSON: `{"events":[]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &mockApplication{
				listResult: tt.events,
			}

			handler := newHandler(mockLogger{}, app)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(
				t,
				"application/json",
				rec.Header().Get("Content-Type"),
			)
			require.JSONEq(t, tt.wantJSON, rec.Body.String())

			require.Equal(t, 1, app.listCalls)
			require.Equal(t, tt.method, app.listMethod)
			require.Equal(t, int64(42), app.listUserID)

			expectedDate := time.Date(
				2026, 11, 9, 0, 0, 0, 0, time.UTC,
			)
			require.True(t, expectedDate.Equal(app.listDate))
		})
	}
}

func TestListEventsHandlerInvalidParams(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{
			name: "missing user ID",
			path: "/events/day?date=2026-11-09",
		},
		{
			name: "invalid user ID",
			path: "/events/day?userId=abc&date=2026-11-09",
		},
		{
			name: "negative user ID",
			path: "/events/day?userId=-1&date=2026-11-09",
		},
		{
			name: "zero user ID",
			path: "/events/day?userId=0&date=2026-11-09",
		},
		{
			name: "user ID overflow",
			path: "/events/day?userId=9223372036854775808&date=2026-11-09",
		},
		{
			name: "missing date",
			path: "/events/day?userId=42",
		},
		{
			name: "invalid date format",
			path: "/events/day?userId=42&date=09-11-2026",
		},
		{
			name: "invalid calendar date",
			path: "/events/day?userId=42&date=2026-02-30",
		},
		{
			name: "empty date",
			path: "/events/day?userId=42&date=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &mockApplication{}
			handler := newHandler(mockLogger{}, app)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Zero(t, app.listCalls)
		})
	}
}

func TestEventMutationsInvalidUserID(t *testing.T) {
	const eventID = "a60baf46-3f79-4f0e-9d33-4a57c4b1a890"

	tests := []struct {
		name   string
		userID int64
	}{
		{
			name:   "zero user ID",
			userID: 0,
		},
		{
			name:   "negative user ID",
			userID: -42,
		},
	}

	methods := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/events",
		},
		{
			name:   "update",
			method: http.MethodPut,
			path:   "/events/" + eventID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, method := range methods {
				t.Run(method.name, func(t *testing.T) {
					body := fmt.Sprintf(`{
						"id": %q,
						"userId": %d,
						"title": "Test event",
						"startsAt": "2026-11-09T14:00:00Z",
						"endsAt": "2026-11-09T15:00:00Z"
					}`, eventID, tt.userID)

					app := &mockApplication{}
					handler := newHandler(mockLogger{}, app)

					req := httptest.NewRequest(
						method.method,
						method.path,
						strings.NewReader(body),
					)
					rec := httptest.NewRecorder()

					handler.ServeHTTP(rec, req)

					require.Equal(t, http.StatusBadRequest, rec.Code)
					require.False(t, app.createCalled)
					require.False(t, app.updateCalled)
				})
			}
		})
	}
}
