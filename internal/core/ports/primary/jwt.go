package primary

import (
	"context"

	"gitlab.com/fcv-2025.net/internal/domain"
)

type KeyOption struct {
	UseFile     bool
	KeyFolder   string
	PublicFile  string
	PrivateFile string
	PrivateKey  string
	PublicKey   string
}

type ResourceKeyType string

type ResourceKey struct {
	Key     string
	Type    ResourceKeyType
	KeyByte []byte
}

type Resource string

type JWTService interface {
	GenerateTokenRSA(ctx context.Context, privateKey string, method string) (string, error)
	GenerateTokenRSAWithFile(ctx context.Context, data []byte, file string, method string) (string, error)
	VerifyTokenRSA(ctx context.Context, token string, publicKey string, method string) (bool, error)
	VerifyTokenRSAWithFile(ctx context.Context, token string, file string, method string) (bool, error)
	GenerateTokenHMAC(ctx context.Context, method string, claims map[string]interface{}) (string, error)
	DecodeTokenPayload(ctx context.Context, token string) (domain.AuthPayload, error)
	EncryptPassword(ctx context.Context, password string) (string, error)
	VerifyPassword(ctx context.Context, passwordHash string, pwd string) (bool, error)
	VerifyTokenHMAC(ctx context.Context, token string, method string) (bool, error)
}
