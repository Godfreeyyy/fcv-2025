package config

import (
	"os"
	"strconv"
	"time"
)

type ScheduleSvcCfg struct {
	SchedulePendingJobInterval   time.Duration
	ScheduleScheduledJobInterval time.Duration
}

func NewScheduleSvcCfg() *ScheduleSvcCfg {
	schedulePendingJobIntervalSec := os.Getenv("SCHEDULE_PENDING_JOBS_INTERVAL_SEC")
	scheduleScheduledJobIntervalSec := os.Getenv("SCHEDULE_SCHEDULED_JOBS_INTERVAL_SEC")
	varInt, err := strconv.Atoi(schedulePendingJobIntervalSec)
	if err != nil {
		varInt = 60
	}
	varInt2, err := strconv.Atoi(scheduleScheduledJobIntervalSec)
	if err != nil {
		varInt2 = 60
	}
	return &ScheduleSvcCfg{
		SchedulePendingJobInterval:   time.Duration(varInt) * time.Second,
		ScheduleScheduledJobInterval: time.Duration(varInt2) * time.Second,
	}
}
