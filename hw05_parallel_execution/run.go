package hw05parallelexecution

import (
	"errors"
	"sync"
)

var ErrErrorsLimitExceeded = errors.New("errors limit exceeded")

type Task func() error

// Run starts tasks in n goroutines and stops its work when receiving m errors from tasks.
func Run(tasks []Task, n, m int) error {
	errMu := sync.Mutex{}
	errorsQty := 0
	errorLimitExceeded := false

	wg := sync.WaitGroup{}
	taskPoolCh := make(chan Task)

	worker := func() {
		defer wg.Done()
		for task := range taskPoolCh {
			if err := task(); err != nil {
				errMu.Lock()
				errorsQty++
				if errorsQty >= m {
					errorLimitExceeded = true
				}
				errMu.Unlock()
			}
		}
	}
	wg.Add(n)

	for i := 0; i < n; i++ {
		go worker()
	}

	for _, task := range tasks {
		errMu.Lock()
		shouldStop := errorLimitExceeded
		errMu.Unlock()
		if shouldStop {
			break
		}
		taskPoolCh <- task
	}

	close(taskPoolCh)
	wg.Wait()
	if errorLimitExceeded {
		return ErrErrorsLimitExceeded
	}
	return nil
}
