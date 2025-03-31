package domain

import (
	"time"

	"github.com/google/uuid"
)

// TestCaseResult represents the result of a single test case execution
type TestCaseResult struct {
	TestCaseID      uuid.UUID
	Passed          bool
	ActualOutput    string
	ErrorMessage    string
	ExecutionTimeMs int64
}

// ExecutionResult represents the result of code execution against test cases
type ExecutionResult struct {
	SubmissionID      uuid.UUID
	Status            Status
	TestCaseResults   []TestCaseResult
	CompilationOutput string
	ExecutionTimeMs   int64
	MemoryUsageBytes  int64
	CompletedAt       time.Time
}
