package auth

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/golang-jwt/jwt/v5"

	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/ports/secondary"
	"gitlab.com/fcv-2025.net/internal/domain"
	"gitlab.com/fcv-2025.net/internal/global/logger"
	"gitlab.com/fcv-2025.net/internal/static/errs"
)

var _ IAuthService = &localAuthService{}

type localAuthService struct {
	userPort    secondary.UserPort
	jwtProvider primary.JWTService
}

func NewLocalAuthService(
	userPort secondary.UserPort,
	jwtProvider primary.JWTService,
) IAuthService {
	return &localAuthService{
		userPort:    userPort,
		jwtProvider: jwtProvider,
	}
}

func (g localAuthService) ProviderName() domain.Provider {
	return domain.ProviderLocal
}

func (g localAuthService) Login(ctx context.Context, users *domain.Users) (string, error) {
	usr, err := g.userPort.GetByUserName(users.UserName)
	if err != nil {
		return "", err
	}
	if usr.PasswordHash == nil {
		return "", errs.InvalidCredentials
	}
	valid, err := g.jwtProvider.VerifyPassword(ctx, *usr.PasswordHash, *users.PasswordHash)
	if err != nil {
		return "", errs.InvalidCredentials
	}
	if !valid {
		return "", errs.InvalidCredentials
	}

	authPayload := domain.AuthPayload{
		Username:   usr.UserName,
		Permission: []string{"job_scheduler.execute"},
	}
	var buf bytes.Buffer

	err = json.NewEncoder(&buf).Encode(authPayload)
	if err != nil {
		return "", errs.InternalError
	}
	var payload map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &authPayload)
	if err != nil {
		logger.Error("Failed to unmarshal auth payload", "error", err)
		return "", errs.InternalError
	}
	token, err := g.jwtProvider.GenerateTokenHMAC(ctx, jwt.SigningMethodHS256.Name, payload)
	if err != nil {
		return "", errs.GeneratingToken
	}
	return token, nil
}
