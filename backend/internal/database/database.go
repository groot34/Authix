// Package database provides the PostgreSQL connection pool and runs schema
// migrations from a directory of ordered SQL files.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Pool is the interface we use against the open pool so callers can depend
// on a narrow surface instead of *sql.DB directly. Close closes the pool.
type Pool interface {
	DB() *sql.DB
	PingContext(ctx context.Context) error
	Close() error
}

type poolWrap struct{ db *sql.DB }

func (p *poolWrap) DB() *sql.DB                      { return p.db }
func (p *poolWrap) PingContext(ctx context.Context) error { return p.db.PingContext(ctx) }
func (p *poolWrap) Close() error                     { return p.db.Close() }

// Params carries everything we need to open a PostgreSQL pool. It mirrors
// the PostgresConfig from internal/config, kept here so the package doesn't
// import internal/config in tests.
type Params struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// URL builds the libpq connection string used by database/sql + lib/pq.
func (p Params) URL() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		p.Host, p.Port, p.User, p.Password, p.DBName,
	)
}

// Open creates and configures a *sql.DB pool for PostgreSQL. It does not
// ping the server yet — call Ping (below) or rely on WaitForReady.
func Open(p Params) (Pool, error) {
	db, err := sql.Open("postgres", p.URL())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	// Conservative pool for an assessment. Tune up later if needed.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	return &poolWrap{db: db}, nil
}

// WaitForReady blocks until the database is reachable or the context is
// cancelled. Returns a descriptive error if PostgreSQL is unavailable so
// startup failures are obvious at server boot instead of at request time.
func WaitForReady(ctx context.Context, pool Pool) error {
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()

	var lastErr error
	for {
		if err := pool.PingContext(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("postgres not ready after context expiry: last error=%w", lastErr)
			}
			return fmt.Errorf("postgres not ready: %w", ctx.Err())
		case <-t.C:
		}
	}
}
