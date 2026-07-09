// Smoke test: verify modernc.org/sqlite can open and close a real file.
package store

import (
	"path/filepath"
	"testing"
)

func TestSQLiteOpensAndCloses(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var v string
	if err := db.QueryRow("SELECT 'ok'").Scan(&v); err != nil {
		t.Fatalf("query: %v", err)
	}
	if v != "ok" {
		t.Errorf("got %q, want ok", v)
	}
}