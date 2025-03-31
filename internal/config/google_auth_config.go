package config

import "os"

type GGAuthConfig struct {
	ForceFPTDomain bool
}

func NewGGAuthConfig() *GGAuthConfig {
	return &GGAuthConfig{
		ForceFPTDomain: os.Getenv("FORCE_FPT_DOMAIN") == "true",
	}
}
