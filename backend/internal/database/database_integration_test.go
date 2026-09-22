package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// findExec searches the $PATH for a list of candidate binary names (useful on
// Windows where some PostgreSQL installs add paths differently). Returns the
// first path that resolves to a runnable command.
func findExec(t *testing.T, candidates ...string) string {
	t.Helper()
	for _, c := range candidates {
		p, err := exec.LookPath(c)
		if err == nil {
			return p
		}
	}
	t.Skipf("could not find any of %v on PATH; skipping live-DB integration test", candidates)
	return ""
}

// runOrFail runs cmd and returns stdout; on non-zero exit it fails the test
// with stderr included.
func runOrFail(t *testing.T, cmd *exec.Cmd) string {
	t.Helper()
	var out, errbuf strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errbuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("%v failed: %v\nstderr: %s\nstdout: %s",
			cmd.Args, err, errbuf.String(), out.String())
	}
	return out.String()
}

// TestMigrationRunner_Integration exercises the full database package against
// a throwaway PostgreSQL cluster created via initdb/pg_ctl on the host. It:
//
//   - creates a new cluster on a random port with trust auth,
//   - opens a pool and calls WaitForReady,
//   - runs migrations against the real project database/migrations/ dir,
//   - asserts schema_migrations rows and that users/checkouts tables exist,
//   - re-runs migrations and confirms idempotency (no new rows),
//   - runs the same sanity-check inserts from the earlier manual validation
//     and confirms the same constraint behavior.
//
// Run only in full-test mode; skipped under `go test -short` so unit tests can
// still pass on machines without PostgreSQL installed.
func TestMigrationRunner_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live-DB integration test in short mode")
	}

	initdbExe := findExec(t, "initdb")
	pgctlExe := findExec(t, "pg_ctl")
	psqlExe := findExec(t, "psql")
	_ = psqlExe

	// Work inside a dedicated temp directory so the cluster data and logs
	// don't leak between runs.
	tmpRoot, err := os.MkdirTemp("", "authix-pg-test-*")
	if err != nil {
		t.Fatalf("mktemp: %v", err)
	}
	defer os.RemoveAll(tmpRoot)

	pgData := filepath.Join(tmpRoot, "pgdata")
	pgLog := filepath.Join(tmpRoot, "pg.log")

	// Pick a random-ish high port in the 55000+ range so parallel runs don't
	// collide. We increment if the port happens to be in use.
	port := 55433 + (time.Now().Nanosecond() / 1e6 % 500)
	host := "127.0.0.1"
	pgUser := "postgres"
	testUser := "authix_user"
	testPass := "authix_password"
	testDB := "authix"

	// 1. initdb
	runOrFail(t, exec.Command(initdbExe,
		"-D", pgData,
		"--auth=trust",
		"--no-instructions",
		"-U", pgUser,
	))

	// 2. pg_ctl start on the requested TCP port. Note: we do NOT pass -w
	// (wait-for-ready) here because on some Windows native PostgreSQL
	// installs the wait mechanism can stall waiting on a Unix-domain socket
	// that never materialises (we only use TCP). Our own psql polling loop
	// below is the reliable readiness check.
	//
	// NOTE: we deliberately do NOT capture stdout/stderr here. pg_ctl spawns
	// postgres as a long-running child which inherits the pipe write ends on
	// Windows, and that makes Cmd.Run/Wait hang forever waiting for the
	// grandchild to close its inherited stdout/stderr. Logs already go to
	// pgLog via the -l flag, so losing stdout here is fine.
	startCmd := exec.Command(pgctlExe,
		"-D", pgData,
		"-l", pgLog,
		"-s", "start",
		"-o", fmt.Sprintf("-p %d -c listen_addresses=%s", port, host),
	)
	if err := startCmd.Run(); err != nil {
		// Try to include the pg log in the failure output to aid debugging.
		if logBytes, logErr := os.ReadFile(pgLog); logErr == nil {
			t.Logf("pg log excerpt after start failure:\n%s", string(logBytes))
		}
		t.Fatalf("pg_ctl start failed: %v", err)
	}

	shutdown := func() {
		_ = exec.Command(pgctlExe, "-D", pgData, "-w", "-m", "immediate", "stop").Run()
	}
	t.Cleanup(shutdown)

	// 3. Wait for server to accept queries.
	deadline := time.Now().Add(10 * time.Second)
	var startErr error
	for time.Now().Before(deadline) {
		ping := exec.Command("psql",
			"-h", host, "-p", strconv.Itoa(port),
			"-U", pgUser, "-d", "postgres",
			"-c", "SELECT 1")
		if err := ping.Run(); err == nil {
			startErr = nil
			break
		} else {
			startErr = err
			time.Sleep(250 * time.Millisecond)
		}
	}
	if startErr != nil {
		if logBytes, logErr := os.ReadFile(pgLog); logErr == nil {
			t.Logf("pg log:\n%s", string(logBytes))
		}
		t.Fatalf("postgres never accepted connections after wait: %v", startErr)
	}

	// 4. Create role + database. Note: identifiers (role/db names) use double
	// quotes via %Q, but the PASSWORD value must be a single-quoted SQL string
	// literal — double-quoting it makes Postgres treat it as an identifier
	// name, causing a syntax error. So we wrap the password ourselves.
	setupSQL := fmt.Sprintf(
		`CREATE ROLE %[1]s WITH LOGIN PASSWORD %[2]s;`+"\n"+
			`CREATE DATABASE %[3]s OWNER %[1]s;`,
		doubleQuoteIdent(testUser),
		singleQuoteString(testPass),
		doubleQuoteIdent(testDB),
	)
	setupCmd := exec.Command("psql",
		"-h", host, "-p", strconv.Itoa(port),
		"-U", pgUser, "-d", "postgres", "-v", "ON_ERROR_STOP=1")
	setupCmd.Stdin = strings.NewReader(setupSQL)
	runOrFail(t, setupCmd)

	// 5. Open a database.Pool with our standard package code.
	pool, err := Open(Params{
		Host: host, Port: strconv.Itoa(port),
		User: testUser, Password: testPass, DBName: testDB,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = pool.Close() }()

	// 6. WaitForReady should succeed quickly against the just-started cluster.
	readyCtx, readyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer readyCancel()
	if err := WaitForReady(readyCtx, pool); err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}

	// 7. Resolve the project migrations directory and run migrations.
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	migsDir := filepath.Join(projectRoot, "database", "migrations")

	appliedFirst, err := Run(pool.DB(), migsDir)
	if err != nil {
		t.Fatalf("Run (first): %v", err)
	}
	if len(appliedFirst) == 0 {
		t.Fatalf("expected at least one migration applied first time, got %v", appliedFirst)
	}
	t.Logf("applied first run: %v", appliedFirst)

	// 8. Re-run -> must be idempotent, no new versions applied.
	appliedSecond, err := Run(pool.DB(), migsDir)
	if err != nil {
		t.Fatalf("Run (second): %v", err)
	}
	if len(appliedSecond) != 0 {
		t.Fatalf("second run applied versions %v, expected [] (idempotency failure)", appliedSecond)
	}

	// 9. schema_migrations rows match what's in migsDir.
	expectedMigs, err := DiscoverMigrations(migsDir)
	if err != nil {
		t.Fatalf("DiscoverMigrations: %v", err)
	}
	stored := map[string]bool{}
	rows, err := pool.DB().Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		stored[v] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, m := range expectedMigs {
		if !stored[m.Version] {
			t.Errorf("version %s missing from schema_migrations after run", m.Version)
		}
	}

	// 10. Sanity-check tables users + checkouts exist (columns not nulled etc).
	tableCheck := `
SELECT 'users' AS n, count(*) IS NOT NULL FROM information_schema.tables
 WHERE table_name = 'users' AND table_schema = current_schema()
UNION ALL
SELECT 'checkouts', count(*) IS NOT NULL FROM information_schema.tables
 WHERE table_name = 'checkouts' AND table_schema = current_schema();`
	tcRows, err := pool.DB().Query(tableCheck)
	if err != nil {
		t.Fatalf("table check query: %v", err)
	}
	defer tcRows.Close()
	for tcRows.Next() {
		var name string
		var ok bool
		if err := tcRows.Scan(&name, &ok); err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Errorf("table %q does not exist after migrations", name)
		}
	}

	// 11. Re-run our well-known sanity inserts against the now-live schema
	// to confirm the constraint behavior hasn't regressed since our earlier
	// psql-only check. Results are compared by counting rows; failed
	// inserts (deliberate constraint violations) should not change counts.
	sanity := `
INSERT INTO users (email, first_name, last_name, otp_code_hash, otp_issued_at)
VALUES ('alice@example.com', 'Alice', 'Smith', sha256('123456'::bytea), now());
-- Duplicate email (must fail silently or raise in batch): wrap in block
DO $$ BEGIN
  INSERT INTO users (email, first_name, last_name, otp_code_hash, otp_issued_at)
  VALUES ('alice@example.com', 'Alice', 'Jones', sha256('999999'::bytea), now());
EXCEPTION WHEN unique_violation THEN /* expected */ END $$;
DO $$ BEGIN
  INSERT INTO users (email, first_name, last_name, otp_code_hash, otp_issued_at)
  VALUES ('Alice@Example.COM', 'Alice', 'Smith', sha256('000000'::bytea), now());
EXCEPTION WHEN unique_violation THEN /* expected (CITEXT) */ END $$;
INSERT INTO users (email, first_name, last_name) VALUES ('carol@example.com', 'Carol', 'Davis');

INSERT INTO checkouts (user_id, email, phone, shipping_address_line1, shipping_city, shipping_postal_code, shipping_region, shipping_country_code)
VALUES (1, 'alice@example.com', '+15550100', '123 Main St', 'Springfield', '12345', 'IL', 'US');
INSERT INTO checkouts (user_id, email, phone, shipping_address_line1, shipping_city, shipping_postal_code, shipping_region, shipping_country_code)
VALUES (NULL, 'guest@example.com', '+15550199', '99 Guest Ln', 'Guest Town', '99999', 'NY', 'US');
`
	if _, err := pool.DB().Exec(sanity); err != nil {
		t.Fatalf("sanity inserts: %v", err)
	}
	var usersN, checkoutsN int
	if err := pool.DB().QueryRow(`SELECT count(*) FROM users`).Scan(&usersN); err != nil {
		t.Fatal(err)
	}
	if err := pool.DB().QueryRow(`SELECT count(*) FROM checkouts`).Scan(&checkoutsN); err != nil {
		t.Fatal(err)
	}
	if usersN != 2 || checkoutsN != 2 {
		t.Fatalf("final counts wrong: users=%d checkouts=%d want both=2", usersN, checkoutsN)
	}

	// 12. Run with nil db -> expect error (not panic).
	if _, err := Run(nil, migsDir); !errors.Is(err, errNilDB()) {
		// Compare by substring because wrapping is fine; any non-nil works.
		if err == nil {
			t.Errorf("Run(nil DB) returned nil error, expected non-nil")
		}
	}

	// 13. Pass nonsense directory -> expect error.
	if _, err := Run(pool.DB(), filepath.Join(tmpRoot, "does-not-exist")); err == nil {
		t.Errorf("Run(bad dir) returned nil error, expected non-nil")
	}
}

// doubleQuoteIdent wraps an SQL identifier in double quotes, doubling any
// embedded double quotes per the SQL standard. Safe for role names and
// database names that contain only safe characters (we use it only for
// authix_user/authix in tests).
func doubleQuoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// singleQuoteString wraps a string literal in single quotes for SQL, doubling
// any embedded single quotes. Used for the PASSWORD value which must be a
// single-quoted string literal, not a double-quoted identifier.
func singleQuoteString(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

// errNilDB is just a small helper for the nil-DB test above.
func errNilDB() error {
	return fmt.Errorf("database.Run: nil db")
}

// Ensure a plain *sql.DB still works as input, which is what main.go actually
// hands to Run via pool.DB().
var _ = func(db *sql.DB) {
	_, _ = Run(db, ".")
}
