package storage

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrEventNotFound = errors.New("event not found")
	ErrEventExists   = errors.New("event already exists")
	ErrDateBusy      = errors.New("date is busy")
	ErrInvalidEvent  = errors.New("invalid event")
)

type Event struct {
	ID           uuid.UUID
	UserID       int64
	Title        string
	Description  string
	StartsAt     time.Time
	EndsAt       time.Time
	NotifyBefore time.Duration
}

func (e Event) Validate() error {
	if e.ID == uuid.Nil {
		return fmt.Errorf("%w: empty event ID", ErrInvalidEvent)
	}

	if e.Title == "" {
		return fmt.Errorf("%w: empty title", ErrInvalidEvent)
	}

	if !e.EndsAt.After(e.StartsAt) {
		return fmt.Errorf("%w: end time must be after start time", ErrInvalidEvent)
	}

	if e.NotifyBefore < 0 {
		return fmt.Errorf("%w: negative notification interval", ErrInvalidEvent)
	}

	return nil
}

func Overlaps(a, b Event) bool {
	return a.StartsAt.Before(b.EndsAt) &&
		b.StartsAt.Before(a.EndsAt)
}
