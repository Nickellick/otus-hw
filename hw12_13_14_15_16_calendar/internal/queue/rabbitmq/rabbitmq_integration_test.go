package rabbitmq

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcrabbitmq "github.com/testcontainers/testcontainers-go/modules/rabbitmq"
)

func setupTestRabbitMQ(t *testing.T) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		2*time.Minute,
	)
	defer cancel()

	container, err := tcrabbitmq.Run(
		ctx,
		"rabbitmq:4-management",
		tcrabbitmq.WithAdminUsername("calendar"),
		tcrabbitmq.WithAdminPassword("calendar"),
		testcontainers.WithEnv(map[string]string{
			"RABBITMQ_DEFAULT_USER": "calendar",
			"RABBITMQ_DEFAULT_PASS": "calendar",
		}),
	)
	require.NoError(t, err)

	testcontainers.CleanupContainer(t, container)

	url, err := container.AmqpURL(ctx)
	require.NoError(t, err)

	return url
}

func TestPublishConsume(t *testing.T) {
	url := setupTestRabbitMQ(t)

	publisher, err := New(url)
	require.NoError(t, err)
	defer publisher.Close()

	consumer, err := New(url)
	require.NoError(t, err)
	defer consumer.Close()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	deliveries, err := consumer.Consume(ctx)
	require.NoError(t, err)

	// Уникальное сообщение для проверки.
	messageID := uuid.NewString()
	body := []byte(`{"eventId":"` + messageID + `"}`)

	require.NoError(t, publisher.Publish(ctx, body))

	select {
	case message, ok := <-deliveries:
		require.True(t, ok)
		require.JSONEq(t, string(body), string(message.Body()))
		require.NoError(t, message.Ack())

	case <-ctx.Done():
		t.Fatal("timeout waiting for RabbitMQ message")
	}
}
