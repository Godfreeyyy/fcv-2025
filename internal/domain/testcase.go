package domain

import "github.com/google/uuid"

// TestCase represents a test case for code execution
type TestCase struct {
	ID             uuid.UUID
	Input          string
	ExpectedOutput string
	IsHidden       bool
}
