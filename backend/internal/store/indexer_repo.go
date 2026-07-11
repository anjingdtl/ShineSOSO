package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/local/easysearch/backend/internal/model"
)

// ErrNotFound is returned when a Get or Update target does not exist.
var ErrNotFound = errors.New("not found")

// ErrDuplicate is returned when an INSERT violates a UNIQUE constraint.
var ErrDuplicate = errors.New("duplicate")

// IndexerRepo persists model.InstalledIndexer rows.
type IndexerRepo struct {
	s *Store
}

func NewIndexerRepo(s *Store) *IndexerRepo {
	return &IndexerRepo{s: s}
}

// Create inserts a new installed indexer. name must be unique.
func (r *IndexerRepo) Create(in model.InstalledIndexer) error {
	_, err := r.s.DB.ExecContext(context.Background(), `
		INSERT INTO installed_indexers(
			id, definition_id, name, base_url, enabled, definition_version,
			status, last_checked_at, last_success_at, last_error,
			response_time_ms, consecutive_fails, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		in.ID, in.DefinitionID, in.Name, in.BaseURL, boolToInt(in.Enabled), in.DefinitionVersion,
		in.Status, timePtrToStr(in.LastCheckedAt), timePtrToStr(in.LastSuccessAt), nullIfEmpty(in.LastError),
		in.ResponseTimeMs, in.ConsecutiveFails, in.CreatedAt.UTC().Format(time.RFC3339), in.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("%w: name=%s", ErrDuplicate, in.Name)
		}
		return fmt.Errorf("insert indexer: %w", err)
	}
	return nil
}

// Get returns one indexer by id.
func (r *IndexerRepo) Get(id string) (model.InstalledIndexer, error) {
	row := r.s.DB.QueryRow(`
		SELECT id, definition_id, name, base_url, enabled, definition_version,
			status, last_checked_at, last_success_at, last_error,
			response_time_ms, consecutive_fails, created_at, updated_at
		FROM installed_indexers WHERE id = ?
	`, id)
	return scanIndexer(row)
}

// List returns all indexers, ordered by created_at ascending.
func (r *IndexerRepo) List() ([]model.InstalledIndexer, error) {
	return r.list(`
		SELECT id, definition_id, name, base_url, enabled, definition_version,
			status, last_checked_at, last_success_at, last_error,
			response_time_ms, consecutive_fails, created_at, updated_at
		FROM installed_indexers ORDER BY created_at ASC
	`)
}

// ListEnabled returns only enabled indexers, ordered by created_at.
func (r *IndexerRepo) ListEnabled() ([]model.InstalledIndexer, error) {
	return r.list(`
		SELECT id, definition_id, name, base_url, enabled, definition_version,
			status, last_checked_at, last_success_at, last_error,
			response_time_ms, consecutive_fails, created_at, updated_at
		FROM installed_indexers WHERE enabled = 1 ORDER BY created_at ASC
	`)
}

func (r *IndexerRepo) list(query string) ([]model.InstalledIndexer, error) {
	rows, err := r.s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.InstalledIndexer
	for rows.Next() {
		idx, err := scanIndexerRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, idx)
	}
	return out, rows.Err()
}

// Update replaces mutable fields on the existing row. updated_at is set
// to the current time.
func (r *IndexerRepo) Update(in model.InstalledIndexer) error {
	now := time.Now().UTC()
	res, err := r.s.DB.ExecContext(context.Background(), `
		UPDATE installed_indexers
		SET name=?, base_url=?, enabled=?, status=?, last_error=?,
			response_time_ms=?, consecutive_fails=?, updated_at=?
		WHERE id=?
	`,
		in.Name, in.BaseURL, boolToInt(in.Enabled), in.Status, nullIfEmpty(in.LastError),
		in.ResponseTimeMs, in.ConsecutiveFails, now.Format(time.RFC3339), in.ID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%w: id=%s", ErrNotFound, in.ID)
	}
	return nil
}

// Delete removes the row by id.
func (r *IndexerRepo) Delete(id string) error {
	res, err := r.s.DB.ExecContext(context.Background(), `DELETE FROM installed_indexers WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%w: id=%s", ErrNotFound, id)
	}
	return nil
}

// BumpDefinitionVersion updates definition_version + updated_at for every
// installed indexer whose DefinitionID matches. Used by the catalog
// updater (Phase 6) after activating a new manifest; user enable state,
// base URL, and health snapshots are preserved.
func (r *IndexerRepo) BumpDefinitionVersion(definitionID, newVersion string) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.s.DB.ExecContext(context.Background(), `
		UPDATE installed_indexers
		SET definition_version=?, updated_at=?
		WHERE definition_id=?
	`,
		newVersion, now, definitionID,
	)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// SetStatus updates health-related fields after a health check.
func (r *IndexerRepo) SetStatus(id, status string, checkedAt *time.Time, lastErr *string, responseTimeMs int64) error {
	var lastErrVal any
	if lastErr != nil {
		lastErrVal = *lastErr
	}
	_, err := r.s.DB.ExecContext(context.Background(), `
		UPDATE installed_indexers
		SET status=?, last_checked_at=?, last_error=?, response_time_ms=?, updated_at=?
		WHERE id=?
	`, status, timePtrToStr(checkedAt), lastErrVal, responseTimeMs, time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// HealthEvent is one row in indexer_health_events.
type HealthEvent struct {
	ID           int64
	IndexerID    string
	Status       string
	DurationMs   int64
	ErrorCode    string
	ErrorMessage string
	CreatedAt    time.Time
}

// RecordHealthEvent appends a row to indexer_health_events.
func (r *IndexerRepo) RecordHealthEvent(indexerID, status string, durationMs int64, code, msg string) error {
	_, err := r.s.DB.ExecContext(context.Background(), `
		INSERT INTO indexer_health_events(indexer_id, status, duration_ms, error_code, error_message, created_at)
		VALUES (?,?,?,?,?,?)
	`, indexerID, status, durationMs, nullIfEmpty(code), nullIfEmpty(msg), time.Now().UTC().Format(time.RFC3339))
	return err
}

// ListHealthEvents returns the most recent N events for an indexer.
func (r *IndexerRepo) ListHealthEvents(indexerID string, limit int) ([]HealthEvent, error) {
	rows, err := r.s.DB.Query(`
		SELECT id, indexer_id, status, duration_ms, error_code, error_message, created_at
		FROM indexer_health_events WHERE indexer_id = ?
		ORDER BY created_at DESC LIMIT ?
	`, indexerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HealthEvent
	for rows.Next() {
		var ev HealthEvent
		var code, msg sql.NullString
		var createdAt string
		if err := rows.Scan(&ev.ID, &ev.IndexerID, &ev.Status, &ev.DurationMs, &code, &msg, &createdAt); err != nil {
			return nil, err
		}
		ev.ErrorCode = code.String
		ev.ErrorMessage = msg.String
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			ev.CreatedAt = t
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// DiagnosticsRow is a flat row used by the diagnostics bundle.
type DiagnosticsRow struct {
	ID               string
	Name             string
	Status           string
	Enabled          bool
	ResponseTimeMs   int64
	ConsecutiveFails int
	LastErrorCode    string
	LastErrorAt      string
}

// DiagnosticsSummary returns one row per installed indexer, augmented
// with the most recent health event's error code (if any). Used by the
// /system/diagnostics bundle; never includes base_url or full error
// messages.
func (r *IndexerRepo) DiagnosticsSummary() ([]DiagnosticsRow, error) {
	rows, err := r.s.DB.Query(`
		SELECT i.id, i.name, i.status, i.enabled, i.response_time_ms,
		       i.consecutive_fails,
		       (SELECT error_code FROM indexer_health_events h
		         WHERE h.indexer_id = i.id
		         ORDER BY created_at DESC LIMIT 1),
		       (SELECT created_at FROM indexer_health_events h
		         WHERE h.indexer_id = i.id
		         ORDER BY created_at DESC LIMIT 1)
		FROM installed_indexers i
		ORDER BY i.created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("diagnostics query: %w", err)
	}
	defer rows.Close()
	var out []DiagnosticsRow
	for rows.Next() {
		var (
			row                 DiagnosticsRow
			enabled             int
			respMs              sql.NullInt64
			code, healthEventAt sql.NullString
		)
		if err := rows.Scan(&row.ID, &row.Name, &row.Status, &enabled, &respMs,
			&row.ConsecutiveFails, &code, &healthEventAt); err != nil {
			return nil, fmt.Errorf("diagnostics scan: %w", err)
		}
		row.Enabled = enabled == 1
		if respMs.Valid {
			row.ResponseTimeMs = respMs.Int64
		}
		row.LastErrorCode = code.String
		row.LastErrorAt = healthEventAt.String
		out = append(out, row)
	}
	return out, rows.Err()
}

func scanIndexer(row *sql.Row) (model.InstalledIndexer, error) {
	var (
		idx           model.InstalledIndexer
		enabled       int
		lastChecked   sql.NullString
		lastSuccess   sql.NullString
		created, upda string
		lastErr       sql.NullString
	)
	err := row.Scan(
		&idx.ID, &idx.DefinitionID, &idx.Name, &idx.BaseURL, &enabled, &idx.DefinitionVersion,
		&idx.Status, &lastChecked, &lastSuccess, &lastErr,
		&idx.ResponseTimeMs, &idx.ConsecutiveFails, &created, &upda,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.InstalledIndexer{}, fmt.Errorf("%w: indexer id", ErrNotFound)
		}
		return model.InstalledIndexer{}, err
	}
	idx.Enabled = enabled != 0
	if t, err := parseNullableTime(lastChecked); err == nil {
		idx.LastCheckedAt = &t
	}
	if t, err := parseNullableTime(lastSuccess); err == nil {
		idx.LastSuccessAt = &t
	}
	idx.LastError = lastErr.String
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		idx.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, upda); err == nil {
		idx.UpdatedAt = t
	}
	return idx, nil
}

func scanIndexerRows(rows *sql.Rows) (model.InstalledIndexer, error) {
	var (
		idx           model.InstalledIndexer
		enabled       int
		lastChecked   sql.NullString
		lastSuccess   sql.NullString
		created, upda string
		lastErr       sql.NullString
	)
	if err := rows.Scan(
		&idx.ID, &idx.DefinitionID, &idx.Name, &idx.BaseURL, &enabled, &idx.DefinitionVersion,
		&idx.Status, &lastChecked, &lastSuccess, &lastErr,
		&idx.ResponseTimeMs, &idx.ConsecutiveFails, &created, &upda,
	); err != nil {
		return model.InstalledIndexer{}, err
	}
	idx.Enabled = enabled != 0
	if t, err := parseNullableTime(lastChecked); err == nil {
		idx.LastCheckedAt = &t
	}
	if t, err := parseNullableTime(lastSuccess); err == nil {
		idx.LastSuccessAt = &t
	}
	idx.LastError = lastErr.String
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		idx.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, upda); err == nil {
		idx.UpdatedAt = t
	}
	return idx, nil
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	return time.Parse(time.RFC3339, s)
}

func parseNullableTime(s sql.NullString) (time.Time, error) {
	if !s.Valid {
		return time.Time{}, fmt.Errorf("null time")
	}
	return time.Parse(time.RFC3339, s.String)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func timePtrToStr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}
