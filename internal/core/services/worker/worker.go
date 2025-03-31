package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"gitlab.com/fcv-2025.net/internal/adapter/logging"
	"gitlab.com/fcv-2025.net/internal/domain"
)

type Worker struct {
	ID        string
	Type      string
	Capacity  int
	ServerURL string
	Logger    *logging.ZapLogger
	client    *http.Client
	jobs      chan domain.JobMessage
	results   chan domain.JobResultMessage
}

func NewWorker(workerType string, capacity int, serverURL string, logger *logging.ZapLogger) *Worker {
	return &Worker{
		ID:        uuid.New().String(),
		Type:      workerType,
		Capacity:  capacity,
		ServerURL: serverURL,
		Logger:    logger,
		client:    &http.Client{Timeout: 30 * time.Second},
		jobs:      make(chan domain.JobMessage, capacity),
		results:   make(chan domain.JobResultMessage, capacity),
	}
}

// Register registers the worker with the scheduler
func (w *Worker) Register() error {
	w.Logger.Info("Registering worker", "id", w.ID, "type", w.Type)

	registerURL := fmt.Sprintf("%s/api/workers", w.ServerURL)

	// Prepare request body
	body := map[string]interface{}{
		"id":       w.ID,
		"type":     w.Type,
		"capacity": w.Capacity,
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Send request
	resp, err := w.client.Post(registerURL, "application/json", bytes.NewBuffer(bodyJSON))
	if err != nil {
		return fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registration failed with status code: %d", resp.StatusCode)
	}

	w.Logger.Info("Worker registered successfully")
	return nil
}

// SendHeartbeats sends periodic heartbeats to the scheduler
func (w *Worker) SendHeartbeats(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a single heartbeat to the scheduler
func (w *Worker) sendHeartbeat() {
	w.Logger.Debug("Sending heartbeat")

	heartbeatURL := fmt.Sprintf("%s/api/workers/%s/heartbeat", w.ServerURL, w.ID)

	// Get current load (number of jobs in the channel)
	currentLoad := len(w.jobs)

	// Prepare request body
	body := map[string]interface{}{
		"load": currentLoad,
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		w.Logger.Error("Failed to marshal heartbeat request body", "error", err)
		return
	}

	// Send request
	resp, err := w.client.Post(heartbeatURL, "application/json", bytes.NewBuffer(bodyJSON))
	if err != nil {
		w.Logger.Error("Failed to send heartbeat", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.Logger.Error("Heartbeat failed", "statusCode", resp.StatusCode)
	}
}

// PollJobs polls the scheduler for available jobs
func (w *Worker) PollJobs(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Only poll if we have capacity
			if len(w.jobs) < w.Capacity {
				w.pollJobs()
			}
		}
	}
}

// pollJobs polls for available jobs
func (w *Worker) pollJobs() {
	pollURL := fmt.Sprintf("%s/api/workers/%s/jobs/poll", w.ServerURL, w.ID)

	// Send request
	resp, err := w.client.Get(pollURL)
	if err != nil {
		w.Logger.Error("Failed to poll for jobs", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		// No jobs available
		return
	}

	if resp.StatusCode != http.StatusOK {
		w.Logger.Error("Job poll failed", "statusCode", resp.StatusCode)
		return
	}

	// Parse response
	var job domain.JobMessage
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		w.Logger.Error("Failed to decode job message", "error", err)
		return
	}

	// Send job to processing channel
	w.jobs <- job
	w.Logger.Info("Received job", "jobId", job.JobID)
}

// ProcessJobs processes jobs from the jobs channel
func (w *Worker) ProcessJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-w.jobs:
			w.processJob(job)
		}
	}
}

// processJob processes a single job
func (w *Worker) processJob(job domain.JobMessage) {
	w.Logger.Info("Processing job", "jobId", job.JobID, "type", job.Type)

	// Simulate processing
	time.Sleep(2 * time.Second)

	// For demonstration, just succeed
	result := domain.JobResultMessage{
		JobID:     job.JobID,
		Success:   true,
		Output:    "Job processed successfully",
		Timestamp: time.Now(),
	}

	// Send result
	w.results <- result
}

// SendResults sends job results to the scheduler
func (w *Worker) SendResults(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case result := <-w.results:
			w.sendResult(result)
		}
	}
}

// sendResult sends a single job result
func (w *Worker) sendResult(result domain.JobResultMessage) {
	w.Logger.Info("Sending job result", "jobId", result.JobID, "success", result.Success)

	resultURL := fmt.Sprintf("%s/api/jobs/%s/result", w.ServerURL, result.JobID)

	// Prepare request body
	bodyJSON, err := json.Marshal(result)
	if err != nil {
		w.Logger.Error("Failed to marshal result request body", "error", err)
		return
	}

	// Send request
	resp, err := w.client.Post(resultURL, "application/json", bytes.NewBuffer(bodyJSON))
	if err != nil {
		w.Logger.Error("Failed to send result", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.Logger.Error("Result submission failed", "statusCode", resp.StatusCode)
	}
}
