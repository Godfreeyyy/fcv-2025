package worker

import (
	"context"

	"gitlab.com/fcv-2025.net/internal/domain"
)

// IWorkerRegistrationService defines the interface for worker registration
type IWorkerRegistrationService interface {
	// RegisterWorker registers a worker as available for jobs
	RegisterWorker(ctx context.Context, workerInfo *domain.WorkerInfo) error

	// Heartbeat updates the worker's status and availability
	Heartbeat(ctx context.Context, workerID string, load int) error

	// GetAvailableWorkers gets all available workers of a given type
	GetAvailableWorkers(ctx context.Context, workerType string) ([]*domain.WorkerInfo, error)

	// GetAllWorkers gets all registered workers
	GetAllWorkers(ctx context.Context) ([]*domain.WorkerInfo, error)

	// GetWorkerTypes gets all available worker types
	GetWorkerTypes(ctx context.Context) ([]string, error)

	// CleanupInactiveWorkers removes workers that haven't sent a heartbeat recently
	CleanupInactiveWorkers(ctx context.Context) error
}
