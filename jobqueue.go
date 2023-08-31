package jobqueue

import "context"

// JobQueue is responsible for returning the next job to be run.
type JobQueue[T any, ID comparable] interface {
	NextJob(ctx context.Context) (*T, ID, error)
}

type JobQueueFunc[T any, ID comparable] func(ctx context.Context) (*T, ID, error)

func (f JobQueueFunc[T, ID]) NextJob(ctx context.Context) (*T, ID, error) {
	return f(ctx)
}
