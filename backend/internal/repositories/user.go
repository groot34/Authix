package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/authix/authix/internal/database"
	"github.com/lib/pq"
)

var (
	ErrDuplicateEmail = errors.New("repositories: duplicate email")
	ErrUserNotFound   = errors.New("repositories: user not found")
)

type User struct {
	ID          int64
	Email       string
	FirstName   string
	LastName    string
	CreatedAt   time.Time
	OTPCodeHash []byte
	OTPIssuedAt *time.Time
	OTPUsedAt   *time.Time
}

type UserRepository struct {
	pool database.Pool
}

func NewUserRepository(pool database.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Insert(ctx context.Context, email, firstName, lastName string) (*User, error) {
	const q = `
INSERT INTO users (email, first_name, last_name)
VALUES ($1, $2, $3)
RETURNING id, email, first_name, last_name, created_at,
          otp_code_hash, otp_issued_at, otp_used_at;`

	u := &User{}
	var otpHash sql.Null[[]byte]
	var otpIssued, otpUsed sql.Null[time.Time]
	err := r.pool.DB().QueryRowContext(ctx, q, email, firstName, lastName).Scan(
		&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.CreatedAt,
		&otpHash, &otpIssued, &otpUsed,
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			switch pqErr.Code.Class() {
			case "23":
				switch pqErr.Constraint {
				case "users_email_unique":
					return nil, ErrDuplicateEmail
				}
				return nil, fmt.Errorf("repositories: insert user: constraint violation: %w", err)
			}
		}
		return nil, fmt.Errorf("repositories: insert user: %w", err)
	}
	if otpHash.Valid {
		u.OTPCodeHash = otpHash.V
	}
	if otpIssued.Valid {
		u.OTPIssuedAt = &otpIssued.V
	}
	if otpUsed.Valid {
		u.OTPUsedAt = &otpUsed.V
	}
	return u, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	const q = `
SELECT id, email, first_name, last_name, created_at,
       otp_code_hash, otp_issued_at, otp_used_at
FROM users
WHERE email = $1;`

	return r.queryUser(ctx, q, email)
}

func (r *UserRepository) FindByID(ctx context.Context, userID int64) (*User, error) {
	const q = `
SELECT id, email, first_name, last_name, created_at,
       otp_code_hash, otp_issued_at, otp_used_at
FROM users
WHERE id = $1;`

	return r.queryUser(ctx, q, userID)
}

func (r *UserRepository) FindForOTPVerify(ctx context.Context, email string) (*User, error) {
	const q = `
SELECT id, email, first_name, last_name, created_at,
       otp_code_hash, otp_issued_at, otp_used_at
FROM users
WHERE email = $1
  AND otp_code_hash IS NOT NULL
  AND otp_issued_at IS NOT NULL;`

	return r.queryUser(ctx, q, email)
}

func (r *UserRepository) UpdateOTP(ctx context.Context, userID int64, codeHash []byte, issuedAt time.Time) error {
	const q = `
UPDATE users
SET otp_code_hash = $1,
    otp_issued_at  = $2,
    otp_used_at    = NULL
WHERE id = $3;`

	res, err := r.pool.DB().ExecContext(ctx, q, codeHash, issuedAt, userID)
	if err != nil {
		return fmt.Errorf("repositories: update otp: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: update otp rows: %w", err)
	}
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) AtomicConsumeOTP(ctx context.Context, userID int64, expectedHash []byte) (bool, error) {
	const q = `
UPDATE users
SET otp_used_at = now()
WHERE id = $1
  AND otp_code_hash IS NOT NULL
  AND otp_issued_at  IS NOT NULL
  AND otp_used_at    IS NULL
  AND otp_code_hash  = $2;`

	res, err := r.pool.DB().ExecContext(ctx, q, userID, expectedHash)
	if err != nil {
		return false, fmt.Errorf("repositories: atomic consume otp: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("repositories: atomic consume otp rows: %w", err)
	}
	if n == 1 {
		return true, nil
	}
	const exists = `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`
	var found bool
	if err := r.pool.DB().QueryRowContext(ctx, exists, userID).Scan(&found); err != nil {
		return false, fmt.Errorf("repositories: atomic consume otp user check: %w", err)
	}
	if !found {
		return false, ErrUserNotFound
	}
	return false, nil
}

func (r *UserRepository) queryUser(ctx context.Context, query string, args ...any) (*User, error) {
	u := &User{}
	var otpHash sql.Null[[]byte]
	var otpIssued, otpUsed sql.Null[time.Time]
	err := r.pool.DB().QueryRowContext(ctx, query, args...).Scan(
		&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.CreatedAt,
		&otpHash, &otpIssued, &otpUsed,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("repositories: query user: %w", err)
	}
	if otpHash.Valid {
		u.OTPCodeHash = otpHash.V
	}
	if otpIssued.Valid {
		u.OTPIssuedAt = &otpIssued.V
	}
	if otpUsed.Valid {
		u.OTPUsedAt = &otpUsed.V
	}
	return u, nil
}
