package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/authix/authix/internal/database"
)

var ErrSessionNotFound = errors.New("repositories: session not found")

type Session struct {
	TokenHash []byte
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type SessionRepository struct {
	pool database.Pool
}

func NewSessionRepository(pool database.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) Create(ctx context.Context, tokenHash []byte, userID int64, createdAt, expiresAt time.Time) error {
	const q = `
INSERT INTO sessions (token_hash, user_id, created_at, expires_at)
VALUES ($1, $2, $3, $4);`

	if _, err := r.pool.DB().ExecContext(ctx, q, tokenHash, userID, createdAt, expiresAt); err != nil {
		return fmt.Errorf("repositories: create session: %w", err)
	}
	return nil
}

func (r *SessionRepository) FindValid(ctx context.Context, tokenHash []byte, now time.Time) (*Session, error) {
	const q = `
SELECT token_hash, user_id, created_at, expires_at, revoked_at
FROM sessions
WHERE token_hash = $1
  AND revoked_at IS NULL
  AND expires_at > $2;`

	s := &Session{}
	var revoked sql.Null[time.Time]
	if err := r.pool.DB().QueryRowContext(ctx, q, tokenHash, now).Scan(
		&s.TokenHash, &s.UserID, &s.CreatedAt, &s.ExpiresAt, &revoked,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("repositories: find session: %w", err)
	}
	if revoked.Valid {
		s.RevokedAt = &revoked.V
	}
	return s, nil
}

func (r *SessionRepository) Revoke(ctx context.Context, tokenHash []byte, revokedAt time.Time) error {
	const q = `
UPDATE sessions
SET revoked_at = $2
WHERE token_hash = $1
  AND revoked_at IS NULL;`

	result, err := r.pool.DB().ExecContext(ctx, q, tokenHash, revokedAt)
	if err != nil {
		return fmt.Errorf("repositories: revoke session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: revoke session rows: %w", err)
	}
	if rows == 0 {
		return ErrSessionNotFound
	}
	return nil
}
