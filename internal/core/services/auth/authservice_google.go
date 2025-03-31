package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"gitlab.com/fcv-2025.net/internal/config"
	"gitlab.com/fcv-2025.net/internal/core/ports/primary"
	"gitlab.com/fcv-2025.net/internal/core/ports/secondary"
	"gitlab.com/fcv-2025.net/internal/domain"
	"gitlab.com/fcv-2025.net/internal/global/logger"
	"gitlab.com/fcv-2025.net/internal/static/errs"
)

var _ IAuthService = &googleAuthService{}

type googleAuthService struct {
	userPort    secondary.UserPort
	jwtProvider primary.JWTService
	Config      *config.GGAuthConfig
}

func NewGoogleAuthService(userPort secondary.UserPort, jwtProvider primary.JWTService, Config *config.GGAuthConfig) IAuthService {
	return &googleAuthService{
		userPort:    userPort,
		jwtProvider: jwtProvider,
		Config:      Config,
	}
}

func (g googleAuthService) ProviderName() domain.Provider {
	return domain.ProviderGoogle
}

func (g googleAuthService) Login(ctx context.Context, users *domain.Users) (string, error) {
	if users.GoogleID == nil {
		return "", errs.InvalidCredentials
	}

	if users.AuthProvider != string(domain.ProviderGoogle) {
		return "", errs.InvalidCredentials
	}

	if users.Email == nil {
		return "", errs.EmailRequired
	}

	if g.Config.ForceFPTDomain && !strings.Contains(*users.Email, "@fpt.edu.vn") {
		return "", errs.ShouldUseFPTEmail
	}

	usr, err := g.userPort.GetByGoogleID(ctx, *users.GoogleID)
	if err != nil {
		return "", err
	}

	if usr != nil {
		return g.generateToken(ctx, usr)
	}
	users.PasswordHash = nil
	users.UserName = strings.Split(*users.Email, "@")[0]
	users.AuthProvider = string(domain.ProviderGoogle)
	users.StudentCode = strings.Split(*users.Email, "@")[0]
	err = g.userPort.Create(ctx, users)
	if err != nil {
		return "", errs.FailedToCreateUser
	}

	return g.generateToken(ctx, users)
}

func (g googleAuthService) generateToken(ctx context.Context, user *domain.Users) (string, error) {
	authPayload := domain.AuthPayload{
		Username:   user.UserName,
		Permission: []string{"job_scheduler.execute"},
	}
	var buf bytes.Buffer

	err := json.NewEncoder(&buf).Encode(authPayload)
	if err != nil {
		return "", errs.InternalError
	}
	var payload map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &payload)
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
