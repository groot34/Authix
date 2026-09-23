package repositories

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"
)

func TestSessionRepository_Lifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live-DB test in short mode")
	}
	pool := setupThrowawayDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	users := NewUserRepository(pool)
	u, err := users.Insert(ctx, "session@example.com", "Session", "User")
	if err != nil {
		t.Fatalf("Insert user: %v", err)
	}
	sessions := NewSessionRepository(pool)
	now := time.Date(2026, 9, 23, 17, 0, 0, 0, time.UTC)
	token := sha256.Sum256([]byte("session-token"))
	if err := sessions.Create(ctx, token[:], u.ID, now, now.Add(time.Hour)); err != nil {
		t.Fatalf("Create session: %v", err)
	}

	found, err := sessions.FindValid(ctx, token[:], now.Add(time.Minute))
	if err != nil {
		t.Fatalf("FindValid: %v", err)
	}
	if found.UserID != u.ID || !found.CreatedAt.Equal(now) || !found.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("session mismatch: %+v", found)
	}

	if _, err := sessions.FindValid(ctx, token[:], now.Add(time.Hour)); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expired session want ErrSessionNotFound, got %v", err)
	}
	if err := sessions.Revoke(ctx, token[:], now.Add(2*time.Minute)); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := sessions.FindValid(ctx, token[:], now.Add(3*time.Minute)); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("revoked session want ErrSessionNotFound, got %v", err)
	}
	if err := sessions.Revoke(ctx, token[:], now.Add(4*time.Minute)); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("second revoke want ErrSessionNotFound, got %v", err)
	}
	unknown := sha256.Sum256([]byte("unknown-token"))
	if _, err := sessions.FindValid(ctx, unknown[:], now); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("unknown session want ErrSessionNotFound, got %v", err)
	}
}
