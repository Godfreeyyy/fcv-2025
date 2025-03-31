package secondary

import (
	"context"

	"gitlab.com/fcv-2025.net/internal/domain"
)

type UserPort interface {
	Create(ctx context.Context, user *domain.Users) error
	Save(user *domain.Users) error
	Delete(id string) error
	Get(id string) (*domain.Users, error)
	GetByEmail(email string) (*domain.Users, error)
	GetByGoogleID(ctx context.Context, googleID string) (*domain.Users, error)
	GetByUserName(userName string) (*domain.Users, error)
	VerifyUserNAmePassword(userName, password string) (*domain.Users, error)
}
