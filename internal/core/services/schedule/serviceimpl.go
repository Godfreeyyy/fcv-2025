package schedule

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"gitlab.com/fcv-2025.net/internal/config"
	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/ports/secondary"
	"gitlab.com/fcv-2025.net/internal/domain"
)

var _ ISchedulerService = &SchedulerService{}

// SchedulerService implements the SchedulerService interface
type SchedulerService struct {
	jobRepo        secondary.JobRepository
	workerRepo     secondary.WorkerRepository
	logger         primary.Logger
	workerNotifier func(jobType string)
	SchedulerCfg   *config.ScheduleSvcCfg
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(
	jobRepo secondary.JobRepository,
	workerRepo secondary.WorkerRepository,
	logger primary.Logger,
	SchedulerCfg *config.ScheduleSvcCfg,
) *SchedulerService {
	return &SchedulerService{
		jobRepo:        jobRepo,
		workerRepo:     workerRepo,
		logger:         logger,
		workerNotifier: func(jobType string) {},
		SchedulerCfg:   SchedulerCfg,
	}
}

// GetJobsForWorker gets jobs for a specific worker
func (s *SchedulerService) GetJobsForWorker(ctx context.Context, workerID string, limit int) ([]*domain.Job, error) {
	s.logger.Debug("Getting jobs for worker", "workerID", workerID, "limit", limit)

	// First get the worker info to find out its type and capacity
	worker, err := s.workerRepo.GetWorker(ctx, workerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get worker info: %w", err)
	}

	if worker == nil {
		return nil, fmt.Errorf("worker not found")
	}

	// Check if worker has available capacity
	if worker.CurrentLoad >= worker.Capacity {
		s.logger.Debug("Worker at capacity", "workerID", workerID, "load", worker.CurrentLoad, "capacity", worker.Capacity)
		return nil, nil
	}

	// Available capacity is the difference between capacity and current load
	availableCapacity := worker.Capacity - worker.CurrentLoad

	// If requested limit is greater than available capacity, reduce it
	if limit > availableCapacity {
		limit = availableCapacity
	}

	// Get scheduled jobs of the worker's type
	jobs, err := s.jobRepo.GetScheduledJobsByType(ctx, string(worker.Type), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled jobs: %w", err)
	}

	if len(jobs) == 0 {
		return nil, nil
	}

	// Assign jobs to this worker
	assignedJobs := make([]*domain.Job, 0, len(jobs))
	for _, job := range jobs {
		// Update job status and worker ID
		now := time.Now()
		job.Status = domain.JobStatusRunning
		job.StartedAt = &now
		job.WorkerID = &workerID

		// Save updated job
		if err := s.jobRepo.SaveJob(ctx, job); err != nil {
			s.logger.Error("Failed to update job status", "jobID", job.ID, "error", err)
			continue
		}

		assignedJobs = append(assignedJobs, job)
		s.logger.Info("Job assigned to worker", "jobID", job.ID, "workerID", workerID)
	}

	// Update worker load
	if len(assignedJobs) > 0 {
		worker.CurrentLoad += len(assignedJobs)
		if err := s.workerRepo.SaveWorker(ctx, worker); err != nil {
			s.logger.Error("Failed to update worker load", "workerID", workerID, "error", err)
		}
	}

	return assignedJobs, nil
}

// SetWorkerNotifier sets a function to notify workers of available jobs
func (s *SchedulerService) SetWorkerNotifier(notifier func(jobType string)) {
	if notifier != nil {
		s.workerNotifier = notifier
	}
}

// ScheduleJobs moves jobs from pending to scheduled state
func (s *SchedulerService) ScheduleJobs(ctx context.Context) error {
	s.logger.Info("Scheduling pending jobs")

	// Get pending jobs
	pendingJobs, err := s.jobRepo.GetPendingJobs(ctx, 100)
	if err != nil {
		s.logger.Error("Failed to get pending jobs", "error", err)
		return err
	}

	s.logger.Info("Found pending jobs", "count", len(pendingJobs))

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

	return nil
}

// AssignJobs assigns jobs to available workers
func (s *SchedulerService) AssignJobs(ctx context.Context) error {
	s.logger.Info("Assigning scheduled jobs to workers")

	// Get scheduled jobs
	scheduledJobs, err := s.jobRepo.GetScheduledJobs(ctx, 100)
	if err != nil {
		s.logger.Error("Failed to get scheduled jobs", "error", err)
		return err
	}

	s.logger.Info("Found scheduled jobs", "count", len(scheduledJobs))

	// Get workers by type for job assignment
	workersByType := make(map[domain.JobType][]*domain.WorkerInfo)

	// Assign each job to an available worker
	for _, job := range scheduledJobs {
		// Get workers for this job type if not already cached
		workers, found := workersByType[job.Type]
		if !found {
			workerType := string(job.Type)
			var err error
			workers, err = s.getWorkersForJobType(ctx, workerType)
			if err != nil {
				s.logger.Error("Failed to get workers for job type", "jobType", job.Type, "error", err)
				continue
			}
			workersByType[job.Type] = workers
		}

		// Find the best worker for this job
		worker := s.findBestWorker(workers)
		if worker == nil {
			s.logger.Info("No available workers for job", "jobId", job.ID, "type", job.Type)
			continue
		}

		// Assign job to worker
		if err := s.jobRepo.AssignJobToWorker(ctx, job.ID, worker.ID); err != nil {
			s.logger.Error("Failed to assign job to worker", "jobId", job.ID, "workerId", worker.ID, "error", err)
			continue
		}

		// Update worker load
		worker.CurrentLoad++
		if err = s.workerRepo.SaveWorker(ctx, worker); err != nil {
			s.logger.Error("Failed to update worker load", "workerId", worker.ID, "error", err)
		}

		s.logger.Info("Job assigned to worker", "jobId", job.ID, "workerId", worker.ID)
	}

	return nil
}

// getWorkersForJobType gets available workers for a specific job type
func (s *SchedulerService) getWorkersForJobType(ctx context.Context, jobType string) ([]*domain.WorkerInfo, error) {
	workers, err := s.workerRepo.GetAvailableWorkers(ctx, jobType)
	if err != nil {
		return nil, err
	}

	// Filter workers with available capacity
	availableWorkers := make([]*domain.WorkerInfo, 0)
	for _, worker := range workers {
		if worker.CurrentLoad < worker.Capacity {
			availableWorkers = append(availableWorkers, worker)
		}
	}

	return availableWorkers, nil
}

// findBestWorker finds the best worker for a job based on current load
func (s *SchedulerService) findBestWorker(workers []*domain.WorkerInfo) *domain.WorkerInfo {
	if len(workers) == 0 {
		return nil
	}

	// Find worker with lowest load relative to capacity
	var bestWorker *domain.WorkerInfo
	var bestLoadRatio float64 = 2.0 // Start with value > 1

	for _, worker := range workers {
		loadRatio := float64(worker.CurrentLoad) / float64(worker.Capacity)
		if loadRatio < bestLoadRatio {
			bestWorker = worker
			bestLoadRatio = loadRatio
		}
	}

	return bestWorker
}

// RequeueFailedJobs requeues jobs that have failed after a timeout
func (s *SchedulerService) RequeueFailedJobs(ctx context.Context) error {
	// In a real system, you would have logic to detect and requeue failed jobs
	// For example, jobs that have been in "RUNNING" state for too long
	return nil
}

// HandleJobResult processes a job result from a worker
// HandleJobResult processes a job result from a worker
func (s *SchedulerService) HandleJobResult(ctx context.Context, jobID uuid.UUID, success bool, output string, errorMsg string) error {
	s.logger.Info("Processing job result", "jobID", jobID, "success", success)

	// Get the job
	job, err := s.jobRepo.GetJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	if job == nil {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Get the worker
	workerID := ""
	if job.WorkerID != nil {
		workerID = *job.WorkerID
	}

	if workerID != "" {
		worker, err := s.workerRepo.GetWorker(ctx, workerID)
		if err != nil {
			s.logger.Error("Failed to get worker", "workerID", workerID, "error", err)
		} else if worker != nil {
			// Decrease worker load
			if worker.CurrentLoad > 0 {
				worker.CurrentLoad--
				if err := s.workerRepo.SaveWorker(ctx, worker); err != nil {
					s.logger.Error("Failed to update worker load", "workerID", workerID, "error", err)
				}
			}
		}
	}

	// Update job status
	now := time.Now()
	job.CompletedAt = &now

	if success {
		job.Status = domain.JobStatusCompleted
	} else {
		job.Status = domain.JobStatusFailed
		job.RetryCount++
	}

	// Store the result
	result := &domain.JobResult{
		JobID:   jobID,
		Output:  output,
		Error:   errorMsg,
		Success: success,
	}

	if err := s.jobRepo.SaveJobResult(ctx, result); err != nil {
		s.logger.Error("Failed to save job result", "jobID", jobID, "error", err)
	}

	// Save the updated job
	if err := s.jobRepo.SaveJob(ctx, job); err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	s.logger.Info("Job result processed", "jobID", jobID, "status", job.Status)
	return nil
}
