package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/authix/authix/internal/repositories"
)

type fakeUserRepo struct {
	insertFn           func(ctx context.Context, email, first, last string) (*repositories.User, error)
	findByEmailFn      func(ctx context.Context, email string) (*repositories.User, error)
	findByIDFn         func(ctx context.Context, id int64) (*repositories.User, error)
	findForOTPVerifyFn func(ctx context.Context, email string) (*repositories.User, error)
	updateOTPFn        func(ctx context.Context, id int64, hash []byte, issued time.Time) error
	atomicConsumeOTPFn func(ctx context.Context, id int64, expectedHash []byte) (bool, error)
}

func (f *fakeUserRepo) Insert(ctx context.Context, email, first, last string) (*repositories.User, error) {
	if f.insertFn != nil {
		return f.insertFn(ctx, email, first, last)
	}
	return nil, nil
}
func (f *fakeUserRepo) FindByEmail(ctx context.Context, email string) (*repositories.User, error) {
	if f.findByEmailFn != nil {
		return f.findByEmailFn(ctx, email)
	}
	return nil, repositories.ErrUserNotFound
}
func (f *fakeUserRepo) FindByID(ctx context.Context, id int64) (*repositories.User, error) {
	if f.findByIDFn != nil {
		return f.findByIDFn(ctx, id)
	}
	return nil, repositories.ErrUserNotFound
}
func (f *fakeUserRepo) FindForOTPVerify(ctx context.Context, email string) (*repositories.User, error) {
	if f.findForOTPVerifyFn != nil {
		return f.findForOTPVerifyFn(ctx, email)
	}
	return nil, repositories.ErrUserNotFound
}
func (f *fakeUserRepo) UpdateOTP(ctx context.Context, id int64, hash []byte, issued time.Time) error {
	if f.updateOTPFn != nil {
		return f.updateOTPFn(ctx, id, hash, issued)
	}
	return nil
}
func (f *fakeUserRepo) AtomicConsumeOTP(ctx context.Context, id int64, expectedHash []byte) (bool, error) {
	if f.atomicConsumeOTPFn != nil {
		return f.atomicConsumeOTPFn(ctx, id, expectedHash)
	}
	return true, nil
}

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }
func fixedOTP(n int) func() (int, error)      { return func() (int, error) { return n, nil } }

func ptr[T any](v T) *T { return &v }

func TestOTPFormatAndRange(t *testing.T) {
	for _, n := range []int{0, 1, 42, 12345, 99999, 123456, 999999} {
		got := formatOTP(n)
		if len(got) != otpNumDigits {
			t.Errorf("n=%d: length want %d got %d (%q)", n, otpNumDigits, len(got), got)
		}
		if len(got) > 0 {
			for i := 0; i < len(got); i++ {
				c := got[i]
				if c < '0' || c > '9' {
					t.Errorf("n=%d: non-digit char at %d: %q", n, i, got)
					break
				}
			}
		}
	}
}

func TestOTPHash_Deterministic(t *testing.T) {
	a := hashOTP("123456")
	b := hashOTP("123456")
	c := hashOTP("654321")
	if !bytes.Equal(a, b) {
		t.Errorf("same input produced different hashes: %x vs %x", a, b)
	}
	if bytes.Equal(a, c) {
		t.Errorf("different inputs produced same hash")
	}
	if len(a) != sha256.Size {
		t.Errorf("hash length want %d got %d", sha256.Size, len(a))
	}
}

func TestAuthService_Register_SuccessAndIssueOTP(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	const issuedOTP = 42
	var updatedUserID int64
	var updatedHash []byte
	var updatedIssued time.Time
	users := &fakeUserRepo{
		insertFn: func(ctx context.Context, email, first, last string) (*repositories.User, error) {
			if email != "alice@example.com" || first != "Alice" || last != "Smith" {
				t.Fatalf("insert args unexpected: %q %q %q", email, first, last)
			}
			return &repositories.User{ID: 7, Email: email, FirstName: first, LastName: last, CreatedAt: now}, nil
		},
		updateOTPFn: func(ctx context.Context, id int64, hash []byte, issued time.Time) error {
			updatedUserID = id
			updatedHash = hash
			updatedIssued = issued
			return nil
		},
	}
	svc := NewAuthService(users, WithTimeNow(fixedClock(now)), WithOTPRNG(fixedOTP(issuedOTP)))

	code, user, err := svc.Register(context.Background(), "  Alice@Example.COM  ", " Alice ", " Smith ")
	if err != nil {
		t.Fatalf("Register error: %v", err)
	}
	wantCode := formatOTP(issuedOTP)
	if code != wantCode {
		t.Errorf("code want %q got %q", wantCode, code)
	}
	if user == nil || user.ID != 7 || user.Email != "alice@example.com" {
		t.Errorf("registered user mismatch: %+v", user)
	}
	if updatedUserID != 7 {
		t.Errorf("UpdateOTP called for user %d, want 7", updatedUserID)
	}
	if !updatedIssued.Equal(now) {
		t.Errorf("issuedAt want %v got %v", now, updatedIssued)
	}
	if !bytes.Equal(updatedHash, hashOTP(wantCode)) {
		t.Errorf("stored hash mismatch")
	}
}

func TestAuthService_Register_Validation(t *testing.T) {
	users := &fakeUserRepo{}
	svc := NewAuthService(users)
	ctx := context.Background()

	cases := []struct {
		name    string
		email   string
		first   string
		last    string
		wantErr error
	}{
		{"empty email", "", "A", "B", ErrInvalidEmail},
		{"no @", "notanemail", "A", "B", ErrInvalidEmail},
		{"whitespace only email", "   ", "A", "B", ErrInvalidEmail},
		{"empty first", "a@b.com", "", "B", ErrInvalidName},
		{"whitespace first", "a@b.com", "  \t", "B", ErrInvalidName},
		{"empty last", "a@b.com", "A", "", ErrInvalidName},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := svc.Register(ctx, c.email, c.first, c.last)
			if !errors.Is(err, c.wantErr) {
				t.Errorf("want %v got %v", c.wantErr, err)
			}
		})
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	users := &fakeUserRepo{
		insertFn: func(ctx context.Context, email, first, last string) (*repositories.User, error) {
			return nil, repositories.ErrDuplicateEmail
		},
	}
	svc := NewAuthService(users, WithOTPRNG(fixedOTP(123456)))
	_, _, err := svc.Register(context.Background(), "alice@example.com", "A", "S")
	if !errors.Is(err, ErrUserAlreadyRegistered) {
		t.Errorf("want ErrUserAlreadyRegistered got %v", err)
	}
}

func TestAuthService_Register_GenericDBErrorWrapped(t *testing.T) {
	boom := errors.New("boom")
	users := &fakeUserRepo{
		insertFn: func(ctx context.Context, email, first, last string) (*repositories.User, error) {
			return nil, boom
		},
	}
	svc := NewAuthService(users)
	_, _, err := svc.Register(context.Background(), "a@b.com", "A", "B")
	if err == nil || !strings.Contains(err.Error(), "register insert") {
		t.Errorf("want wrapped insert error got %v", err)
	}
	if !errors.Is(err, boom) {
		t.Errorf("original error should be preserved via errors.Is chain: %v", err)
	}
}

func TestAuthService_LookupRegistered(t *testing.T) {
	now := time.Now()
	users := &fakeUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*repositories.User, error) {
			if email == "registered@example.com" {
				return &repositories.User{ID: 1, Email: email, FirstName: "F", LastName: "L", CreatedAt: now}, nil
			}
			return nil, repositories.ErrUserNotFound
		},
	}
	svc := NewAuthService(users)
	ctx := context.Background()

	found, u, err := svc.LookupRegistered(ctx, "  Registered@Example.COM  ")
	if err != nil {
		t.Fatalf("lookup err: %v", err)
	}
	if !found || u == nil || u.ID != 1 {
		t.Errorf("registered lookup want found=true user={1,...} got found=%v u=%+v", found, u)
	}

	found, u, err = svc.LookupRegistered(ctx, "missing@example.com")
	if err != nil {
		t.Fatalf("missing lookup err: %v", err)
	}
	if found || u != nil {
		t.Errorf("missing lookup want found=false nil user, got found=%v u=%+v", found, u)
	}

	_, _, err = svc.LookupRegistered(ctx, "   ")
	if !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("empty email want ErrInvalidEmail got %v", err)
	}
}

func TestAuthService_VerifyOTP_Success(t *testing.T) {
	now := time.Now()
	const code = "999888"
	hash := hashOTP(code)
	var consumeID int64
	var consumeHash []byte
	users := &fakeUserRepo{
		findForOTPVerifyFn: func(ctx context.Context, email string) (*repositories.User, error) {
			if email != "bob@example.com" {
				return nil, repositories.ErrUserNotFound
			}
			return &repositories.User{
				ID: 2, Email: email, FirstName: "Bob", LastName: "Jones", CreatedAt: now,
				OTPCodeHash: hash, OTPIssuedAt: &now, OTPUsedAt: nil,
			}, nil
		},
		atomicConsumeOTPFn: func(ctx context.Context, id int64, expected []byte) (bool, error) {
			consumeID = id
			consumeHash = expected
			return true, nil
		},
	}
	svc := NewAuthService(users, WithTimeNow(fixedClock(now)))
	user, err := svc.VerifyOTP(context.Background(), "Bob@Example.COM", code)
	if err != nil {
		t.Fatalf("verify err: %v", err)
	}
	if user == nil || user.ID != 2 || user.FirstName != "Bob" {
		t.Errorf("verify user mismatch: %+v", user)
	}
	if consumeID != 2 {
		t.Errorf("AtomicConsumeOTP called for user %d, want 2", consumeID)
	}
	if !bytes.Equal(consumeHash, hash) {
		t.Errorf("AtomicConsumeOTP received different hash bytes than expected sha256(code)")
	}
}

func TestAuthService_VerifyOTP_Incorrect(t *testing.T) {
	now := time.Now()
	users := &fakeUserRepo{
		findForOTPVerifyFn: func(ctx context.Context, email string) (*repositories.User, error) {
			return &repositories.User{
				ID: 3, Email: email, FirstName: "X", LastName: "Y", CreatedAt: now,
				OTPCodeHash: hashOTP("000000"), OTPIssuedAt: &now, OTPUsedAt: nil,
			}, nil
		},
	}
	svc := NewAuthService(users)
	_, err := svc.VerifyOTP(context.Background(), "x@y.com", "999999")
	if !errors.Is(err, ErrOTPIncorrect) {
		t.Errorf("want ErrOTPIncorrect got %v", err)
	}
}

func TestAuthService_VerifyOTP_AlreadyUsed(t *testing.T) {
	now := time.Now()
	used := now.Add(time.Minute)
	const code = "111222"
	users := &fakeUserRepo{
		findForOTPVerifyFn: func(ctx context.Context, email string) (*repositories.User, error) {
			return &repositories.User{
				ID: 4, Email: email, FirstName: "A", LastName: "B", CreatedAt: now,
				OTPCodeHash: hashOTP(code), OTPIssuedAt: &now, OTPUsedAt: &used,
			}, nil
		},
	}
	svc := NewAuthService(users)
	_, err := svc.VerifyOTP(context.Background(), "a@b.com", code)
	if !errors.Is(err, ErrOTPAlreadyUsed) {
		t.Errorf("want ErrOTPAlreadyUsed got %v", err)
	}
}

func TestAuthService_VerifyOTP_Expiry(t *testing.T) {
	issued := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	code := "123456"
	users := &fakeUserRepo{findForOTPVerifyFn: func(context.Context, string) (*repositories.User, error) {
		return &repositories.User{ID: 5, Email: "expiry@example.com", OTPCodeHash: hashOTP(code), OTPIssuedAt: &issued}, nil
	}}
	for name, now := range map[string]time.Time{
		"within lifetime":     issued.Add(otpTTL - time.Second),
		"exactly at boundary": issued.Add(otpTTL),
		"expired":             issued.Add(otpTTL + time.Second),
		"future issue":        issued.Add(-time.Second),
	} {
		t.Run(name, func(t *testing.T) {
			svc := NewAuthService(users, WithTimeNow(fixedClock(now)))
			_, err := svc.VerifyOTP(context.Background(), "expiry@example.com", code)
			if name == "within lifetime" {
				if err != nil {
					t.Fatalf("valid code rejected: %v", err)
				}
				return
			}
			if name == "future issue" {
				if !errors.Is(err, ErrOTPExpired) {
					t.Errorf("future issue want ErrOTPExpired, got %v", err)
				}
				return
			}
			if !errors.Is(err, ErrOTPExpired) {
				t.Errorf("want ErrOTPExpired, got %v", err)
			}
		})
	}
}

func TestAuthService_VerifyOTP_RateLimitWindowAndSuccess(t *testing.T) {
	current := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	var nowMu sync.Mutex
	clock := func() time.Time {
		nowMu.Lock()
		defer nowMu.Unlock()
		return current
	}
	users := &fakeUserRepo{findForOTPVerifyFn: func(context.Context, string) (*repositories.User, error) {
		issued := current.Add(-time.Minute)
		return &repositories.User{ID: 6, Email: "limit@example.com", OTPCodeHash: hashOTP("000000"), OTPIssuedAt: &issued}, nil
	}}
	svc := NewAuthService(users, WithTimeNow(clock))
	for i := 0; i < otpRateLimitAttempts; i++ {
		_, err := svc.VerifyOTP(context.Background(), "limit@example.com", "111111")
		if !errors.Is(err, ErrOTPIncorrect) {
			t.Fatalf("failure %d want incorrect, got %v", i+1, err)
		}
	}
	if _, err := svc.VerifyOTP(context.Background(), "limit@example.com", "111111"); !errors.Is(err, ErrOTPRateLimited) {
		t.Fatalf("sixth attempt want rate limited, got %v", err)
	}

	nowMu.Lock()
	current = current.Add(otpRateLimitWindow)
	nowMu.Unlock()
	if _, err := svc.VerifyOTP(context.Background(), "limit@example.com", "000000"); err != nil {
		t.Fatalf("valid code after window rejected: %v", err)
	}
}

func TestAuthService_VerifyOTP_ConcurrentFailuresAreLimited(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	issued := now.Add(-time.Minute)
	users := &fakeUserRepo{findForOTPVerifyFn: func(context.Context, string) (*repositories.User, error) {
		return &repositories.User{ID: 8, Email: "race@example.com", OTPCodeHash: hashOTP("000000"), OTPIssuedAt: &issued}, nil
	}}
	svc := NewAuthService(users, WithTimeNow(fixedClock(now)))
	var wg sync.WaitGroup
	var limited atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.VerifyOTP(context.Background(), "race@example.com", "111111")
			if errors.Is(err, ErrOTPRateLimited) {
				limited.Add(1)
			}
		}()
	}
	wg.Wait()
	if limited.Load() == 0 {
		t.Fatal("concurrent failures never reached the rate limit")
	}
}

func TestAuthService_VerifyOTP_NotIssued(t *testing.T) {
	users := &fakeUserRepo{
		findForOTPVerifyFn: func(ctx context.Context, email string) (*repositories.User, error) {
			return nil, repositories.ErrUserNotFound
		},
	}
	svc := NewAuthService(users)
	_, err := svc.VerifyOTP(context.Background(), "nobody@example.com", "123456")
	if !errors.Is(err, ErrOTPNotIssued) {
		t.Errorf("want ErrOTPNotIssued got %v", err)
	}
}

func TestAuthService_VerifyOTP_BadFormat(t *testing.T) {
	users := &fakeUserRepo{}
	svc := NewAuthService(users)
	ctx := context.Background()

	bad := []string{"", "1", "12345", "1234567", "abcdef", "12345a", "12 456"}
	for _, c := range bad {
		_, err := svc.VerifyOTP(ctx, "x@y.com", c)
		if !errors.Is(err, ErrInvalidOTPFormat) {
			t.Errorf("code %q: want ErrInvalidOTPFormat got %v", c, err)
		}
	}
}

func TestAuthService_VerifyOTP_InvalidEmail(t *testing.T) {
	svc := NewAuthService(&fakeUserRepo{})
	_, err := svc.VerifyOTP(context.Background(), "", "123456")
	if !errors.Is(err, ErrInvalidEmail) {
		t.Errorf("want ErrInvalidEmail got %v", err)
	}
}

func TestAuthService_ReissueOTP_ResetsUsedAndReplacesHash(t *testing.T) {
	now := time.Date(2026, 9, 23, 14, 0, 0, 0, time.UTC)
	issued := now.Add(-time.Hour)
	used := now.Add(-30 * time.Minute)
	var newID int64
	var newHash []byte
	var newIssued time.Time
	users := &fakeUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*repositories.User, error) {
			if email != "carol@example.com" {
				return nil, repositories.ErrUserNotFound
			}
			return &repositories.User{
				ID: 9, Email: email, FirstName: "Carol", LastName: "Davis", CreatedAt: now,
				OTPCodeHash: hashOTP("oldcode"), OTPIssuedAt: &issued, OTPUsedAt: &used,
			}, nil
		},
		updateOTPFn: func(ctx context.Context, id int64, hash []byte, issuedAt time.Time) error {
			newID = id
			newHash = hash
			newIssued = issuedAt
			return nil
		},
	}
	svc := NewAuthService(users, WithTimeNow(fixedClock(now)), WithOTPRNG(fixedOTP(777)))

	newCode, user, err := svc.ReissueOTP(context.Background(), "  Carol@Example.COM ")
	if err != nil {
		t.Fatalf("Reissue err: %v", err)
	}
	wantCode := formatOTP(777)
	if newCode != wantCode {
		t.Errorf("new code want %q got %q", wantCode, newCode)
	}
	if user == nil || user.ID != 9 {
		t.Errorf("reissue user mismatch: %+v", user)
	}
	if newID != 9 {
		t.Errorf("UpdateOTP id want 9 got %d", newID)
	}
	if !newIssued.Equal(now) {
		t.Errorf("new issuedAt want %v got %v", now, newIssued)
	}
	if !bytes.Equal(newHash, hashOTP(wantCode)) {
		t.Errorf("new hash does not match reissued code")
	}
}

func TestAuthService_ReissueOTP_NotRegistered(t *testing.T) {
	users := &fakeUserRepo{
		findByEmailFn: func(ctx context.Context, email string) (*repositories.User, error) {
			return nil, repositories.ErrUserNotFound
		},
	}
	svc := NewAuthService(users)
	_, _, err := svc.ReissueOTP(context.Background(), "missing@example.com")
	if !errors.Is(err, ErrUserNotRegistered) {
		t.Errorf("want ErrUserNotRegistered got %v", err)
	}
}

func TestAuthService_CryptoRNG_Bounds(t *testing.T) {
	const samples = 2000
	seen := map[int]bool{}
	outOfRange := 0
	for i := 0; i < samples; i++ {
		n, err := cryptoOTP()
		if err != nil {
			t.Fatalf("cryptoOTP err: %v", err)
		}
		if n < 0 || n >= otpDenominator {
			outOfRange++
		}
		seen[n] = 0 != 0
		_ = seen
	}
	if outOfRange != 0 {
		t.Errorf("%d OTP samples out of range", outOfRange)
	}
}

func TestValidateOTPFormat(t *testing.T) {
	good := []string{"000000", "123456", "999999", "000001"}
	for _, c := range good {
		if err := validateOTPFormat(c); err != nil {
			t.Errorf("code %q unexpectedly invalid: %v", c, err)
		}
	}
	bad := []string{"", "0", "00000", "0000000", "abcdef", "12345a", " 12345", "12345 ", "12 456"}
	for _, c := range bad {
		if err := validateOTPFormat(c); err == nil {
			t.Errorf("code %q unexpectedly valid", c)
		}
	}
}

func TestAuthService_Register_RepoErrorsMapped(t *testing.T) {
	updateErr := fmt.Errorf("repositories: update otp: %w", errors.New("db down"))
	users := &fakeUserRepo{
		insertFn: func(ctx context.Context, email, first, last string) (*repositories.User, error) {
			return &repositories.User{ID: 1, Email: email, FirstName: first, LastName: last}, nil
		},
		updateOTPFn: func(ctx context.Context, id int64, hash []byte, issued time.Time) error {
			return updateErr
		},
	}
	svc := NewAuthService(users, WithOTPRNG(fixedOTP(1)))
	_, _, err := svc.Register(context.Background(), "a@b.com", "A", "B")
	if err == nil {
		t.Fatalf("want error, got nil")
	}
	if !strings.Contains(err.Error(), "register update otp") {
		t.Errorf("error should describe UpdateOTP failure, got %v", err)
	}
}
