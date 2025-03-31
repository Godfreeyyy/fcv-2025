// package postgres contains PostgreSQL implementations of repositories
package jobrepository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jmoiron/sqlx"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/domain"
	querybuilder "gitlab.com/fcv-2025.net/internal/utils"
)

// JobRepository implements the JobRepository interface with PostgreSQL
type JobRepository struct {
	db     *sqlx.DB
	dbx    *sqlx.DB
	logger primary.Logger
}

// NewJobRepository creates a new PostgreSQL job repository
func NewJobRepository(db *sqlx.DB, logger primary.Logger) *JobRepository {
	return &JobRepository{
		db:     db,
		logger: logger,
	}
}

// SaveJob saves a job to PostgreSQL
func (r *JobRepository) SaveJob(ctx context.Context, job *domain.Job) error {
	// Serialize the payload to JSON
	payloadJSON, err := json.Marshal(job.Payload)
	if err != nil {
		r.logger.Error("Failed to marshal job payload", "error", err)
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}

	// Prepare the query
	query := `
		INSERT INTO jobs (
			id, type, status, payload, priority, created_at, 
			scheduled_at, started_at, completed_at, worker_id, retry_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			payload = EXCLUDED.payload,
			priority = EXCLUDED.priority,
			scheduled_at = EXCLUDED.scheduled_at,
			started_at = EXCLUDED.started_at,
			completed_at = EXCLUDED.completed_at,
			worker_id = EXCLUDED.worker_id,
			retry_count = EXCLUDED.retry_count
	`

	// Execute the query
	_, err = r.db.ExecContext(
		ctx,
		query,
		job.ID,
		job.Type,
		job.Status,
		payloadJSON,
		job.Priority,
		job.CreatedAt,
		job.ScheduledAt,
		job.StartedAt,
		job.CompletedAt,
		job.WorkerID,
		job.RetryCount,
	)

	if err != nil {
		r.logger.Error("Failed to save job", "error", err)
		return fmt.Errorf("failed to save job: %w", err)
	}

	return nil
}

func (r *JobRepository) SaveBatches(ctx context.Context, batches []*domain.Job) error {
	jobTbl := domain.GetJobTable()
	args := make([]interface{}, 0, len(batches)*14)
	query, args := querybuilder.NewQueryBuilder("public").
		Insert(
			jobTbl.Type,
			jobTbl.Status,
			jobTbl.Payload,
			jobTbl.Priority,
			jobTbl.FileObjectID,
			jobTbl.ScheduledAt,
			jobTbl.StartedAt,
			jobTbl.CompletedAt,
			jobTbl.WorkerID,
			jobTbl.RetryCount,
			jobTbl.RetryCount,
			jobTbl.RetryLimit,
			jobTbl.TimeoutSec,
			jobTbl.FileObjectID,
		).Into(jobTbl.TableName()).Values(args...).Build()

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("Failed to save job", "error", err)
		return fmt.Errorf("failed to save job: %w", err)
	}

	return nil
}

// GetJob retrieves a job from PostgreSQL by ID
func (r *JobRepository) GetJob(ctx context.Context, jobID uuid.UUID) (*domain.Job, error) {
	// Prepare the query
	query := `
		SELECT id, type, status, payload, priority, created_at,
			   scheduled_at, started_at, completed_at, worker_id, retry_count
		FROM jobs
		WHERE id = $1
	`

	// Execute the query
	var job domain.Job
	var payloadJSON []byte
	var workerID sql.NullString
	var scheduledAt, startedAt, completedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, jobID).Scan(
		&job.ID,
		&job.Type,
		&job.Status,
		&payloadJSON,
		&job.Priority,
		&job.CreatedAt,
		&scheduledAt,
		&startedAt,
		&completedAt,
		&workerID,
		&job.RetryCount,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get job", "error", err)
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Handle nullable fields
	if scheduledAt.Valid {
		job.ScheduledAt = &scheduledAt.Time
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}
	if workerID.Valid {
		job.WorkerID = &workerID.String
	}

	// Deserialize the payload
	if err := json.Unmarshal(payloadJSON, &job.Payload); err != nil {
		r.logger.Error("Failed to unmarshal job payload", "error", err)
		return nil, fmt.Errorf("failed to unmarshal job payload: %w", err)
	}

	return &job, nil
}

// GetScheduledJobsByType retrieves scheduled jobs of a specific type from PostgreSQL
func (r *JobRepository) GetScheduledJobsByType(ctx context.Context, jobType string, limit int) ([]*domain.Job, error) {
	// Prepare the query with locking to prevent other workers from getting the same jobs
	query := `
		SELECT id, type, status, payload, priority, created_at,
			   scheduled_at, started_at, completed_at, worker_id, retry_count
		FROM jobs
		WHERE status = $1 AND type = $2
		ORDER BY priority DESC, scheduled_at ASC
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	`

	// Execute the query within a transaction to maintain locks
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		r.logger.Error("Failed to begin transaction", "error", err)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Will be ignored if the transaction is committed

	// Execute the query
	rows, err := tx.QueryContext(ctx, query, domain.JobStatusScheduled, jobType, limit)
	if err != nil {
		r.logger.Error("Failed to get scheduled jobs by type", "error", err)
		return nil, fmt.Errorf("failed to get scheduled jobs by type: %w", err)
	}
	defer rows.Close()

	// Process the results
	jobs := make([]*domain.Job, 0)
	for rows.Next() {
		var job domain.Job
		var payloadJSON []byte
		var workerID sql.NullString
		var scheduledAt, startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.Status,
			&payloadJSON,
			&job.Priority,
			&job.CreatedAt,
			&scheduledAt,
			&startedAt,
			&completedAt,
			&workerID,
			&job.RetryCount,
		)

		if err != nil {
			r.logger.Error("Failed to scan job row", "error", err)
			return nil, fmt.Errorf("failed to scan job row: %w", err)
		}

		// Handle nullable fields
		if scheduledAt.Valid {
			job.ScheduledAt = &scheduledAt.Time
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}
		if workerID.Valid {
			job.WorkerID = &workerID.String
		}

		// Deserialize the payload
		if err := json.Unmarshal(payloadJSON, &job.Payload); err != nil {
			r.logger.Error("Failed to unmarshal job payload", "error", err)
			return nil, fmt.Errorf("failed to unmarshal job payload: %w", err)
		}

		jobs = append(jobs, &job)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating job rows", "error", err)
		return nil, fmt.Errorf("error iterating job rows: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		r.logger.Error("Failed to commit transaction", "error", err)
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return jobs, nil
}

// SaveJobResult saves a job result to PostgreSQL
func (r *JobRepository) SaveJobResult(ctx context.Context, result *domain.JobResult) error {
	// Prepare the query
	query := `
		INSERT INTO job_results (
			job_id, output, error, completed_at, success
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (job_id) DO UPDATE SET
			output = EXCLUDED.output,
			error = EXCLUDED.error,
			completed_at = EXCLUDED.completed_at,
			success = EXCLUDED.success
	`

	// Execute the query
	_, err := r.db.ExecContext(
		ctx,
		query,
		result.JobID,
		result.Output,
		result.Error,
		result.CompletedAt,
		result.Success,
	)

	if err != nil {
		r.logger.Error("Failed to save job result", "error", err)
		return fmt.Errorf("failed to save job result: %w", err)
	}

	return nil
}

// GetJobResult retrieves a job result by job ID
func (r *JobRepository) GetJobResult(ctx context.Context, jobID uuid.UUID) (*domain.JobResult, error) {
	// Prepare the query
	query := `
		SELECT job_id, output, error, completed_at, success
		FROM job_results
		WHERE job_id = $1
	`

	// Execute the query
	var result domain.JobResult

	err := r.db.QueryRowContext(ctx, query, jobID).Scan(
		&result.JobID,
		&result.Output,
		&result.Error,
		&result.CompletedAt,
		&result.Success,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		r.logger.Error("Failed to get job result", "error", err)
		return nil, fmt.Errorf("failed to get job result: %w", err)
	}

	return &result, nil
}

// Additional methods required by the JobRepository interface would be implemented here:
// - GetPendingJobs
// - GetScheduledJobs
// - UpdateJobStatus

// GetPendingJobs retrieves pending jobs from PostgreSQL
func (r *JobRepository) GetPendingJobs(ctx context.Context, limit int) ([]*domain.Job, error) {
	// Prepare the query
	query := `
		SELECT id, type, status, payload, priority, created_at,
			   scheduled_at, started_at, completed_at, worker_id, retry_count
		FROM jobs
		WHERE status = $1
		ORDER BY priority DESC, created_at ASC
		LIMIT $2
	`

	// Execute the query
	rows, err := r.db.QueryContext(ctx, query, "PENDING", limit)
	if err != nil {
		r.logger.Error("Failed to get pending jobs", "error", err)
		return nil, fmt.Errorf("failed to get pending jobs: %w", err)
	}
	defer rows.Close()

	// Process the results
	jobs := make([]*domain.Job, 0)
	for rows.Next() {
		var job domain.Job
		var payloadJSON []byte
		var workerID sql.NullString
		var scheduledAt, startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.Status,
			&payloadJSON,
			&job.Priority,
			&job.CreatedAt,
			&scheduledAt,
			&startedAt,
			&completedAt,
			&workerID,
			&job.RetryCount,
		)

		if err != nil {
			r.logger.Error("Failed to scan job row", "error", err)
			return nil, fmt.Errorf("failed to scan job row: %w", err)
		}

		// Handle nullable fields
		if scheduledAt.Valid {
			job.ScheduledAt = &scheduledAt.Time
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}
		if workerID.Valid {
			job.WorkerID = &workerID.String
		}

		// Deserialize the payload
		if err := json.Unmarshal(payloadJSON, &job.Payload); err != nil {
			r.logger.Error("Failed to unmarshal job payload", "error", err)
			return nil, fmt.Errorf("failed to unmarshal job payload: %w", err)
		}

		jobs = append(jobs, &job)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating job rows", "error", err)
		return nil, fmt.Errorf("error iterating job rows: %w", err)
	}

	return jobs, nil
}

// GetScheduledJobs retrieves scheduled jobs from PostgreSQL
func (r *JobRepository) GetScheduledJobs(ctx context.Context, limit int) ([]*domain.Job, error) {
	// Prepare the query
	query := `
		SELECT id, type, status, payload, priority, created_at,
			   scheduled_at, started_at, completed_at, worker_id, retry_count
		FROM jobs
		WHERE status = $1
		ORDER BY priority DESC, scheduled_at ASC
		LIMIT $2
	`

	// Execute the query
	rows, err := r.db.QueryContext(ctx, query, "SCHEDULED", limit)
	if err != nil {
		r.logger.Error("Failed to get scheduled jobs", "error", err)
		return nil, fmt.Errorf("failed to get scheduled jobs: %w", err)
	}
	defer rows.Close()

	// Process the results
	jobs := make([]*domain.Job, 0)
	for rows.Next() {
		var job domain.Job
		var payloadJSON []byte
		var workerID sql.NullString
		var scheduledAt, startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.Status,
			&payloadJSON,
			&job.Priority,
			&job.CreatedAt,
			&scheduledAt,
			&startedAt,
			&completedAt,
			&workerID,
			&job.RetryCount,
		)

		if err != nil {
			r.logger.Error("Failed to scan job row", "error", err)
			return nil, fmt.Errorf("failed to scan job row: %w", err)
		}

		// Handle nullable fields
		if scheduledAt.Valid {
			job.ScheduledAt = &scheduledAt.Time
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}
		if workerID.Valid {
			job.WorkerID = &workerID.String
		}

		// Deserialize the payload
		if err := json.Unmarshal(payloadJSON, &job.Payload); err != nil {
			r.logger.Error("Failed to unmarshal job payload", "error", err)
			return nil, fmt.Errorf("failed to unmarshal job payload: %w", err)
		}

		jobs = append(jobs, &job)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating job rows", "error", err)
		return nil, fmt.Errorf("error iterating job rows: %w", err)
	}

	return jobs, nil
}

// UpdateJobStatus updates a job's status in PostgreSQL
func (r *JobRepository) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status domain.JobStatus) error {
	// Prepare the query
	query := `
		UPDATE jobs
		SET status = $1
		WHERE id = $2
	`

	// Execute the query
	result, err := r.db.ExecContext(ctx, query, status, jobID)
	if err != nil {
		r.logger.Error("Failed to update job status", "error", err)
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Check if the job was found
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.logger.Error("Error checking rows affected", "error", err)
		return fmt.Errorf("error checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found: %s", jobID)
	}

	return nil
}

func (r *JobRepository) AssignJobToWorker(ctx context.Context, jobID uuid.UUID, workerID string) error {
	// Start a transaction
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelRepeatableRead,
	})
	if err != nil {
		r.logger.Error("Failed to begin transaction", "error", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Will be ignored if the transaction is committed

	// First, check if the job is still in scheduled state
	var status string
	query := "SELECT status FROM jobs WHERE id = $1 FOR UPDATE"
	err = tx.QueryRowContext(ctx, query, jobID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("job not found: %s", jobID)
		}
		r.logger.Error("Failed to check job status", "jobId", jobID, "error", err)
		return fmt.Errorf("failed to check job status: %w", err)
	}

	// Verify the job is in the right state to be assigned
	if status != string(domain.JobStatusScheduled) {
		return fmt.Errorf("job is not in scheduled state: %s (current state: %s)", jobID, status)
	}

	// Update the job with worker assignment and change status to running
	now := time.Now()
	updateQuery := `
		UPDATE jobs
		SET status = $1, worker_id = $2, started_at = $3
		WHERE id = $4
	`

	result, err := tx.ExecContext(ctx, updateQuery,
		string(domain.JobStatusRunning),
		workerID,
		now,
		jobID)
	if err != nil {
		r.logger.Error("Failed to assign job to worker", "jobId", jobID, "workerId", workerID, "error", err)
		return fmt.Errorf("failed to assign job to worker: %w", err)
	}

	// Check that the update was successful
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.logger.Error("Error checking rows affected", "error", err)
		return fmt.Errorf("error checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("job not found or already assigned: %s", jobID)
	}

	// Log the job assignment
	r.logger.Info("Job assigned to worker", "jobId", jobID, "workerId", workerID)

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		r.logger.Error("Failed to commit transaction", "error", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
