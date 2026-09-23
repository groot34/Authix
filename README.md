# Authix

Authix is a full-stack web application built as a full-stack engineering assessment.

It demonstrates a simple two-flow application: (1) a user registers with email, first name, and last name, and receives a six-digit login code, and (2) on a checkout form, typing a valid registered email triggers a background check and a code-entry modal — if the code matches, the user is recognised; otherwise checkout continues as a guest. No real payment is processed, only submitted checkout data is persisted.

## Purpose

The project meets the assessment requirements while keeping the architecture clean and the codebase professional.

## Status

**Phase 3.3B — Backend HTTP API, secure session cookies, and CORS complete; frontend next**

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

- Frontend: Vite + React + TypeScript scaffold
- Frontend: minimal Authix landing page
- Backend: Go module `github.com/groot34/Authix`, entry point
- Backend: `GET /health` JSON endpoint
- Backend: unit test for `/health`
- Docker Compose: local PostgreSQL service (wired to API via env vars)
- Database schema: `users` + `checkouts` tables as ordered SQL migrations
- Backend ↔ PostgreSQL connection pool, startup liveness check, and idempotent migration runner
- Backend: registration, authentication, session, checkout, and lookup HTTP endpoints
- Backend: HttpOnly session cookies with configurable development/production security
- Backend: explicit credentialed CORS for configured frontend origins

## Planned Features

- Frontend registration and checkout forms
- Real-time email format validation
- Background email-owner check modal with skip option
- In-app user greeting and session-aware checkout UI
- Public deployment (Vercel + Supabase or similar free tier)

## High-Level Architecture

```
┌─────────────────┐    HTTP/JSON     ┌──────────────────┐     SQL      ┌────────────┐
│  Frontend  │ ───────────────▶ │  Go API       │ ──────────▶ │ PostgreSQL │
│ React/Vite   │ ◀────────────── │  (handlers/   │ ◀──────── │          │
└────────────┘                   │  services/     │            └────────────┘
                                 │  repositories/│
                                 └──────────────────┘
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

```bash
cd backend
go mod download   # if dependencies are added
go test ./...
go run ./cmd/api
# API listens on :8080 (configurable via BACKEND_PORT env)
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

## Tests

- Frontend: none yet.
- Backend: `cd backend && go test ./...`

## Assessment Deliverables (Pending)

- [ ] Publicly hosted website
- [ ] GitHub repository with full source
- [ ] Access granted to `boltapp-hiring`
- [ ] Database schema committed as `.sql` files in `database/`
- [x] `prompts.md` started
