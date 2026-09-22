package main

import (
	"log"
	"net/http"
	"os"

	"github.com/authix/authix/internal/config"
	"github.com/authix/authix/internal/handlers"
)

func main() {
	cfg := config.Load()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handlers.Health(cfg.Env))
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	})

	addr := ":" + cfg.Port
	log.Printf("authix api listening on %s (env=%s)", addr, cfg.Env)

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}

	_ = os.Stdout.Sync()
}
