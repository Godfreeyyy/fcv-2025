package schedule

import (
	"context"

	"github.com/google/uuid"
	"gitlab.com/fcv-2025.net/internal/domain"
)

// JobType represents the type of job
type JobType string

// JobStatus represents the status of a job
type JobStatus string

// ISchedulerService defines the interface for the job scheduler
type ISchedulerService interface {
	// ScheduleJobs moves jobs from pending to scheduled state
	ScheduleJobs(ctx context.Context) error

	// AssignJobs assigns jobs to available workers
	AssignJobs(ctx context.Context) error

	// GetJobsForWorker gets jobs for a specific worker
	GetJobsForWorker(ctx context.Context, workerID string, limit int) ([]*domain.Job, error)

	// HandleJobResult processes a job result from a worker
	HandleJobResult(ctx context.Context, jobID uuid.UUID, success bool, output string, errorMsg string) error

	// RequeueFailedJobs requeues jobs that have failed
	RequeueFailedJobs(ctx context.Context) error

	// SetWorkerNotifier sets a function to notify workers of available jobs
	SetWorkerNotifier(notifier func(jobType string))
}
