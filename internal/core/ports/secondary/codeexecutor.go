package secondary

import (
	"context"

	"gitlab.com/fcv-2025.net/internal/domain"
)

type CodeExecutor interface {
	// Execute executes code against test cases
	Execute(ctx context.Context, submission *domain.Submission, testCases []*domain.TestCase) (*domain.ExecutionResult, error)
}
