package sqlstorage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupTestDatabase(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()

	container, err := postgres.Run(
		ctx,
		"postgres:17",
		postgres.WithDatabase("calendar_test"),
		postgres.WithUsername("calendar"),
		postgres.WithPassword("calendar"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	testcontainers.CleanupContainer(t, container)

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := Connect(ctx, dsn)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	require.NoError(t, goose.SetDialect("postgres"))

	migrationsDir := filepath.Join(
		"..", "..", "..", "migrations",
	)

	require.NoError(t, goose.Up(db, migrationsDir))

	return db
}

func TestStorage_CreateDateBusy(t *testing.T) {
	db := setupTestDatabase(t)
	store := New(db)

	ctx := context.Background()

	start := time.Date(
		2026, 10, 12,
		10, 0, 0, 0,
		time.UTC,
	)

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

	err := store.Create(ctx, second)

	require.ErrorIs(t, err, storage.ErrDateBusy)
}

func TestStorage_ConcurrentCreate(t *testing.T) {
	db := setupTestDatabase(t)
	store := New(db)

	ctx := context.Background()

	const count = 20

	var wg sync.WaitGroup
	var success atomic.Int32

	start := time.Date(
		2026, 10, 12,
		10, 0, 0, 0,
		time.UTC,
	)

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

			switch {
			case err == nil:
				success.Add(1)

			case errors.Is(err, storage.ErrDateBusy):
				// This conflict is expected

			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	require.Equal(t, int32(1), success.Load())
}

func TestStorage_DueEvents(t *testing.T) {
	db := setupTestDatabase(t)
	store := New(db)
	ctx := context.Background()

	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		startsIn     time.Duration
		notifyBefore time.Duration
		wantDue      bool
	}{
		{
			name:         "notification due",
			startsIn:     10 * time.Minute,
			notifyBefore: 15 * time.Minute,
			wantDue:      true,
		},
		{
			name:         "notification not due yet",
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
			// Разные UserID позволяют избежать конфликтов
			// по EXCLUDE constraint между под-тестами.
			var userID int64
			require.NoError(t, db.QueryRowContext(
				ctx,
				"SELECT COALESCE(MAX(user_id), 0) + 1 FROM events",
			).Scan(&userID))

			event := storage.Event{
				ID:           uuid.New(),
				UserID:       userID,
				Title:        "Test event",
				StartsAt:     now.Add(tt.startsIn),
				EndsAt:       now.Add(tt.startsIn + time.Hour),
				NotifyBefore: tt.notifyBefore,
			}

			require.NoError(t, store.Create(ctx, event))

			events, err := store.DueEvents(ctx, now, 100)
			require.NoError(t, err)

			var found bool
			for _, e := range events {
				if e.ID == event.ID {
					found = true
					break
				}
			}

			require.Equal(t, tt.wantDue, found)

			if !tt.wantDue {
				return
			}

			marked, err := store.MarkNotified(
				ctx,
				event.ID,
				event.StartsAt,
				event.NotifyBefore,
			)
			require.NoError(t, err)
			require.True(t, marked)

			marked, err = store.MarkNotified(
				ctx,
				event.ID,
				event.StartsAt,
				event.NotifyBefore,
			)
			require.NoError(t, err)
			require.False(t, marked)

			events, err = store.DueEvents(ctx, now, 100)
			require.NoError(t, err)

			for _, e := range events {
				require.NotEqual(t, event.ID, e.ID)
			}
		})
	}
}

func TestStorage_DeleteOldEvents(t *testing.T) {
	db := setupTestDatabase(t)
	store := New(db)
	ctx := context.Background()

	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	cutoff := now.AddDate(-1, 0, 0)

	oldEvent := storage.Event{
		ID:       uuid.New(),
		UserID:   42,
		Title:    "Old event",
		StartsAt: cutoff.Add(-2 * time.Hour),
		EndsAt:   cutoff.Add(-time.Hour),
	}

	boundaryEvent := storage.Event{
		ID:       uuid.New(),
		UserID:   42,
		Title:    "Boundary event",
		StartsAt: cutoff.Add(-time.Hour),
		EndsAt:   cutoff,
	}

	newEvent := storage.Event{
		ID:       uuid.New(),
		UserID:   42,
		Title:    "New event",
		StartsAt: now.Add(time.Hour),
		EndsAt:   now.Add(2 * time.Hour),
	}

	require.NoError(t, store.Create(ctx, oldEvent))
	require.NoError(t, store.Create(ctx, boundaryEvent))
	require.NoError(t, store.Create(ctx, newEvent))

	deleted, err := store.DeleteOldEvents(ctx, cutoff)
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)

	var ids []uuid.UUID

	rows, err := db.QueryContext(
		ctx,
		"SELECT id FROM events ORDER BY starts_at",
	)
	require.NoError(t, err)

	for rows.Next() {
		var id uuid.UUID
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}

	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())

	require.NotContains(t, ids, oldEvent.ID)
	require.Contains(t, ids, boundaryEvent.ID)
	require.Contains(t, ids, newEvent.ID)

	deleted, err = store.DeleteOldEvents(ctx, cutoff)
	require.NoError(t, err)
	require.Zero(t, deleted)
}
