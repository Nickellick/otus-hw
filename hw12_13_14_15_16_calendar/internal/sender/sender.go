package sender

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/notification"
	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/queue"
	"github.com/google/uuid"
)

type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

type Storage interface {
	IsNotificationActual(
		ctx context.Context,
		eventID uuid.UUID,
		version int64,
	) (bool, error)
}

type Sender struct {
	consumer queue.Consumer
	storage  Storage
	logger   Logger
}

func New(consumer queue.Consumer, storage Storage, logger Logger) *Sender {
	return &Sender{
		consumer: consumer,
		storage:  storage,
		logger:   logger,
	}
}

func (s *Sender) Run(ctx context.Context) error {
	deliveries, err := s.consumer.Consume(ctx)
	if err != nil {
		return fmt.Errorf("start consuming notifications: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case message, ok := <-deliveries:
			if !ok {
				if ctx.Err() != nil {
					return fmt.Errorf("notification delivery channel closed")
				}
				return nil
			}
			s.handle(ctx, message)
		}
	}
}

func (s *Sender) handle(ctx context.Context, message queue.Delivery) {
	var notification notification.Notification

	if err := json.Unmarshal(message.Body(), &notification); err != nil {
		s.logger.Error(
			"invalid notification",
			"error", err,
		)

		if err := message.Nack(false); err != nil {
			s.logger.Error(
				"failed to reject notification",
				"error", err,
			)
		}

		return
	}

	actual, err := s.storage.IsNotificationActual(
		ctx,
		notification.EventID,
		notification.EventVersion,
	)
	if err != nil {
		s.logger.Error(
			"check notification actuality failed",
			"event_id", notification.EventID.String(),
			"error", err,
		)

		if err := message.Nack(true); err != nil {
			s.logger.Error(
				"failed to requeue notification",
				"error", err,
			)
		}
		return
	}

	if !actual {
		s.logger.Info(
			"stale notification skipped",
			"event_id", notification.EventID.String(),
			"event_version", notification.EventVersion,
		)

		if err := message.Ack(); err != nil {
			s.logger.Error(
				"failed to acknowledge stale notification",
				"event_id", notification.EventID.String(),
				"error", err,
			)
		}
		return
	}
	s.logger.Info(
		"notification received",
		"event_id", notification.EventID.String(),
		"title", notification.Title,
		"starts_at", notification.StartsAt,
		"user_id", notification.UserID,
	)

	if err := message.Ack(); err != nil {
		s.logger.Error(
			"failed to acknowledge notification",
			"event_id", notification.EventID.String(),
			"error", err,
		)
	}
}
