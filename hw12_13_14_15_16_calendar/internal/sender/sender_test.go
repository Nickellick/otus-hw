package sender

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/notification"
	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/queue"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeDelivery struct {
	body []byte

	acked   bool
	nacked  bool
	requeue bool

	ackErr  error
	nackErr error
}

func (d *fakeDelivery) Body() []byte {
	return d.body
}

func (d *fakeDelivery) Ack() error {
	d.acked = true
	return d.ackErr
}

func (d *fakeDelivery) Nack(requeue bool) error {
	d.nacked = true
	d.requeue = requeue
	return d.nackErr
}

type fakeConsumer struct {
	deliveries <-chan queue.Delivery
	err        error
}

func (c *fakeConsumer) Consume(
	_ context.Context,
) (<-chan queue.Delivery, error) {
	return c.deliveries, c.err
}

type fakeLogger struct {
	infoCount  int
	errorCount int
}

func (l *fakeLogger) Info(string, ...any) {
	l.infoCount++
}

func (l *fakeLogger) Error(string, ...any) {
	l.errorCount++
}

func validDelivery(t *testing.T) *fakeDelivery {
	t.Helper()

	body, err := json.Marshal(notification.Notification{
		EventID:  uuid.New(),
		Title:    "Team meeting",
		StartsAt: time.Now().Add(time.Hour),
		UserID:   42,
	})
	require.NoError(t, err)

	return &fakeDelivery{body: body}
}

func TestSenderHandle(t *testing.T) {
	tests := []struct {
		name       string
		delivery   func(*testing.T) *fakeDelivery
		wantAck    bool
		wantNack   bool
		wantInfo   int
		wantErrors int
	}{
		{
			name:     "valid notification",
			delivery: validDelivery,
			wantAck:  true,
			wantInfo: 1,
		},
		{
			name: "invalid JSON",
			delivery: func(_ *testing.T) *fakeDelivery {
				return &fakeDelivery{
					body: []byte(`{"eventId":`),
				}
			},
			wantNack:   true,
			wantErrors: 1,
		},
		{
			name: "ack failure",
			delivery: func(t *testing.T) *fakeDelivery {
				t.Helper()

				d := validDelivery(t)
				d.ackErr = errors.New("connection closed")
				return d
			},
			wantAck:    true,
			wantInfo:   1,
			wantErrors: 1,
		},
		{
			name: "nack failure",
			delivery: func(_ *testing.T) *fakeDelivery {
				return &fakeDelivery{
					body:    []byte(`invalid`),
					nackErr: errors.New("connection closed"),
				}
			},
			wantNack:   true,
			wantErrors: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &fakeLogger{}
			s := New(nil, logger)
			message := tt.delivery(t)

			s.handle(message)

			require.Equal(t, tt.wantAck, message.acked)
			require.Equal(t, tt.wantNack, message.nacked)
			require.False(t, message.requeue)
			require.Equal(t, tt.wantInfo, logger.infoCount)
			require.Equal(t, tt.wantErrors, logger.errorCount)
		})
	}
}

func TestSenderRun(t *testing.T) {
	t.Run("consume failure", func(t *testing.T) {
		expected := errors.New("consume failed")

		s := New(
			&fakeConsumer{err: expected},
			&fakeLogger{},
		)

		err := s.Run(context.Background())
		require.ErrorIs(t, err, expected)
	})

	t.Run("closed delivery channel", func(t *testing.T) {
		deliveries := make(chan queue.Delivery)
		close(deliveries)

		s := New(
			&fakeConsumer{deliveries: deliveries},
			&fakeLogger{},
		)

		err := s.Run(context.Background())
		require.NoError(t, err)
	})

	t.Run("context cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		s := New(
			&fakeConsumer{
				deliveries: make(chan queue.Delivery),
			},
			&fakeLogger{},
		)

		require.NoError(t, s.Run(ctx))
	})
}
