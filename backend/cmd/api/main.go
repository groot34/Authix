package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/groot34/Authix/internal/config"
	"github.com/groot34/Authix/internal/database"
	"github.com/groot34/Authix/internal/handlers"
	"github.com/groot34/Authix/internal/repositories"
	"github.com/groot34/Authix/internal/services"
)

func main() {
	cfg := config.Load()

	dbParams := database.Params{
		Host:     cfg.Postgres.Host,
		Port:     cfg.Postgres.Port,
		User:     cfg.Postgres.User,
		Password: cfg.Postgres.Password,
		DBName:   cfg.Postgres.DBName,
	}

	pool, err := database.Open(dbParams)
	if err != nil {
		log.Fatalf("open database pool: %v", err)
	}
	defer func() {
		if cerr := pool.Close(); cerr != nil {
			log.Printf("warn: closing db pool: %v", cerr)
		}
	}()

	// Fail clearly at startup if PostgreSQL isn't reachable — better than a
	// 500 on the first request. We allow overriding for the smoke tests in
	// environments where Postgres isn't available by setting
	// AUTHIX_SKIP_DB_STARTUP_CHECK=1.
	if os.Getenv("AUTHIX_SKIP_DB_STARTUP_CHECK") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := database.WaitForReady(ctx, pool); err != nil {
			cancel()
			log.Fatalf("postgres not reachable at startup: %v", err)
		}
		cancel()
		log.Printf("postgres ready at %s:%s db=%s user=%s",
			cfg.Postgres.Host, cfg.Postgres.Port,
			cfg.Postgres.DBName, cfg.Postgres.User)
	}

	// Apply any pending migrations. We always do this at boot for an
	// assessment project — it's simple and obvious. For production we'd
	// run migrations as a separate step, but that's overkill right now.
	applied, err := database.Run(pool.DB(), cfg.MigrationsDir)
	if err != nil {
		log.Fatalf("apply migrations: %v", err)
	}
	if len(applied) > 0 {
		log.Printf("applied migrations: %v", applied)
	} else {
		log.Printf("no new migrations to apply")
	}

	userRepo := repositories.NewUserRepository(pool)
	checkoutRepo := repositories.NewCheckoutRepository(pool)
	sessionRepo := repositories.NewSessionRepository(pool)
	authService := services.NewAuthService(userRepo)
	checkoutService := services.NewCheckoutService(checkoutRepo)
	sessionService := services.NewSessionService(sessionRepo)
	api := handlers.NewAPI(authService, checkoutService, sessionService, cfg.SessionCookieName, cfg.SessionCookieSecure)

	mux := http.NewServeMux()
	mux.Handle("/api/", api.Routes())
	mux.HandleFunc("GET /health", handlers.Health(cfg.Env))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	})

	addr := ":" + cfg.Port
	log.Printf("authix api listening on %s (env=%s)", addr, cfg.Env)

	srv := &http.Server{
		Addr:              addr,
		Handler:           handlers.CORS(mux, cfg.AllowedOrigins),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if serr := srv.Shutdown(ctx); serr != nil {
			log.Printf("warn: server shutdown: %v", serr)
		}
		close(idleConnsClosed)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}

	<-idleConnsClosed
	_ = os.Stdout.Sync()
	log.Printf("authix api shut down cleanly")
}
