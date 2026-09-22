# Architecture

## Overview

Authix is a three-layer web application:

```
┌─────────────────────────────────────────────────────────────┐
│                     Frontend                        │
│              React + TypeScript + Vite                 │
│  Landing page, registration form, checkout form, OTP modal  │
│  Real-time email validation, async user lookup    │
└──────────────────────┬──────────────────────────────────┘
                       │ HTTP / JSON
                       ▼
┌──────────────────────────────────────────────────────────────┐
│                        Go API                              │
│  cmd/api/main.go  — entry point, HTTP server, router       │
│  internal/config  — env-based configuration                      │
│  internal/handlers — HTTP layer: decode req, encode resp     │
│  internal/services — business rules, OTP generation, validation │
│  internal/repositories — data access (SQL queries)         │
│  internal/database — PostgreSQL connection pool             │
└──────────────────────┬───────────────────────────────────────┘
                       │ SQL
                       ▼
┌──────────────────────────────────────────────────────────────┐
│                      PostgreSQL                             │
│  users, checkouts tables (planned)                       │
│  managed via SQL migrations in database/migrations/      │
└──────────────────────────────────────────────────────────────┘
```

## Layer Responsibilities

### Frontend

Single-page React app built with Vite. Responsibility:
- Render forms (registration, checkout) and UI (landing page, OTP modal).
- Client-side validation (email format in real time).
- Call the Go API over HTTP/JSON.
- React to responses: show OTP modal when a registered email is detected; show logged-in name; show errors.
- No business logic about OTP correctness or user registration state beyond what the API returns.

Folders inside `frontend/src/`:
- `components/` — shared presentational components.
- `features/` — feature-scoped modules (registration, checkout).
- `lib/` — shared utilities.
- `services/` — API client wrappers.
- `types/` — shared TypeScript types.

Implemented in Phase 1: scaffold and landing page only. Forms are planned.

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


## Typical Request Flow (Planned)

1. User submits registration form → Frontend POSTs JSON to `POST /api/register`.
2. Handler validates payload → service generates 6-digit numeric code → repository inserts user → handler returns code to frontend.
3. On checkout, after email field becomes valid → Frontend calls `GET /api/users/lookup?email=...`.
4. If registered → Frontend opens OTP modal.
5. User enters code → Frontend POSTs to `POST /api/login` → service compares against stored code.
6. On match → frontend receives user name → closes modal and shows greeting at top of checkout form.
7. User submits checkout → Frontend POSTs `POST /api/checkout` → repository saves checkout row.

## What Is Implemented vs Planned

Implemented (Phase 1):
- ✅ Frontend scaffold (Vite + React + TS) + landing page
- ✅ Backend entry point and router
- ✅ Backend `GET /health` + unit test
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

Planned:
- Registration form page and API endpoint
- OTP generation (6-digit numeric, hashed on save)
- Email-owner lookup endpoint
- OTP verify endpoint
- Checkout form + validation + OTP modal UI
- Checkout submission endpoint
- Deployment + hosting

## Principles

- No unnecessary frameworks. Go stdlib for HTTP. React without state libraries until needed.
- Each layer is independently runnable locally. Frontend dev server on 5173, API on 8080, DB on 5432.
- Environment variables for all configuration. No hardcoded secrets.
- Small, focused functions. No overengineering for an assessment.
