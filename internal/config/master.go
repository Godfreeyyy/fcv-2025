package config

import "os"

type AppConfig struct {
	DebugMode      bool
	ScheduleSvcCfg *ScheduleSvcCfg
	RedisConfig    *RedisConfig
	PostgresConfig *PostgresConfig
	JwtConfig      *JwtConfig
	GGAuthConfig   *GGAuthConfig
}

func NewSystemConfig() *AppConfig {
	return &AppConfig{
		DebugMode:      os.Getenv("DEBUG_MODE") == "true",
		ScheduleSvcCfg: NewScheduleSvcCfg(),
		RedisConfig:    NewRedisConfig(),
		PostgresConfig: NewPostgresConfig(),
		JwtConfig:      NewJwtConfig(),
		GGAuthConfig:   NewGGAuthConfig(),
	}
}
