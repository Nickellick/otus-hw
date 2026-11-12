package memorystorage

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage"
	"github.com/google/uuid"
)

type Storage struct {
	mu       sync.RWMutex
	events   map[uuid.UUID]storage.Event
	notified map[uuid.UUID]struct{}
}

func New() *Storage {
	return &Storage{
		events:   make(map[uuid.UUID]storage.Event),
		notified: make(map[uuid.UUID]struct{}),
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

	old := s.events[event.ID]
	s.events[event.ID] = event
	if old.UserID != event.UserID ||
		old.Title != event.Title ||
		!old.StartsAt.Equal(event.StartsAt) ||
		old.NotifyBefore != event.NotifyBefore {
		delete(s.notified, event.ID)
	}
	return nil
}

func (s *Storage) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.events[id]; !exists {
		return storage.ErrEventNotFound
	}

	delete(s.events, id)
	delete(s.notified, id)
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

func (s *Storage) DueEvents(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]storage.Event, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	events := make([]storage.Event, 0)

	for _, event := range s.events {
		if event.NotifyBefore <= 0 {
			continue
		}

		if _, ok := s.notified[event.ID]; ok {
			continue
		}

		notifyAt := event.StartsAt.Add(-event.NotifyBefore)

		if notifyAt.After(now) || !event.StartsAt.After(now) {
			continue
		}

		events = append(events, event)
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].StartsAt.Equal(events[j].StartsAt) {
			return events[i].ID.String() < events[j].ID.String()
		}

		return events[i].StartsAt.Before(events[j].StartsAt)
	})

	if len(events) > limit {
		events = events[:limit]
	}

	return events, nil
}

func (s *Storage) MarkNotified(
	ctx context.Context,
	id uuid.UUID,
	startsAt time.Time,
	notifyBefore time.Duration,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.events[id]
	if !ok {
		return false, nil
	}

	if !event.StartsAt.Equal(startsAt) ||
		event.NotifyBefore != notifyBefore {
		return false, nil
	}

	if _, ok := s.notified[id]; ok {
		return false, nil
	}

	s.notified[id] = struct{}{}

	return true, nil
}

func (s *Storage) DeleteOldEvents(
	ctx context.Context,
	before time.Time,
) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted int64

	for id, event := range s.events {
		if !event.EndsAt.Before(before) {
			continue
		}

		delete(s.events, id)
		delete(s.notified, id)
		deleted++
	}

	return deleted, nil
}
