# prompts.md

Chronological record of prompts used with LLMs while building Authix.

---

## 2026-09-22 — Phase 1: project foundation

Set up the Authix project from scratch. Build a clean frontend/backend/database structure with a React + TypeScript + Vite frontend (simple landing page only, working build), a Go HTTP API using stdlib net/http with a GET /health endpoint returning JSON and a unit test for it, and a Docker Compose PostgreSQL setup for local dev. Create the required project files: README.md, AGENTS.md, LOCAL_AGENT_CONTEXT.md (gitignored), docs/ARCHITECTURE.md, prompts.md, .gitignore, and .env.example. Run verification checks for real and report actual results. Don't implement registration, OTP, checkout, migrations, database wiring, or deployment yet.
