package connectionmanager

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/tcp/defs"
)

// ConnectionManager handles TCP connections and worker management
type ConnectionManager struct {
	Connections     map[string]net.Conn
	WorkerTypes     map[string]string // workerId -> workerType
	ConnMutex       sync.RWMutex
	workerTypeMutex sync.RWMutex
	Logger          primary.Logger
}

type // ErrorData represents data sent with error responses
	ErrorData struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(logger primary.Logger) *ConnectionManager {
	return &ConnectionManager{
		Connections: make(map[string]net.Conn),
		WorkerTypes: make(map[string]string),
		Logger:      logger,
	}
}

// RegisterWorker registers a worker connection
func (cm *ConnectionManager) RegisterWorker(workerID string, workerType string, conn net.Conn) {
	cm.ConnMutex.Lock()
	cm.Connections[workerID] = conn
	cm.ConnMutex.Unlock()

	cm.workerTypeMutex.Lock()
	cm.WorkerTypes[workerID] = workerType
	cm.workerTypeMutex.Unlock()
}

// RemoveWorker removes a worker when its connection is closed
func (cm *ConnectionManager) RemoveWorker(workerID string) {
	cm.ConnMutex.Lock()
	delete(cm.Connections, workerID)
	cm.ConnMutex.Unlock()

	cm.workerTypeMutex.Lock()
	delete(cm.WorkerTypes, workerID)
	cm.workerTypeMutex.Unlock()
}

// GetWorkerType returns the type for a specific worker
func (cm *ConnectionManager) GetWorkerType(workerID string) (string, bool) {
	cm.workerTypeMutex.RLock()
	defer cm.workerTypeMutex.RUnlock()

	workerType, exists := cm.WorkerTypes[workerID]
	return workerType, exists
}

// GetWorkersOfType returns workers of a specific type
func (cm *ConnectionManager) GetWorkersOfType(workerType string) []string {
	cm.workerTypeMutex.RLock()
	defer cm.workerTypeMutex.RUnlock()

	matchingWorkers := make([]string, 0)
	for workerID, wType := range cm.WorkerTypes {
		if wType == workerType {
			matchingWorkers = append(matchingWorkers, workerID)
		}
	}

	return matchingWorkers
}

// GetConnection returns the connection for a specific worker
func (cm *ConnectionManager) GetConnection(workerID string) (net.Conn, bool) {
	cm.ConnMutex.RLock()
	defer cm.ConnMutex.RUnlock()

	conn, exists := cm.Connections[workerID]
	return conn, exists
}

// SendErrorMessage sends an error message to a worker
func SendErrorMessage(conn net.Conn, code int, message string) {
	errorData := ErrorData{
		Code:    code,
		Message: message,
	}

	errorBytes, err := json.Marshal(errorData)
	if err != nil {
		// Can't do much if marshaling fails
		return
	}

	// Ignore errors here as the connection might be closing
	_ = SendMessage(conn, defs.MsgError, errorBytes)
}

// SendMessage sends a message to a worker
func SendMessage(conn net.Conn, msgType byte, payload []byte) error {
	// Prepare header
	header := make([]byte, 8)
	binary.BigEndian.PutUint16(header[0:2], defs.MagicNumber)
	header[2] = msgType
	header[3] = 0 // Reserved
	binary.BigEndian.PutUint32(header[4:8], uint32(len(payload)))

	// Send header
	if _, err := conn.Write(header); err != nil {
		return fmt.Errorf("failed to write message header: %w", err)
	}

	// Send payload (if any)
	if len(payload) > 0 {
		if _, err := conn.Write(payload); err != nil {
			return fmt.Errorf("failed to write message payload: %w", err)
		}
	}

	return nil
}
