package database

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"testing/fstest"
)

func TestReadMigrationsDirFS_OrdersAndFilters(t *testing.T) {
	files := fstest.MapFS{
		"README.md":         &fstest.MapFile{Data: []byte("ignored")},
		".gitkeep":          &fstest.MapFile{Data: []byte("ignored")},
		"0001_a.sql":        &fstest.MapFile{Data: []byte("1")},
		"0002-b.sql":        &fstest.MapFile{Data: []byte("2")},
		"3_something.sql":   &fstest.MapFile{Data: []byte("3")},
		"nodigitprefix.sql": &fstest.MapFile{Data: []byte("ignored")},
		"notasql.txt":       &fstest.MapFile{Data: []byte("ignored")},
	}

	migs, err := ReadMigrationsDirFS(files)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// Expect numeric ordering by prefix: 3 ("3"), 0001, 0002.
	// (Our sort orders shorter version strings first; then lexicographic
	// within equal lengths.)
	got := make([]string, len(migs))
	for i, m := range migs {
		got[i] = m.Version + ":" + m.Name
	}
	sort.Strings(got)
	want := []string{
		"0001:0001_a.sql",
		"0002:0002-b.sql",
		"3:3_something.sql",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected migs got=%v want=%v", got, want)
	}
}

func TestDiscoverMigrations_OnRealDir(t *testing.T) {
	// Use the project's actual database/migrations dir. It contains the
	// numbered migrations committed with the project.
	projectRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	migsDir := filepath.Join(projectRoot, "database", "migrations")
	if _, err := os.Stat(migsDir); err != nil {
		t.Skipf("migrations dir not present: %v", err)
	}

	migs, err := DiscoverMigrations(migsDir)
	if err != nil {
		t.Fatalf("DiscoverMigrations err: %v", err)
	}

	if len(migs) < 2 {
		t.Fatalf("expected at least 2 migrations, got %d", len(migs))
	}

	// The first two versions must be "0001" and "0002".
	firstTwo := [2]string{migs[0].Version, migs[1].Version}
	if firstTwo != [2]string{"0001", "0002"} {
		t.Fatalf("first two versions = %v, want [0001 0002]", firstTwo)
	}

	// Paths must resolve to real readable files.
	for _, m := range migs {
		if _, err := os.Stat(m.Path); err != nil {
			t.Errorf("migration %s path %s not stat-able: %v", m.Version, m.Path, err)
		}
	}
}

func TestDiscoverMigrations_DuplicateVersionRejected(t *testing.T) {
	dir := t.TempDir()
	write := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("--hi"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("0001_a.sql")
	write("0001_b.sql")

	_, err := DiscoverMigrations(dir)
	if err == nil {
		t.Fatalf("expected duplicate version error, got nil")
	}
}

func TestDiscoverMigrations_MissingDir(t *testing.T) {
	_, err := DiscoverMigrations(filepath.Join(os.TempDir(), "this-dir-definitely-does-not-exist-authix"))
	if err == nil {
		t.Fatalf("expected error for missing dir, got nil")
	}
}

func TestParamsURL(t *testing.T) {
	p := Params{
		Host: "h", Port: "5432", User: "u", Password: "pw", DBName: "d",
	}
	got := p.URL()
	want := "host=h port=5432 user=u password=pw dbname=d sslmode=disable"
	if got != want {
		t.Errorf("URL = %q\nwant = %q", got, want)
	}
}

// Compile-time guard that ReadMigrationsDirFS accepts fs.FS (used by tests).
var _ func(fs.FS) ([]Migration, error) = ReadMigrationsDirFS
