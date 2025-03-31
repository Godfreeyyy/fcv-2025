// package internal
package tcp

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"gitlab.com/fcv-2025.net/internal/core/services/job"
	"gitlab.com/fcv-2025.net/internal/tcp/publishers"
	"io"
	"net"
	"time"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/services/schedule"
	"gitlab.com/fcv-2025.net/internal/core/services/worker"
	"gitlab.com/fcv-2025.net/internal/domain"
	"gitlab.com/fcv-2025.net/internal/tcp/connectionmanager"
	"gitlab.com/fcv-2025.net/internal/tcp/defs"
	"gitlab.com/fcv-2025.net/internal/tcp/handlers"
)

// TCPServer handles TCP connections from workers
type TCPServer struct {
	address          string
	workerService    worker.IWorkerRegistrationService
	schedulerService schedule.ISchedulerService
	jobService       job.IJobService
	logger           primary.Logger
	listener         net.Listener
	connectionMgr    *connectionmanager.ConnectionManager
	stopCh           chan struct{}
	handlers         map[byte]primary.MessageHandler
	publisher        map[byte]primary.MessagePublisher
}

// TCPServerOption configures a TCPServer
type TCPServerOption func(*TCPServer)

// WithAddress sets the server address
func WithAddress(address string) TCPServerOption {
	return func(s *TCPServer) {
		s.address = address
	}
}

// NewTCPServer creates a new TCP server
func NewTCPServer(
	workerService worker.IWorkerRegistrationService,
	schedulerService schedule.ISchedulerService,
	jobService job.IJobService,
	logger primary.Logger,
	options ...TCPServerOption,
) *TCPServer {
	server := &TCPServer{
		address:          ":9000", // Default address
		workerService:    workerService,
		schedulerService: schedulerService,
		logger:           logger,
		connectionMgr:    connectionmanager.NewConnectionManager(logger),
		stopCh:           make(chan struct{}),
		jobService:       jobService,
	}

	// Apply options
	for _, option := range options {
		option(server)
	}

	// Register message handlers
	server.setupMessageHandlers()

	return server
}

// setupMessageHandlers registers all message handlers
func (s *TCPServer) setupMessageHandlers() {
	s.handlers = map[byte]primary.MessageHandler{
		defs.MsgWorkerRegister:  &handlers.WorkerRegistrationHandler{WorkerService: s.workerService, ConnectionMgr: s.connectionMgr, Logger: s.logger},
		defs.MsgWorkerHeartbeat: &handlers.WorkerHeartbeatHandler{WorkerService: s.workerService, Logger: s.logger},
		defs.MsgJobRequest:      &handlers.JobRequestHandler{SchedulerService: s.schedulerService, ConnectionMgr: s.connectionMgr, Logger: s.logger},
		defs.MsgJobResult:       &handlers.JobResultHandler{SchedulerService: s.schedulerService, Logger: s.logger},
	}

	s.publisher = map[byte]primary.MessagePublisher{
		defs.MsgJobAssign: publishers.NewJobAssignPublisher(s.jobService, s.workerService, s.connectionMgr, s.logger),
	}
}

// Start starts the TCP server
func (s *TCPServer) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}

	s.logger.Info("TCP server listening", "address", s.address)

	// Accept connections in a goroutine
	go s.acceptConnections()

	return nil
}

// Stop stops the TCP server
func (s *TCPServer) Stop(ctx context.Context) error {
	close(s.stopCh)

	// Close listener
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			s.logger.Error("Failed to close listener", "error", err)
		}
	}

	// Close all connections
	s.closeAllConnections()

	<-ctx.Done()

	return nil
}

// closeAllConnections closes all worker connections
func (s *TCPServer) closeAllConnections() {
	s.connectionMgr.ConnMutex.Lock()
	defer s.connectionMgr.ConnMutex.Unlock()

	for workerID, conn := range s.connectionMgr.Connections {
		if err := conn.Close(); err != nil {
			s.logger.Error("Failed to close connection", "workerID", workerID, "error", err)
		}
	}
}

// NotifyJobAvailable notifies workers of available jobs
func (s *TCPServer) NotifyJobAvailable(jobType string) error {
	// Find workers of the given type
	matchingWorkers := s.connectionMgr.GetWorkersOfType(jobType)

	if len(matchingWorkers) == 0 {
		return nil // No matching workers
	}

	// Prepare notification message
	payload := defs.JobAvailableData{
		JobTypes: []string{jobType},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal job available payload: %w", err)
	}

	// Send notification to matching workers
	for _, workerID := range matchingWorkers {
		conn, exists := s.connectionMgr.GetConnection(workerID)
		if exists {
			if err := connectionmanager.SendMessage(conn, defs.MsgJobAvailable, payloadBytes); err != nil {
				s.logger.Error("Failed to send job available notification", "workerID", workerID, "error", err)
				continue
			}
			s.logger.Info("Sent job available notification", "workerID", workerID, "jobType", jobType)
		}
	}

	return nil
}

// acceptConnections accepts incoming connections
func (s *TCPServer) acceptConnections() {
	for {
		select {
		case <-s.stopCh:
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.stopCh:
					return
				default:
					s.logger.Error("Failed to accept connection", "error", err)
					time.Sleep(defs.ConnectionRetryDelay) // Avoid tight loop on error
					continue
				}
			}

			// Handle connection in a goroutine
			go s.handleConnection(conn)
		}
	}
}

// handleConnection handles a single worker connection
func (s *TCPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Set initial timeout for registration
	conn.SetDeadline(time.Now().Add(defs.InitialRegistrationTimeout))

	// Read and process messages
	var workerID string
	for {
		select {
		case <-s.stopCh:
			return
		default:
			// Read and parse message
			msgType, payload, err := readMessage(conn)
			if err != nil {
				if err != io.EOF {
					s.logger.Error("Failed to read message", "error", err)
				}
				// Remove connection on error
				if workerID != "" {
					s.connectionMgr.RemoveWorker(workerID)
					s.logger.Info("Worker disconnected", "workerID", workerID)
				}
				return
			}

			// Find handler for message type
			handler, exists := s.handlers[msgType]
			if !exists {
				s.logger.Error("Unknown message type", "type", msgType)
				connectionmanager.SendErrorMessage(conn, 1016, fmt.Sprintf("Unknown message type: %d", msgType))
				continue
			}

			// Create context for message handling
			ctx := context.Background()

			// Handle message
			err = handler.HandleMessage(ctx, conn, payload, &workerID)
			if err != nil {
				s.logger.Error("Error handling message", "type", msgType, "error", err)
				if workerID != "" {
					s.connectionMgr.RemoveWorker(workerID)
					s.logger.Info("Worker disconnected due to error", "workerID", workerID)
				}
				return
			}

			// After successful registration, remove timeout
			if msgType == defs.MsgWorkerRegister {
				conn.SetDeadline(time.Time{}) // No timeout
			}
		}
	}
}

// readMessage reads a message from a connection
func readMessage(conn net.Conn) (byte, []byte, error) {
	// Read message header
	header := make([]byte, 8)
	if _, err := io.ReadFull(conn, header); err != nil {
		return 0, nil, err
	}

	// Parse header
	magic := binary.BigEndian.Uint16(header[0:2])
	msgType := header[2]
	payloadLen := binary.BigEndian.Uint32(header[4:8])

	// Validate magic number
	if magic != defs.MagicNumber {
		return 0, nil, fmt.Errorf("invalid magic number: %x", magic)
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return 0, nil, err
	}

	return msgType, payload, nil
}

func (s *TCPServer) AssignJob(ctx context.Context, jobInfo *domain.Job) error {
	// query workers in redis
	//workers ,err := s.workerService.GetAvailableWorkers(ctx, string(jobInfo.Type))
	workers, err := s.workerService.GetAvailableWorkers(ctx, string(jobInfo.Type))
	if err != nil {
		s.logger.Error("Failed to get available workers", "error", err)
		return err
	}

	if len(workers) == 0 {
		s.logger.Info("No workers available")
		return nil
	}

	workerInstance := FindBestWorker(workers)
	if workerInstance == nil {
		s.logger.Info("No workers available")
		return nil
	}

	conn, exists := s.connectionMgr.GetConnection(workerInstance.ID)
	if !exists {
		s.logger.Error("Worker connection not found", "workerID", workerInstance.ID)
		return nil
	}

	jobBytes, err := json.Marshal(jobInfo)
	if err != nil {
		s.logger.Error("Failed to marshal job info", "error", err)
		return err
	}

	return s.publisher[defs.MsgJobAssign].PublishMessage(ctx, conn, jobBytes, *workerInstance)
}

func (s *TCPServer) SendJobAssignments(ctx context.Context, jobInfo *domain.Job) error {
	hdl, ok := s.handlers[defs.MsgJobRequest]
	if !ok {
		return fmt.Errorf("message handler not found")
	}
	workers, err := s.workerService.GetAllWorkers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get workers: %w", err)
	}

	var bestWorker *domain.WorkerInfo
	bestWorker = FindBestWorker(workers)
	if bestWorker == nil {
		return errors.New("no workers available")
	}

	conn, exists := s.connectionMgr.GetConnection(bestWorker.ID)
	if !exists {
		return fmt.Errorf("worker not connected: %s", bestWorker.ID)
	}

	bb, err := json.Marshal(jobInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal job info: %w", err)
	}

	err = hdl.HandleMessage(ctx, conn, bb, &bestWorker.ID)

	if err != nil {
		return errors.New("failed to send job assignments")
	}

	return nil
}

// FindBestWorker finds the best worker for a job based on current load
func FindBestWorker(workers []*domain.WorkerInfo) *domain.WorkerInfo {
	if len(workers) == 0 {
		return nil
	}

	// Find worker with lowest load relative to capacity
	var bestWorker *domain.WorkerInfo
	var bestLoadRatio = 2.0 // Start with value > 1

	for _, w := range workers {

		loadRatio := float64(w.CurrentLoad) / float64(w.Capacity)
		if loadRatio < bestLoadRatio {
			bestWorker = w
			bestLoadRatio = loadRatio
		}
	}

	return bestWorker
}
