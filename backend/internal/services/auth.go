package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/authix/authix/internal/repositories"
)

const (
	otpNumDigits   = 6
	otpDenominator = 1_000_000

	otpTTL               = 10 * time.Minute
	otpRateLimitAttempts = 5
	otpRateLimitWindow   = 15 * time.Minute
)

var (
	ErrUserAlreadyRegistered = errors.New("services: user already registered")
	ErrUserNotRegistered     = errors.New("services: user not registered")
	ErrOTPNotIssued          = errors.New("services: otp not issued")
	ErrOTPIncorrect          = errors.New("services: otp incorrect")
	ErrOTPAlreadyUsed        = errors.New("services: otp already used")
	ErrOTPExpired            = errors.New("services: otp expired")
	ErrOTPInvalidState       = errors.New("services: otp invalid state")
	ErrOTPRateLimited        = errors.New("services: otp attempts rate limited")
	ErrInvalidEmail          = errors.New("services: invalid email")
	ErrInvalidName           = errors.New("services: invalid name")
	ErrInvalidOTPFormat      = errors.New("services: invalid otp format")
)

type RegisteredUser struct {
	ID        int64
	Email     string
	FirstName string
	LastName  string
}

type userRepository interface {
	Insert(ctx context.Context, email, firstName, lastName string) (*repositories.User, error)
	FindByEmail(ctx context.Context, email string) (*repositories.User, error)
	FindForOTPVerify(ctx context.Context, email string) (*repositories.User, error)
	UpdateOTP(ctx context.Context, userID int64, codeHash []byte, issuedAt time.Time) error
	AtomicConsumeOTP(ctx context.Context, userID int64, expectedHash []byte) (bool, error)
}

type otpAttemptLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	now      func() time.Time
}

func newOTPAttemptLimiter(nowFn func() time.Time) *otpAttemptLimiter {
	return &otpAttemptLimiter{
		attempts: make(map[string][]time.Time),
		now:      nowFn,
	}
}

func (l *otpAttemptLimiter) cleanupLocked(cutoff time.Time) {
	for email, list := range l.attempts {
		kept := list[:0]
		for _, t := range list {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.attempts, email)
		} else {
			l.attempts[email] = kept
		}
	}
}

func (l *otpAttemptLimiter) isLimited(email string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	cutoff := now.Add(-otpRateLimitWindow)
	l.cleanupLocked(cutoff)
	return len(l.attempts[email]) >= otpRateLimitAttempts
}

func (l *otpAttemptLimiter) recordFailure(email string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	cutoff := now.Add(-otpRateLimitWindow)
	l.cleanupLocked(cutoff)
	if len(l.attempts[email]) >= otpRateLimitAttempts {
		return true
	}
	l.attempts[email] = append(l.attempts[email], now)
	return false
}

type AuthService struct {
	users   userRepository
	timeNow func() time.Time
	otpRng  func() (int, error)
	limiter *otpAttemptLimiter
}

type AuthServiceOption func(*AuthService)

func WithTimeNow(fn func() time.Time) AuthServiceOption {
	return func(s *AuthService) {
		s.timeNow = fn
		if s.limiter != nil {
			s.limiter.now = fn
		}
	}
}

func WithOTPRNG(fn func() (int, error)) AuthServiceOption {
	return func(s *AuthService) { s.otpRng = fn }
}

func NewAuthService(users userRepository, opts ...AuthServiceOption) *AuthService {
	s := &AuthService{
		users:   users,
		timeNow: time.Now,
		otpRng:  cryptoOTP,
	}
	s.limiter = newOTPAttemptLimiter(s.timeNow)
	for _, o := range opts {
		o(s)
	}
	return s
}

func cryptoOTP() (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(otpDenominator)))
	if err != nil {
		return 0, fmt.Errorf("services: generate otp: %w", err)
	}
	return int(n.Int64()), nil
}

func formatOTP(n int) string {
	return fmt.Sprintf("%0*d", otpNumDigits, n)
}

func hashOTP(code string) []byte {
	sum := sha256.Sum256([]byte(code))
	return sum[:]
}

func trimSpace(s string) string { return strings.TrimSpace(s) }

func normalizeEmail(s string) string { return strings.ToLower(trimSpace(s)) }

func looksLikeEmail(s string) bool {
	n := normalizeEmail(s)
	if n == "" {
		return false
	}
	at := strings.IndexByte(n, '@')
	return at > 0 && at < len(n)-1
}

func validateRegistrationFields(email, firstName, lastName string) error {
	if !looksLikeEmail(email) {
		return ErrInvalidEmail
	}
	if trimSpace(firstName) == "" {
		return ErrInvalidName
	}
	if trimSpace(lastName) == "" {
		return ErrInvalidName
	}
	return nil
}

func validateOTPFormat(code string) error {
	if len(code) != otpNumDigits {
		return ErrInvalidOTPFormat
	}
	for i := 0; i < len(code); i++ {
		c := code[i]
		if c < '0' || c > '9' {
			return ErrInvalidOTPFormat
		}
	}
	return nil
}

func (s *AuthService) Register(ctx context.Context, email, firstName, lastName string) (string, *RegisteredUser, error) {
	if err := validateRegistrationFields(email, firstName, lastName); err != nil {
		return "", nil, err
	}
	normEmail := normalizeEmail(email)
	first := trimSpace(firstName)
	last := trimSpace(lastName)

	n, err := s.otpRng()
	if err != nil {
		return "", nil, err
	}
	plaintext := formatOTP(n)
	codeHash := hashOTP(plaintext)
	issuedAt := s.timeNow().UTC()

	u, err := s.users.Insert(ctx, normEmail, first, last)
	if err != nil {
		if errors.Is(err, repositories.ErrDuplicateEmail) {
			return "", nil, ErrUserAlreadyRegistered
		}
		return "", nil, fmt.Errorf("services: register insert: %w", err)
	}
	if err := s.users.UpdateOTP(ctx, u.ID, codeHash, issuedAt); err != nil {
		return "", nil, fmt.Errorf("services: register update otp: %w", err)
	}
	return plaintext, &RegisteredUser{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}, nil
}

func (s *AuthService) LookupRegistered(ctx context.Context, email string) (bool, *RegisteredUser, error) {
	if !looksLikeEmail(email) {
		return false, nil, ErrInvalidEmail
	}
	e := normalizeEmail(email)
	u, err := s.users.FindByEmail(ctx, e)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("services: lookup: %w", err)
	}
	return true, &RegisteredUser{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}, nil
}

func (s *AuthService) VerifyOTP(ctx context.Context, email, code string) (*RegisteredUser, error) {
	if !looksLikeEmail(email) {
		return nil, ErrInvalidEmail
	}
	e := normalizeEmail(email)
	if err := validateOTPFormat(code); err != nil {
		return nil, err
	}
	if s.limiter.isLimited(e) {
		return nil, ErrOTPRateLimited
	}
	u, err := s.users.FindForOTPVerify(ctx, e)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			if s.limiter.recordFailure(e) {
				return nil, ErrOTPRateLimited
			}
			return nil, ErrOTPNotIssued
		}
		return nil, fmt.Errorf("services: verify find: %w", err)
	}
	if u.OTPUsedAt != nil {
		if s.limiter.recordFailure(e) {
			return nil, ErrOTPRateLimited
		}
		return nil, ErrOTPAlreadyUsed
	}
	if u.OTPIssuedAt == nil || len(u.OTPCodeHash) == 0 {
		return nil, ErrOTPInvalidState
	}
	issued := u.OTPIssuedAt.UTC()
	now := s.timeNow().UTC()
	if now.Before(issued) || !now.Before(issued.Add(otpTTL)) {
		if s.limiter.recordFailure(e) {
			return nil, ErrOTPRateLimited
		}
		return nil, ErrOTPExpired
	}
	submitted := hashOTP(code)
	if subtle.ConstantTimeCompare(submitted, u.OTPCodeHash) != 1 {
		if s.limiter.recordFailure(e) {
			return nil, ErrOTPRateLimited
		}
		return nil, ErrOTPIncorrect
	}
	consumed, err := s.users.AtomicConsumeOTP(ctx, u.ID, submitted)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return nil, ErrOTPIncorrect
		}
		return nil, fmt.Errorf("services: verify atomic consume: %w", err)
	}
	if !consumed {
		if s.limiter.recordFailure(e) {
			return nil, ErrOTPRateLimited
		}
		return nil, ErrOTPIncorrect
	}
	return &RegisteredUser{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}, nil
}

func (s *AuthService) ReissueOTP(ctx context.Context, email string) (string, *RegisteredUser, error) {
	if !looksLikeEmail(email) {
		return "", nil, ErrInvalidEmail
	}
	e := normalizeEmail(email)
	u, err := s.users.FindByEmail(ctx, e)
	if err != nil {
		if errors.Is(err, repositories.ErrUserNotFound) {
			return "", nil, ErrUserNotRegistered
		}
		return "", nil, fmt.Errorf("services: reissue find: %w", err)
	}
	n, err := s.otpRng()
	if err != nil {
		return "", nil, err
	}
	plaintext := formatOTP(n)
	codeHash := hashOTP(plaintext)
	issuedAt := s.timeNow().UTC()
	if err := s.users.UpdateOTP(ctx, u.ID, codeHash, issuedAt); err != nil {
		return "", nil, fmt.Errorf("services: reissue update otp: %w", err)
	}
	return plaintext, &RegisteredUser{
		ID:        u.ID,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}, nil
}
