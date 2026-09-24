# Authix

Authix is a full-stack web application built as a full-stack engineering assessment.

It demonstrates a simple two-flow application: (1) a user registers with email, first name, and last name, and receives a six-digit login code, and (2) on a checkout form, typing a valid registered email triggers a background check and a code-entry modal — if the code matches, the user is recognised; otherwise checkout continues as a guest. No real payment is processed, only submitted checkout data is persisted.

## Purpose

The project meets the assessment requirements while keeping the architecture clean and the codebase professional.

## Status

**Live and verified deployment available**

The Authix frontend is publicly reachable at https://frontend-woad-pi-43.vercel.app/ and returns an HTTP 200 response. The backend API is live at https://authix-wel2.onrender.com and responds on `/health`. The OTP registration flow, authenticated checkout flow, guest checkout, session restoration, and database-backed integration are implemented and verified.

The remaining external requirement is reviewer access for `boltapp-hiring`, if that access is required by the assessment process after the hosted app is accepted.

### Technology Stack

| Layer | Technology |
|---|---|
| Frontend | React + TypeScript + Vite |
| Backend | Go (`net/http`) |
| Database | PostgreSQL |
| Local DB | Docker Compose (PostgreSQL 16 Alpine) |
| SQL Driver | `github.com/lib/pq` via `database/sql` |
| Migrations | Ordered `.sql` files in `database/migrations/`, applied at API startup, tracked in `schema_migrations` |

## Implemented Features

- Frontend: Vite + React + TypeScript app with account registration, checkout form, OTP modal, guest/registered flow, and session-aware greeting
- Frontend: real-time email validation and background recognition for registered-email flows
- Backend: Go module `github.com/groot34/Authix` with API server entry point, health route, and JSON handlers
- Backend: registration, OTP lookup, verify, reissue, logout, and checkout endpoints
- Backend: HttpOnly session cookies with environment-aware Secure handling and configured same-site policy
- Backend: explicit credentialed CORS with configured frontend origins
- Database: PostgreSQL schema for `users`, `checkouts`, and `sessions` with ordered migrations and startup migration runner
- Backend ↔ PostgreSQL connection pool, readiness checks, and idempotent migration execution
- Business logic: OTP generation, hashing, expiry, rate limiting, atomic one-time consumption, checkout validation, and guest/authenticated checkout handling

## Deployment and verification

- Frontend hosting: live and responding at https://frontend-woad-pi-43.vercel.app/
- Backend health: responding locally at http://localhost:8080/health
- Production environment vars and deployment config are documented in [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)
- External reviewer access for `boltapp-hiring` is the remaining non-code handoff item if required by the assessment process

## High-Level Architecture

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

Frontend talks to the Go API over HTTP. The API is split into handlers (HTTP layer), services (business logic), and repositories (data access). PostgreSQL provides persistent storage.

## Running Locally

### Prerequisites

- Node.js >= 20+ and npm
- Go >= 1.22+
- Docker (for local Postgres optional in Phase 1)

### Frontend

```bash
cd frontend
npm install
npm run dev        # http://localhost:5173
npm run build      # production build into dist/
```

### Backend

From the repo root, the simplest local start is:

```bash
docker compose up -d
bash run-backend-local.sh
# API listens on :8080 (configurable via BACKEND_PORT env)
```

If you prefer to start it manually, run:

```bash
cd backend
set -a
. ../.env
set +a
go run ./cmd/api
```

Health check:

```bash
curl http://localhost:8080/health
```

### Local Database (Phase 2+)

```bash
docker compose up -d
```

Starts PostgreSQL on `localhost:5432` with database `authix`, user `authix_user`, password `authix_password`. See `.env.example` for variables. The API reads these same env vars, opens a pool, applies pending migrations from `database/migrations/` automatically on startup, and logs which migrations were applied (or "no new migrations to apply" when everything is current). If PostgreSQL isn't running and you just want a quick API smoke test, start the API with `AUTHIX_SKIP_DB_STARTUP_CHECK=1` — it will still refuse DB operations but the HTTP listener will come up.

## Environment Variables

Copy `.env.example` to `.env` and fill in values. The backend reads `BACKEND_PORT` (default `8080`), `BACKEND_ENV`, `MIGRATIONS_DIR`, and the full set of `POSTGRES_*` variables (Host/Port/User/Password/DB). All defaults match the `docker-compose.yml` service so local dev works out of the box without a `.env` if the compose DB is running on `localhost:5432`.

## Tests and verification

- Frontend: `cd frontend && npm install && npm run build` — passed.
- Backend: `cd backend && go test ./... && go build ./...` — passed.
- Public frontend health check: `curl -I -L https://frontend-woad-pi-43.vercel.app/` — returned HTTP 200 OK.
- Local backend health check: `curl -I -L http://localhost:8080/health` — returned HTTP 200 OK.

## Assessment Deliverables

- [x] Publicly hosted website
- [x] GitHub repository with full source
- [ ] Access granted to `boltapp-hiring` if required by the external review process
- [x] Database schema committed as `.sql` files in `database/`
- [x] `prompts.md` started
