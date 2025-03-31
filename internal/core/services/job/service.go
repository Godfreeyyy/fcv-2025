package job

import (
	"context"

	"github.com/google/uuid"

	"gitlab.com/fcv-2025.net/internal/domain"
)

// JobService defines the interface for managing jobs
type IJobService interface {
	// EnqueueJob adds a job to the queue
	EnqueueJob(ctx context.Context, jobType string, payload map[string]interface{}, priority int) (uuid.UUID, error)

	// GetJob retrieves a job by ID
	GetJob(ctx context.Context, jobID uuid.UUID) (*domain.Job, error)

	// CancelJob cancels a pending or scheduled job
	CancelJob(ctx context.Context, jobID uuid.UUID) error

	// GetJobTypes retrieves available job types
	GetJobTypes(ctx context.Context) ([]string, error)
	ScheduleJobs(ctx context.Context) ([]*domain.Job, error)
}
