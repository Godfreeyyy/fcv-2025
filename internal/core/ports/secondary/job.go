package secondary

import (
	"context"

	"github.com/google/uuid"

	"gitlab.com/fcv-2025.net/internal/domain"
)

type JobRepository interface {
	// SaveJob saves a job
	SaveJob(ctx context.Context, job *domain.Job) error

	SaveBatches(ctx context.Context, batches []*domain.Job) error

	// GetJob retrieves a job by ID
	GetJob(ctx context.Context, jobID uuid.UUID) (*domain.Job, error)

	// GetPendingJobs retrieves pending jobs
	GetPendingJobs(ctx context.Context, limit int) ([]*domain.Job, error)

	// GetScheduledJobs retrieves scheduled jobs
	GetScheduledJobs(ctx context.Context, limit int) ([]*domain.Job, error)

	// UpdateJobStatus updates a job's status
	UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status domain.JobStatus) error

	// AssignJobToWorker assigns a job to a worker
	AssignJobToWorker(ctx context.Context, jobID uuid.UUID, workerID string) error

	// GetScheduledJobsByType retrieves scheduled jobs of a specific type
	GetScheduledJobsByType(ctx context.Context, jobType string, limit int) ([]*domain.Job, error)

	// SaveJobResult saves a job result
	SaveJobResult(ctx context.Context, result *domain.JobResult) error

	// GetJobResult retrieves a job result by job ID
	GetJobResult(ctx context.Context, jobID uuid.UUID) (*domain.JobResult, error)
}

type JobTypeConfigRepository interface {
	// GetJobTypeConfig retrieves configuration for a specific job type
	GetJobTypeConfig(ctx context.Context, jobType string) (*domain.JobTypeConfig, error)

	// GetAllJobTypeConfigs retrieves all job type configurations, including inactive ones
	GetAllJobTypeConfigs(ctx context.Context) ([]*domain.JobTypeConfig, error)

	// GetActiveJobTypes retrieves all active job type identifiers
	GetActiveJobTypes(ctx context.Context) ([]string, error)

	// SaveJobTypeConfig saves a job type configuration
	SaveJobTypeConfig(ctx context.Context, config *domain.JobTypeConfig) error

	// DeactivateJobType deactivates a job type
	DeactivateJobType(ctx context.Context, jobType string) error

	// ActivateJobType activates a job type
	ActivateJobType(ctx context.Context, jobType string) error

	// DeleteJobTypeConfig permanently deletes a job type configuration
	DeleteJobTypeConfig(ctx context.Context, jobType string) error
}
