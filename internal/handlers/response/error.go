package response

import (
	"encoding/json"
	"net/http"
)

type ErrorMessage struct {
	Message    string `json:"message"`
	StatusCode int    `json:"status_code"`
}

func WriteError(w http.ResponseWriter, err ErrorMessage) {
	w.WriteHeader(err.StatusCode)
	_ = json.NewEncoder(w).Encode(err)
}

func WriteSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}
