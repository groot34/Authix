package config

import (
	"os"
	"testing"
)

func setEnvs(t *testing.T, pairs map[string]string) func() {
	t.Helper()
	saved := map[string]string{}
	for k, v := range pairs {
		if cur, ok := os.LookupEnv(k); ok {
			saved[k] = cur
		} else {
			saved[k] = ""
		}
		_ = os.Setenv(k, v)
	}
	return func() {
		for k, orig := range saved {
			if orig == "" {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, orig)
			}
		}
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Clear anything set in the test environment.
	restore := setEnvs(t, map[string]string{
		"BACKEND_PORT":           "",
		"PORT":                   "",
		"BACKEND_ENV":            "",
		"MIGRATIONS_DIR":         "",
		"POSTGRES_HOST":          "",
		"POSTGRES_PORT":          "",
		"POSTGRES_USER":          "",
		"POSTGRES_PASSWORD":      "",
		"POSTGRES_DB":            "",
		"AUTHIX_ALLOWED_ORIGINS": "",
		"AUTHIX_SESSION_COOKIE":  "",
		"AUTHIX_COOKIE_SECURE":   "",
	})
	defer restore()

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("Port default = %q, want 8080", cfg.Port)
	}
	if cfg.Env != "development" {
		t.Errorf("Env default = %q, want development", cfg.Env)
	}
	if len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != "http://localhost:5173" {
		t.Errorf("AllowedOrigins default = %v", cfg.AllowedOrigins)
	}
	if cfg.SessionCookieName != "authix_session" || cfg.SessionCookieSecure {
		t.Errorf("unexpected cookie defaults: name=%q secure=%v", cfg.SessionCookieName, cfg.SessionCookieSecure)
	}
	if cfg.MigrationsDir != "../database/migrations" {
		t.Errorf("MigrationsDir default = %q, want ../database/migrations", cfg.MigrationsDir)
	}
	if cfg.Postgres.Host != "localhost" {
		t.Errorf("Postgres.Host default = %q, want localhost", cfg.Postgres.Host)
	}
	if cfg.Postgres.Port != "5432" {
		t.Errorf("Postgres.Port default = %q, want 5432", cfg.Postgres.Port)
	}
	if cfg.Postgres.User != "authix_user" {
		t.Errorf("Postgres.User default = %q, want authix_user", cfg.Postgres.User)
	}
	if cfg.Postgres.Password != "" {
		t.Errorf("Postgres.Password default = %q, want empty (env only)", cfg.Postgres.Password)
	}
	if cfg.Postgres.DBName != "authix" {
		t.Errorf("Postgres.DBName default = %q, want authix", cfg.Postgres.DBName)
	}
}

func TestLoad_OverrideAll(t *testing.T) {
	restore := setEnvs(t, map[string]string{
		"BACKEND_PORT":           "9090",
		"PORT":                   "9191",
		"BACKEND_ENV":            "test",
		"MIGRATIONS_DIR":         "/tmp/migs",
		"AUTHIX_ALLOWED_ORIGINS": " http://localhost:5173, https://app.example.com ",
		"AUTHIX_SESSION_COOKIE":  "session",
		"AUTHIX_COOKIE_SECURE":   "1",
		"POSTGRES_HOST":          "pg.local",
		"POSTGRES_PORT":          "6432",
		"POSTGRES_USER":          "u",
		"POSTGRES_PASSWORD":      "secret",
		"POSTGRES_DB":            "d",
	})
	defer restore()

	cfg := Load()

	if cfg.Port != "9090" || cfg.Env != "test" || cfg.MigrationsDir != "/tmp/migs" {
		t.Fatalf("unexpected top-level cfg: %+v", cfg)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.SessionCookieName != "session" || !cfg.SessionCookieSecure {
		t.Fatalf("unexpected HTTP cfg: origins=%v cookie=%q secure=%v", cfg.AllowedOrigins, cfg.SessionCookieName, cfg.SessionCookieSecure)
	}
	if cfg.Postgres.Host != "pg.local" || cfg.Postgres.Port != "6432" ||
		cfg.Postgres.User != "u" || cfg.Postgres.Password != "secret" ||
		cfg.Postgres.DBName != "d" {
		t.Fatalf("unexpected postgres cfg: %+v", cfg.Postgres)
	}
}

func TestLoad_ProductionRequiresExplicitOrigins(t *testing.T) {
	restore := setEnvs(t, map[string]string{
		"BACKEND_ENV":            "production",
		"AUTHIX_ALLOWED_ORIGINS": "",
		"AUTHIX_COOKIE_SECURE":   "",
	})
	defer restore()

	cfg := Load()
	if len(cfg.AllowedOrigins) != 0 {
		t.Fatalf("production should not default to development origins: %v", cfg.AllowedOrigins)
	}
	if !cfg.SessionCookieSecure {
		t.Fatal("production cookies should be Secure by default")
	}
}
