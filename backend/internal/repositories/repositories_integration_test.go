package repositories

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/authix/authix/internal/database"
)

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

func doubleQuoteIdent(s string) string  { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
func singleQuoteString(s string) string { return `'` + strings.ReplaceAll(s, `'`, `''`) + `'` }

func setupThrowawayDB(t *testing.T) database.Pool {
	t.Helper()

	initdbExe := findExec(t, "initdb")
	pgctlExe := findExec(t, "pg_ctl")

	tmpRoot, err := os.MkdirTemp("", "authix-repo-test-*")
	if err != nil {
		t.Fatalf("mktemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpRoot) })

	pgData := filepath.Join(tmpRoot, "pgdata")
	pgLog := filepath.Join(tmpRoot, "pg.log")

	port := 56000 + (time.Now().Nanosecond() / 1e6 % 500)
	host := "127.0.0.1"
	pgUser := "postgres"
	testUser := "repo_user"
	testPass := "repo_password"
	testDB := "repo"

	runOrFail(t, exec.Command(initdbExe,
		"-D", pgData, "--auth=trust", "--no-instructions", "-U", pgUser,
	))

	startCmd := exec.Command(pgctlExe, "-D", pgData, "-l", pgLog, "-s", "start",
		"-o", "-p "+strconv.Itoa(port)+" -c listen_addresses="+host)
	if err := startCmd.Run(); err != nil {
		if logBytes, logErr := os.ReadFile(pgLog); logErr == nil {
			t.Logf("pg log excerpt after start failure:\n%s", string(logBytes))
		}
		t.Fatalf("pg_ctl start failed: %v", err)
	}
	t.Cleanup(func() {
		_ = exec.Command(pgctlExe, "-D", pgData, "-w", "-m", "immediate", "stop").Run()
	})

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
		}
		startErr = err
		time.Sleep(250 * time.Millisecond)
	}
	if startErr != nil {
		if logBytes, logErr := os.ReadFile(pgLog); logErr == nil {
			t.Logf("pg log:\n%s", string(logBytes))
		}
		t.Fatalf("postgres never accepted connections: %v", startErr)
	}

	setupSQL := strings.Join([]string{
		`CREATE ROLE ` + doubleQuoteIdent(testUser) + ` WITH LOGIN PASSWORD ` + singleQuoteString(testPass) + `;`,
		`CREATE DATABASE ` + doubleQuoteIdent(testDB) + ` OWNER ` + doubleQuoteIdent(testUser) + `;`,
	}, "\n")
	setupCmd := exec.Command("psql",
		"-h", host, "-p", strconv.Itoa(port),
		"-U", pgUser, "-d", "postgres", "-v", "ON_ERROR_STOP=1")
	setupCmd.Stdin = strings.NewReader(setupSQL)
	runOrFail(t, setupCmd)

	pool, err := database.Open(database.Params{
		Host: host, Port: strconv.Itoa(port),
		User: testUser, Password: testPass, DBName: testDB,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer readyCancel()
	if err := database.WaitForReady(readyCtx, pool); err != nil {
		t.Fatalf("WaitForReady: %v", err)
	}

	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	migsDir := filepath.Join(projectRoot, "database", "migrations")
	applied, err := database.Run(pool.DB(), migsDir)
	if err != nil {
		t.Fatalf("Run migrations: %v", err)
	}
	t.Logf("applied migrations: %v", applied)

	return pool
}

func sha256Sum(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

func TestUserRepository_InsertAndFindByEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live-DB test in short mode")
	}
	pool := setupThrowawayDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	users := NewUserRepository(pool)

	u, err := users.Insert(ctx, "Alice@Example.COM", "Alice", "Smith")
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if u.ID <= 0 {
		t.Errorf("expected positive ID, got %d", u.ID)
	}
	if u.Email != "Alice@Example.COM" {
		t.Errorf("email round-trip: got %q", u.Email)
	}
	if u.FirstName != "Alice" || u.LastName != "Smith" {
		t.Errorf("names mismatch: %+v", u)
	}

	found, err := users.FindByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("FindByEmail lowercase: %v", err)
	}
	if found.ID != u.ID {
		t.Errorf("CITEXT lookup mismatch: id %d vs %d", found.ID, u.ID)
	}

	_, err = users.Insert(ctx, "ALICE@example.com", "Duplicate", "Row")
	if !errors.Is(err, ErrDuplicateEmail) {
		t.Errorf("duplicate insert want ErrDuplicateEmail, got %v", err)
	}

	_, err = users.FindByEmail(ctx, "nobody@example.com")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("missing user want ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_OTPLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live-DB test in short mode")
	}
	pool := setupThrowawayDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	users := NewUserRepository(pool)

	u, err := users.Insert(ctx, "bob@example.com", "Bob", "Jones")
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if u.OTPCodeHash != nil || u.OTPIssuedAt != nil || u.OTPUsedAt != nil {
		t.Errorf("fresh user should have nil OTP fields, got %+v", u)
	}

	_, err = users.FindForOTPVerify(ctx, "bob@example.com")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("FindForOTPVerify before issue should return ErrUserNotFound, got %v", err)
	}

	codeHash := sha256Sum("654321")
	issuedAt := time.Now().Truncate(time.Microsecond).UTC()
	if err := users.UpdateOTP(ctx, u.ID, codeHash, issuedAt); err != nil {
		t.Fatalf("UpdateOTP: %v", err)
	}

	got, err := users.FindForOTPVerify(ctx, "bob@example.com")
	if err != nil {
		t.Fatalf("FindForOTPVerify after issue: %v", err)
	}
	if got.OTPIssuedAt == nil || !got.OTPIssuedAt.Equal(issuedAt) {
		t.Errorf("issued at mismatch: want %v got %v", issuedAt, got.OTPIssuedAt)
	}
	if !bytes.Equal(got.OTPCodeHash, codeHash) {
		t.Errorf("otp hash mismatch: want %x got %x", codeHash, got.OTPCodeHash)
	}
	if got.OTPUsedAt != nil {
		t.Errorf("fresh code should not be used yet, got %v", got.OTPUsedAt)
	}

	consumed, err := users.AtomicConsumeOTP(ctx, u.ID, codeHash)
	if err != nil {
		t.Fatalf("AtomicConsumeOTP first: %v", err)
	}
	if !consumed {
		t.Errorf("first AtomicConsumeOTP should return consumed=true")
	}

	refreshed, err := users.FindByEmail(ctx, "bob@example.com")
	if err != nil {
		t.Fatalf("FindByEmail after consume: %v", err)
	}
	if refreshed.OTPUsedAt == nil {
		t.Errorf("OTPUsedAt should be set after AtomicConsumeOTP success")
	}

	consumedAgain, err := users.AtomicConsumeOTP(ctx, u.ID, codeHash)
	if err != nil {
		t.Fatalf("AtomicConsumeOTP second (spent): %v", err)
	}
	if consumedAgain {
		t.Errorf("second consume on already-spent code should return false")
	}

	newHash := sha256Sum("111111")
	newIssued := time.Now().Truncate(time.Microsecond).UTC()
	if err := users.UpdateOTP(ctx, u.ID, newHash, newIssued); err != nil {
		t.Fatalf("UpdateOTP reissue: %v", err)
	}

	consumedOld, err := users.AtomicConsumeOTP(ctx, u.ID, codeHash)
	if err != nil {
		t.Fatalf("AtomicConsumeOTP with old hash after reissue: %v", err)
	}
	if consumedOld {
		t.Errorf("consume with OLD hash after reissue should return false (hash replaced concurrently)")
	}
	consumedNew, err := users.AtomicConsumeOTP(ctx, u.ID, newHash)
	if err != nil {
		t.Fatalf("AtomicConsumeOTP with new hash after reissue: %v", err)
	}
	if !consumedNew {
		t.Errorf("consume with NEW hash after reissue should return true")
	}

	reissued, err := users.FindByEmail(ctx, "bob@example.com")
	if err != nil {
		t.Fatalf("FindByEmail after reissue consume: %v", err)
	}
	if reissued.OTPUsedAt == nil {
		t.Errorf("OTPUsedAt should be set after new-hash consume")
	}

	if err := users.UpdateOTP(ctx, 999999, newHash, newIssued); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("UpdateOTP missing user want ErrUserNotFound, got %v", err)
	}
	_, err = users.AtomicConsumeOTP(ctx, 999999, newHash)
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("AtomicConsumeOTP missing user want wrapped ErrUserNotFound, got %v", err)
	}
}

func TestUserRepository_AtomicConsumeOTP_Concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live-DB test in short mode")
	}
	pool := setupThrowawayDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	users := NewUserRepository(pool)

	u, err := users.Insert(ctx, "concurrent@example.com", "Concurrent", "User")
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	codeHash := sha256Sum("246810")
	if err := users.UpdateOTP(ctx, u.ID, codeHash, time.Now().UTC()); err != nil {
		t.Fatalf("UpdateOTP: %v", err)
	}

	const attempts = 12
	start := make(chan struct{})
	results := make(chan bool, attempts)
	errorsCh := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			consumed, consumeErr := users.AtomicConsumeOTP(ctx, u.ID, codeHash)
			if consumeErr != nil {
				errorsCh <- consumeErr
				return
			}
			results <- consumed
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errorsCh)

	for err := range errorsCh {
		if err != nil {
			t.Fatalf("AtomicConsumeOTP concurrent error: %v", err)
		}
	}
	successes := 0
	for consumed := range results {
		if consumed {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent consumption successes want 1 got %d", successes)
	}
}

func TestCheckoutRepository_InsertGuestAndUser(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live-DB test in short mode")
	}
	pool := setupThrowawayDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	users := NewUserRepository(pool)
	checkouts := NewCheckoutRepository(pool)

	alice, err := users.Insert(ctx, "alice@example.com", "Alice", "Smith")
	if err != nil {
		t.Fatalf("Insert alice: %v", err)
	}

	guest := &Checkout{
		Email:                "guest@example.com",
		Phone:                "+15550100",
		ShippingAddressLine1: "99 Guest Ln",
		ShippingCity:         "Guest Town",
		ShippingPostalCode:   "99999",
		ShippingRegion:       "NY",
		ShippingCountryCode:  "US",
	}
	gotGuest, err := checkouts.Insert(ctx, guest)
	if err != nil {
		t.Fatalf("Insert guest checkout: %v", err)
	}
	if gotGuest.ID <= 0 {
		t.Errorf("expected positive checkout id, got %d", gotGuest.ID)
	}
	if gotGuest.UserID != nil {
		t.Errorf("guest checkout user_id should be nil, got %v", *gotGuest.UserID)
	}
	if gotGuest.CreatedAt.IsZero() {
		t.Errorf("guest checkout CreatedAt should be set")
	}
	if gotGuest.ShippingAddressLine2 != nil {
		t.Errorf("line2 should be nil when omitted, got %q", *gotGuest.ShippingAddressLine2)
	}

	line2 := "Apt 4B"
	loggedIn := &Checkout{
		UserID:               &alice.ID,
		Email:                alice.Email,
		Phone:                "+15550200",
		ShippingAddressLine1: "123 Main St",
		ShippingAddressLine2: &line2,
		ShippingCity:         "Springfield",
		ShippingPostalCode:   "12345",
		ShippingRegion:       "IL",
		ShippingCountryCode:  "US",
	}
	gotUser, err := checkouts.Insert(ctx, loggedIn)
	if err != nil {
		t.Fatalf("Insert logged-in checkout: %v", err)
	}
	if gotUser.UserID == nil || *gotUser.UserID != alice.ID {
		t.Errorf("user_id mismatch: want %d got %v", alice.ID, gotUser.UserID)
	}
	if gotUser.ShippingAddressLine2 == nil || *gotUser.ShippingAddressLine2 != line2 {
		t.Errorf("line2 mismatch: want %q got %v", line2, gotUser.ShippingAddressLine2)
	}

	totalRows := 0
	if err := pool.DB().QueryRowContext(ctx, `SELECT count(*) FROM checkouts`).Scan(&totalRows); err != nil {
		t.Fatalf("count checkouts: %v", err)
	}
	if totalRows != 2 {
		t.Errorf("checkouts count want 2 got %d", totalRows)
	}
}
