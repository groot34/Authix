# prompts.md

Chronological record of prompts used with LLMs while building Authix.

---

## 2026-09-22 — Phase 1a: Scaffold project folders and base files

Set up the initial Authix folders: frontend, backend, and database. Add the basic README, agent notes, architecture notes, prompts record, environment example, gitignore, and local handover file. Keep the folders empty for now and do not write application code.

---

## 2026-09-22 — Phase 1b: React + TypeScript + Vite frontend skeleton

Create a small React, TypeScript, and Vite app in frontend/. Show only the Authix name and a short description for now. Add no forms, routing, authentication, checkout, or API calls. Verify the frontend builds successfully.

---

## 2026-09-22 — Phase 1c: Go API skeleton with GET /health

Create the first Go API in backend/ using net/http. Add GET /health with a JSON response and a small unit test. Leave a simple cmd/api entrypoint with a configurable port and clean shutdown.

---

## 2026-09-22 — Phase 1d: Docker Compose Postgres + verification pass

Add a PostgreSQL 16 Docker Compose service using the same local credentials as the environment example. Do not start it. Run the frontend build and the backend tests, vet, and build commands, then record the real results.

---

## 2026-09-22 — Phase 2a.1: Draft users table and OTP storage

Create the first users migration with CITEXT email, names, timestamps, and sensible checks. Store only a hash, issue time, and used time for OTPs; never keep the six-digit code as a permanent plaintext value. Add the consistency check for those three OTP fields.

---

## 2026-09-22 — Phase 2a.2: Draft checkouts table

Add the checkouts migration for both guest and registered users. Include the submitted email, phone, shipping address, nullable user foreign key, timestamps, useful checks, and indexes. Do not add payment or product tables.

---

## 2026-09-22 — Phase 2a.3: Quick schema review pass

Review both migrations, especially the OTP null combinations, checkout constraints, indexes, and guest foreign-key behavior. Fix only a real schema issue and add a short architecture note about hashed OTP storage if needed.

---

## 2026-09-22 — Phase 2b.1: Go database driver, pool, and config

Wire the Go backend to PostgreSQL with database/sql and lib/pq. Read connection settings from POSTGRES_* variables, add a small pool abstraction, readiness checking, and the startup-check escape hatch. Do not add migrations yet.

---

## 2026-09-22 — Phase 2b.2: Idempotent migration runner

Add a migration runner that discovers and orders numbered SQL files, tracks applied versions, and applies each migration transactionally. Add offline tests for ordering, duplicate versions, and missing directories.

---

## 2026-09-22 — Phase 2b.3: Wire startup order in main.go

Start the API in the correct order: load config, open and check the database, run migrations, then create the mux and listen. Fail before binding the port if the database or migrations are unavailable.

---

## 2026-09-22 — Phase 2b.4: Review the OTP CHECK constraint

Recheck every NULL and non-NULL combination for the OTP columns. Keep the migration unchanged if the constraint already allows only valid fresh and used OTP states.

---

## 2026-09-22 — Phase 2b.5: Tests and real verification

Add focused config and migration tests plus a live PostgreSQL integration test when the local binaries are available. Verify migration idempotency and the important schema checks, then run the complete backend test, vet, and build commands. Update the project notes without adding repositories or handlers.

---

## 2026-09-23 — Phase 3.1.1: UserRepository basics

Add a testable UserRepository with parameterised, context-aware Insert and FindByEmail methods. Map duplicate email and missing user cases to repository errors while preserving underlying database errors.

---

## 2026-09-23 — Phase 3.1.2: UserRepository OTP lifecycle

Add methods to find users with an issued OTP, issue or reissue an OTP, and mark one as used. Reissuing must clear the used time, and all updates must remain parameterised and context-aware.

---

## 2026-09-23 — Phase 3.1.3: CheckoutRepository insert

Add a small CheckoutRepository insert method for guest and registered checkouts. Preserve nullable user IDs and optional address line two, return the created row, and keep the SQL parameterised.

---

## 2026-09-23 — Phase 3.1.4: Repository integration tests

Use the existing throwaway PostgreSQL setup to test user lookup, duplicate emails, the OTP lifecycle, and guest and registered checkout inserts. Keep the tests skippable in short mode.

---

## 2026-09-23 — Phase 3.1.5: Repository verification and notes

Run the repository tests, full backend tests, vet, and build. Record the actual results and update the local handover. Do not add HTTP endpoints, frontend work, migrations, or commits.

---

## 2026-09-23 — Phase 3.2.1: AuthService foundation

Start the pure service layer with an AuthService and a small local repository interface. Add crypto/rand six-digit OTP generation, strict format validation, and injectable clock and RNG options so tests stay deterministic.

---

## 2026-09-23 — Phase 3.2.2: Registration and lookup

Add registration and registered-user lookup to AuthService. Validate and normalise the inputs, issue and hash the OTP, map repository errors to service errors, and return the plaintext code only from registration or reissue.

---

## 2026-09-23 — Phase 3.2.3: OTP hashing and verification

Use SHA-256 for the six-digit OTP and document honestly that it hides plaintext at rest but does not prevent brute force. Add constant-time comparison, single-use verification, reissue support, and clear service errors while keeping the verification order deliberate.

---

## 2026-09-23 — Phase 3.2.4: CheckoutService

Add CheckoutService with validation, trimming, email normalisation, country-code normalisation, optional address handling, and a small repository interface. Return a simple checkout receipt and preserve wrapped errors.

---

## 2026-09-23 — Phase 3.2.5: AuthService tests

Add offline tests for OTP generation, hashing, validation, registration, lookup, verification, reissue, and repository error handling using small fakes and deterministic seams.

---

## 2026-09-23 — Phase 3.2.6: CheckoutService tests

Add offline tests for guest and registered checkout submissions, field normalisation, optional address values, all validation branches, and wrapped repository errors.

---

## 2026-09-23 — Phase 3.2.7: Service verification and security review

Run the service tests, short backend suite, vet, and build. Review the OTP schema and hashing decision, document the remaining expiry and brute-force limitations, and leave HTTP handlers and frontend work for the next phase.

---

## 2026-09-23 — Phase 3.2.1: OTP security hardening

Before adding handlers, enforce a ten-minute OTP lifetime, reject future or inconsistent issue times, add a process-local five-failure rate limit with cleanup, and make OTP consumption atomic when the stored hash still matches. Add focused service and PostgreSQL tests, document the single-instance limitation, and do not change the schema or commit anything.

---

## 2026-09-23 — Phase 3.3A: Session management foundation

Add the server-side session foundation needed after OTP verification: a new sessions migration, context-aware repository methods for create, validate, and revoke, and a small service that generates opaque crypto-random tokens, stores only their hashes, and applies expiry and revocation checks. Add deterministic service tests and a PostgreSQL lifecycle test. Keep handlers, cookies, routes, frontend work, and commits for later phases.
