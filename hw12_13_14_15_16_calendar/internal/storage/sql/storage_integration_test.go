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
