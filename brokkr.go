package brokkr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/clinknclank/brokkr/component/background"
)

type (
	// Brokkr manages the lifecycle of an application.
	// This structure is responsible for overseeing the processes and tasks that constitute the app,
	// and provides mechanisms for controlled starts, stops, and signal handling.
	Brokkr struct {
		// signals are the OS signals that Brokkr listens to. When one of these signals
		// is received, Brokkr will initiate its stopping procedures. Typical signals
		// might include SIGINT (Ctrl+C) or SIGTERM (termination request).
		signals []os.Signal
		// stopTimeout specifies the maximum duration Brokkr will wait when trying to
		// gracefully stop the application. If this timeout is exceeded, Brokkr will
		// forcefully terminate the app. This ensures that applications do not hang
		// indefinitely during shutdown.
		stopTimeout time.Duration
		// backgroundTasks is a list of processes that Brokkr will run in the background
		// during the application's lifecycle. These tasks run concurrently with the main
		// application process and can be thought of as auxiliary services or routines
		// that support the primary functions of the application.
		backgroundTasks []background.Process
		// mainContext is the primary context for the Brokkr. It governs the entire lifecycle
		// of the application and its associated processes. When this context is cancelled,
		// it signals all derived contexts (like those of background tasks) to begin their
		// shutdown procedures.
		mainContext context.Context
		// mainContextCancel is the associated cancel function for the mainContext. Invoking
		// this function will cancel the mainContext and begin the shutdown process for
		// Brokkr and its managed tasks.
		mainContextCancel func()
	}

	// Options sets of configurations for Brokkr.
	Options func(o *Brokkr)

	// coreContextKey child context key.
	contextOfBrokkr interface{}
)

// SetForceStopTimeout redefines force shutdown timeout.
func SetForceStopTimeout(t time.Duration) Options {
	return func(c *Brokkr) { c.stopTimeout = t }
}

// AddBackgroundTasks that will be executed in background of main loop.
func AddBackgroundTasks(bt ...background.Process) Options {
	return func(c *Brokkr) {
		c.backgroundTasks = append(c.backgroundTasks, bt...)
	}
}

// NewBrokkr framework instance
func NewBrokkr(opts ...Options) (b *Brokkr) {
	b = &Brokkr{
		signals:     []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT},
		stopTimeout: 60 * time.Second,
	}

	b.mainContext, b.mainContextCancel = context.WithCancel(context.Background())
	for _, o := range opts {
		o(b)
	}

	return
}

// Start initializes and runs the Brokkr's main loop.
// It launches the background tasks and monitors for interrupt signals.
// In the event of an interrupt or an error in a background task, the function will begin shutdown procedures.
func (c *Brokkr) Start() error {
	// The WaitGroup helps to synchronize background tasks, ensuring
	// all tasks have completed before exiting the function.
	var bgTasksWG sync.WaitGroup
	// This channel listens for OS-level interrupt signals.
	interruptSignal := make(chan os.Signal, 1)
	// Create a context and its associated error group for the background tasks.
	// This facilitates error handling and cancellation of tasks.
	TaskErrorGroup, TaskErrorGroupCtx := errgroup.WithContext(c.mainContext)

	// Initialization of the background tasks / internal processes.
	for _, t := range c.backgroundTasks {
		task := t

		// Define a background task's termination workflow.
		TaskErrorGroup.Go(func() error {
			<-TaskErrorGroupCtx.Done()

			newUUID, errNewUUID := uuid.NewUUID()
			if errNewUUID != nil {
				return fmt.Errorf("unable to generate background task UUID, err: %v", errNewUUID)
			}

			taskStopCtx, taskStopCtxCancel := context.WithTimeout(
				c.createChildContext(newUUID.String(), t.GetName()),
				c.stopTimeout,
			)
			defer taskStopCtxCancel()

			return task.OnStop(taskStopCtx)
		})

		bgTasksWG.Add(1)

		// Define the background task's execution logic.
		TaskErrorGroup.Go(func() error {
			defer bgTasksWG.Done()

			taskErr := task.OnStart(TaskErrorGroupCtx)
			if taskErr != nil && background.IsCriticalToStop(task) {
				return taskErr
			}

			return nil
		})
	}

	// Register the Brokkr's signals to the interruptSignal channel.
	signal.Notify(interruptSignal, c.signals...)

	// Main loop monitors for interrupts or cancellations in the TaskErrorGroup's context.
	TaskErrorGroup.Go(func() error {
		for {
			select {
			case <-TaskErrorGroupCtx.Done():
				// Context is done, probably due to an error or forced shutdown.
				return TaskErrorGroupCtx.Err()
			case <-interruptSignal:
				// Interrupt received, begin the shutdown process.
				return c.Stop()
			}
		}
	})

	// Wait for all tasks in the error group to complete.
	// If any error occurs, and it's not due to cancellation, it's returned.
	if err := TaskErrorGroup.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

// Stop in graceful mode and terminate all background tasks.
func (c *Brokkr) Stop() error {
	if c.mainContextCancel != nil {
		c.mainContextCancel()
	}

	return nil
}

// createChildContext from parent.
func (c *Brokkr) createChildContext(k contextOfBrokkr, v string) context.Context {
	return context.WithValue(c.mainContext, k, v)
}
