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

func rabbitMQAppCommand(
	ctx context.Context,
	t *testing.T,
	container *tcrabbitmq.RabbitMQContainer,
	command string,
) {
	t.Helper()

	exitCode, _, err := container.Exec(
		ctx,
		[]string{"rabbitmqctl", command},
	)

	require.NoError(t, err)
	require.Equal(t, 0, exitCode)
}

func setupTestRabbitMQContainer(
	t *testing.T,
) (*tcrabbitmq.RabbitMQContainer, string) {
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

	return container, url
}

func setupTestRabbitMQ(t *testing.T) string {
	t.Helper()
	_, url := setupTestRabbitMQContainer(t)
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

func TestConsumerReconnect(t *testing.T) {
	container, url := setupTestRabbitMQContainer(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		90*time.Second,
	)
	defer cancel()

	consumer, err := New(url)
	require.NoError(t, err)
	defer consumer.Close()

	deliveries, err := consumer.Consume(ctx)
	require.NoError(t, err)

	publisher, err := New(url)
	require.NoError(t, err)

	first := []byte(`{"eventId":"first"}`)

	require.NoError(t, publisher.Publish(ctx, first))

	select {
	case message, ok := <-deliveries:
		require.True(t, ok)
		require.JSONEq(t, string(first), string(message.Body()))
		require.NoError(t, message.Ack())

	case <-ctx.Done():
		t.Fatal("initial delivery timeout")
	}

	require.NoError(t, publisher.Close())

	rabbitMQAppCommand(ctx, t, container, "stop_app")
	rabbitMQAppCommand(ctx, t, container, "start_app")

	var newPublisher *Client

	require.Eventually(t, func() bool {
		client, err := New(url)
		if err != nil {
			return false
		}

		newPublisher = client
		return true
	}, 30*time.Second, 250*time.Millisecond,
		"RabbitMQ did not become available after restart",
	)

	defer newPublisher.Close()

	second := []byte(`{"eventId":"second"}`)

	require.NoError(t, newPublisher.Publish(ctx, second))

	select {
	case message, ok := <-deliveries:
		require.True(t, ok)
		require.JSONEq(t, string(second), string(message.Body()))
		require.NoError(t, message.Ack())

	case <-ctx.Done():
		t.Fatal("consumer did not reconnect")
	}
}

func TestConsumerStopsDuringReconnect(t *testing.T) {
	container, url := setupTestRabbitMQContainer(t)

	consumer, err := New(url)
	require.NoError(t, err)
	defer consumer.Close()

	ctx, cancel := context.WithCancel(context.Background())

	deliveries, err := consumer.Consume(ctx)
	require.NoError(t, err)

	stopCtx, stopCancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer stopCancel()

	require.NoError(t, container.Stop(stopCtx, nil))

	cancel()

	select {
	case _, ok := <-deliveries:
		require.False(t, ok, "delivery channel must be closed")

	case <-stopCtx.Done():
		t.Fatal("consumer did not stop after context cancellation")
	}
}
