package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/notification"
	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	memorystorage "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage/memory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakePublisher struct {
	messages [][]byte
	err      error
}

func (p *fakePublisher) Publish(
	_ context.Context,
	body []byte,
) error {
	if p.err != nil {
		return p.err
	}

	p.messages = append(p.messages, append([]byte(nil), body...))
	return nil
}

type noopLogger struct{}

func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

func TestSchedulerSendNotifications(t *testing.T) {
	ctx := context.Background()
	store := memorystorage.New()
	publisher := &fakePublisher{}

	now := time.Date(
		2026, 11, 12, 12, 0, 0, 0, time.UTC,
	)

	event := storage.Event{
		ID:           uuid.New(),
		UserID:       42,
		Title:        "Team meeting",
		StartsAt:     now.Add(10 * time.Minute),
		EndsAt:       now.Add(time.Hour),
		NotifyBefore: 15 * time.Minute,
	}

	require.NoError(t, store.Create(ctx, event))

	s, err := New(
		store,
		publisher,
		noopLogger{},
		time.Second,
		100,
	)
	require.NoError(t, err)

	require.NoError(t, s.sendNotifications(ctx, now))
	require.Len(t, publisher.messages, 1)

	var got notification.Notification

	require.NoError(
		t,
		json.Unmarshal(publisher.messages[0], &got),
	)

	require.Equal(t, event.ID, got.EventID)
	require.Equal(t, event.UserID, got.UserID)
	require.Equal(t, event.Title, got.Title)
	require.True(t, event.StartsAt.Equal(got.StartsAt))

	require.NoError(t, s.sendNotifications(ctx, now))
	require.Len(t, publisher.messages, 1)
}

func TestSchedulerPublishFailure(t *testing.T) {
	ctx := context.Background()
	store := memorystorage.New()

	now := time.Date(
		2026, 11, 12, 12, 0, 0, 0, time.UTC,
	)

	event := storage.Event{
		ID:           uuid.New(),
		UserID:       42,
		Title:        "Team meeting",
		StartsAt:     now.Add(10 * time.Minute),
		EndsAt:       now.Add(time.Hour),
		NotifyBefore: 15 * time.Minute,
	}

	require.NoError(t, store.Create(ctx, event))

	publishErr := errors.New("RabbitMQ unavailable")
	publisher := &fakePublisher{err: publishErr}

	s, err := New(
		store,
		publisher,
		noopLogger{},
		time.Second,
		100,
	)
	require.NoError(t, err)

	err = s.sendNotifications(ctx, now)
	require.ErrorIs(t, err, publishErr)

	due, err := store.DueEvents(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Equal(t, event.ID, due[0].ID)

	publisher.err = nil

	require.NoError(t, s.sendNotifications(ctx, now))
	require.Len(t, publisher.messages, 1)

	due, err = store.DueEvents(ctx, now, 10)
	require.NoError(t, err)
	require.Empty(t, due)
}
