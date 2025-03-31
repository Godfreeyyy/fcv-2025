package domain

import (
	"time"

	"github.com/google/uuid"
)

// Status represents the status of execution
type Status string

const (
	StatusSuccess             Status = "SUCCESS"
	StatusCompilationError    Status = "COMPILATION_ERROR"
	StatusRuntimeError        Status = "RUNTIME_ERROR"
	StatusTimeout             Status = "TIMEOUT"
	StatusMemoryLimitExceeded Status = "MEMORY_LIMIT_EXCEEDED"
)

// JobStatus represents the status of a job
type JobStatus string

const (
	JobStatusPending   JobStatus = "PENDING"
	JobStatusScheduled JobStatus = "SCHEDULED"
	JobStatusRunning   JobStatus = "RUNNING"
	JobStatusCompleted JobStatus = "COMPLETED"
	JobStatusFailed    JobStatus = "FAILED"
	JobStatusCancelled JobStatus = "CANCELLED"
)

// JobType represents the type of job
type JobType string

const (
	JobTypeCodeExecution JobType = "CODE_EXECUTION"
)

// Job represents a job to be scheduled and executed
type Job struct {
	ID           uuid.UUID              `db:"id"`
	Type         JobType                `db:"type"`
	Status       JobStatus              `db:"status"`
	Payload      map[string]interface{} `db:"payload"`
	Priority     int                    `db:"priority"`
	CreatedAt    time.Time              `db:"created_at"`
	ScheduledAt  *time.Time             `db:"scheduled_at"`
	StartedAt    *time.Time             `db:"started_at"`
	CompletedAt  *time.Time             `db:"completed_at"`
	WorkerID     *string                `db:"worker_id"`
	RetryCount   int                    `db:"retry_count"`
	RetryLimit   int                    `db:"retry_limit"`
	TimeoutSec   int                    `db:"timeout_seconds"`
	FileObjectID *string                `db:"file_object_id"`
	OwnerID      *uuid.UUID             `db:"owner_id"`
}

type JobTable struct {
	ID           string
	Type         string
	Status       string
	Payload      string
	Priority     string
	CreatedAt    string
	ScheduledAt  string
	StartedAt    string
	CompletedAt  string
	WorkerID     string
	RetryCount   string
	RetryLimit   string
	TimeoutSec   string
	FileObjectID string
	OwnerID      string
}

func GetJobTable() JobTable {
	return JobTable{
		ID:           "id",
		Type:         "type",
		Status:       "status",
		Payload:      "payload",
		Priority:     "priority",
		CreatedAt:    "created_at",
		ScheduledAt:  "scheduled_at",
		StartedAt:    "started_at",
		CompletedAt:  "completed_at",
		WorkerID:     "worker_id",
		RetryCount:   "retry_count",
		RetryLimit:   "retry_limit",
		TimeoutSec:   "timeout_seconds",
		FileObjectID: "file_object_id",
		OwnerID:      "owner_id",
	}
}

func (JobTable) TableName() string {
	return "jobs"
}

type JobResult struct {
	JobID       uuid.UUID
	Output      string
	Error       string
	CompletedAt time.Time
	Success     bool
}

// NewJob creates a new job
func NewJob(jobType JobType, payload map[string]interface{}, priority int) *Job {
	return &Job{
		ID:         uuid.New(),
		Type:       jobType,
		Status:     JobStatusPending,
		Payload:    payload,
		Priority:   priority,
		CreatedAt:  time.Now(),
		RetryCount: 0,
	}
}

// JobMessage represents a job message received from the scheduler
type JobMessage struct {
	JobID   uuid.UUID              `json:"jobId"`
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// JobResultMessage represents a job result message sent to the scheduler
type JobResultMessage struct {
	JobID     uuid.UUID `json:"jobId"`
	Success   bool      `json:"success"`
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// JobTypeConfig represents configuration for a job type
type JobTypeConfig struct {
	Type           string    // Type identifier (e.g., "java", "python", "cpp")
	Description    string    // Human-readable description
	TimeoutSeconds int       // Maximum execution time in seconds
	RetryLimit     int       // Maximum number of retries
	MemoryLimitMB  int       // Memory limit in MB
	CPULimit       float64   // CPU limit (1.0 = one core)
	Active         bool      // Whether this job type is active
	CreatedAt      time.Time // When the config was created
	UpdatedAt      time.Time // When the config was last updated
}
