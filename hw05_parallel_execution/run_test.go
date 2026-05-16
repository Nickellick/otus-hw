package hw05parallelexecution

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestRun(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("if were errors in first M tasks, than finished not more N+M tasks", func(t *testing.T) {
		tasksCount := 50
		tasks := make([]Task, 0, tasksCount)

		var runTasksCount int32

		for i := 0; i < tasksCount; i++ {
			err := fmt.Errorf("error from task %d", i)
			tasks = append(tasks, func() error {
				atomic.AddInt32(&runTasksCount, 1)
				return err
			})
		}

		workersCount := 10
		maxErrorsCount := 23
		maxTotalTasks := workersCount + maxErrorsCount

		err := Run(tasks, workersCount, maxErrorsCount)

		require.Truef(t, errors.Is(err, ErrErrorsLimitExceeded), "actual err - %v", err)
		require.LessOrEqual(t, runTasksCount, int32(maxTotalTasks), "extra tasks were started")
	})

	t.Run("tasks without errors", func(t *testing.T) {
		tasksCount := 50
		tasks := make([]Task, 0, tasksCount)

		var startedTaskCount int32
		var finishedTasksCount int32

		eventuallyWaitFor := time.Second
		eventuallyTick := time.Millisecond * 10

		taskLocker := make(chan struct{})

		for i := 0; i < tasksCount; i++ {
			tasks = append(tasks, func() error {
				atomic.AddInt32(&startedTaskCount, 1)
				<-taskLocker
				atomic.AddInt32(&finishedTasksCount, 1)
				return nil
			})
		}

		workersCount := 5
		maxErrorsCount := 1

		done := make(chan error)

		go func() {
			done <- Run(tasks, workersCount, maxErrorsCount)
		}()

		require.Eventually(t, func() bool {
			return atomic.LoadInt32(&startedTaskCount) == int32(workersCount)
		}, eventuallyWaitFor, eventuallyTick, "not all tasks were started in time")

		close(taskLocker)
		err := <-done

		require.NoError(t, err)
		require.Equal(t, int32(tasksCount), atomic.LoadInt32(&finishedTasksCount), "not all tasks were completed")
	})
}
