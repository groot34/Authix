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
- Store registered users and their issued OTP code.
- Store checkout submissions.

Schema goes in `database/migrations/` as `.sql` files. Migrations/files will be applied with a simple tool (TBD — probably just `psql -f` initially, migrate tooling added when needed).

Not yet connected in Phase 1.

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

Planned:
- Registration form page and API endpoint
- OTP generation (6-digit numeric)
- Email-owner lookup endpoint
- OTP verify endpoint
- Checkout form + validation + OTP modal UI
- Checkout submission endpoint
- PostgreSQL schema + migrations
- Go backend ↔ PostgreSQL wiring
- Deployment + hosting

## Principles

- No unnecessary frameworks. Go stdlib for HTTP. React without state libraries until needed.
- Each layer is independently runnable locally. Frontend dev server on 5173, API on 8080, DB on 5432.
- Environment variables for all configuration. No hardcoded secrets.
- Small, focused functions. No overengineering for an assessment.
