package rabbitmq

import (
	"time"
)

const (
	minRetryDelay = time.Second
	maxRetryDelay = 10 * time.Second
)

func nextRetryDelay(delay time.Duration) time.Duration {
	if delay >= maxRetryDelay/2 {
		return maxRetryDelay
	}
	return delay * 2
}
