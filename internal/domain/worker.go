package domain

import "time"

// WorkerInfo represents information about a worker
type WorkerInfo struct {
	ID            string    `db:"id"`
	Type          string    `db:"type"`
	Capacity      int       `db:"capacity"`
	CurrentLoad   int       `db:"current_load"`
	LastHeartbeat time.Time `db:"last_heartbeat"`
	IpAddress     string    `db:"ip_address"`
	OS            string    `db:"os"`
	Version       string    `db:"version"`
	IsActive      bool
}

type WorkerTable struct {
	ID            string
	Type          string
	Capacity      string
	CurrentLoad   string
	LastHeartbeat string
	IpAddress     string
	OS            string
	Version       string
}

func (t WorkerTable) Name() string {
	return "workers"
}

func GetWorkerTable() WorkerTable {
	return WorkerTable{
		ID:            "id",
		Type:          "type",
		Capacity:      "capacity",
		CurrentLoad:   "current_load",
		LastHeartbeat: "last_heartbeat",
		IpAddress:     "ip_address",
		OS:            "os",
		Version:       "version",
	}
}
