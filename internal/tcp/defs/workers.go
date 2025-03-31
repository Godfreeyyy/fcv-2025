package defs

// Protocol data structures
type (
	// WorkerRegistrationData represents the data sent during worker registration
	WorkerRegistrationData struct {
		WorkerID string `json:"worker_id"`
		Type     string `json:"type"`
		Capacity int    `json:"capacity"`
		Ip       string `json:"ip_address"`
	}

	// WorkerHeartbeatData represents the data sent during worker heartbeat
	WorkerHeartbeatData struct {
		WorkerID  string `json:"worker_id"`
		Load      int    `json:"load"`
		Timestamp int64  `json:"timestamp"`
	}
)
