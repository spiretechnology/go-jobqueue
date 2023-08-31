package jobqueue

import (
	"context"
	"errors"
	"log"
	"runtime"
	"sort"
	"sync"
	"time"
)

// Manager is a type that pulls jobs from a queue and runs them.
type Manager[T any, ID comparable] interface {
	Run(ctx context.Context) error
	ActiveJobs() []T
	CancelJob(jobID ID) error
}

// Options defines the options for a Manager.
type Options struct {
	Workers      int
	PollInterval time.Duration
	Logger       *log.Logger
	LogFlags     LogFlag
}

// New creates a new Manager using the given job queue and job runner, using default options.
func New[T any, ID comparable](queue JobQueue[T, ID], runner JobRunner[T]) Manager[T, ID] {
	return NewWithOptions(queue, runner, Options{
		Workers:      runtime.NumCPU(),
		PollInterval: 15 * time.Second,
		Logger:       log.Default(),
		LogFlags:     LogFlagAll,
	})
}

// NewWithOptions creates a new Manager using the given job queue, job runner, and options.
func NewWithOptions[T any, ID comparable](queue JobQueue[T, ID], runner JobRunner[T], options Options) Manager[T, ID] {
	if options.Workers < 1 {
		options.Workers = 1
	}
	return &manager[T, ID]{
		queue:      queue,
		runner:     runner,
		options:    options,
		activeJobs: make(map[ID]jobInfo[T, ID]),
	}
}

type manager[T any, ID comparable] struct {
	queue         JobQueue[T, ID]
	runner        JobRunner[T]
	options       Options
	activeJobsMut sync.RWMutex
	activeJobs    map[ID]jobInfo[T, ID]
}

type jobInfo[T any, ID comparable] struct {
	index      uint64
	id         ID
	job        T
	cancelFunc context.CancelFunc
}

func (m *manager[T, ID]) Run(ctx context.Context) error {
	// Create channels for the workers to notify that they are ready for a job, and for
	// the manager to send the workers jobs.
	chanSlotReady := make(chan struct{})
	chanJobs := make(chan jobInfo[T, ID])

	// Start the workers
	var wg sync.WaitGroup
	wg.Add(m.options.Workers)
	for i := 0; i < m.options.Workers; i++ {
		go func() {
			defer wg.Done()
			m.runWorker(ctx, chanSlotReady, chanJobs)
		}()
	}

	m.flagPrintf(LogManagerStarted, "started running manager")

	// Poll for jobs and send them to the workers
	m.pollForJobs(ctx, chanSlotReady, chanJobs)
	close(chanJobs)

	// Wait for all the workers to finish
	m.flagPrintf(LogManagerStopped, "stopping job queue manager...")
	wg.Wait()
	m.flagPrintf(LogManagerStopped, "stopped job queue manager")
	close(chanSlotReady)
	return ctx.Err()
}

func (m *manager[T, ID]) ActiveJobs() []T {
	m.activeJobsMut.RLock()
	defer m.activeJobsMut.RUnlock()

	// Copy the active jobs into a slice
	wrappedJobs := make([]jobInfo[T, ID], 0, len(m.activeJobs))
	for _, job := range m.activeJobs {
		wrappedJobs = append(wrappedJobs, job)
	}

	// Sort the jobs by index
	sort.Slice(wrappedJobs, func(i, j int) bool {
		return wrappedJobs[i].index < wrappedJobs[j].index
	})

	// Extract the jobs from the wrapped jobs
	jobs := make([]T, len(wrappedJobs))
	for i, job := range wrappedJobs {
		jobs[i] = job.job
	}
	return jobs
}

func (m *manager[T, ID]) CancelJob(jobID ID) error {
	// Get the job from the active jobs map
	m.activeJobsMut.RLock()
	job, ok := m.activeJobs[jobID]
	m.activeJobsMut.RUnlock()
	if !ok {
		return nil
	}

	// Cancel the job
	if job.cancelFunc != nil {
		job.cancelFunc()
	}
	return nil
}

func (m *manager[T, ID]) pollForJobs(ctx context.Context, chanSlotReady chan struct{}, chanJobs chan<- jobInfo[T, ID]) {
	// The amount of time to wait before attempting to find a job.
	var nextJobTimeout time.Duration
	var nextJobIndex uint64

	// Loop indefinitely, pulling jobs from the queue and sending them to workers
	for {
		// Wait for the designated timeout
		if nextJobTimeout > 0 {
			m.flagPrintf(LogWaiting, "waiting for %v", nextJobTimeout)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(nextJobTimeout):
		}

		// Wait for a slot to be ready
		select {
		case <-ctx.Done():
			return
		case <-chanSlotReady:
		}

		// Fetch the next job in the queue
		job, jobID, err := m.safelyGetNextJob(ctx)
		if err != nil {
			m.printf("failed to fetch job: %s", err.Error())
		}
		if err != nil || job == nil {
			// Make the slot available again and try again after a timeout
			// of 15 seconds to prevent us from DOSing the API
			chanSlotReady <- struct{}{}
			nextJobTimeout = m.options.PollInterval
			continue
		}

		// Wrap the job in jobInfo
		jobInfo := jobInfo[T, ID]{
			index: nextJobIndex,
			id:    jobID,
			job:   *job,
		}
		nextJobIndex++

		// Send the job to a worker
		select {
		case <-ctx.Done():
			return
		case chanJobs <- jobInfo:
		}

		// Immediately attempt to fetch the next job
		nextJobTimeout = 0
	}
}

func (m *manager[T, ID]) safelyGetNextJob(ctx context.Context) (job *T, jobID ID, err error) {
	defer func() {
		if r := recover(); r != nil {
			m.printf("recovered from panic getting next job: ", r)
			job = nil
			err = errors.New("panic getting next job")
		}
	}()

	// Get the next job
	return m.queue.NextJob(ctx)
}

func (m *manager[T, ID]) runWorker(ctx context.Context, chanReady chan<- struct{}, chanJobs <-chan jobInfo[T, ID]) {
	for {
		// Notify the manager that we're ready for a job
		select {
		case <-ctx.Done():
			return
		case chanReady <- struct{}{}:
		}

		// Wait for a job in the channel
		var jobInfo jobInfo[T, ID]
		var ok bool
		select {
		case <-ctx.Done():
			return
		case jobInfo, ok = <-chanJobs:
		}

		// If the channel was closed, we're done
		if !ok {
			return
		}

		// Runt the job
		m.runJob(ctx, jobInfo)
	}
}

func (m *manager[T, ID]) runJob(ctx context.Context, jobInfo jobInfo[T, ID]) {
	// Create a context for the job
	ctx, jobInfo.cancelFunc = context.WithCancel(ctx)
	defer jobInfo.cancelFunc()

	// Add the job to the active jobs map
	m.activeJobsMut.Lock()
	m.activeJobs[jobInfo.id] = jobInfo
	m.activeJobsMut.Unlock()

	// Run the job
	m.flagPrintf(LogJobStarted, "running job %v", jobInfo.id)
	err := m.safelyRunJob(ctx, jobInfo.job)
	if err == nil {
		m.flagPrintf(LogJobCompleted, "completed job %v", jobInfo.id)
	} else if errors.Is(err, context.Canceled) {
		m.flagPrintf(LogJobFailed, "canceled job: %v", jobInfo.id)
	} else {
		m.flagPrintf(LogJobFailed, "error running job: %s", err.Error())
	}

	// Remove the job from the active jobs map
	m.activeJobsMut.Lock()
	delete(m.activeJobs, jobInfo.id)
	m.activeJobsMut.Unlock()
}

func (m *manager[T, ID]) safelyRunJob(ctx context.Context, job T) (err error) {
	defer func() {
		if r := recover(); r != nil {
			m.printf("recovered from panic in job runner: ", r)
			err = errors.New("panic in job runner")
		}
	}()

	// Run the job
	return m.runner.RunJob(ctx, job)
}
