package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/notification"
	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/queue"
)

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type Scheduler struct {
	storage   Storage
	publisher queue.Publisher
	logger    Logger

	interval  time.Duration
	batchSize int
}

func New(
	storage Storage,
	publisher queue.Publisher,
	logger Logger,
	interval time.Duration,
	batchSize int,
) (*Scheduler, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("scheduler interval must be positive")
	}

	if batchSize <= 0 {
		return nil, fmt.Errorf("scheduler batch size must be positive")
	}

	return &Scheduler{
		storage:   storage,
		publisher: publisher,
		logger:    logger,
		interval:  interval,
		batchSize: batchSize,
	}, nil
}

func (s *Scheduler) Run(ctx context.Context) error {
	// Не ждём первого тика после запуска.
	s.runCycle(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			s.runCycle(ctx)
		}
	}
}

func (s *Scheduler) runCycle(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}

	now := time.Now().UTC()

	if err := s.sendNotifications(ctx, now); err != nil {
		s.logger.Error("send notifications failed", "error", err)
	}

	if err := ctx.Err(); err != nil {
		return
	}

	cutoff := now.AddDate(-1, 0, 0)

	deleted, err := s.storage.DeleteOldEvents(ctx, cutoff)
	if err != nil {
		s.logger.Error("delete old events failed", "error", err)
		return
	}

	if deleted > 0 {
		s.logger.Info(
			"old events deleted",
			"count", deleted,
		)
	}
}

func (s *Scheduler) sendNotifications(
	ctx context.Context,
	now time.Time,
) error {
	events, err := s.storage.DueEvents(
		ctx,
		now,
		s.batchSize,
	)
	if err != nil {
		return fmt.Errorf("get due events: %w", err)
	}

	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return err
		}

		msg := notification.Notification{
			EventID:  event.ID,
			Title:    event.Title,
			StartsAt: event.StartsAt,
			UserID:   event.UserID,
		}

		body, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf(
				"marshal notification for event %s: %w",
				event.ID,
				err,
			)
		}

		if err := s.publisher.Publish(ctx, body); err != nil {
			return fmt.Errorf(
				"publish notification for event %s: %w",
				event.ID,
				err,
			)
		}

		marked, err := s.storage.MarkNotified(
			ctx,
			event.ID,
			event.StartsAt,
			event.NotifyBefore,
		)
		if err != nil {
			return fmt.Errorf(
				"mark event %s notified: %w",
				event.ID,
				err,
			)
		}

		if !marked {
			s.logger.Info(
				"event changed before notification was marked",
				"event_id", event.ID.String(),
			)
			continue
		}

		s.logger.Info(
			"notification published",
			"event_id", event.ID.String(),
			"user_id", event.UserID,
		)
	}

	return nil
}
