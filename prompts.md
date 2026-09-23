# prompts.md

Chronological record of prompts used with LLMs while building Authix.

---

## 2026-09-22 — Phase 1a: Scaffold the project

Set up the first Authix folders: frontend, backend, and database. Add the README, agent notes, architecture notes, prompts file, environment example, gitignore, and local handover file. Keep the folders empty for now.

---

## 2026-09-22 — Phase 1b: Start the frontend

Create a small React, TypeScript, and Vite app in frontend/. Show the Authix name and a short description. Keep forms, routing, authentication, checkout, and API calls for later. Verify the frontend build.

---

## 2026-09-22 — Phase 1c: Start the Go API

Create the first Go API in backend/ with net/http. Add GET /health and return a small JSON response. Add a direct unit test and leave a simple cmd/api entrypoint with clean shutdown.

---

## 2026-09-22 — Phase 1d: Add local PostgreSQL

Add a PostgreSQL 16 Docker Compose service using the local credentials from the environment example. Do not start it yet. Run the frontend build and the backend tests, vet, and build commands.

---

## 2026-09-22 — Phase 2a.1: Design the users table

Create the first users migration with CITEXT email, names, timestamps, and basic checks. Think through how OTP data should be stored without keeping a reusable plaintext code.

---

## 2026-09-22 — Phase 2a.2: Add OTP columns

Add the OTP hash, issue time, and used time to the users table. Add a check that allows only the valid empty, fresh, and used combinations. Include a short comment explaining the choice.

---

## 2026-09-22 — Phase 2a.3: Add the checkouts table

Create the checkouts migration for both guests and registered users. Store the submitted contact and shipping fields, use a nullable user foreign key, and add the useful indexes and non-empty checks.

---

## 2026-09-22 — Phase 2a.4: Review the schema

Re-read both migrations and walk through the OTP null combinations, checkout constraints, indexes, and guest-delete behavior. Fix only a real schema problem and add a short architecture note if needed.

---

## 2026-09-22 — Phase 2b.1: Connect Go to PostgreSQL

Wire the backend to PostgreSQL with database/sql and lib/pq. Read the connection values from POSTGRES_* environment variables and add a small pool abstraction.

---

## 2026-09-22 — Phase 2b.2: Add database readiness

Add a startup Ping retry with a timeout so the API fails clearly when PostgreSQL is unavailable. Keep an environment escape hatch for local smoke tests.

---

## 2026-09-22 — Phase 2b.3: Build the migration runner

Discover numbered SQL files, sort them, and reject duplicate versions. Track applied migrations in schema_migrations.

---

## 2026-09-22 — Phase 2b.4: Make migrations transactional

Apply each migration in its own transaction and record its version in that same transaction. Make repeated application safe and add offline tests for ordering and bad directories.

---

## 2026-09-22 — Phase 2b.5: Wire startup

Update cmd/api so configuration, database readiness, and migrations happen before the HTTP listener binds. Preserve the existing health endpoint and shutdown behavior.

---

## 2026-09-22 — Phase 2b.6: Run the foundation checks

Add config tests and a live migration test when the local PostgreSQL binaries are available. Check idempotency and the important schema constraints, then run the full backend verification commands.

---

## 2026-09-23 — Phase 3.1.1: Add the user repository

Create a testable UserRepository with parameterised, context-aware Insert and FindByEmail methods. Add repository errors for duplicate email and missing users.

---

## 2026-09-23 — Phase 3.1.2: Add OTP repository reads

Add FindForOTPVerify and keep it limited to users with OTP material. Reuse the existing user scan logic and preserve context-aware queries.

---

## 2026-09-23 — Phase 3.1.3: Add OTP repository writes

Add UpdateOTP and MarkOTPUsed. Reissuing must clear the used time, and both methods should report a missing user cleanly.

---

## 2026-09-23 — Phase 3.1.4: Add checkout persistence

Create CheckoutRepository.Insert for guest and registered submissions. Preserve nullable user IDs and optional address line two, then return the created row.

---

## 2026-09-23 — Phase 3.1.5: Test the repositories

Use the throwaway PostgreSQL setup to test case-insensitive email lookup, duplicate registration, the OTP lifecycle, and both checkout types.

---

## 2026-09-23 — Phase 3.1.6: Verify the repository layer

Run the repository tests, the full backend tests, vet, and build. Update the project notes and leave HTTP endpoints for a later phase.

---

## 2026-09-23 — Phase 3.2.1: Start AuthService

Create AuthService with a small local repository interface. Add six-digit crypto/rand OTP generation and strict ASCII digit validation.

---

## 2026-09-23 — Phase 3.2.2: Add test seams

Put the clock and OTP generator behind small options so service tests can use fixed values without touching the production path.

---

## 2026-09-23 — Phase 3.2.3: Add registration

Implement registration with email and name validation, normalisation, OTP issuance, repository error mapping, and a one-time plaintext code in the result.

---

## 2026-09-23 — Phase 3.2.4: Add user lookup

Implement registered-user lookup using the existing repository. An unknown but valid email should return a normal unregistered result rather than a database error.

---

## 2026-09-23 — Phase 3.2.5: Choose the OTP hash

Use SHA-256 for the six-digit code and document the limitation honestly: it keeps plaintext out of the database but does not stop brute force on a one-million-value code space.

---

## 2026-09-23 — Phase 3.2.6: Add OTP verification

Implement format checking, unused-code checking, constant-time hash comparison, single-use verification, and OTP reissue. Keep the verification order deliberate.

---

## 2026-09-23 — Phase 3.2.7: Add CheckoutService

Create CheckoutService with its own small repository interface. Validate the checkout fields and return clear service errors.

---

## 2026-09-23 — Phase 3.2.8: Normalise checkout input

Trim submitted fields, lower-case email, uppercase the country code, and turn a whitespace-only optional address line into NULL before persistence.

---

## 2026-09-23 — Phase 3.2.9: Test the services

Add offline AuthService and CheckoutService tests using small fakes. Cover validation, normalisation, OTP behavior, and wrapped repository errors.

---

## 2026-09-23 — Phase 3.2.10: Review the service security

Run the service and short backend tests, vet, and build. Record the hashing limitations and the remaining need for OTP expiry and attempt limiting before handlers go live.

---

## 2026-09-23 — Phase 3.2.11: Add OTP expiry

Use the existing otp_issued_at value and an injected clock to enforce a ten-minute lifetime. Treat the exact boundary and future issue times as invalid.

---

## 2026-09-23 — Phase 3.2.12: Add OTP attempt limiting

Add a mutex-protected, process-local limiter for five failed attempts per email in a rolling fifteen-minute window. Clean up old entries and keep successful verification out of the failure count.

---

## 2026-09-23 — Phase 3.2.13: Make OTP consumption atomic

Replace the separate mark-used step with a parameterised update that requires an unused row and the matching verified hash. Return whether the database actually consumed the code.

---

## 2026-09-23 — Phase 3.2.14: Test the hardening

Add expiry, rate-limit, concurrency, and atomic-consumption tests. Keep the limiter limitation documented and do not change the existing schema for these policies.

---

## 2026-09-23 — Phase 3.3A.1: Design server-side sessions

Before adding HTTP handlers, design a server-side session around an opaque random token. The raw token should go only to the future cookie setter; the database should keep only its hash.

---

## 2026-09-23 — Phase 3.3A.2: Add the sessions migration

Add a new ordered sessions migration with a unique token hash, user foreign key, creation and expiry timestamps, revocation time, and lookup indexes. Leave earlier migrations unchanged.

---

## 2026-09-23 — Phase 3.3A.3: Add SessionRepository

Implement context-aware create, valid-session lookup, and revoke methods. Expired and revoked records should behave like missing sessions.

---

## 2026-09-23 — Phase 3.3A.4: Add SessionService

Generate 32-byte crypto-random tokens, encode them for a cookie, hash them before persistence, and return the user ID and expiry without exposing database details.

---

## 2026-09-23 — Phase 3.3A.5: Test sessions

Add deterministic service tests and a PostgreSQL lifecycle test for token hashing, validation, expiry, revocation, unknown tokens, and repository errors.

---

## 2026-09-23 — Phase 3.3B.1: Define the HTTP routes

Connect the existing services to JSON routes for registration, auth lookup, OTP verification, OTP reissue, session me, logout, and checkout. Keep the request and response shapes small and explicit.

---

## 2026-09-23 — Phase 3.3B.2: Add secure cookies

Create a session only after OTP verification succeeds. Set an HttpOnly cookie with SameSite and environment-aware Secure settings, and clear it during logout.

---

## 2026-09-23 — Phase 5: Integration testing and deployment readiness

Validate the Authix assessment requirements against the implemented OTP login and checkout flows, then run the actual frontend build and backend verification commands. Check whether the local PostgreSQL and Docker environment is available for live flow testing, and document deployment blockers and configuration gaps without claiming a deployment was performed.

Outcome: the real frontend build passed (`cd frontend && npm install && npm run build`), the backend test/build suite passed (`cd backend && go test ./... && go build ./...`), and local Docker-backed live flow testing could not be completed in this environment because Docker Desktop was not running, so the DB-backed end-to-end flow remains unverified here.

---

## 2026-09-23 — Phase 3.3B.3: Protect checkout identity

Load the authenticated user from the validated session cookie. Reject a browser-supplied user ID while still allowing guest checkout without a session.

---

## 2026-09-23 — Phase 3.3B.4: Add CORS and API wiring

Allow credentialed requests only from configured origins. Handle preflight requests, wire the repositories and services in main.go, and add sensible HTTP server timeouts.

---

## 2026-09-23 — Phase 3.3B.5: Test the HTTP layer

Add httptest coverage for registration, lookup, verification, reissue, session me, logout, checkout identity, guest checkout, errors, cookies, and CORS.

---

## 2026-09-23 — Phase 3.3B.6: Align the Go module path

Update the Go module and internal imports to match the actual repository at github.com/groot34/Authix. Run the backend tests, vet, and build after the rename.

---

## 2026-09-23 — Phase 4.1.1: Refresh the frontend shell

Replace the original placeholder landing page with a clean responsive Authix application shell. Keep registration, OTP, and checkout interactions for the next steps.

---

## 2026-09-23 — Phase 4.1.2: Add the frontend API client

Add a small typed client for the real backend routes. Use the configured API base URL and include credentials so the browser sends the session cookie.

---

## 2026-09-23 — Phase 4.1.3: Add frontend configuration

Add the Vite environment type and a frontend environment example for VITE_API_BASE_URL. Do not add real credentials.

---

## 2026-09-23 — Phase 4.1.4: Verify the frontend foundation

Run the frontend build and record that no frontend tests or lint script exist yet.

---

## 2026-09-23 — Phase 4.2.1: Build registration

Add a registration form for email, first name, and last name. Validate the email while typing and show loading, error, and success states.

---

## 2026-09-23 — Phase 4.2.2: Show the real OTP

Submit registration through the existing API client and display the six-digit code returned by the backend. Do not mock the response or store session tokens in the browser.

---

## 2026-09-23 — Phase 4.2.3: Recognise returning emails

Add reusable email recognition that waits for valid syntax before making a lookup request. Debounce input, cancel obsolete requests, and ignore stale responses.

---

## 2026-09-23 — Phase 4.2.4: Add the OTP modal

Build a reusable numeric OTP modal for verification and error states. Add reissue support and a dismiss action that Phase 4.3 can connect to guest checkout.

---

## 2026-09-23 — Phase 4.2.5: Restore authenticated state

After successful verification, update the visible user state from the API response and confirm the cookie-backed session through the existing /me endpoint. Leave checkout UI for Phase 4.3.

---

## 2026-09-23 — Phase 4.2.6: Verify the auth flow

Run the frontend build and record the current frontend test and lint availability. Do not commit or push until the phase is reviewed.

---

## 2026-09-23 — Phase 4.3.1: Start the checkout form

Add the checkout section to the existing Authix screen. Start with email, phone, and the shipping fields, and keep the layout consistent with the registration panel.

---

## 2026-09-23 — Phase 4.3.2: Validate checkout details

Add simple inline validation for the checkout email and required contact and shipping fields. Keep the messages clear and use the same validation style as the registration form.

---

## 2026-09-23 — Phase 4.3.3: Connect email recognition

Use the existing email recognition hook with the checkout email field. Keep the lookup debounced, avoid duplicate calls, and do not let an old response replace a newer email result.

---

## 2026-09-23 — Phase 4.3.4: Open OTP from checkout

When the checkout email belongs to a registered user, open the existing OTP modal. Let the user dismiss it and continue as a guest, and make sure the same email does not reopen the modal repeatedly.

---

## 2026-09-23 — Phase 4.3.5: Keep member checkout trusted

Show the authenticated user above the checkout form after login. Keep the checkout email consistent with that session and never send a browser-supplied user ID as proof of authentication.

---

## 2026-09-23 — Phase 4.3.6: Submit the checkout

Submit guest and authenticated checkout details through the existing checkout API. Add loading, duplicate-submit protection, saved, and error states. Do not add payment processing.

---

## 2026-09-23 — Phase 4.3.7: Verify the checkout flow

Run the frontend build and any available checks. Note the remaining end-to-end work with a running backend, and leave deployment for a later phase.
