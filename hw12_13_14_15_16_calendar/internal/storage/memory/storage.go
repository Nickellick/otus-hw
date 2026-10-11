package memorystorage

import (
	"context"
	"sync"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
)

type Storage struct {
	mu     sync.RWMutex
	events map[uuid.UUID]storage.Event
}

func New() *Storage {
	return &Storage{
		events: make(map[uuid.UUID]storage.Event),
	}
}

func (s *Storage) Create(_ context.Context, event storage.Event) error {
	if err := event.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.events[event.ID]; exists {
		return storage.ErrEventExists
	}

	for _, existing := range s.events {
		if existing.UserID != event.UserID {
			continue
		}

		if storage.Overlaps(existing, event) {
			return storage.ErrDateBusy
		}
	}

	s.events[event.ID] = event
	return nil
}

func (s *Storage) Update(_ context.Context, event storage.Event) error {
	if err := event.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.events[event.ID]; !exists {
		return storage.ErrEventNotFound
	}

	for _, existing := range s.events {
		if existing.ID == event.ID {
			continue
		}

		if existing.UserID != event.UserID {
			continue
		}

		if storage.Overlaps(existing, event) {
			return storage.ErrDateBusy
		}
	}

	s.events[event.ID] = event
	return nil
}

func (s *Storage) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.events[id]; !exists {
		return storage.ErrEventNotFound
	}

	delete(s.events, id)
	return nil
}

func (s *Storage) List(
	_ context.Context,
	userID int64,
	from time.Time,
	to time.Time,
) ([]storage.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]storage.Event, 0)

	for _, event := range s.events {
		if event.UserID != userID {
			continue
		}

		if event.StartsAt.Before(to) && event.EndsAt.After(from) {
			result = append(result, event)
		}
	}

	return result, nil
}
