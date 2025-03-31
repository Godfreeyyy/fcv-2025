package workerport

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/domain"
)

const (
	workerKeyPrefix  = "worker:"
	workerTypePrefix = "worker:type:"
	workerExpiration = 5 * time.Minute
)

// WorkerRepository implements the WorkerRepository interface with Redis
type WorkerRepository struct {
	redisClient *redis.Client
	logger      primary.Logger
}

func (r *WorkerRepository) GetWorkersByType(ctx context.Context, workerType string) ([]*domain.WorkerInfo, error) {
	//TODO implement me
	panic("implement me")
}

// GetAllWorkers retrieves all worker information from Redis.
func (r *WorkerRepository) GetAllWorkers(ctx context.Context) ([]*domain.WorkerInfo, error) {
	var cursor uint64
	var workerKeys []string
	var workers []*domain.WorkerInfo
	var err error

	// Use SCAN to iterate over keys with the specified prefix
	for {
		var keys []string
		keys, cursor, err = r.redisClient.Scan(ctx, cursor, workerKeyPrefix+"*", 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan worker keys: %w", err)
		}
		workerKeys = append(workerKeys, keys...)
		if cursor == 0 {
			break
		}
	}

	if len(workerKeys) == 0 {
		return workers, nil // No workers found
	}

	// Use MGET to retrieve all worker data at once
	workerData, err := r.redisClient.MGet(ctx, workerKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve worker data: %w", err)
	}

	// Deserialize each worker data
	for _, data := range workerData {
		if data == nil {
			continue
		}
		var worker domain.WorkerInfo
		if err := json.Unmarshal([]byte(data.(string)), &worker); err != nil {
			return nil, fmt.Errorf("failed to unmarshal worker data: %w", err)
		}
		workers = append(workers, &worker)
	}

	return workers, nil
}

func (r *WorkerRepository) GetWorkerTypes(ctx context.Context) ([]string, error) {
	//TODO implement me
	panic("implement me")
}

// NewWorkerRepository creates a new Redis worker repository
func NewWorkerRepository(redisClient *redis.Client, logger primary.Logger) *WorkerRepository {
	return &WorkerRepository{
		redisClient: redisClient,
		logger:      logger,
	}
}

// SaveWorker saves worker information to Redis
func (r *WorkerRepository) SaveWorker(ctx context.Context, worker *domain.WorkerInfo) error {
	// Serialize worker info
	workerJSON, err := json.Marshal(worker)
	if err != nil {
		r.logger.Error("Failed to marshal worker info", "error", err)
		return fmt.Errorf("failed to marshal worker info: %w", err)
	}

	// Save worker info with expiration
	workerKey := fmt.Sprintf("%s%s", workerKeyPrefix, worker.ID)
	if err := r.redisClient.Set(ctx, workerKey, workerJSON, workerExpiration).Err(); err != nil {
		r.logger.Error("Failed to save worker info", "error", err)
		return fmt.Errorf("failed to save worker info: %w", err)
	}

	// Add worker to type index
	typeKey := fmt.Sprintf("%s%s", workerTypePrefix, worker.Type)
	if err := r.redisClient.SAdd(ctx, typeKey, worker.ID).Err(); err != nil {
		r.logger.Error("Failed to add worker to type index", "error", err)
		return fmt.Errorf("failed to add worker to type index: %w", err)
	}

	return nil
}

// GetWorker retrieves worker information from Redis by ID
func (r *WorkerRepository) GetWorker(ctx context.Context, workerID string) (*domain.WorkerInfo, error) {
	// Get worker info
	workerKey := fmt.Sprintf("%s%s", workerKeyPrefix, workerID)
	workerJSON, err := r.redisClient.Get(ctx, workerKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		r.logger.Error("Failed to get worker info", "error", err)
		return nil, fmt.Errorf("failed to get worker info: %w", err)
	}

	// Deserialize worker info
	var worker domain.WorkerInfo
	if err := json.Unmarshal(workerJSON, &worker); err != nil {
		r.logger.Error("Failed to unmarshal worker info", "error", err)
		return nil, fmt.Errorf("failed to unmarshal worker info: %w", err)
	}

	return &worker, nil
}

// GetAvailableWorkers retrieves available workers of a given type from Redis
func (r *WorkerRepository) GetAvailableWorkers(ctx context.Context, workerType string) ([]*domain.WorkerInfo, error) {
	// Get worker IDs for the given type
	typeKey := fmt.Sprintf("%s%s", workerTypePrefix, workerType)
	workerIDs, err := r.redisClient.SMembers(ctx, typeKey).Result()
	if err != nil {
		r.logger.Error("Failed to get worker IDs", "error", err)
		return nil, fmt.Errorf("failed to get worker IDs: %w", err)
	}

	// Get worker info for each ID
	workers := make([]*domain.WorkerInfo, 0, len(workerIDs))
	for _, workerID := range workerIDs {
		worker, err := r.GetWorker(ctx, workerID)
		if err != nil {
			r.logger.Error("Failed to get worker", "workerId", workerID, "error", err)
			continue
		}

		if worker != nil {
			workers = append(workers, worker)
		}
	}

	return workers, nil
}

// UpdateWorkerHeartbeat updates a worker's heartbeat and load in Redis
func (r *WorkerRepository) UpdateWorkerHeartbeat(ctx context.Context, workerID string, load int, heartbeatTime time.Time) error {
	// Get worker info
	worker, err := r.GetWorker(ctx, workerID)
	if err != nil {
		return err
	}

	if worker == nil {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	// Update worker info
	worker.CurrentLoad = load
	worker.LastHeartbeat = heartbeatTime

	// Save worker info
	return r.SaveWorker(ctx, worker)
}

// RemoveInactiveWorkers removes workers that haven't sent a heartbeat recently from Redis
func (r *WorkerRepository) RemoveInactiveWorkers(ctx context.Context, cutoffTime time.Time) error {
	// This is automatically handled by Redis expiration
	// We just need to remove expired workers from the type index

	// Get all worker type keys
	pattern := fmt.Sprintf("%s*", workerTypePrefix)
	typeKeys, err := r.redisClient.Keys(ctx, pattern).Result()
	if err != nil {
		r.logger.Error("Failed to get worker type keys", "error", err)
		return fmt.Errorf("failed to get worker type keys: %w", err)
	}

	// For each type, check and remove expired workers
	for _, typeKey := range typeKeys {
		workerIDs, err := r.redisClient.SMembers(ctx, typeKey).Result()
		if err != nil {
			r.logger.Error("Failed to get worker IDs", "typeKey", typeKey, "error", err)
			continue
		}

		for _, workerID := range workerIDs {
			workerKey := fmt.Sprintf("%s%s", workerKeyPrefix, workerID)
			exists, err := r.redisClient.Exists(ctx, workerKey).Result()
			if err != nil {
				r.logger.Error("Failed to check if worker exists", "workerId", workerID, "error", err)
				continue
			}

			if exists == 0 {
				// Worker has expired, remove from type index
				if err := r.redisClient.SRem(ctx, typeKey, workerID).Err(); err != nil {
					r.logger.Error("Failed to remove worker from type index", "workerId", workerID, "error", err)
				}
			}
		}
	}

	return nil
}
