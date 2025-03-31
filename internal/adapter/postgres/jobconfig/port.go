package jobconfig

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/domain"
)

// JobTypeConfigRepository implements the JobTypeConfigRepository interface with PostgreSQL
type JobTypeConfigRepository struct {
	db     *sqlx.DB
	logger primary.Logger
}

// NewJobTypeConfigRepository creates a new PostgreSQL job type config repository
func NewJobTypeConfigRepository(db *sqlx.DB, logger primary.Logger) *JobTypeConfigRepository {
	return &JobTypeConfigRepository{
		db:     db,
		logger: logger,
	}
}

// GetJobTypeConfig retrieves configuration for a specific job type
func (r *JobTypeConfigRepository) GetJobTypeConfig(ctx context.Context, jobType string) (*domain.JobTypeConfig, error) {
	query := `
		SELECT 
			type, description, timeout_seconds, retry_limit, 
			memory_limit_mb, cpu_limit, active, created_at, updated_at
		FROM job_type_config
		WHERE type = $1
	`

	var config domain.JobTypeConfig
	err := r.db.QueryRowContext(ctx, query, jobType).Scan(
		&config.Type,
		&config.Description,
		&config.TimeoutSeconds,
		&config.RetryLimit,
		&config.MemoryLimitMB,
		&config.CPULimit,
		&config.Active,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// If not found, return default configuration
			return r.createDefaultConfig(jobType), nil
		}
		r.logger.Error("Failed to get job type config", "type", jobType, "error", err)
		return nil, fmt.Errorf("failed to get job type config: %w", err)
	}

	return &config, nil
}

// GetAllJobTypeConfigs retrieves all job type configurations, including inactive ones
func (r *JobTypeConfigRepository) GetAllJobTypeConfigs(ctx context.Context) ([]*domain.JobTypeConfig, error) {
	query := `
		SELECT 
			type, description, timeout_seconds, retry_limit, 
			memory_limit_mb, cpu_limit, active, created_at, updated_at
		FROM job_type_config
		ORDER BY type
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		r.logger.Error("Failed to get all job type configs", "error", err)
		return nil, fmt.Errorf("failed to get all job type configs: %w", err)
	}
	defer rows.Close()

	var configs []*domain.JobTypeConfig
	for rows.Next() {
		var config domain.JobTypeConfig
		if err := rows.Scan(
			&config.Type,
			&config.Description,
			&config.TimeoutSeconds,
			&config.RetryLimit,
			&config.MemoryLimitMB,
			&config.CPULimit,
			&config.Active,
			&config.CreatedAt,
			&config.UpdatedAt,
		); err != nil {
			r.logger.Error("Failed to scan job type config", "error", err)
			return nil, fmt.Errorf("failed to scan job type config: %w", err)
		}
		configs = append(configs, &config)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating job type configs", "error", err)
		return nil, fmt.Errorf("error iterating job type configs: %w", err)
	}

	// If no configured job types, return defaults
	if len(configs) == 0 {
		configs = []*domain.JobTypeConfig{
			r.createDefaultConfig("java"),
			r.createDefaultConfig("python"),
			r.createDefaultConfig("cpp"),
		}
	}

	return configs, nil
}

// GetActiveJobTypes retrieves all active job types
func (r *JobTypeConfigRepository) GetActiveJobTypes(ctx context.Context) ([]string, error) {
	query := `
		SELECT type
		FROM job_type_config
		WHERE active = true
		ORDER BY type
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		r.logger.Error("Failed to get active job types", "error", err)
		return nil, fmt.Errorf("failed to get active job types: %w", err)
	}
	defer rows.Close()

	var jobTypes []string
	for rows.Next() {
		var jobType string
		if err := rows.Scan(&jobType); err != nil {
			r.logger.Error("Failed to scan job type", "error", err)
			return nil, fmt.Errorf("failed to scan job type: %w", err)
		}
		jobTypes = append(jobTypes, jobType)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating job types", "error", err)
		return nil, fmt.Errorf("error iterating job types: %w", err)
	}

	// If no configured job types, return defaults
	if len(jobTypes) == 0 {
		return []string{"java", "python", "cpp"}, nil
	}

	return jobTypes, nil
}

// SaveJobTypeConfig saves a job type configuration
func (r *JobTypeConfigRepository) SaveJobTypeConfig(ctx context.Context, config *domain.JobTypeConfig) error {
	// Validation
	if config.Type == "" {
		return fmt.Errorf("job type cannot be empty")
	}
	if config.TimeoutSeconds <= 0 {
		config.TimeoutSeconds = 30 // Default timeout
	}
	if config.RetryLimit < 0 {
		config.RetryLimit = 0 // Minimum retry limit
	}
	if config.MemoryLimitMB <= 0 {
		config.MemoryLimitMB = 256 // Default memory limit
	}
	if config.CPULimit <= 0 {
		config.CPULimit = 1.0 // Default CPU limit
	}

	query := `
		INSERT INTO job_type_config (
			type, description, timeout_seconds, retry_limit, 
			memory_limit_mb, cpu_limit, active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (type) DO UPDATE SET
			description = EXCLUDED.description,
			timeout_seconds = EXCLUDED.timeout_seconds,
			retry_limit = EXCLUDED.retry_limit,
			memory_limit_mb = EXCLUDED.memory_limit_mb,
			cpu_limit = EXCLUDED.cpu_limit,
			active = EXCLUDED.active,
			updated_at = EXCLUDED.updated_at
	`

	// Set timestamps
	now := time.Now()
	if config.CreatedAt.IsZero() {
		config.CreatedAt = now
	}
	config.UpdatedAt = now

	// Execute query
	_, err := r.db.ExecContext(ctx, query,
		config.Type,
		config.Description,
		config.TimeoutSeconds,
		config.RetryLimit,
		config.MemoryLimitMB,
		config.CPULimit,
		config.Active,
		config.CreatedAt,
		config.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to save job type config", "type", config.Type, "error", err)
		return fmt.Errorf("failed to save job type config: %w", err)
	}

	r.logger.Info("Saved job type config", "type", config.Type, "active", config.Active)
	return nil
}

// DeactivateJobType deactivates a job type
func (r *JobTypeConfigRepository) DeactivateJobType(ctx context.Context, jobType string) error {
	return r.setJobTypeActive(ctx, jobType, false)
}

// ActivateJobType activates a job type
func (r *JobTypeConfigRepository) ActivateJobType(ctx context.Context, jobType string) error {
	return r.setJobTypeActive(ctx, jobType, true)
}

// setJobTypeActive sets the active status of a job type
func (r *JobTypeConfigRepository) setJobTypeActive(ctx context.Context, jobType string, active bool) error {
	// Check if job type exists
	config, err := r.GetJobTypeConfig(ctx, jobType)
	if err != nil {
		return err
	}

	// Update active status
	config.Active = active
	config.UpdatedAt = time.Now()

	// Save the updated config
	return r.SaveJobTypeConfig(ctx, config)
}

// DeleteJobTypeConfig permanently deletes a job type configuration
func (r *JobTypeConfigRepository) DeleteJobTypeConfig(ctx context.Context, jobType string) error {
	query := `
		DELETE FROM job_type_config
		WHERE type = $1
	`

	result, err := r.db.ExecContext(ctx, query, jobType)
	if err != nil {
		r.logger.Error("Failed to delete job type config", "type", jobType, "error", err)
		return fmt.Errorf("failed to delete job type config: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.logger.Error("Error checking rows affected", "error", err)
		return fmt.Errorf("error checking rows affected: %w", err)
	}

	if rowsAffected == 0 {
		r.logger.Warn("Job type not found for deletion", "type", jobType)
		return nil // Not found, but we don't want to return an error for idempotency
	}

	r.logger.Info("Deleted job type config", "type", jobType)
	return nil
}

// createDefaultConfig creates a default configuration for a job type
func (r *JobTypeConfigRepository) createDefaultConfig(jobType string) *domain.JobTypeConfig {
	now := time.Now()

	// Set default values based on job type
	memoryLimit := 256
	cpuLimit := 1.0
	timeout := 30

	switch jobType {
	case "java":
		memoryLimit = 512
		timeout = 45
	case "python":
		memoryLimit = 256
		timeout = 30
	case "cpp":
		memoryLimit = 256
		timeout = 30
	case "csharp":
		memoryLimit = 512
		timeout = 45
	case "javascript":
		memoryLimit = 256
		timeout = 30
	}

	return &domain.JobTypeConfig{
		Type:           jobType,
		Description:    fmt.Sprintf("%s code execution", jobType),
		TimeoutSeconds: timeout,
		RetryLimit:     3,
		MemoryLimitMB:  memoryLimit,
		CPULimit:       cpuLimit,
		Active:         true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// Ensure JobTypeConfig table exists
func (r *JobTypeConfigRepository) EnsureTableExists(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS job_type_config (
			type VARCHAR(50) PRIMARY KEY,
			description TEXT,
			timeout_seconds INTEGER NOT NULL DEFAULT 30,
			retry_limit INTEGER NOT NULL DEFAULT 3,
			memory_limit_mb INTEGER NOT NULL DEFAULT 256,
			cpu_limit REAL NOT NULL DEFAULT 1.0,
			active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL
		)
	`

	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		r.logger.Error("Failed to create job_type_config table", "error", err)
		return fmt.Errorf("failed to create job_type_config table: %w", err)
	}

	// Insert default configurations if table is empty
	count := 0
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM job_type_config").Scan(&count)
	if err != nil {
		r.logger.Error("Failed to count job type configs", "error", err)
		return fmt.Errorf("failed to count job type configs: %w", err)
	}

	if count == 0 {
		defaultTypes := []string{"java", "python", "cpp", "csharp", "javascript"}
		for _, jobType := range defaultTypes {
			config := r.createDefaultConfig(jobType)
			if err := r.SaveJobTypeConfig(ctx, config); err != nil {
				r.logger.Error("Failed to save default job type config", "type", jobType, "error", err)
				// Continue with other types even if one fails
				continue
			}
		}
	}

	return nil
}
