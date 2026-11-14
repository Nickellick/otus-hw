package sqlstorage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/outbox"
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

func checkUpdateCancelsOutbox(
	ctx context.Context,
	t *testing.T,
	db *sql.DB,
	store *Storage,
	claim bool,
) {
	t.Helper()

	now := time.Now().UTC()

	event := storage.Event{
		ID:           uuid.New(),
		UserID:       42,
		Title:        "Original title",
		StartsAt:     now.Add(10 * time.Minute),
		EndsAt:       now.Add(30 * time.Minute),
		NotifyBefore: 15 * time.Minute,
	}

	require.NoError(t, store.Create(ctx, event))

	count, err := store.EnqueueDue(ctx, now, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)

	if claim {
		messages, err := store.Claim(ctx, 1, time.Minute)
		require.NoError(t, err)
		require.Len(t, messages, 1)
		require.Equal(t, event.ID, messages[0].EventID)
	}

	event.Title = "Updated title"

	require.NoError(t, store.Update(ctx, event))

	var version int64
	require.NoError(t, db.QueryRowContext(
		ctx,
		`SELECT version FROM events WHERE id = $1`,
		event.ID,
	).Scan(&version))
	require.EqualValues(t, 2, version)

	var status string
	var claimID sql.NullString

	require.NoError(t, db.QueryRowContext(
		ctx,
		`SELECT status, claim_id::text
		 FROM notification_outbox
		 WHERE event_id = $1 AND event_version = 1`,
		event.ID,
	).Scan(&status, &claimID))

	require.Equal(t, "cancelled", status)
	require.False(t, claimID.Valid)

	count, err = store.EnqueueDue(ctx, now, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)

	var newVersion int64
	require.NoError(t, db.QueryRowContext(
		ctx,
		`SELECT event_version
		 FROM notification_outbox
		 WHERE event_id = $1 AND status = 'pending'`,
		event.ID,
	).Scan(&newVersion))

	require.EqualValues(t, 2, newVersion)
}

func createTestOutboxMessage(
	ctx context.Context,
	t *testing.T,
	store *Storage,
) uuid.UUID {
	t.Helper()

	now := time.Now().UTC()

	event := storage.Event{
		ID:           uuid.New(),
		UserID:       42,
		Title:        "Outbox test",
		StartsAt:     now.Add(10 * time.Minute),
		EndsAt:       now.Add(30 * time.Minute),
		NotifyBefore: 15 * time.Minute,
	}

	require.NoError(t, store.Create(ctx, event))

	count, err := store.EnqueueDue(ctx, now, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)

	return event.ID
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

func TestStorage_UpdateCancelsOutbox(t *testing.T) {
	tests := []struct {
		name  string
		claim bool
	}{
		{
			name:  "pending notification",
			claim: false,
		},
		{
			name:  "processing notification",
			claim: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDatabase(t)
			store := New(db)
			ctx := context.Background()

			checkUpdateCancelsOutbox(
				ctx,
				t,
				db,
				store,
				tt.claim,
			)
		})
	}
}

func TestStorage_MarkSentAfterUpdate(t *testing.T) {
	db := setupTestDatabase(t)
	store := New(db)
	ctx := context.Background()

	now := time.Now().UTC()

	event := storage.Event{
		ID:           uuid.New(),
		UserID:       42,
		Title:        "Original",
		StartsAt:     now.Add(10 * time.Minute),
		EndsAt:       now.Add(30 * time.Minute),
		NotifyBefore: 15 * time.Minute,
	}

	require.NoError(t, store.Create(ctx, event))

	_, err := store.EnqueueDue(ctx, now, 10)
	require.NoError(t, err)

	messages, err := store.Claim(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, messages, 1)

	oldMessage := messages[0]

	event.Title = "Updated"
	require.NoError(t, store.Update(ctx, event))

	err = store.MarkSent(ctx, oldMessage)
	require.ErrorContains(t, err, "outbox claim no longer owned")

	var sentAt sql.NullTime

	require.NoError(t, db.QueryRowContext(
		ctx,
		`SELECT notification_sent_at
		 FROM events
		 WHERE id = $1`,
		event.ID,
	).Scan(&sentAt))

	require.False(t, sentAt.Valid)

	count, err := store.EnqueueDue(ctx, now, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
}

func TestStorage_OutboxLeaseRecovery(t *testing.T) {
	db := setupTestDatabase(t)
	store := New(db)
	ctx := context.Background()

	eventID := createTestOutboxMessage(ctx, t, store)

	first, err := store.Claim(ctx, 1, 200*time.Millisecond)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, eventID, first[0].EventID)

	// Имитируем падение Worker - не вызываем ни MarkSent(), ни Cancel()
	// После истечения аренды сообщение должно снова стать доступным
	var second []outbox.ClaimedMessage

	require.Eventually(t, func() bool {
		messages, claimErr := store.Claim(
			ctx,
			1,
			time.Minute,
		)
		if claimErr != nil {
			return false
		}

		second = messages
		return len(second) == 1
	}, 5*time.Second, 50*time.Millisecond)

	require.Equal(t, first[0].ID, second[0].ID)
	require.Equal(t, first[0].EventID, second[0].EventID)
	require.NotEqual(t, first[0].ClaimID, second[0].ClaimID)

	// Старый Worker больше не владеет сообщением
	err = store.MarkSent(ctx, first[0])
	require.ErrorContains(t, err, "outbox claim no longer owned")

	// Новый Worker подхватывает обработку
	require.NoError(t, store.MarkSent(ctx, second[0]))

	var status string

	require.NoError(t, db.QueryRowContext(
		ctx,
		`SELECT status
		 FROM notification_outbox
		 WHERE id = $1`,
		second[0].ID,
	).Scan(&status))

	require.Equal(t, "sent", status)
}

func TestStorage_OutboxConcurrentClaim(t *testing.T) {
	db := setupTestDatabase(t)
	store := New(db)
	ctx := context.Background()

	eventID := createTestOutboxMessage(ctx, t, store)

	type claimResult struct {
		messages []outbox.ClaimedMessage
		err      error
	}

	const workers = 2

	results := make(chan claimResult, workers)
	start := make(chan struct{})

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			// Оба Worker стартуют одновременно.
			<-start

			messages, err := store.Claim(
				ctx,
				1,
				time.Minute,
			)

			results <- claimResult{
				messages: messages,
				err:      err,
			}
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	var claimed []outbox.ClaimedMessage

	for result := range results {
		require.NoError(t, result.err)
		claimed = append(claimed, result.messages...)
	}

	// Только один Worker получил сообщение
	require.Len(t, claimed, 1)
	require.Equal(t, eventID, claimed[0].EventID)

	var status string
	var claimID uuid.UUID

	require.NoError(t, db.QueryRowContext(
		ctx,
		`SELECT status, claim_id
		 FROM notification_outbox
		 WHERE id = $1`,
		claimed[0].ID,
	).Scan(&status, &claimID))

	require.Equal(t, "processing", status)
	require.Equal(t, claimed[0].ClaimID, claimID)
}

func TestStorage_ConcurrentUpdateAndMarkSent(t *testing.T) {
	db := setupTestDatabase(t)
	store := New(db)

	const iterations = 20

	for i := 0; i < iterations; i++ {
		t.Run(fmt.Sprintf("iteration_%d", i), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				10*time.Second,
			)
			defer cancel()

			now := time.Now().UTC()

			event := storage.Event{
				ID:           uuid.New(),
				UserID:       int64(i + 100),
				Title:        "Original",
				StartsAt:     now.Add(10 * time.Minute),
				EndsAt:       now.Add(30 * time.Minute),
				NotifyBefore: 15 * time.Minute,
			}

			require.NoError(t, store.Create(ctx, event))

			count, err := store.EnqueueDue(ctx, now, 1)
			require.NoError(t, err)
			require.EqualValues(t, 1, count)

			messages, err := store.Claim(ctx, 1, time.Minute)
			require.NoError(t, err)
			require.Len(t, messages, 1)

			msg := messages[0]

			updatedEvent := event
			updatedEvent.Title = "Updated"

			type result struct {
				operation string
				err       error
			}

			results := make(chan result, 2)
			start := make(chan struct{})

			var wg sync.WaitGroup

			wg.Add(2)

			go func() {
				defer wg.Done()
				<-start

				err := store.Update(ctx, updatedEvent)
				results <- result{
					operation: "update",
					err:       err,
				}
			}()

			go func() {
				defer wg.Done()
				<-start

				err := store.MarkSent(ctx, msg)
				results <- result{
					operation: "mark_sent",
					err:       err,
				}
			}()

			// Одновременно запускаем обе операции.
			close(start)

			wg.Wait()
			close(results)

			var updateErr, markSentErr error

			for r := range results {
				switch r.operation {
				case "update":
					updateErr = r.err
				case "mark_sent":
					markSentErr = r.err
				}
			}

			require.NoError(t, updateErr)

			if markSentErr != nil {
				require.ErrorContains(
					t,
					markSentErr,
					"outbox claim no longer owned",
				)
			}

			var version int64
			var sentAt sql.NullTime

			require.NoError(t, db.QueryRowContext(
				ctx,
				`SELECT version, notification_sent_at
				 FROM events WHERE id = $1`,
				event.ID,
			).Scan(&version, &sentAt))

			require.EqualValues(t, 2, version)
			require.False(t, sentAt.Valid)

			var status string

			require.NoError(t, db.QueryRowContext(
				ctx,
				`SELECT status
				 FROM notification_outbox
				 WHERE id = $1`,
				msg.ID,
			).Scan(&status))

			if markSentErr == nil {
				require.Equal(t, "sent", status)
			} else {
				require.Equal(t, "cancelled", status)
			}
		})
	}
}
