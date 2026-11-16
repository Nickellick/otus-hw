package memorystorage

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestStorage_CreateDateBusy(t *testing.T) {
	ctx := context.Background()
	store := New()

	start := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)

	first := storage.Event{
		ID:       uuid.New(),
		UserID:   1,
		Title:    "First",
		StartsAt: start,
		EndsAt:   start.Add(time.Hour),
	}

	second := storage.Event{
		ID:       uuid.New(),
		UserID:   1,
		Title:    "Second",
		StartsAt: start.Add(30 * time.Minute),
		EndsAt:   start.Add(90 * time.Minute),
	}
	require.NoError(t, store.Create(ctx, first))
	require.ErrorIs(t, store.Create(ctx, second), storage.ErrDateBusy)
}

func TestStorage_DifferentUsers(t *testing.T) {
	ctx := context.Background()
	store := New()

	start := time.Now()

	first := storage.Event{
		ID:       uuid.New(),
		UserID:   1,
		Title:    "First",
		StartsAt: start,
		EndsAt:   start.Add(time.Hour),
	}

	second := storage.Event{
		ID:       uuid.New(),
		UserID:   2,
		Title:    "Second",
		StartsAt: start,
		EndsAt:   start.Add(time.Hour),
	}

	require.NoError(t, store.Create(ctx, first))
	require.NoError(t, store.Create(ctx, second))
}

func TestStorage_ConcurrentCreate(t *testing.T) {
	ctx := context.Background()
	store := New()

	const count = 100

	var wg sync.WaitGroup
	var success atomic.Int32

	start := time.Now()

	for i := 0; i < count; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			event := storage.Event{
				ID:       uuid.New(),
				UserID:   1,
				Title:    "Concurrent",
				StartsAt: start,
				EndsAt:   start.Add(time.Hour),
			}

			err := store.Create(ctx, event)
			if err == nil {
				success.Add(1)
			} else if !errors.Is(err, storage.ErrDateBusy) {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	require.Equal(t, int32(1), success.Load())
}

func TestStorage_DueEvents(t *testing.T) {
	ctx := context.Background()

	now := time.Date(
		2026, 11, 12, 12, 0, 0, 0, time.UTC,
	)

	tests := []struct {
		name         string
		startsIn     time.Duration
		notifyBefore time.Duration
		wantDue      bool
	}{
		{
			name:         "notification is due",
			startsIn:     10 * time.Minute,
			notifyBefore: 15 * time.Minute,
			wantDue:      true,
		},
		{
			name:         "notification is not due yet",
			startsIn:     30 * time.Minute,
			notifyBefore: 15 * time.Minute,
		},
		{
			name:         "notification at exact boundary",
			startsIn:     15 * time.Minute,
			notifyBefore: 15 * time.Minute,
			wantDue:      true,
		},
		{
			name:         "notifications disabled",
			startsIn:     10 * time.Minute,
			notifyBefore: 0,
		},
		{
			name:         "event already started",
			startsIn:     -10 * time.Minute,
			notifyBefore: 15 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New()

			event := storage.Event{
				ID:           uuid.New(),
				UserID:       42,
				Title:        "Test event",
				StartsAt:     now.Add(tt.startsIn),
				EndsAt:       now.Add(tt.startsIn + time.Hour),
				NotifyBefore: tt.notifyBefore,
			}

			require.NoError(t, s.Create(ctx, event))

			events, err := s.DueEvents(ctx, now, 10)
			require.NoError(t, err)

			if !tt.wantDue {
				require.Empty(t, events)
				return
			}

			require.Len(t, events, 1)
			require.Equal(t, event.ID, events[0].ID)

			marked, err := s.MarkNotified(
				ctx,
				event.ID,
				event.StartsAt,
				event.NotifyBefore,
			)
			require.NoError(t, err)
			require.True(t, marked)

			events, err = s.DueEvents(ctx, now, 10)
			require.NoError(t, err)
			require.Empty(t, events)

			marked, err = s.MarkNotified(
				ctx,
				event.ID,
				event.StartsAt,
				event.NotifyBefore,
			)
			require.NoError(t, err)
			require.False(t, marked)
		})
	}
}

func TestStorage_MarkNotifiedAfterEventUpdate(t *testing.T) {
	ctx := context.Background()
	s := New()

	now := time.Date(
		2026, 11, 12, 12, 0, 0, 0, time.UTC,
	)

	event := storage.Event{
		ID:           uuid.New(),
		UserID:       42,
		Title:        "Meeting",
		StartsAt:     now.Add(10 * time.Minute),
		EndsAt:       now.Add(time.Hour),
		NotifyBefore: 15 * time.Minute,
	}

	require.NoError(t, s.Create(ctx, event))

	due, err := s.DueEvents(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)

	updated := event
	updated.StartsAt = updated.StartsAt.Add(time.Hour)
	updated.EndsAt = updated.EndsAt.Add(time.Hour)

	require.NoError(t, s.Update(ctx, updated))

	marked, err := s.MarkNotified(
		ctx,
		due[0].ID,
		due[0].StartsAt,
		due[0].NotifyBefore,
	)

	require.NoError(t, err)
	require.False(t, marked)

	futureNow := updated.StartsAt.Add(-10 * time.Minute)

	events, err := s.DueEvents(ctx, futureNow, 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, updated.StartsAt, events[0].StartsAt)
}

func TestStorage_DeleteOldEvents(t *testing.T) {
	ctx := context.Background()
	s := New()

	now := time.Date(
		2026, 11, 12, 12, 0, 0, 0, time.UTC,
	)

	cutoff := now.AddDate(-1, 0, 0)

	oldEvent := storage.Event{
		ID:       uuid.New(),
		UserID:   42,
		Title:    "Old event",
		StartsAt: cutoff.Add(-2 * time.Hour),
		EndsAt:   cutoff.Add(-time.Hour),
	}

	newEvent := storage.Event{
		ID:       uuid.New(),
		UserID:   42,
		Title:    "New event",
		StartsAt: now.Add(time.Hour),
		EndsAt:   now.Add(2 * time.Hour),
	}

	require.NoError(t, s.Create(ctx, oldEvent))
	require.NoError(t, s.Create(ctx, newEvent))

	deleted, err := s.DeleteOldEvents(ctx, cutoff)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	events, err := s.List(
		ctx,
		42,
		cutoff.AddDate(-1, 0, 0),
		now.AddDate(1, 0, 0),
	)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, newEvent.ID, events[0].ID)

	deleted, err = s.DeleteOldEvents(ctx, cutoff)
	require.NoError(t, err)
	require.Zero(t, deleted)
}
