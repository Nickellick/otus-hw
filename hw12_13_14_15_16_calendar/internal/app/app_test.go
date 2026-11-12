package app

import (
	"context"
	"testing"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type listStorageMock struct {
	from   time.Time
	to     time.Time
	userID int64
	called bool
}

func (m *listStorageMock) Create(context.Context, storage.Event) error {
	return nil
}

func (m *listStorageMock) Update(context.Context, storage.Event) error {
	return nil
}

func (m *listStorageMock) Delete(context.Context, uuid.UUID) error {
	return nil
}

func (m *listStorageMock) List(
	_ context.Context,
	userID int64,
	from, to time.Time,
) ([]storage.Event, error) {
	m.called = true
	m.userID = userID
	m.from = from
	m.to = to

	return []storage.Event{}, nil
}

type mockLogger struct{}

func (mockLogger) Info(string, ...any)  {}
func (mockLogger) Error(string, ...any) {}

func TestListEventsPeriods(t *testing.T) {
	tests := []struct {
		name      string
		date      time.Time
		list      func(*App, context.Context, int64, time.Time) ([]storage.Event, error)
		wantFrom  time.Time
		wantUntil time.Time
	}{
		{
			name:      "day",
			date:      time.Date(2026, 11, 7, 15, 30, 0, 0, time.UTC),
			list:      (*App).ListEventsForDay,
			wantFrom:  time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC),
			wantUntil: time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "week",
			date:      time.Date(2026, 11, 7, 15, 30, 0, 0, time.UTC),
			list:      (*App).ListEventsForWeek,
			wantFrom:  time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
			wantUntil: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "month",
			date:      time.Date(2026, 11, 7, 15, 30, 0, 0, time.UTC),
			list:      (*App).ListEventsForMonth,
			wantFrom:  time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
			wantUntil: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &listStorageMock{}
			app := New(mockLogger{}, store)

			_, err := tt.list(app, context.Background(), 42, tt.date)

			require.NoError(t, err)
			require.True(t, store.called)
			require.Equal(t, int64(42), store.userID)
			require.True(t, tt.wantFrom.Equal(store.from))
			require.True(t, tt.wantUntil.Equal(store.to))
		})
	}
}
