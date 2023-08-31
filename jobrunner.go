package jobqueue

import "context"

// JobRunner runs a single job.
type JobRunner[T any] interface {
	RunJob(ctx context.Context, job T) error
}

type JobRunnerFunc[T any] func(ctx context.Context, job T) error

func (f JobRunnerFunc[T]) RunJob(ctx context.Context, job T) error {
	return f(ctx, job)
}
