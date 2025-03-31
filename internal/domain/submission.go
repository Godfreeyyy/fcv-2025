package domain

import (
	"time"

	"github.com/google/uuid"
)

// Submission represents a code submission to be executed
type Submission struct {
	ID          uuid.UUID
	UserID      string
	Code        string
	Language    string
	ProblemID   string
	SubmittedAt time.Time
}

// NewSubmission creates a new submission
func NewSubmission(userID, code, language, problemID string) *Submission {
	return &Submission{
		ID:          uuid.New(),
		UserID:      userID,
		Code:        code,
		Language:    language,
		ProblemID:   problemID,
		SubmittedAt: time.Now(),
	}
}
