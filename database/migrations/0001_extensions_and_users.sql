-- Migration 0001: extensions and users table
-- Run against the `authix` PostgreSQL database.

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS users (
    id               BIGSERIAL PRIMARY KEY,
    email            CITEXT       NOT NULL,
    first_name       TEXT         NOT NULL,
    last_name        TEXT         NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL  DEFAULT now(),

    --
    -- OTP representation
    --
    -- The six-digit code issued at registration is NOT stored as plaintext.
    -- The API generates the plaintext value, returns it to the client ONCE
    -- for display, then hashes it (e.g. SHA-256 or bcrypt) and stores only
    -- the hash here. Verification re-hashes the submitted code and compares
    -- it against otp_code_hash, then flips otp_used_at so a captured code
    -- cannot be replayed. If the user re-registers (or a new code is ever
    -- issued for any reason), otp_code_hash / otp_issued_at are replaced
    -- and otp_used_at is reset to NULL — but we never keep a plaintext copy
    -- around as a reusable credential.
    --
    otp_code_hash    BYTEA,
    otp_issued_at    TIMESTAMPTZ,
    otp_used_at      TIMESTAMPTZ,

    CONSTRAINT users_email_unique UNIQUE (email),
    CONSTRAINT users_name_nonempty CHECK (
        length(trim(first_name)) > 0
        AND length(trim(last_name)) > 0
    ),
    CONSTRAINT users_otp_fields_consistent CHECK (
        (otp_code_hash IS NULL AND otp_issued_at IS NULL AND otp_used_at IS NULL)
        OR (otp_code_hash IS NOT NULL AND otp_issued_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS users_email_idx ON users (email);
