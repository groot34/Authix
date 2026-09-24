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

---

## 2026-09-23 — Add checkout input validation

Strengthen client-side and server-side validation for standard email addresses, international phone numbers, place names, and postal codes. Reject alphabetic or emoji phone input and invalid city, region, postal, or email formats while preserving realistic address data.

---

## 2026-09-23 — Gate invalid checkout submissions

Prevent checkout requests from being sent when any required field is missing or malformed. Show a clear validation error in the form, while retaining backend validation as the authoritative persistence guard.

---

## 2026-09-23 — Add standalone existing-account code generation

Provide a separate existing-account section with an email field and Generate a new code button. Keep new-account registration code generation unchanged, show the returned code for registered emails, and show a clear error for unregistered emails.

---

## 2026-09-23 — Move OTP reissue to account access

Move the Generate a new code action out of the checkout delivery modal and place it beside account creation, so checkout only handles entering the existing code or continuing as a guest.

---

## 2026-09-23 — Simplify account access UI

Use one account panel with Register new user and Existing user toggle modes. Keep new-account registration and existing-account OTP generation separate, and hide displayed OTPs automatically after 60 seconds.

---

## 2026-09-23 — Make account modes mutually exclusive

Show only one account action at a time through the Register new user / Existing user toggle, including for restored sessions. Reduce displayed OTP visibility from 60 seconds to 15 seconds.

---

## 2026-09-23 — Hide account controls for signed-in users

Hide registration and OTP-generation controls whenever an active session exists. Show only the signed-in greeting and sign-out action; restore the account toggle after sign-out.

---

## 2026-09-23 — Prepare deployment configuration

Prepare a provider-neutral deployment path using Vercel for the frontend, Render for the Go API, and managed PostgreSQL such as Neon or Supabase. Add production SSL and cross-origin cookie support, deployment manifests, environment documentation, and a smoke-test runbook without inventing credentials or deployment results.

Outcome: backend and frontend verification passed; no public deployment was performed because provider authentication and account details are required.

---

## 2026-09-23 — Disable incomplete checkout submission

Keep postal-code validation international because some valid formats contain letters, such as Canadian and UK postcodes. Disable Save checkout details until every required checkout field passes validation, with inline hints for missing or malformed fields.

---

## 2026-09-23 — Guard checkout after logout

When a user signs out with checkout details still on screen, reject submission using that same account email with an explicit session-ended message. Preserve guest checkout when the user changes to a different email.

---

## 2026-09-23 — Add country selector

Replace free-text country-code entry with a searchable selector using the provided country list. Display country names and codes while submitting the selected two-letter code to the API.

---

## 2026-09-23 — Clear checkout details after use

Clear all checkout fields after a successful submission and when the user signs out, so previously used delivery and contact details do not remain visible in the form.

---

## 2026-09-23 — Add React success and error toasts

Add auto-dismissing React toast notifications for account creation, OTP generation, OTP errors, logout, checkout validation, and saved checkout details. Use checkout wording instead of purchase wording because no payment is processed.

Outcome: frontend production build passed.

---

## 2026-09-23 — Prevent repeated OTP modal reopening

Keep a dismissed or successfully submitted registered email suppressed when the checkout form is cleared. Start a new recognition flow only when the user enters a different email.

Outcome: frontend production build passed.

---

## 2026-09-23 — Make OTP suppression race-safe

Guard the recognition modal with an immediate email ref as well as React state so stale debounced lookup responses cannot reopen it after checkout submission or guest dismissal.

Outcome: frontend production build passed.

---

## 2026-09-23 — Limit OTP prompt per email session

Track the last registered email that opened the OTP modal and prevent repeated prompts for that same email during the page session. Reset the guard only when a different email is entered or the user signs out.

Outcome: frontend production build passed.

---

## 2026-09-23 — Confirm guest checkout choice

When a registered user chooses Continue as guest, hide the verification hint for that email and show a success toast confirming guest checkout. Restore recognition behavior when a different email is entered.

Outcome: frontend production build passed.

---

## 2026-09-23 — Make guest continuation explicit

Remove the OTP modal close icon and backdrop dismissal so Continue as guest is the only way to leave the verification modal without authenticating.

---

## 2026-09-23 — Remove duplicate account error text

Use the toast as the only account-level error notification for duplicate registration, invalid registration input, and OTP generation failures. Keep field hints and OTP modal verification errors where they help the user correct input.

---

## 2026-09-24 — Warm the deployed backend on frontend load

Add a non-blocking frontend `GET /health` request on initial load so the Render backend starts waking while the deployed UI is being opened. Keep session restoration through `/api/auth/me` unchanged.