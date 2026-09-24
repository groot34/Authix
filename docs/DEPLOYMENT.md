# Deployment

This deployment plan uses Vercel for the Vite frontend, Render for the Go API, and a managed PostgreSQL provider such as Neon or Supabase.

No provider URLs, credentials, or deployment results are committed to this repository.

## 1. Create the database

Create a PostgreSQL database on Neon or Supabase and collect its connection values:

- Host
- Port
- Database name
- Username
- Password

The API expects these environment variables:

```text
POSTGRES_HOST=<managed-postgres-host>
POSTGRES_PORT=5432
POSTGRES_USER=<managed-postgres-user>
POSTGRES_PASSWORD=<managed-postgres-password>
POSTGRES_DB=<managed-postgres-database>
POSTGRES_SSLMODE=require
```

The API applies the SQL files in `database/migrations/` during startup. Do not run the local Docker database credentials in production.

## 2. Deploy the API to Render

1. Push the repository to GitHub.
2. In Render, create a new Blueprint or Web Service from the repository.
3. Use the committed `render.yaml`, or configure these values manually:
   - Build command: `cd backend && go build -o ../authix-api ./cmd/api`
   - Start command: `./authix-api`
   - Health check: `/health`
4. Set the managed PostgreSQL values from step 1.
5. Set `AUTHIX_ALLOWED_ORIGINS` temporarily to the eventual Vercel URL, or update it immediately after the frontend is deployed.
6. Set `BACKEND_ENV=production` and `AUTHIX_COOKIE_SECURE=1`.
7. Deploy and verify `https://<render-api-host>/health` returns JSON.

Required API environment variables:

```text
BACKEND_ENV=production
AUTHIX_ALLOWED_ORIGINS=https://<vercel-frontend-host>
AUTHIX_COOKIE_SECURE=1
AUTHIX_SESSION_COOKIE=authix_session
MIGRATIONS_DIR=database/migrations
POSTGRES_HOST=<managed-postgres-host>
POSTGRES_PORT=5432
POSTGRES_USER=<managed-postgres-user>
POSTGRES_PASSWORD=<managed-postgres-password>
POSTGRES_DB=<managed-postgres-database>
POSTGRES_SSLMODE=require
```

Production uses Secure, HttpOnly, SameSite=None cookies so a Vercel frontend can authenticate to a separately hosted API. HTTPS is required.

## 3. Deploy the frontend to Vercel

1. Import the GitHub repository into Vercel.
2. Set the project root directory to `frontend`.
3. Vercel will use `frontend/vercel.json`:
   - Install: `npm install`
   - Build: `npm run build`
   - Output: `dist`
4. Set this Vercel environment variable for Production:

```text
VITE_API_BASE_URL=https://<render-api-host>
```

5. Deploy and open the generated Vercel URL.

## 4. Final CORS update

After the final Vercel URL is known, set the Render variable exactly to that origin, without a trailing slash:

```text
AUTHIX_ALLOWED_ORIGINS=https://<vercel-frontend-host>
```

Redeploy or restart the API after changing it.

## 5. Smoke test

Verify:

1. `GET /health` succeeds.
2. Register a new user and record the displayed OTP.
3. Enter the registered email at checkout and verify the modal appears.
4. Confirm a wrong OTP shows an error.
5. Confirm the correct OTP signs in and the name appears.
6. Submit an authenticated checkout and inspect the `checkouts` table.
7. Test guest checkout with an unregistered email.
8. Refresh after login and confirm the session remains active.
9. Confirm browser requests have no CORS errors and the session cookie is Secure and HttpOnly.

## Manual requirements

The deployment owner must still provide provider account access, production database credentials, the final public URLs, and GitHub access for `boltapp-hiring`. These cannot be safely invented or committed by the repository.
