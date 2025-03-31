package config

type RedisConfig struct {
	DB       int
	Url      string
	Password string
}

func NewRedisConfig() *RedisConfig {
	return &RedisConfig{
		DB:       0,
		Url:      "localhost:6379",
		Password: "",
	}
}
