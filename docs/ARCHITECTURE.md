# Architecture

## Overview

Authix is a three-layer web application:

```
┌──────────────────────────────────────────────────────────────┐
│                         Frontend                             │
│                 React + TypeScript + Vite                    │
│       Landing page, registration, checkout, and OTP modal    │
│          Real-time email validation and async lookup         │
└──────────────────────────────┬───────────────────────────────┘
                               │ HTTP / JSON
                               ▼
┌──────────────────────────────────────────────────────────────┐
│                           Go API                             │
│        cmd/api/main.go - entry point, HTTP server, router    │
│        internal/config - environment-based configuration     │
│        internal/handlers - HTTP request and response layer   │
│        internal/services - business rules and validation     │
│        internal/repositories - SQL data access               │
│        internal/database - PostgreSQL connection pool        │
└──────────────────────────────┬───────────────────────────────┘
                               │ SQL
                               ▼
┌──────────────────────────────────────────────────────────────┐
│                         PostgreSQL                           │
│                 users, checkouts, and sessions               │
│             Managed via SQL migrations in database/          │
└──────────────────────────────────────────────────────────────┘
```

## Layer Responsibilities

### Frontend

Single-page React app built with Vite. Responsibility:
- Render registration, checkout, and OTP modal UI.
- Validate email format in real time and trigger registered-email recognition.
- Call the Go API over HTTP/JSON with credentialed requests.
- React to responses: show OTP modal for recognized users, allow guest checkout, sign in with a valid OTP, and show the authenticated user name at checkout.
- Keep payment logic out of scope and persist only checkout data through the API.

Folders inside `frontend/src/`:
- `components/` — shared presentational components.
- `features/` — feature-scoped modules (registration, checkout, email recognition).
- `lib/` — API client and validation helpers.
- `styles/` — app styling via the main CSS entry point.

The current working tree includes the full app flow: registration form, live email recognition hook, OTP modal, authenticated/guest checkout, and cookie-backed session restoration.

### Go API

Standard-library `net/http` server. Responsibility:
- Expose HTTP endpoints, validate request payloads, return JSON.
- Encode business rules (generate OTP, match OTP, save checkout).
- Read/write PostgreSQL through a data-access layer so HTTP handlers stay thin.

Package layout:
- `cmd/api/main.go` — wire dependencies and start server.
- `internal/config/` — read PORT, POSTGRES_* from env.
- `internal/handlers/` — per-endpoint funcs returning `http.HandlerFunc`.
- `internal/services/` — pure-logic services (planned after foundation).
- `internal/repositories/` — SQL wrapping (planned).
- `internal/database/` — open pool (planned).

Implemented in Phase 1: `GET /health` only. All other endpoints planned.

### PostgreSQL

Relational persistence. Responsibility:
- Store registered users and the issued OTP code.
- Store checkout submissions.

Schema lives in `database/migrations/` as ordered `.sql` files. Migrations
are applied automatically at API startup by the Go binary in numerical
order; already-applied versions are tracked in a `schema_migrations` table
so re-runs are idempotent and already-applied files are never re-executed.

Connected to the Go API in Phase 2: pool, startup readiness check, and
migration runner all live in `internal/database/`.

#### Schema notes (Phase 2)

- **users** (see `0001_extensions_and_users.sql`): `email` is stored as
  `CITEXT` with a unique constraint, so `Alice@Example.com` and
  `alice@example.com` are treated as the same user.
- **OTP is hashed, not stored as plaintext.** The six-digit login code is
  returned by the API ONCE for screen display at registration time. After
  that, only the hash is kept in `users.otp_code_hash` (BYTEA) alongside
  `otp_issued_at` and `otp_used_at`. This avoids keeping a reusable
  plaintext credential at rest and lets us flip `otp_used_at` to block a
  captured code from being replayed.
- **checkouts** (see `0002_checkouts.sql`): `user_id` is nullable with
  `ON DELETE SET NULL` so guest checkouts work and historical submissions
  are preserved even if a user is deleted. Email, phone and shipping
  fields are denormalised onto the submission row so each row records
  exactly what the user submitted, independent of later profile edits.
- **sessions** (see `0003_sessions.sql`): successful OTP verification creates
  a server-side session. The client receives only an opaque token for
  an HttpOnly cookie; PostgreSQL stores its hash, user ID, creation time,
  expiry, and optional revocation time. Checkout must use the trusted user ID
  from that session rather than a browser-supplied ID.


## Typical Request Flow (Planned)

1. User submits registration form → Frontend POSTs JSON to `POST /api/register`.
2. Handler validates payload → service generates 6-digit numeric code → repository inserts user → handler returns code to frontend.
3. On checkout, after email field becomes valid → Frontend calls `GET /api/users/lookup?email=...`.
4. If registered → Frontend opens OTP modal.
5. User enters code → Frontend POSTs to `POST /api/login` → service compares against stored code.
6. On match → the API creates a server-side session and sets an opaque HttpOnly cookie; frontend receives the user name and shows the greeting.
7. User submits checkout → the API resolves the optional user ID from the session cookie, never from an untrusted request field, then saves the checkout row.

The backend API now provides registration, authentication lookup/verify/reissue,
session-backed `/me` and logout, and checkout endpoints. Credentialed CORS is
limited to explicitly configured origins. The frontend remains the next phase.

## What Is Implemented vs Planned

Implemented (current working tree):
- ✅ Frontend registration and checkout application with OTP modal and guest flow
- ✅ Backend entry point, router, JSON API, and secure session cookies
- ✅ Backend `GET /health` + unit tests
- ✅ Docker Compose Postgres service definition

Implemented (Phase 2 — DB wiring):
- ✅ PostgreSQL schema: `users` + `checkouts` tables as ordered `.sql` migrations
  under `database/migrations/`, with CITEXT email, hashed OTP, non-empty CHECKs,
  and `schema_migrations` tracking table created by the runner.
- ✅ Go backend ↔ PostgreSQL wiring:
  - `internal/database/` — `lib/pq` driver, pool open/close, conservative
    pool sizes, `WaitForReady` startup check with escape hatch.
  - Migration runner: discovers files, orders by version, applies each
    unapplied migration in its own transaction, records version in
    `schema_migrations`, idempotent across restarts.
  - `cmd/api/main.go` wires pool open → readiness check → migrations →
    HTTP listen, preserving clean-shutdown behaviour.
- ✅ Config: `internal/config` reads `POSTGRES_*` and `MIGRATIONS_DIR` from
  env, defaults match `.env.example` + docker-compose.
- ✅ Tests: config parsing (defaults + overrides), offline migration
  discovery / ordering / duplicate-version / missing-dir / URL builder,
  and a live-DB integration test against a throwaway cluster (when PG
  binaries are on PATH; skipped in `-short` mode).

Implemented and verified:
- Registration form and API endpoint
- OTP generation (6-digit numeric, hashed on save)
- Registered-email lookup endpoint
- OTP verify and reissue endpoints
- Checkout form + validation + OTP modal UI
- Checkout submission endpoint
- Session restoration for authenticated users
- Deployment config and environment examples, without public hosting being performed here

## Principles

- No unnecessary frameworks. Go stdlib for HTTP. React without state libraries until needed.
- Each layer is independently runnable locally. Frontend dev server on 5173, API on 8080, DB on 5432.
- Environment variables for all configuration. No hardcoded secrets.
- Small, focused functions. No overengineering for an assessment.
