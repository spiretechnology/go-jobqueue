package main

import (
	"context"
	"fmt"

	"github.com/spiretechnology/go-jobqueue"
)

var _ jobqueue.JobQueue[MyJob, int] = &MyJobQueue{}

type MyJobQueue struct {
	nextJobID int
}

func (q *MyJobQueue) NextJob(ctx context.Context) (*MyJob, int, error) {
	q.nextJobID++
	job := MyJob{
		ID:   q.nextJobID,
		Name: fmt.Sprintf("My Job #%d", q.nextJobID),
	}
	return &job, job.ID, nil
}
