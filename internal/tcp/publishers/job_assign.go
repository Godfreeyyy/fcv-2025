package publishers

import (
	"context"
	"encoding/json"
	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/services/job"
	"gitlab.com/fcv-2025.net/internal/core/services/worker"
	"gitlab.com/fcv-2025.net/internal/domain"
	"gitlab.com/fcv-2025.net/internal/tcp/connectionmanager"
	"gitlab.com/fcv-2025.net/internal/tcp/defs"
	"net"
)

var _ primary.MessagePublisher = (*JobAssignPublisher)(nil)

type JobAssignPublisher struct {
	JobService    job.IJobService
	WorkerSvc     worker.IWorkerRegistrationService
	ConnectionMgr *connectionmanager.ConnectionManager
	Logger        primary.Logger
}

func NewJobAssignPublisher(
	jobService job.IJobService, workerSvc worker.IWorkerRegistrationService,
	connectionMgr *connectionmanager.ConnectionManager, logger primary.Logger,
) *JobAssignPublisher {
	return &JobAssignPublisher{
		JobService:    jobService,
		WorkerSvc:     workerSvc,
		ConnectionMgr: connectionMgr,
		Logger:        logger,
	}
}

func (j *JobAssignPublisher) PublishMessage(ctx context.Context, conn net.Conn, payload []byte, w domain.WorkerInfo) error {
	var jobData domain.Job
	if err := json.Unmarshal(payload, &jobData); err != nil {
		j.Logger.Error("Failed to parse worker registration", "error", err)
		connectionmanager.SendErrorMessage(conn, 1001, "Invalid registration data")
		return err
	}

	err := connectionmanager.SendMessage(conn, defs.MsgJobAssign, payload)
	if err != nil {
		j.Logger.Error("Failed to send job assignment", "error", err)
	}

	return nil
}
