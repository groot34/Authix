package database

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// Migration is a single migration file on disk. Version is the numeric
// prefix (e.g. "0001") and Path is the absolute path to the .sql file.
type Migration struct {
	Version string
	Name    string // file basename
	Path    string
}

var migrationFileRe = regexp.MustCompile(`^(\d+)[-_]?.*\.sql$`)

// DiscoverMigrations reads dir and returns the ordered list of SQL
// migrations that match the naming convention (numeric prefix + .sql
// suffix, e.g. "0001_extensions_and_users.sql").
//
// The function is pure — no database access needed — so it can be unit
// tested without PostgreSQL.
func DiscoverMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %q: %w", dir, err)
	}

	var migs []Migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		m := migrationFileRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		abs, err := filepath.Abs(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("abs path for %q: %w", name, err)
		}
		migs = append(migs, Migration{
			Version: m[1],
			Name:    name,
			Path:    abs,
		})
	}

	sort.SliceStable(migs, func(i, j int) bool {
		if len(migs[i].Version) != len(migs[j].Version) {
			return len(migs[i].Version) < len(migs[j].Version)
		}
		return migs[i].Version < migs[j].Version
	})

	// Duplicate-version check — two files can't share a version prefix.
	seen := map[string]string{}
	for _, m := range migs {
		if prev, ok := seen[m.Version]; ok {
			return nil, fmt.Errorf("duplicate migration version %s: %s and %s",
				m.Version, prev, m.Name)
		}
		seen[m.Version] = m.Name
	}

	return migs, nil
}

// EnsureMigrationsTable creates (if missing) the schema_migrations table
// the runner uses to decide whether a migration has already been applied.
func EnsureMigrationsTable(tx *sql.Tx) error {
	_, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    TEXT         PRIMARY KEY,
    applied_at TIMESTAMPTZ  NOT NULL  DEFAULT now()
);`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	return nil
}

// AppliedVersions returns the set of migration versions that have already
// been recorded in schema_migrations. If the table does not exist yet an
// empty set is returned (it will be created before migrations run).
func AppliedVersions(db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		// Heuristic: if the table doesn't exist yet, treat it as "nothing
		// applied". We'll create the table inside EnsureMigrationsTable
		// when we actually start running.
		return map[string]struct{}{}, nil
	}
	defer rows.Close()

	out := map[string]struct{}{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		out[v] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Run applies all migrations from dir that have not yet been recorded in
// schema_migrations, in version order, each inside its own transaction.
// Passing a nil db is a programmer error (panic free: returns error).
//
// Returns the list of versions that were actually applied during this run
// (so callers can log them) or the first error.
func Run(db *sql.DB, migrationsDir string) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database.Run: nil db")
	}

	migs, err := DiscoverMigrations(migrationsDir)
	if err != nil {
		return nil, err
	}

	applied, err := AppliedVersions(db)
	if err != nil {
		return nil, err
	}

	var newlyApplied []string

	for _, m := range migs {
		if _, ok := applied[m.Version]; ok {
			continue
		}

		if err := runOne(db, m); err != nil {
			return newlyApplied, fmt.Errorf("migration %s (%s): %w",
				m.Version, m.Name, err)
		}
		newlyApplied = append(newlyApplied, m.Version)
	}

	return newlyApplied, nil
}

func runOne(db *sql.DB, m Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := EnsureMigrationsTable(tx); err != nil {
		return err
	}

	body, err := os.ReadFile(m.Path)
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}
	if len(body) == 0 {
		return fmt.Errorf("migration file is empty")
	}

	// Note: we use Exec directly rather than a prepared statement because
	// migration bodies are multi-statement DDL. lib/pq accepts that via the
	// simple query protocol when the string contains statements separated
	// by semicolons (plus CREATE EXTENSION etc.).
	if _, err := tx.Exec(string(body)); err != nil {
		return fmt.Errorf("apply SQL: %w", err)
	}

	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING`,
		m.Version,
	); err != nil {
		return fmt.Errorf("record version: %w", err)
	}

	return tx.Commit()
}

// ReadMigrationsDirFS is a helper so tests can provide a fake fs.FS and
// verify that our prefix/regex logic works. It's kept separate from
// DiscoverMigrations because on-disk path resolution matters for the
// real runner (we need absolute paths for error messages).
func ReadMigrationsDirFS(f fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(f, ".")
	if err != nil {
		return nil, err
	}
	var migs []Migration
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		migs = append(migs, Migration{
			Version: m[1],
			Name:    e.Name(),
			Path:    e.Name(),
		})
	}
	sort.SliceStable(migs, func(i, j int) bool {
		if len(migs[i].Version) != len(migs[j].Version) {
			return len(migs[i].Version) < len(migs[j].Version)
		}
		return migs[i].Version < migs[j].Version
	})
	return migs, nil
}
