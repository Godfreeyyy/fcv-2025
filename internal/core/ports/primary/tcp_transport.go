package primary

import (
	"context"
	"gitlab.com/fcv-2025.net/internal/domain"
	"net"
)

// MessageHandler defines an interface for handling different message types
type MessageHandler interface {
	HandleMessage(ctx context.Context, conn net.Conn, payload []byte, workerID *string) error
}

type MessagePublisher interface {
	PublishMessage(ctx context.Context, conn net.Conn, payload []byte, w domain.WorkerInfo) error
}
