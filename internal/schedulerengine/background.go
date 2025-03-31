package schedulerengine

import (
	"context"
	"sync"
	"time"

	"gitlab.com/fcv-2025.net/internal/config"
	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/ports/secondary"
	"gitlab.com/fcv-2025.net/internal/domain"
	"gitlab.com/fcv-2025.net/internal/tcp"
)

type SchedulerEngine struct {
	SchedulerCfg *config.ScheduleSvcCfg
	jobRepo      secondary.JobRepository
	logger       primary.Logger
	tcpServer    *tcp.TCPServer
}

func NewSchedulerEngine(
	SchedulerCfg *config.ScheduleSvcCfg,
	jobRepo secondary.JobRepository,
	logger primary.Logger,
	tcpServer *tcp.TCPServer,
) *SchedulerEngine {
	return &SchedulerEngine{
		SchedulerCfg: SchedulerCfg,
		jobRepo:      jobRepo,
		logger:       logger,
		tcpServer:    tcpServer,
	}
}

func (s *SchedulerEngine) StartJobScheduleEngine(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)
	// In a real system, you would start a goroutine to run the scheduling engine
	// For example, a goroutine that runs the ScheduleJobs method every minute
	ticker := time.NewTicker(s.SchedulerCfg.SchedulePendingJobInterval)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				//s.schedulePendingJobs(ctx)
				s.logger.Info("Schedule pending jobs")
			}
		}
	}()
	tickerSchedule := time.NewTicker(s.SchedulerCfg.ScheduleScheduledJobInterval)

	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tickerSchedule.C:
				s.AssignScheduledJobs(ctx)
			}
		}
	}()
}

func (s *SchedulerEngine) schedulePendingJobs(ctx context.Context) {
	// In a real system, you would start a goroutine to run the scheduling engine
	// For example, a goroutine that runs the ScheduleJobs method every minute
	pendingJobs, err := s.jobRepo.GetPendingJobs(ctx, 100)
	if err != nil {
		s.logger.Error("Failed to get pending jobs", "error", err)
		return
	}

	// Schedule each job
	for _, job := range pendingJobs {
		now := time.Now()
		job.Status = domain.JobStatusScheduled
		job.ScheduledAt = &now

		if err := s.jobRepo.SaveJob(ctx, job); err != nil {
			s.logger.Error("Failed to update job status", "jobId", job.ID, "error", err)
			continue
		}

		s.logger.Info("Job scheduled", "jobId", job.ID)
	}
}

func (s *SchedulerEngine) AssignScheduledJobs(ctx context.Context) {
	var workerSize int = 2
	// Get scheduled jobs
	scheduledJobs, err := s.jobRepo.GetScheduledJobs(ctx, 100)
	if err != nil {
		s.logger.Error("Failed to get scheduled jobs", "error", err)
		return
	}
	if len(scheduledJobs) == 0 {
		s.logger.Info("No scheduled jobs found")
		return
	}
	jobCh := make(chan *domain.Job, len(scheduledJobs))
	errCh := make(chan error, len(scheduledJobs))
	go func() {
		defer close(jobCh)
		for _, job := range scheduledJobs {
			jobCh <- job
		}
	}()
	var wg sync.WaitGroup
	wg.Add(workerSize)
	for i := 0; i < workerSize; i++ {
		go func() {
			defer wg.Done()
			for job := range jobCh {
				time.Sleep(time.Second * 4)
				s.logger.Info("Job assigned", "jobId", job.ID)
				var err error = s.tcpServer.SendJobAssignments(ctx, job)
				if err != nil {
					errCh <- err
				}
			}
		}()
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for err := range errCh {
			s.logger.Error("Failed to assign job", "error", err)
		}
		s.logger.Info("All jobs assigned")
	}()
	wg.Wait()
	close(errCh)
	<-done
	s.logger.Info("error handling done")
	s.logger.Info("Found scheduled jobs", "count", len(scheduledJobs))
}
