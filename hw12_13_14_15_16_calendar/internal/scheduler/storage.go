package scheduler

import (
	"context"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
)

type Storage interface {
	DueEvents(
		ctx context.Context,
		now time.Time,
		limit int,
	) ([]storage.Event, error)

	MarkNotified(
		ctx context.Context,
		id uuid.UUID,
		startsAt time.Time,
		notifyBefore time.Duration,
	) (bool, error)

	DeleteOldEvents(
		ctx context.Context,
		before time.Time,
	) (int64, error)
}
