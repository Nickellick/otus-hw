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
