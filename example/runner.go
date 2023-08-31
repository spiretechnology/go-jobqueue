package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spiretechnology/go-jobqueue"
)

var _ jobqueue.JobRunner[MyJob] = &MyJobRunner{}

type MyJobRunner struct{}

func (r *MyJobRunner) RunJob(ctx context.Context, job MyJob) error {
	fmt.Printf("Running job %q...\n", job.Name)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(3 * time.Second):
	}
	fmt.Printf("Completed job %q!\n", job.Name)
	return nil
}
