package outbox

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID           uuid.UUID
	EventID      uuid.UUID
	EventVersion int64
	Payload      []byte
}

type ClaimedMessage struct {
	Message
	ClaimID uuid.UUID
}

type Storage interface {
	EnqueueDue(
		ctx context.Context,
		now time.Time,
		limit int,
	) (int64, error)

	Claim(
		ctx context.Context,
		limit int,
		lease time.Duration,
	) ([]ClaimedMessage, error)

	IsCurrent(
		ctx context.Context,
		msg ClaimedMessage,
	) (bool, error)

	MarkSent(
		ctx context.Context,
		msg ClaimedMessage,
	) error

	Cancel(
		ctx context.Context,
		msg ClaimedMessage,
	) error

	DeleteOldEvents(
		ctx context.Context,
		before time.Time,
	) (int64, error)
}
