package outbox

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeOutboxStorage struct {
	messages   []ClaimedMessage
	claimCalls int
	claimLimit []int
	sent       []uuid.UUID
}

func (s *fakeOutboxStorage) EnqueueDue(
	context.Context,
	time.Time,
	int,
) (int64, error) {
	return 0, nil
}

func (s *fakeOutboxStorage) Claim(
	_ context.Context,
	limit int,
	_ time.Duration,
) ([]ClaimedMessage, error) {
	s.claimCalls++
	s.claimLimit = append(s.claimLimit, limit)

	if len(s.messages) == 0 {
		return nil, nil
	}

	msg := s.messages[0]
	s.messages = s.messages[1:]

	return []ClaimedMessage{msg}, nil
}

func (s *fakeOutboxStorage) IsCurrent(
	context.Context,
	ClaimedMessage,
) (bool, error) {
	return true, nil
}

func (s *fakeOutboxStorage) MarkSent(
	_ context.Context,
	msg ClaimedMessage,
) error {
	s.sent = append(s.sent, msg.ID)
	return nil
}

func (s *fakeOutboxStorage) Cancel(
	context.Context,
	ClaimedMessage,
) error {
	return nil
}

func (s *fakeOutboxStorage) DeleteOldEvents(
	context.Context,
	time.Time,
) (int64, error) {
	return 0, nil
}

type fakePublisher struct {
	messages [][]byte
}

func (p *fakePublisher) Publish(
	_ context.Context,
	body []byte,
) error {
	p.messages = append(p.messages, body)
	return nil
}

type fakeLogger struct{}

func (fakeLogger) Info(string, ...any)  {}
func (fakeLogger) Error(string, ...any) {}

func TestWorkerClaimsOneMessageAtATime(t *testing.T) {
	store := &fakeOutboxStorage{
		messages: []ClaimedMessage{
			{
				Message: Message{
					ID:      uuid.New(),
					EventID: uuid.New(),
					Payload: []byte(`{"title":"first"}`),
				},
				ClaimID: uuid.New(),
			},
			{
				Message: Message{
					ID:      uuid.New(),
					EventID: uuid.New(),
					Payload: []byte(`{"title":"second"}`),
				},
				ClaimID: uuid.New(),
			},
		},
	}

	publisher := &fakePublisher{}

	worker, err := New(
		store,
		publisher,
		fakeLogger{},
		time.Second,
		10,
	)
	require.NoError(t, err)

	err = worker.publishPending(context.Background())
	require.NoError(t, err)

	require.Len(t, publisher.messages, 2)
	require.Len(t, store.sent, 2)
	require.Equal(t, []int{1, 1, 1}, store.claimLimit)
}
