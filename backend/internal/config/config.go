package config

import "os"

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

type Config struct {
	Port          string
	Env           string
	MigrationsDir string
	Postgres      PostgresConfig
}

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func Load() Config {
	return Config{
		Port: getenv("BACKEND_PORT", "8080"),
		Env:  getenv("BACKEND_ENV", "development"),
		// Default assumes the binary is invoked as `go run ./cmd/api` from
		// the backend/ directory. Override with MIGRATIONS_DIR env if
		// calling from elsewhere or in production.
		MigrationsDir: getenv("MIGRATIONS_DIR", "../database/migrations"),
		Postgres: PostgresConfig{
			Host:     getenv("POSTGRES_HOST", "localhost"),
			Port:     getenv("POSTGRES_PORT", "5432"),
			User:     getenv("POSTGRES_USER", "authix_user"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			DBName:   getenv("POSTGRES_DB", "authix"),
		},
	}
}
