package config

type PostgresConfig struct {
	Url string
}

func NewPostgresConfig() *PostgresConfig {
	return &PostgresConfig{
		Url: "postgres://root:123456@localhost:5432/postgres?sslmode=disable",
	}
}
