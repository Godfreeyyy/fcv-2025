package handlers

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/services/worker"
	"gitlab.com/fcv-2025.net/internal/domain"
	"gitlab.com/fcv-2025.net/internal/tcp/connectionmanager"
)

type // WorkerRegistrationData represents the data sent during worker registration
WorkerRegistrationData struct {
	WorkerID string `json:"worker_id"`
	Type     string `json:"type"`
	Capacity int    `json:"capacity"`
	Ip       string `json:"ip_address"`
}

// Implementation of message handlers
// Each handler deals with one specific message type

var _ primary.MessageHandler = (*WorkerRegistrationHandler)(nil)

// WorkerRegistrationHandler handles worker registration messages
type WorkerRegistrationHandler struct {
	WorkerService worker.IWorkerRegistrationService
	ConnectionMgr *connectionmanager.ConnectionManager
	Logger        primary.Logger
}

// HandleMessage implements the MessageHandler interface
func (h *WorkerRegistrationHandler) HandleMessage(ctx context.Context, conn net.Conn, payload []byte, workerID *string) error {
	h.Logger.Info("Worker registration received")
	var registerData WorkerRegistrationData
	if err := json.Unmarshal(payload, &registerData); err != nil {
		h.Logger.Error("Failed to parse worker registration", "error", err)
		connectionmanager.SendErrorMessage(conn, 1001, "Invalid registration data")
		return err
	}

	h.Logger.Info("Worker registration received", "workerID", registerData.WorkerID)

	// Store worker ID and connection
	*workerID = registerData.WorkerID
	h.ConnectionMgr.RegisterWorker(registerData.WorkerID, registerData.Type, conn)

	// Register worker with service
	workerInfo := &domain.WorkerInfo{
		ID:            registerData.WorkerID,
		Type:          registerData.Type,
		Capacity:      registerData.Capacity,
		CurrentLoad:   0,
		LastHeartbeat: time.Now(),
		IpAddress:     registerData.Ip,
	}

	if err := h.WorkerService.RegisterWorker(ctx, workerInfo); err != nil {
		h.Logger.Error("Failed to register worker", "error", err)
		connectionmanager.SendErrorMessage(conn, 1002, "Failed to register worker")
		return err
	}

	h.Logger.Info(
		"Worker registered",
		"workerID", registerData.WorkerID,
		"type", registerData.Type,
		"ip address", registerData.Ip,
	)
	return nil
}
