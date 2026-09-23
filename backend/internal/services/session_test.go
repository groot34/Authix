package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/authix/authix/internal/repositories"
)

type fakeSessionRepo struct {
	createFn  func(context.Context, []byte, int64, time.Time, time.Time) error
	findFn    func(context.Context, []byte, time.Time) (*repositories.Session, error)
	revokeFn  func(context.Context, []byte, time.Time) error
	created   []byte
	createdAt time.Time
	expiresAt time.Time
}

func (f *fakeSessionRepo) Create(ctx context.Context, hash []byte, userID int64, createdAt, expiresAt time.Time) error {
	f.created = append([]byte(nil), hash...)
	f.createdAt = createdAt
	f.expiresAt = expiresAt
	if f.createFn != nil {
		return f.createFn(ctx, hash, userID, createdAt, expiresAt)
	}
	return nil
}

func (f *fakeSessionRepo) FindValid(ctx context.Context, hash []byte, now time.Time) (*repositories.Session, error) {
	if f.findFn != nil {
		return f.findFn(ctx, hash, now)
	}
	return nil, repositories.ErrSessionNotFound
}

func (f *fakeSessionRepo) Revoke(ctx context.Context, hash []byte, revokedAt time.Time) error {
	if f.revokeFn != nil {
		return f.revokeFn(ctx, hash, revokedAt)
	}
	return repositories.ErrSessionNotFound
}

func TestSessionService_CreateGeneratesOpaqueToken(t *testing.T) {
	now := time.Date(2026, 9, 23, 16, 0, 0, 0, time.UTC)
	seed := make([]byte, sessionTokenBytes)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	var storedHash []byte
	repo := &fakeSessionRepo{createFn: func(_ context.Context, hash []byte, userID int64, createdAt, expiresAt time.Time) error {
		storedHash = append([]byte(nil), hash...)
		if userID != 7 || !createdAt.Equal(now) || !expiresAt.Equal(now.Add(2*time.Hour)) {
			t.Fatalf("unexpected create args: user=%d created=%v expires=%v", userID, createdAt, expiresAt)
		}
		return nil
	}}
	svc := NewSessionService(repo,
		WithSessionTimeNow(fixedClock(now)),
		WithSessionLifetime(2*time.Hour),
		WithSessionTokenGenerator(func() ([]byte, error) { return append([]byte(nil), seed...), nil }),
	)

	credentials, err := svc.Create(context.Background(), 7)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if len(credentials.Token) != 43 {
		t.Fatalf("token length want 43 got %d", len(credentials.Token))
	}
	if credentials.UserID != 7 || !credentials.ExpiresAt.Equal(now.Add(2*time.Hour)) {
		t.Fatalf("unexpected credentials: %+v", credentials)
	}
	if string(storedHash) != string(hashSessionToken(seed)) {
		t.Fatal("repository received a hash different from the raw token")
	}
	if credentials.Token == string(seed) {
		t.Fatal("session token should be encoded, not raw bytes")
	}
}

func TestSessionService_RandomTokensAreUnique(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 200; i++ {
		token, err := randomSessionToken()
		if err != nil {
			t.Fatalf("randomSessionToken error: %v", err)
		}
		if len(token) != sessionTokenBytes {
			t.Fatalf("token length want %d got %d", sessionTokenBytes, len(token))
		}
		encoded := encodeSessionToken(token)
		if _, exists := seen[encoded]; exists {
			t.Fatalf("duplicate token at sample %d", i)
		}
		seen[encoded] = struct{}{}
	}
}

func TestSessionService_ValidateExpiredRevokedAndUnknown(t *testing.T) {
	now := time.Date(2026, 9, 23, 16, 0, 0, 0, time.UTC)
	raw := []byte("01234567890123456789012345678901")
	encoded := encodeSessionToken(raw)
	cases := []struct {
		name   string
		record *repositories.Session
		want   error
	}{
		{"expired", &repositories.Session{UserID: 7, CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Second)}, ErrSessionNotFound},
		{"revoked", &repositories.Session{UserID: 7, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), RevokedAt: ptr(now.Add(-time.Minute))}, ErrSessionNotFound},
		{"unknown", nil, ErrSessionNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeSessionRepo{findFn: func(context.Context, []byte, time.Time) (*repositories.Session, error) {
				if tc.record == nil {
					return nil, repositories.ErrSessionNotFound
				}
				if tc.record.RevokedAt != nil || !now.Before(tc.record.ExpiresAt) {
					return nil, repositories.ErrSessionNotFound
				}
				return tc.record, nil
			}}
			svc := NewSessionService(repo, WithSessionTimeNow(fixedClock(now)))
			_, err := svc.Validate(context.Background(), encoded)
			if !errors.Is(err, tc.want) {
				t.Errorf("want %v got %v", tc.want, err)
			}
		})
	}
}

func TestSessionService_ValidateSuccessAndRevoke(t *testing.T) {
	now := time.Date(2026, 9, 23, 16, 0, 0, 0, time.UTC)
	raw := []byte("01234567890123456789012345678901")
	encoded := encodeSessionToken(raw)
	var foundHash, revokedHash []byte
	repo := &fakeSessionRepo{
		findFn: func(_ context.Context, hash []byte, gotNow time.Time) (*repositories.Session, error) {
			foundHash = append([]byte(nil), hash...)
			if !gotNow.Equal(now) {
				t.Errorf("validation time want %v got %v", now, gotNow)
			}
			return &repositories.Session{UserID: 9, CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}, nil
		},
		revokeFn: func(_ context.Context, hash []byte, revokedAt time.Time) error {
			revokedHash = append([]byte(nil), hash...)
			if !revokedAt.Equal(now) {
				t.Errorf("revocation time want %v got %v", now, revokedAt)
			}
			return nil
		},
	}
	svc := NewSessionService(repo, WithSessionTimeNow(fixedClock(now)))
	identity, err := svc.Validate(context.Background(), encoded)
	if err != nil {
		t.Fatalf("Validate error: %v", err)
	}
	if identity.UserID != 9 {
		t.Errorf("user ID want 9 got %d", identity.UserID)
	}
	if string(foundHash) != string(hashSessionToken(raw)) {
		t.Error("Validate sent the wrong token hash")
	}
	if err := svc.Revoke(context.Background(), encoded); err != nil {
		t.Fatalf("Revoke error: %v", err)
	}
	if string(revokedHash) != string(hashSessionToken(raw)) {
		t.Error("Revoke sent the wrong token hash")
	}
}

func TestSessionService_InvalidTokensAndRepositoryErrors(t *testing.T) {
	svc := NewSessionService(&fakeSessionRepo{})
	for _, token := range []string{"", "short", "not valid token"} {
		if _, err := svc.Validate(context.Background(), token); !errors.Is(err, ErrInvalidSessionToken) {
			t.Errorf("token %q want invalid token, got %v", token, err)
		}
	}
	if _, err := svc.Create(context.Background(), 0); !errors.Is(err, ErrInvalidSessionUser) {
		t.Errorf("zero user ID want invalid user, got %v", err)
	}

	boom := errors.New("database unavailable")
	repo := &fakeSessionRepo{
		createFn: func(context.Context, []byte, int64, time.Time, time.Time) error { return boom },
		findFn:   func(context.Context, []byte, time.Time) (*repositories.Session, error) { return nil, boom },
		revokeFn: func(context.Context, []byte, time.Time) error { return boom },
	}
	svc = NewSessionService(repo, WithSessionTokenGenerator(func() ([]byte, error) {
		return make([]byte, sessionTokenBytes), nil
	}))
	if _, err := svc.Create(context.Background(), 1); !errors.Is(err, boom) {
		t.Errorf("Create should preserve repository error: %v", err)
	}
	token := encodeSessionToken(make([]byte, sessionTokenBytes))
	if _, err := svc.Validate(context.Background(), token); !errors.Is(err, boom) {
		t.Errorf("Validate should preserve repository error: %v", err)
	}
	if err := svc.Revoke(context.Background(), token); !errors.Is(err, boom) {
		t.Errorf("Revoke should preserve repository error: %v", err)
	}
}
