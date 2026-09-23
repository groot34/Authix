package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/groot34/Authix/internal/repositories"
)

const (
	sessionTokenBytes      = 32
	defaultSessionLifetime = 24 * time.Hour
)

var (
	ErrInvalidSessionToken = errors.New("services: invalid session token")
	ErrInvalidSessionUser  = errors.New("services: invalid session user")
	ErrSessionNotFound     = errors.New("services: session not found")
)

type SessionCredentials struct {
	Token     string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

type AuthenticatedSession struct {
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

type sessionRepository interface {
	Create(ctx context.Context, tokenHash []byte, userID int64, createdAt, expiresAt time.Time) error
	FindValid(ctx context.Context, tokenHash []byte, now time.Time) (*repositories.Session, error)
	Revoke(ctx context.Context, tokenHash []byte, revokedAt time.Time) error
}

type SessionService struct {
	sessions sessionRepository
	now      func() time.Time
	generate func() ([]byte, error)
	lifetime time.Duration
}

type SessionServiceOption func(*SessionService)

func WithSessionTimeNow(fn func() time.Time) SessionServiceOption {
	return func(s *SessionService) { s.now = fn }
}

func WithSessionTokenGenerator(fn func() ([]byte, error)) SessionServiceOption {
	return func(s *SessionService) { s.generate = fn }
}

func WithSessionLifetime(lifetime time.Duration) SessionServiceOption {
	return func(s *SessionService) { s.lifetime = lifetime }
}

func NewSessionService(sessions sessionRepository, opts ...SessionServiceOption) *SessionService {
	s := &SessionService{
		sessions: sessions,
		now:      time.Now,
		generate: randomSessionToken,
		lifetime: defaultSessionLifetime,
	}
	for _, option := range opts {
		option(s)
	}
	return s
}

func randomSessionToken() ([]byte, error) {
	token := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(token); err != nil {
		return nil, fmt.Errorf("services: generate session token: %w", err)
	}
	return token, nil
}

func hashSessionToken(token []byte) []byte {
	sum := sha256.Sum256(token)
	return sum[:]
}

func encodeSessionToken(token []byte) string {
	return base64.RawURLEncoding.EncodeToString(token)
}

func decodeSessionToken(encoded string) ([]byte, error) {
	token, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(token) != sessionTokenBytes {
		return nil, ErrInvalidSessionToken
	}
	return token, nil
}

func (s *SessionService) Create(ctx context.Context, userID int64) (*SessionCredentials, error) {
	if userID <= 0 {
		return nil, ErrInvalidSessionUser
	}
	if s.lifetime <= 0 {
		return nil, errors.New("services: invalid session lifetime")
	}

	token, err := s.generate()
	if err != nil {
		return nil, err
	}
	if len(token) != sessionTokenBytes {
		return nil, errors.New("services: invalid generated session token")
	}
	createdAt := s.now().UTC()
	expiresAt := createdAt.Add(s.lifetime)
	if err := s.sessions.Create(ctx, hashSessionToken(token), userID, createdAt, expiresAt); err != nil {
		return nil, fmt.Errorf("services: create session: %w", err)
	}
	return &SessionCredentials{
		Token:     encodeSessionToken(token),
		UserID:    userID,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *SessionService) Validate(ctx context.Context, token string) (*AuthenticatedSession, error) {
	raw, err := decodeSessionToken(token)
	if err != nil {
		return nil, err
	}
	record, err := s.sessions.FindValid(ctx, hashSessionToken(raw), s.now().UTC())
	if err != nil {
		if errors.Is(err, repositories.ErrSessionNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("services: validate session: %w", err)
	}
	return &AuthenticatedSession{
		UserID:    record.UserID,
		CreatedAt: record.CreatedAt,
		ExpiresAt: record.ExpiresAt,
	}, nil
}

func (s *SessionService) Revoke(ctx context.Context, token string) error {
	raw, err := decodeSessionToken(token)
	if err != nil {
		return err
	}
	if err := s.sessions.Revoke(ctx, hashSessionToken(raw), s.now().UTC()); err != nil {
		if errors.Is(err, repositories.ErrSessionNotFound) {
			return ErrSessionNotFound
		}
		return fmt.Errorf("services: revoke session: %w", err)
	}
	return nil
}
