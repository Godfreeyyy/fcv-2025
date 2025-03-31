package secondary

import (
	"context"
	"time"

	"gitlab.com/fcv-2025.net/internal/domain"
)

type WorkerRepository interface {
	// SaveWorker saves worker information
	SaveWorker(ctx context.Context, worker *domain.WorkerInfo) error

	// GetWorker retrieves worker information by ID
	GetWorker(ctx context.Context, workerID string) (*domain.WorkerInfo, error)

	// GetAvailableWorkers retrieves available workers of a given type
	GetAvailableWorkers(ctx context.Context, workerType string) ([]*domain.WorkerInfo, error)

	// UpdateWorkerHeartbeat updates a worker's heartbeat and load
	UpdateWorkerHeartbeat(ctx context.Context, workerID string, load int, time time.Time) error

	// RemoveInactiveWorkers removes workers that haven't sent a heartbeat recently
	RemoveInactiveWorkers(ctx context.Context, cutoffTime time.Time) error

	GetWorkersByType(ctx context.Context, workerType string) ([]*domain.WorkerInfo, error)

	GetAllWorkers(ctx context.Context) ([]*domain.WorkerInfo, error)

	GetWorkerTypes(ctx context.Context) ([]string, error)
}
