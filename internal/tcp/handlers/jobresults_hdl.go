package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/services/schedule"
	"gitlab.com/fcv-2025.net/internal/tcp/connectionmanager"
	"gitlab.com/fcv-2025.net/internal/tcp/defs"
)

var _ primary.MessageHandler = (*JobResultHandler)(nil)

// JobResultHandler handles job result messages
type JobResultHandler struct {
	SchedulerService schedule.ISchedulerService
	Logger           primary.Logger
}

// HandleMessage implements the MessageHandler interface
func (h *JobResultHandler) HandleMessage(ctx context.Context, conn net.Conn, payload []byte, workerID *string) error {
	if *workerID == "" {
		connectionmanager.SendErrorMessage(conn, 1013, "Worker not registered")
		return fmt.Errorf("worker not registered")
	}

	var resultData defs.JobResultData

	if err := json.Unmarshal(payload, &resultData); err != nil {
		h.Logger.Error("Failed to parse job result", "error", err)
		connectionmanager.SendErrorMessage(conn, 1014, "Invalid job result data")
		return err
	}

	// Handle job result
	if err := h.SchedulerService.HandleJobResult(
		ctx,
		resultData.JobID,
		resultData.Success,
		resultData.Output,
		resultData.Error,
	); err != nil {
		h.Logger.Error("Failed to handle job result", "error", err)
		connectionmanager.SendErrorMessage(conn, 1015, "Failed to handle job result")
		return err
	}

	h.Logger.Info("Job result received", "jobId", resultData.JobID, "workerID", *workerID, "success", resultData.Success)
	return nil
}
