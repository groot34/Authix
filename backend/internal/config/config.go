package config

import (
	"os"
	"strings"
)

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type Config struct {
	Port                string
	Env                 string
	MigrationsDir       string
	AllowedOrigins      []string
	SessionCookieName   string
	SessionCookieSecure bool
	Postgres            PostgresConfig
}

func getenv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func Load() Config {
	env := getenv("BACKEND_ENV", "development")
	port := getenv("BACKEND_PORT", getenv("PORT", "8080"))
	origins := splitCSV(os.Getenv("AUTHIX_ALLOWED_ORIGINS"))
	if len(origins) == 0 && env != "production" {
		origins = []string{"http://localhost:5173"}
	}
	return Config{
		Port: port,
		Env:  env,
		// Default assumes the binary is invoked as `go run ./cmd/api` from
		// the backend/ directory. Override with MIGRATIONS_DIR env if
		// calling from elsewhere or in production.
		MigrationsDir:       getenv("MIGRATIONS_DIR", "../database/migrations"),
		AllowedOrigins:      origins,
		SessionCookieName:   getenv("AUTHIX_SESSION_COOKIE", "authix_session"),
		SessionCookieSecure: getenv("AUTHIX_COOKIE_SECURE", "") == "1" || (env == "production" && getenv("AUTHIX_COOKIE_SECURE", "") != "0"),
		Postgres: PostgresConfig{
			Host:     getenv("POSTGRES_HOST", "localhost"),
			Port:     getenv("POSTGRES_PORT", "5432"),
			User:     getenv("POSTGRES_USER", "authix_user"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			DBName:   getenv("POSTGRES_DB", "authix"),
			SSLMode:  getenv("POSTGRES_SSLMODE", "disable"),
		},
	}
}

func splitCSV(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
