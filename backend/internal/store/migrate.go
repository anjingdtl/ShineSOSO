package store

import (
	"context"
	"fmt"
	"sort"
)

// migrate applies every migration in order. Idempotent: rerunning is a
// no-op. We keep schema in code (not files) so it lives next to the code
// that uses it.
func (s *Store) migrate() error {
	ctx := context.Background()
	if _, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	versions := make([]int, 0, len(migrations))
	for v := range migrations {
		versions = append(versions, v)
	}
	sort.Ints(versions)
	for _, v := range versions {
		sql := migrations[v]
		var seen int
		if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, v).Scan(&seen); err != nil {
			return fmt.Errorf("check migration %d: %w", v, err)
		}
		if seen > 0 {
			continue
		}
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", v, err)
		}
		if _, err := tx.ExecContext(ctx, sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", v, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, datetime('now'))`, v); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", v, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", v, err)
		}
	}
	return nil
}

// migrations are applied in ascending version order. Add new entries
// at the end; never edit an applied migration.
var migrations = map[int]string{
	1: `
CREATE TABLE installed_indexers (
    id TEXT PRIMARY KEY,
    definition_id TEXT NOT NULL,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    definition_version TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unknown',
    last_checked_at TEXT,
    last_success_at TEXT,
    last_error TEXT,
    response_time_ms INTEGER,
    consecutive_fails INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE(name)
);

CREATE INDEX idx_installed_indexers_enabled ON installed_indexers(enabled);

CREATE TABLE indexer_health_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    indexer_id TEXT NOT NULL,
    status TEXT NOT NULL,
    duration_ms INTEGER,
    error_code TEXT,
    error_message TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_health_events_indexer ON indexer_health_events(indexer_id, created_at);

CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE imported_definitions (
    id TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    content TEXT NOT NULL,
    checksum TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
`,
}
