package jobs

import "github.com/google/uuid"

// CreateJobRequest represents a request to create a job
type CreateJobRequest struct {
	Type     string                 `json:"type"`
	Payload  map[string]interface{} `json:"payload"`
	Priority int                    `json:"priority"`
}

// CreateJobResponse represents a response to a create job request
type CreateJobResponse struct {
	JobID uuid.UUID `json:"jobId"`
}
