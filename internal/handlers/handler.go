package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/services/worker"
	"gitlab.com/fcv-2025.net/internal/domain"
)

// WorkerHandler handles worker API requests
type WorkerHandler struct {
	workerService worker.IWorkerRegistrationService
	logger        primary.Logger
}

// NewWorkerHandler creates a new worker handler
func NewWorkerHandler(workerService worker.IWorkerRegistrationService, logger primary.Logger) *WorkerHandler {
	return &WorkerHandler{
		workerService: workerService,
		logger:        logger,
	}
}

// RegisterRoutes registers the API routes for WorkerHandler
func (h *WorkerHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/workers", h.RegisterWorker).Methods("POST")
	router.HandleFunc("/api/workers/{workerId}/heartbeat", h.Heartbeat).Methods("POST")
	router.HandleFunc("/api/workers/types", h.GetWorkerTypes).Methods("GET")
	router.HandleFunc("/api/workers", h.GetWorkers).Methods("GET")
}

// RegisterWorkerRequest represents a request to register a worker
type RegisterWorkerRequest struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Capacity int    `json:"capacity"`
}

// RegisterWorker handles worker registration requests
func (h *WorkerHandler) RegisterWorker(w http.ResponseWriter, r *http.Request) {
	var req RegisterWorkerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	workerInfo := &domain.WorkerInfo{
		ID:            req.ID,
		Type:          req.Type,
		Capacity:      req.Capacity,
		CurrentLoad:   0,
		LastHeartbeat: time.Now(),
	}

	if err := h.workerService.RegisterWorker(r.Context(), workerInfo); err != nil {
		h.logger.Error("Failed to register worker", "error", err)
		http.Error(w, "Failed to register worker", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// HeartbeatRequest represents a worker heartbeat request
type HeartbeatRequest struct {
	Load int `json:"load"`
}

// Heartbeat handles worker heartbeat requests
func (h *WorkerHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workerID := vars["workerId"]

	var req HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode request", "error", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.workerService.Heartbeat(r.Context(), workerID, req.Load); err != nil {
		h.logger.Error("Failed to process heartbeat", "error", err)
		http.Error(w, "Failed to process heartbeat", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetWorkerTypes handles worker types retrieval requests
func (h *WorkerHandler) GetWorkerTypes(w http.ResponseWriter, r *http.Request) {
	workerTypes, err := h.workerService.GetWorkerTypes(r.Context())
	if err != nil {
		h.logger.Error("Failed to get worker types", "error", err)
		http.Error(w, "Failed to get worker types", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]string{"types": workerTypes})
}

// GetWorkers handles workers retrieval requests
func (h *WorkerHandler) GetWorkers(w http.ResponseWriter, r *http.Request) {
	workerType := r.URL.Query().Get("type")

	var workers []*domain.WorkerInfo
	var err error

	if workerType != "" {
		workers, err = h.workerService.GetAvailableWorkers(r.Context(), workerType)
	} else {
		workers, err = h.workerService.GetAllWorkers(r.Context())
	}

	if err != nil {
		h.logger.Error("Failed to get workers", "error", err)
		http.Error(w, "Failed to get workers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]*domain.WorkerInfo{"workers": workers})
}

func ResponseWithJson(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

func ResponseError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
