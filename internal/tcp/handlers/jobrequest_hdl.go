package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/services/schedule"
	"gitlab.com/fcv-2025.net/internal/domain"
	"gitlab.com/fcv-2025.net/internal/tcp/connectionmanager"
	"gitlab.com/fcv-2025.net/internal/tcp/defs"
)

// JobRequestHandler handles job request messages
type JobRequestHandler struct {
	SchedulerService schedule.ISchedulerService
	ConnectionMgr    *connectionmanager.ConnectionManager
	Logger           primary.Logger
}

func NewTCPJobRequestHandler(
	schedulerService schedule.ISchedulerService,
	connectionMgr *connectionmanager.ConnectionManager,
	logger primary.Logger,
) *JobRequestHandler {
	return &JobRequestHandler{
		SchedulerService: schedulerService,
		ConnectionMgr:    connectionMgr,
		Logger:           logger,
	}
}

// HandleMessage implements the MessageHandler interface
func (h *JobRequestHandler) HandleMessage(ctx context.Context, conn net.Conn, payload []byte, workerID *string) error {
	if *workerID == "" {
		connectionmanager.SendErrorMessage(conn, 1007, "Worker not registered")
		return fmt.Errorf("worker not registered")
	}

	var requestData *domain.Job

	if err := json.Unmarshal(payload, &requestData); err != nil {
		h.Logger.Error("Failed to parse job request", "error", err)
		connectionmanager.SendErrorMessage(conn, 1008, "Invalid job request data")
		return err
	}

	//// Validate worker ID
	//if requestData.WorkerID != *workerID {
	//	h.Logger.Error("Worker ID mismatch in job request", "expected", *workerID, "actual", requestData.WorkerID)
	//	connectionmanager.SendErrorMessage(conn, 1009, "Worker ID mismatch")
	//	return fmt.Errorf("worker ID mismatch")
	//}

	// Get worker type
	_, exists := h.ConnectionMgr.GetWorkerType(*workerID)

	if !exists {
		h.Logger.Error("Worker type not found", "workerID", *workerID)
		connectionmanager.SendErrorMessage(conn, 1010, "Worker type not found")
		return fmt.Errorf("worker type not found")
	}

	//// Get next job for worker
	//jobs, err := h.SchedulerService.GetJobsForWorker(ctx, *workerID, 1)
	//if err != nil {
	//	h.Logger.Error("Failed to get jobs for worker", "error", err)
	//	connectionmanager.SendErrorMessage(conn, 1011, "Failed to get jobs")
	//	return err
	//}
	//
	//if len(jobs) == 0 {
	//	// No jobs available, send empty response
	//	if err := connectionmanager.SendMessage(conn, defs.MsgJobAssign, []byte("{}")); err != nil {
	//		return err
	//	}
	//	return nil
	//}
	//
	//// Assign job to worker
	//job := jobs[0]
	//assignData := defs.JobAssignData{
	//	JobID:   job.ID,
	//	Type:    string(job.Type),
	//	Payload: job.Payload,
	//}

	// Marshal job data
	jobDataBytes, err := json.Marshal(requestData)
	if err != nil {
		h.Logger.Error("Failed to marshal job data", "error", err)
		connectionmanager.SendErrorMessage(conn, 1012, "Failed to prepare job data")
		return err
	}

	// Send job assignment
	if err := connectionmanager.SendMessage(conn, defs.MsgJobAssign, jobDataBytes); err != nil {
		h.Logger.Error("Failed to send job assignment", "error", err)
		return err
	}

	h.Logger.Info("Job assigned to worker", "jobId", requestData.ID, "workerID", *workerID)
	return nil
}
