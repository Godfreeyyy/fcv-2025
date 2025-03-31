package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.com/fcv-2025.net/internal/global/logger"
	"net"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/services/worker"
	"gitlab.com/fcv-2025.net/internal/tcp/connectionmanager"
	"gitlab.com/fcv-2025.net/internal/tcp/defs"
)

var _ primary.MessageHandler = (*WorkerHeartbeatHandler)(nil)

// WorkerHeartbeatHandler handles worker heartbeat messages
type WorkerHeartbeatHandler struct {
	WorkerService worker.IWorkerRegistrationService
	Logger        primary.Logger
}

// HandleMessage implements the MessageHandler interface
func (h *WorkerHeartbeatHandler) HandleMessage(ctx context.Context, conn net.Conn, payload []byte, workerID *string) error {

	if *workerID == "" {
		connectionmanager.SendErrorMessage(conn, 1003, "Worker not registered")
		return fmt.Errorf("worker not registered")
	}

	var heartbeatData defs.WorkerHeartbeatData

	if err := json.Unmarshal(payload, &heartbeatData); err != nil {
		h.Logger.Error("Failed to parse worker heartbeat", "error", err)
		connectionmanager.SendErrorMessage(conn, 1004, "Invalid heartbeat data")
		return err
	}
	logger.Info("receving heartbeat....from", heartbeatData.WorkerID)

	// Validate worker ID
	if heartbeatData.WorkerID != *workerID {
		h.Logger.Error("Worker ID mismatch in heartbeat", "expected", *workerID, "actual", heartbeatData.WorkerID)
		connectionmanager.SendErrorMessage(conn, 1005, "Worker ID mismatch")
		return fmt.Errorf("worker ID mismatch")
	}

	// Update worker heartbeat
	if err := h.WorkerService.Heartbeat(ctx, *workerID, heartbeatData.Load); err != nil {
		h.Logger.Error("Failed to update worker heartbeat", "error", err)
		connectionmanager.SendErrorMessage(conn, 1006, "Failed to update heartbeat")
		return err
	}

	h.Logger.Info("Worker heartbeat received", "workerID", *workerID, "load", heartbeatData.Load)
	return nil
}
