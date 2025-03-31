package auth

import (
	"context"

	"gitlab.com/fcv-2025.net/internal/domain"
)

type IAuthService interface {
	ProviderName() domain.Provider
	Login(ctx context.Context, users *domain.Users) (string, error)
}
