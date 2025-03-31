package worker

import (
	"context"
	"fmt"
	"time"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/ports/secondary"
	"gitlab.com/fcv-2025.net/internal/domain"
)

var _ IWorkerRegistrationService = &WorkerRegistrationService{}

// WorkerRegistrationService implements the WorkerRegistrationService interface
type WorkerRegistrationService struct {
	workerRepo secondary.WorkerRepository
	logger     primary.Logger
}

func (s *WorkerRegistrationService) GetAllWorkers(ctx context.Context) ([]*domain.WorkerInfo, error) {
	s.logger.Debug("Getting all workers")

	// Get all workers from the repository
	workers, err := s.workerRepo.GetAllWorkers(ctx)
	if err != nil {
		s.logger.Error("Failed to get all workers", "error", err)
		return nil, fmt.Errorf("failed to get all workers: %w", err)
	}

	// Add status information
	heartbeatThreshold := time.Now().Add(-2 * time.Minute)
	for _, worker := range workers {
		// Annotate with active status (not modifying the actual worker object)
		worker.IsActive = worker.LastHeartbeat.After(heartbeatThreshold)
	}

	return workers, nil
}

func (s *WorkerRegistrationService) GetWorkerTypes(ctx context.Context) ([]string, error) {
	s.logger.Debug("Getting worker types")

	// Get distinct worker types
	types, err := s.workerRepo.GetWorkerTypes(ctx)
	if err != nil {
		s.logger.Error("Failed to get worker types", "error", err)
		return nil, fmt.Errorf("failed to get worker types: %w", err)
	}

	return types, nil
}

// NewWorkerRegistrationService creates a new worker registration service
func NewWorkerRegistrationService(workerRepo secondary.WorkerRepository, logger primary.Logger) *WorkerRegistrationService {
	return &WorkerRegistrationService{
		workerRepo: workerRepo,
		logger:     logger,
	}
}

// RegisterWorker registers a worker as available for jobs
func (s *WorkerRegistrationService) RegisterWorker(ctx context.Context, workerInfo *domain.WorkerInfo) error {
	s.logger.Info("Registering worker", "workerId", workerInfo.ID, "type", workerInfo.Type)

	// Set last heartbeat to now
	workerInfo.LastHeartbeat = time.Now()

	// Save worker information
	if err := s.workerRepo.SaveWorker(ctx, workerInfo); err != nil {
		s.logger.Error("Failed to save worker", "error", err)
		return fmt.Errorf("failed to register worker: %w", err)
	}

	return nil
}

// Heartbeat updates the worker's status and availability
func (s *WorkerRegistrationService) Heartbeat(ctx context.Context, workerID string, load int) error {
	s.logger.Debug("Received worker heartbeat", "workerId", workerID, "load", load)

	// Retrieve the worker to ensure it exists
	worker, err := s.workerRepo.GetWorker(ctx, workerID)
	if err != nil {
		s.logger.Error("Failed to get worker for heartbeat", "workerId", workerID, "error", err)
		return fmt.Errorf("failed to get worker: %w", err)
	}

	if worker == nil {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	// Update worker heartbeat and load
	worker.LastHeartbeat = time.Now()
	worker.CurrentLoad = load

	// Save the updated worker
	if err := s.workerRepo.SaveWorker(ctx, worker); err != nil {
		s.logger.Error("Failed to update worker heartbeat", "workerId", workerID, "error", err)
		return fmt.Errorf("failed to update worker heartbeat: %w", err)
	}

	return nil
}

// GetAvailableWorkers gets all available workers of a given type
func (s *WorkerRegistrationService) GetAvailableWorkers(ctx context.Context, workerType string) ([]*domain.WorkerInfo, error) {
	s.logger.Debug("Getting available workers", "type", workerType)

	// Get all workers of the specified type
	workers, err := s.workerRepo.GetWorkersByType(ctx, workerType)
	if err != nil {
		s.logger.Error("Failed to get workers by type", "type", workerType, "error", err)
		return nil, fmt.Errorf("failed to get workers by type: %w", err)
	}

	// Filter active workers with available capacity
	availableWorkers := make([]*domain.WorkerInfo, 0)
	heartbeatThreshold := time.Now().Add(-2 * time.Minute) // Consider workers inactive after 2 minutes without heartbeat

	for _, worker := range workers {
		if worker.LastHeartbeat.After(heartbeatThreshold) && worker.CurrentLoad < worker.Capacity {
			availableWorkers = append(availableWorkers, worker)
		}
	}

	return availableWorkers, nil
}

// CleanupInactiveWorkers removes workers that haven't sent a heartbeat recently
func (s *WorkerRegistrationService) CleanupInactiveWorkers(ctx context.Context) error {
	s.logger.Info("Cleaning up inactive workers")

	// Set cutoff time (5 minutes ago)
	cutoffTime := time.Now().Add(-5 * time.Minute)

	// Remove inactive workers
	if err := s.workerRepo.RemoveInactiveWorkers(ctx, cutoffTime); err != nil {
		s.logger.Error("Failed to remove inactive workers", "error", err)
		return fmt.Errorf("failed to clean up inactive workers: %w", err)
	}

	return nil
}
