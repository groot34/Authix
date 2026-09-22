# prompts.md

Chronological record of prompts used with LLMs while building Authix.

---

## 2026-09-22 — Phase 1: project foundation

Set up the Authix project from scratch. Build a clean frontend/backend/database structure with a React + TypeScript + Vite frontend (simple landing page only, working build), a Go HTTP API using stdlib net/http with a GET /health endpoint returning JSON and a unit test for it, and a Docker Compose PostgreSQL setup for local dev. Create the required project files: README.md, AGENTS.md, LOCAL_AGENT_CONTEXT.md (gitignored), docs/ARCHITECTURE.md, prompts.md, .gitignore, and .env.example. Run verification checks for real and report actual results. Don't implement registration, OTP, checkout, migrations, database wiring, or deployment yet.

---

## 2026-09-22 — Phase 2: database schema and SQL migrations

Design and write the PostgreSQL schema for Authix. Create the SQL migration files under database/migrations/: a users table (primary key, unique email, first name, last name, timestamps, sensible constraints) and a checkouts submissions table that stores email, phone, shipping address, and an optional associated user so guest checkouts still work. Choose sensible data types and constraints. Think carefully about OTP representation — do not store the six-digit code as a permanent plaintext reusable credential; document the choice briefly in a SQL comment or architecture doc. Keep the schema simple and fit for the assessment, no extra tables or infrastructure. Update docs/ARCHITECTURE.md only if a meaningful design decision needs documenting. Append a short natural entry to prompts.md for this task. Run any SQL validation checks available. Do not implement Go DB wiring, API endpoints, UI, or deploy yet.

---

## 2026-09-22 — Phase 2: PostgreSQL connection layer and migration runner

Wire the existing Go backend to PostgreSQL. Choose a driver, read POSTGRES_* env vars consistently with .env.example / docker-compose, add an internal/database package that opens a *sql.DB pool, configures reasonable pool sizes, exposes a close method, and runs a startup reachability check that fails clearly when Postgres is unavailable (with an escape hatch env var for smoke tests). Add a migration runner that discovers ordered .sql files from database/migrations/, creates schema_migrations if needed, applies each unapplied migration in its own transaction, and never reapplies already-recorded versions — idempotent across restarts. Update cmd/api/main.go to open the pool, wait for readiness, and run migrations before binding the HTTP listener. Review the users OTP CHECK constraint across every hash/issued/used NULL combination for genuine schema holes. Add offline tests for config env parsing, migration discovery/ordering/duplicate-version rejection, nil-DB guard, and bad-dir handling; run a live-DB integration test (spinning up a throwaway cluster with initdb/pg_ctl) if Postgres binaries are on PATH, verifying that migrations apply, tables exist, idempotency holds, and the documented constraint behavior accepts/rejects the documented inserts. Update prompts.md, keep LOCAL_AGENT_CONTEXT.md current, update README/ARCHITECTURE only where they became stale. No repositories, no endpoints, no UI, no commits, no deploy.
