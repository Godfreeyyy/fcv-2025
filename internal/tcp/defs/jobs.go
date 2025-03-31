package defs

import (
	"github.com/google/uuid"
)

// Protocol data structures
type (

	// JobRequestData represents the data sent during job request
	JobRequestData struct {
		WorkerID string `json:"worker_id"`
	}

	// JobAssignData represents the data sent during job assignment
	JobAssignData struct {
		JobID   uuid.UUID              `json:"job_id"`
		Type    string                 `json:"type"`
		Payload map[string]interface{} `json:"payload"`
	}

	// JobResultData represents the data sent with job results
	JobResultData struct {
		JobID            uuid.UUID `json:"job_id"`
		Success          bool      `json:"success"`
		Output           string    `json:"output"`
		Error            string    `json:"error,omitempty"`
		ExecutionTimeMs  int64     `json:"execution_time_ms"`
		MemoryUsageBytes int64     `json:"memory_usage_bytes"`
	}

	// JobAvailableData represents data sent to notify workers of available jobs
	JobAvailableData struct {
		JobTypes []string `json:"job_types"`
	}
)
