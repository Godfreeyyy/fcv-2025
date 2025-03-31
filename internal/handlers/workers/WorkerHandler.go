package workers

import (
	"net/http"

	"github.com/gorilla/mux"

	"gitlab.com/fcv-2025.net/internal/core/services/worker"
	"gitlab.com/fcv-2025.net/internal/handlers"
)

type ApiHandler struct {
	WorkerService worker.IWorkerRegistrationService
}

func NewHandler(WorkerService worker.IWorkerRegistrationService) *ApiHandler {
	return &ApiHandler{
		WorkerService: WorkerService,
	}
}

func (api *ApiHandler) Register(r *mux.Router) {
	r.HandleFunc("/api/workers", api.GetWorkers).Methods("GET")
	r.HandleFunc("/api/workers/types", api.GetWorkerTypes).Methods("GET")
}

func (api *ApiHandler) GetWorkers(w http.ResponseWriter, r *http.Request) {
	workers, err := api.WorkerService.GetAllWorkers(r.Context())
	if err != nil {
		http.Error(w, "Failed to get workers", http.StatusInternalServerError)
		return
	}

	handlers.ResponseWithJson(w, http.StatusOK, workers)
}

func (api *ApiHandler) GetWorkerTypes(w http.ResponseWriter, r *http.Request) {
	types, err := api.WorkerService.GetWorkerTypes(r.Context())
	if err != nil {
		http.Error(w, "Failed to get worker types", http.StatusInternalServerError)
		return
	}

	handlers.ResponseWithJson(w, http.StatusOK, types)
}
