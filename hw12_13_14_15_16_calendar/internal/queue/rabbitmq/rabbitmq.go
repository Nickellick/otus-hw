package rabbitmq

import (
	"context"
	"fmt"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/queue"
	amqp "github.com/rabbitmq/amqp091-go"
)

const QueueName = "calendar.notifications"

type Client struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func New(url string) (*Client, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open AMQP channel: %w", err)
	}

	cleanup := func(err error) (*Client, error) {
		_ = channel.Close()
		_ = conn.Close()
		return nil, err
	}

	_, err = channel.QueueDeclare(
		QueueName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,
	)
	if err != nil {
		return cleanup(fmt.Errorf("declare queue: %w", err))
	}

	if err := channel.Confirm(false); err != nil {
		return cleanup(fmt.Errorf("enable confirms: %w", err))
	}

	return &Client{
		conn:    conn,
		channel: channel,
	}, nil
}

func (c *Client) Publish(ctx context.Context, body []byte) error {
	confirm, err := c.channel.PublishWithDeferredConfirmWithContext(
		ctx,
		"",        // default exchange
		QueueName, // routing key
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish notification: %w", err)
	}

	acked, err := confirm.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for publisher confirm: %w", err)
	}
	if !acked {
		return fmt.Errorf("RabbitMQ rejected publication")
	}

	return nil
}

type delivery struct {
	message amqp.Delivery
}

func (d delivery) Body() []byte {
	return d.message.Body
}

func (d delivery) Ack() error {
	return d.message.Ack(false)
}

func (d delivery) Nack(requeue bool) error {
	return d.message.Nack(false, requeue)
}

func (c *Client) Consume(
	ctx context.Context,
) (<-chan queue.Delivery, error) {
	if err := c.channel.Qos(10, 0, false); err != nil {
		return nil, fmt.Errorf("configure prefetch: %w", err)
	}

	messages, err := c.channel.ConsumeWithContext(
		ctx,
		QueueName,
		"",
		false, // autoAck: false
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("consume notifications: %w", err)
	}

	result := make(chan queue.Delivery)

	go func() {
		defer close(result)

		for message := range messages {
			select {
			case result <- delivery{message: message}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return result, nil
}

func (c *Client) Close() error {
	channelErr := c.channel.Close()
	connErr := c.conn.Close()

	if channelErr != nil {
		return channelErr
	}
	return connErr
}

var (
	_ queue.Publisher = (*Client)(nil)
	_ queue.Consumer  = (*Client)(nil)
)
