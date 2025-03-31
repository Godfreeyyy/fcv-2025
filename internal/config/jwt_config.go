package config

import "os"

type JwtConfig struct {
	Secret         string
	KeyVaultFolder string
}

func NewJwtConfig() *JwtConfig {
	return &JwtConfig{
		Secret: os.Getenv("JWT_SECRET"),
	}
}
