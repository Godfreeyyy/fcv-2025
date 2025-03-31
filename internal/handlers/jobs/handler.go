package jobs

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/services/job"
	"gitlab.com/fcv-2025.net/internal/domain"
	"gitlab.com/fcv-2025.net/internal/tcp"
)

// JobHandler handles job API requests
type JobHandler struct {
	jobService job.IJobService
	logger     primary.Logger
	tcpServer  *tcp.TCPServer
}

var _ job.IJobService = &job.JobService{}

// NewJobHandler creates a new job handler
func NewJobHandler(jobService job.IJobService, logger primary.Logger) *JobHandler {
	return &JobHandler{
		jobService: jobService,
		logger:     logger,
	}
}

// RegisterRoutes registers the API routes for JobHandler
func (h *JobHandler) RegisterRoutes(router *mux.Router, tcpServer *tcp.TCPServer) {
	h.tcpServer = tcpServer
	router.HandleFunc("/api/jobs", h.CreateJob).Methods("POST")
	router.HandleFunc("/api/jobs/{jobId}", h.GetJob).Methods("GET")
	router.HandleFunc("/api/jobs/{jobId}/cancel", h.CancelJob).Methods("POST")
	router.HandleFunc("/api/jobs/types", h.GetJobTypes).Methods("GET")

	//debug---not availble in prod env
	router.HandleFunc("/api/jobs/debugs", h.AssignJobs).Methods("POST")
}

// CreateJob handles job creation requests
func (h *JobHandler) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	jobID, err := h.jobService.EnqueueJob(r.Context(), req.Type, req.Payload, req.Priority)
	if err != nil {
		h.logger.Error("Failed to create job", "error", err)
		http.Error(w, "Failed to create job", http.StatusInternalServerError)
		return
	}

	resp := CreateJobResponse{
		JobID: jobID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}

// AssignJobs handles job assignment requests
func (h *JobHandler) AssignJobs(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	//find jobs
	h.tcpServer.AssignJob(r.Context(), &domain.Job{})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
}

// GetJob handles job retrieval requests
func (h *JobHandler) GetJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobIDStr := vars["jobId"]

	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		h.logger.Error("Invalid job ID", "id", jobIDStr)
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	job, err := h.jobService.GetJob(r.Context(), jobID)
	if err != nil {
		h.logger.Error("Failed to get job", "error", err)
		http.Error(w, "Failed to get job", http.StatusInternalServerError)
		return
	}

	if job == nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// CancelJob handles job cancellation requests
func (h *JobHandler) CancelJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobIDStr := vars["jobId"]

	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		h.logger.Error("Invalid job ID", "id", jobIDStr)
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	if err := h.jobService.CancelJob(r.Context(), jobID); err != nil {
		h.logger.Error("Failed to cancel job", "error", err)
		http.Error(w, "Failed to cancel job", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetJobTypes handles job types retrieval requests
func (h *JobHandler) GetJobTypes(w http.ResponseWriter, r *http.Request) {
	jobTypes, err := h.jobService.GetJobTypes(r.Context())
	if err != nil {
		h.logger.Error("Failed to get job types", "error", err)
		http.Error(w, "Failed to get job types", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"types": jobTypes})
}
