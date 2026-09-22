# Authix

Authix is a full-stack web application built as a full-stack engineering assessment.

It demonstrates a simple two-flow application: (1) a user registers with email, first name, and last name, and receives a six-digit login code, and (2) on a checkout form, typing a valid registered email triggers a background check and a code-entry modal — if the code matches, the user is recognised; otherwise checkout continues as a guest. No real payment is processed, only submitted checkout data is persisted.

## Purpose

The project meets the assessment requirements while keeping the architecture clean and the codebase professional.

## Status

**Phase 1 — Foundation complete**

### Technology Stack

| Layer | Technology |
|---|---|
| Frontend | React + TypeScript + Vite |
| Backend | Go (`net/http`) |
| Database | PostgreSQL |
| Local DB | Docker Compose (PostgreSQL 16 Alpine) |

## Implemented Features

- Frontend: Vite + React + TypeScript scaffold
- Frontend: minimal Authix landing page
- Backend: Go module `github.com/authix/authix`, entry point
- Backend: `GET /health` JSON endpoint
- Backend: unit test for `/health`
- Docker Compose: local PostgreSQL service (not yet wired to the API)

## Planned Features

- User registration (email, first name, last name)
- Six-digit OTP code generation and display
- Checkout form (email, phone, shipping address)
- Real-time email format validation
- Background email-owner check modal with skip option
- OTP verification and in-app user greeting
- Checkout submission and persistence
- Database migrations with migrations/schema.sql
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

### Local Database (Phase 1+)

```bash
docker compose up -d
```

Starts PostgreSQL on `localhost:5432` with database `authix`, user `authix_user`, password `authix_password`. See `.env.example` for variables. In this phase the API does not yet connect to PostgreSQL.

## Environment Variables

Copy `.env.example` to `.env` and fill in values. The backend reads `BACKEND_PORT` (default `8080`) and will later read `POSTGRES_*` variables.

## Tests

- Frontend: none yet.
- Backend: `cd backend && go test ./...`

## Assessment Deliverables (Pending)

- [ ] Publicly hosted website
- [ ] GitHub repository with full source
- [ ] Access granted to `boltapp-hiring`
- [ ] Database schema committed as `.sql` files in `database/`
- [x] `prompts.md` started
