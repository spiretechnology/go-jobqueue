# go-jobqueue

A multi-threaded polling job queue for Go.

## Example Usage

```go
// Create a manager
manager := jobqueue.New[MyJob, int](
    &MyJobQueue{},
    &MyJobRunner{},
)

// Run the manager
err := manager.Run(ctx)
```

You just need to create your own types to implement the JobQueue and JobRunner interfaces:

```go
// JobQueue is responsible for returning the next job to be run.
type JobQueue[T any, ID comparable] interface {
	NextJob(ctx context.Context) (*T, ID, error)
}

// JobRunner runs a single job.
type JobRunner[T any] interface {
	RunJob(ctx context.Context, job T) error
}
```

## Internal Details

The manager creates a number of worker goroutines to run jobs concurrently. The number of workers is configurable.

Each worker signals when it is ready for a new job. The manager than calls the `NextJob` method on the queue to get the next job to run. If there are no jobs available at that time, the manager will wait for a period of time before trying again. The wait time is configurable.

All error handling is done internally, in order to ensure the manager continues to run. The only condition that will cause the manager to stop is if the context passed to the `Run` method is cancelled.
