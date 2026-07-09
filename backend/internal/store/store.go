// Package store wraps the SQLite database. It owns the *sql.DB and the
// schema migrations. Higher-level repositories live in subpackages.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; registers itself as "sqlite"
)

// Store is the application's database handle.
type Store struct {
	DB *sql.DB
}

// Open opens (creating if needed) a SQLite database at path and applies
// migrations. WAL mode and foreign keys are enabled.
func Open(path string) (*Store, error) {
	// _pragma options turn on WAL mode and foreign keys per-connection;
	// safe=off lets us create the file on first open.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close releases the underlying connection pool.
func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

// QueryRow is a convenience wrapper around the underlying *sql.DB.
func (s *Store) QueryRow(query string, args ...any) *sql.Row {
	return s.DB.QueryRow(query, args...)
}

// SchemaVersion returns the highest applied migration version. Returns
// 0 if the schema_migrations table is empty or missing (which only
// happens before migrate() has run — callers usually hit Open first).
func (s *Store) SchemaVersion() (int, error) {
	if s == nil || s.DB == nil {
		return 0, fmt.Errorf("store: nil")
	}
	var v sql.NullInt64
	err := s.DB.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("schema version: %w", err)
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}