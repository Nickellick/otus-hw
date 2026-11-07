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

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/stretchr/testify/require"
)

type mockApplication struct {
	createdEvent storage.Event
	createCalled bool
	createErr    error
}

func (m *mockApplication) CreateEvent(
	_ context.Context,
	event storage.Event,
) error {
	m.createCalled = true
	m.createdEvent = event
	return m.createErr
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
