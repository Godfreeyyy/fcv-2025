package secondary

import (
	"context"

	"github.com/google/uuid"

	"gitlab.com/fcv-2025.net/internal/domain"
)

// ResultRepository defines the interface for storing and retrieving execution results
type ResultRepository interface {
	// SaveResult saves an execution result
	SaveResult(ctx context.Context, result *domain.ExecutionResult) error

	// GetResult retrieves an execution result by submission ID
	GetResult(ctx context.Context, submissionID uuid.UUID) (*domain.ExecutionResult, error)
}
