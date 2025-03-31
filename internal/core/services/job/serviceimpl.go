package job

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/ports/secondary"
	"gitlab.com/fcv-2025.net/internal/domain"
)

var _ IJobService = (*JobService)(nil)

// JobService implements the JobService interface
type JobService struct {
	jobRepo           secondary.JobRepository
	jobTypeConfigRepo secondary.JobTypeConfigRepository
	logger            primary.Logger
	workerNotifier    func(jobType string)
}

// NewJobService creates a new job service
func NewJobService(
	jobRepo secondary.JobRepository,
	jobTypeConfigRepo secondary.JobTypeConfigRepository,
	logger primary.Logger,
) *JobService {
	return &JobService{
		jobRepo:           jobRepo,
		jobTypeConfigRepo: jobTypeConfigRepo,
		logger:            logger,
		// Default no-op notifier
		workerNotifier: func(jobType string) {},
	}
}

// SetWorkerNotifier sets the function to call when notifying workers of available jobs
func (s *JobService) SetWorkerNotifier(notifier func(jobType string)) {
	if notifier != nil {
		s.workerNotifier = notifier
	}
}

// EnqueueJob adds a job to the queue
func (s *JobService) EnqueueJob(ctx context.Context, jobType string, payload map[string]interface{}, priority int) (uuid.UUID, error) {
	// Create a new job ID
	jobID := uuid.New()

	s.logger.Info("Enqueueing job",
		"jobId", jobID,
		"type", jobType,
		"priority", priority)

	// Validate job type
	if err := s.validateJobType(ctx, jobType); err != nil {
		return uuid.Nil, fmt.Errorf("invalid job type: %w", err)
	}

	// Get job type configuration for defaults
	_, err := s.jobTypeConfigRepo.GetJobTypeConfig(ctx, jobType)
	if err != nil {
		s.logger.Error("Failed to get job type config", "type", jobType, "error", err)
		return uuid.Nil, fmt.Errorf("failed to get job type config: %w", err)
	}

	// Create a new job
	job := &domain.Job{
		ID:          jobID,
		Type:        domain.JobType(jobType),
		Status:      domain.JobStatusPending,
		Payload:     payload,
		Priority:    priority,
		CreatedAt:   time.Now(),
		ScheduledAt: nil,
		StartedAt:   nil,
		CompletedAt: nil,
		WorkerID:    nil,
		RetryCount:  0,
	}

	// Save the job to the repository
	if err := s.jobRepo.SaveJob(ctx, job); err != nil {
		s.logger.Error("Failed to save job", "jobId", jobID, "error", err)
		return uuid.Nil, fmt.Errorf("failed to save job: %w", err)
	}

	// Log job created
	s.logger.Info("Job enqueued successfully", "jobId", jobID)

	// Notify any worker that a new job is available
	// This is done asynchronously to avoid blocking
	go func() {
		// Create a new context since the original may expire
		_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if s.workerNotifier != nil {
			s.workerNotifier(jobType)
		}
	}()

	return jobID, nil
}

// GetJob retrieves a job by ID
func (s *JobService) GetJob(ctx context.Context, jobID uuid.UUID) (*domain.Job, error) {
	s.logger.Debug("Getting job", "jobId", jobID)

	// Get the job from the repository
	job, err := s.jobRepo.GetJob(ctx, jobID)
	if err != nil {
		s.logger.Error("Failed to get job", "jobId", jobID, "error", err)
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	if job == nil {
		return nil, nil // Not found
	}

	// Optionally get job result if completed or failed
	if job.Status == domain.JobStatusCompleted || job.Status == domain.JobStatusFailed {
		result, err := s.jobRepo.GetJobResult(ctx, jobID)
		if err != nil {
			s.logger.Error("Failed to get job result", "jobId", jobID, "error", err)
			// Continue even if result not found - the job itself is still valid
		}

		if result != nil {
			// We could add the result to the job object here if needed
			// For now, just log that we found it
			s.logger.Debug("Found job result", "jobId", jobID, "success", result.Success)
		}
	}

	return job, nil
}

// CancelJob cancels a pending or scheduled job
func (s *JobService) CancelJob(ctx context.Context, jobID uuid.UUID) error {
	s.logger.Info("Cancelling job", "jobId", jobID)

	// Get the current job state
	job, err := s.jobRepo.GetJob(ctx, jobID)
	if err != nil {
		s.logger.Error("Failed to get job for cancellation", "jobId", jobID, "error", err)
		return fmt.Errorf("failed to get job: %w", err)
	}

	if job == nil {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Can only cancel pending or scheduled jobs
	if job.Status != domain.JobStatusPending && job.Status != domain.JobStatusScheduled {
		return fmt.Errorf("cannot cancel job with status '%s'", job.Status)
	}

	// Update job status to cancelled
	err = s.jobRepo.UpdateJobStatus(ctx, jobID, domain.JobStatusCancelled)
	if err != nil {
		s.logger.Error("Failed to update job status", "jobId", jobID, "error", err)
		return fmt.Errorf("failed to cancel job: %w", err)
	}

	s.logger.Info("Job cancelled", "jobId", jobID)
	return nil
}

// GetJobTypes retrieves available job types
func (s *JobService) GetJobTypes(ctx context.Context) ([]string, error) {
	s.logger.Debug("Getting job types")

	// Get available job types from configuration
	activeTypes, err := s.jobTypeConfigRepo.GetActiveJobTypes(ctx)
	if err != nil {
		s.logger.Error("Failed to get job types", "error", err)
		return nil, fmt.Errorf("failed to get job types: %w", err)
	}

	return activeTypes, nil
}

// validateJobType checks if the job type is valid
func (s *JobService) validateJobType(ctx context.Context, jobType string) error {
	// Get available job types from configuration
	availableTypes, err := s.jobTypeConfigRepo.GetActiveJobTypes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get job types: %w", err)
	}

	// Check if the requested type is available
	for _, availableType := range availableTypes {
		if availableType == jobType {
			return nil
		}
	}

	return fmt.Errorf("job type '%s' is not supported", jobType)
}

// ScheduleJobs schedules jobs for execution, default get first 100 pending jobs
func (s *JobService) ScheduleJobs(ctx context.Context) ([]*domain.Job, error) {
	s.logger.Debug("Scheduling jobs")

	// Get pending jobs
	pendingJobs, err := s.jobRepo.GetPendingJobs(ctx, 100)
	if err != nil {
		s.logger.Error("Failed to get pending jobs", "error", err)
		return nil, nil
	}

	if len(pendingJobs) == 0 {
		s.logger.Debug("No pending jobs found")
		return nil, nil
	}

	// Schedule each job
	for _, job := range pendingJobs {
		now := time.Now()
		job.Status = domain.JobStatusScheduled
		job.ScheduledAt = &now

		if err := s.jobRepo.SaveJob(ctx, job); err != nil {
			s.logger.Error("Failed to update job status", "jobId", job.ID, "error", err)
			continue
		}

		s.logger.Info("Job scheduled", "jobId", job.ID)
	}

	return pendingJobs, nil
}
