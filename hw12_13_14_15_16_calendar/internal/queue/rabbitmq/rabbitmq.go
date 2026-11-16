package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/queue"
	amqp "github.com/rabbitmq/amqp091-go"
)

const QueueName = "calendar.notifications"

type Client struct {
	url string

	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
	closed  bool

	stop chan struct{}
}

func connect(url string) (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, nil, fmt.Errorf("dial RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("open AMQP channel: %w", err)
	}

	cleanup := func(err error) (*amqp.Connection, *amqp.Channel, error) {
		_ = channel.Close()
		_ = conn.Close()
		return nil, nil, err
	}

	_, err = channel.QueueDeclare(
		QueueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return cleanup(fmt.Errorf("declare queue: %w", err))
	}

	if err := channel.Confirm(false); err != nil {
		return cleanup(fmt.Errorf("enable confirms: %w", err))
	}

	if err := channel.Qos(10, 0, false); err != nil {
		return cleanup(fmt.Errorf("configure QoS: %w", err))
	}

	return conn, channel, nil
}

func (c *Client) reconnect(ctx context.Context) error {
	delay := minRetryDelay

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		select {
		case <-c.stop:
			return fmt.Errorf("RabbitMQ client is closed")
		default:
		}

		conn, channel, err := connect(c.url)
		if err == nil {
			c.mu.Lock()

			if c.closed {
				c.mu.Unlock()
				_ = channel.Close()
				_ = conn.Close()
				return fmt.Errorf("RabbitMQ client is closed")
			}

			oldConn := c.conn
			oldChannel := c.channel

			c.conn = conn
			c.channel = channel
			c.mu.Unlock()

			if oldChannel != nil {
				_ = oldChannel.Close()
			}
			if oldConn != nil {
				_ = oldConn.Close()
			}

			return nil
		}

		if err := c.waitReconnect(ctx, delay); err != nil {
			return err
		}

		delay = nextRetryDelay(delay)
	}
}

func New(url string) (*Client, error) {
	conn, channel, err := connect(url)
	if err != nil {
		return nil, err
	}

	return &Client{
		url:     url,
		conn:    conn,
		channel: channel,
		stop:    make(chan struct{}),
	}, nil
}

func (c *Client) Publish(ctx context.Context, body []byte) error {
	delay := minRetryDelay

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return fmt.Errorf("RabbitMQ client is closed")
		}

		channel := c.channel
		c.mu.Unlock()

		if channel == nil || channel.IsClosed() {
			if err := c.reconnect(ctx); err != nil {
				return err
			}
			continue
		}

		confirm, err := channel.PublishWithDeferredConfirmWithContext(
			ctx,
			"",
			QueueName,
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent,
				Body:         body,
			},
		)

		if err == nil {
			var acked bool
			acked, err = confirm.WaitContext(ctx)
			if err == nil && acked {
				return nil
			}
			if err == nil {
				return fmt.Errorf("RabbitMQ rejected publication")
			}
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if err := c.waitReconnect(ctx, delay); err != nil {
			return err
		}
		delay = nextRetryDelay(delay)

		if err := c.reconnect(ctx); err != nil {
			return err
		}
	}
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
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()

	if closed {
		return nil, errors.New("RabbitMQ client is closed")
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	out := make(chan queue.Delivery)

	go c.consumeLoop(ctx, out)

	return out, nil
}

func (c *Client) consumeLoop(
	ctx context.Context,
	out chan<- queue.Delivery,
) {
	defer close(out)

	delay := minRetryDelay

	for {
		if c.stopping(ctx) {
			return
		}

		conn, channel, err := connect(c.url)
		if err == nil {
			delay = minRetryDelay

			_ = c.consumeSession(ctx, channel, out)

			_ = channel.Close()
			_ = conn.Close()
		}

		if c.stopping(ctx) {
			return
		}

		if err := c.waitReconnect(ctx, delay); err != nil {
			return
		}

		delay = nextRetryDelay(delay)
	}
}

func (c *Client) consumeSession(
	ctx context.Context,
	channel *amqp.Channel,
	out chan<- queue.Delivery,
) error {
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	messages, err := channel.ConsumeWithContext(
		sessionCtx,
		QueueName,
		"",
		false, // autoAck
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		return fmt.Errorf("start RabbitMQ consumer: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-c.stop:
			return errors.New("RabbitMQ client closed")

		case message, ok := <-messages:
			if !ok {
				return errors.New("RabbitMQ delivery channel closed")
			}

			select {
			case out <- delivery{message: message}:
			case <-ctx.Done():
				return ctx.Err()
			case <-c.stop:
				return errors.New("RabbitMQ client closed")
			}
		}
	}
}

func (c *Client) stopping(ctx context.Context) bool {
	if ctx.Err() != nil {
		return true
	}

	select {
	case <-c.stop:
		return true
	default:
		return false
	}
}

func (c *Client) waitReconnect(
	ctx context.Context,
	delay time.Duration,
) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stop:
		return errors.New("RabbitMQ client closed")
	case <-timer.C:
		return nil
	}
}

func (c *Client) Close() error {
	c.mu.Lock()

	if c.closed {
		c.mu.Unlock()
		return nil
	}

	c.closed = true
	close(c.stop)

	conn := c.conn
	channel := c.channel

	c.conn = nil
	c.channel = nil

	c.mu.Unlock()

	if channel != nil {
		_ = channel.Close()
	}

	if conn != nil {
		return conn.Close()
	}

	return nil
}

var (
	_ queue.Publisher = (*Client)(nil)
	_ queue.Consumer  = (*Client)(nil)
)
