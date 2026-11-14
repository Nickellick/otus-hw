package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/queue"
)

const (
	claimLease     = time.Minute
	publishTimeout = 30 * time.Second
)

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type Worker struct {
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
) (*Worker, error) {
	if interval <= 0 || batchSize <= 0 {
		return nil, fmt.Errorf("invalid outbox worker settings")
	}

	return &Worker{
		storage:   storage,
		publisher: publisher,
		logger:    logger,
		interval:  interval,
		batchSize: batchSize,
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	w.runCycle(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.runCycle(ctx)
		}
	}
}

func (w *Worker) runCycle(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	now := time.Now().UTC()

	count, err := w.storage.EnqueueDue(ctx, now, w.batchSize)
	if err != nil {
		w.logger.Error("enqueue notifications failed", "error", err)
	} else if count > 0 {
		w.logger.Info("notifications enqueued", "count", count)
	}

	if ctx.Err() != nil {
		return
	}

	if err := w.publishPending(ctx); err != nil {
		if ctx.Err() == nil {
			w.logger.Error("publish outbox failed", "error", err)
		}
	}

	if ctx.Err() != nil {
		return
	}

	deleted, err := w.storage.DeleteOldEvents(
		ctx,
		now.AddDate(-1, 0, 0),
	)
	if err != nil {
		w.logger.Error("delete old events failed", "error", err)
		return
	}

	if deleted > 0 {
		w.logger.Info("old events deleted", "count", deleted)
	}
}

func (w *Worker) publishPending(ctx context.Context) error {
	for i := 0; i < w.batchSize; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		messages, err := w.storage.Claim(ctx, 1, claimLease)
		if err != nil {
			return fmt.Errorf("claim outbox message: %w", err)
		}

		if len(messages) == 0 {
			return nil
		}

		msg := messages[0]

		current, err := w.storage.IsCurrent(ctx, msg)
		if err != nil {
			return fmt.Errorf("check message %s: %w", msg.ID, err)
		}

		if !current {
			if err := w.storage.Cancel(ctx, msg); err != nil {
				return fmt.Errorf("cancel message %s: %w", msg.ID, err)
			}

			continue
		}

		publishCtx, cancel := context.WithTimeout(
			ctx,
			publishTimeout,
		)

		err = w.publisher.Publish(publishCtx, msg.Payload)
		cancel()

		if err != nil {
			return fmt.Errorf("publish message %s: %w", msg.ID, err)
		}

		if err := w.storage.MarkSent(ctx, msg); err != nil {
			return fmt.Errorf("mark message %s sent: %w", msg.ID, err)
		}

		w.logger.Info(
			"notification published",
			"event_id", msg.EventID.String(),
			"outbox_id", msg.ID.String(),
		)
	}

	return nil
}
