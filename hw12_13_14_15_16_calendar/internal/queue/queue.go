package queue

import "context"

type Publisher interface {
	Publish(ctx context.Context, body []byte) error
}

type Delivery interface {
	Body() []byte
	Ack() error
	Nack(requeue bool) error
}

type Consumer interface {
	Consume(ctx context.Context) (<-chan Delivery, error)
}
