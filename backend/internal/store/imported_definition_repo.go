package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/local/easysearch/backend/internal/model"
)

// ImportedDefinitionRepo persists user-imported YAML indexer
// definitions verbatim (spec §20.4). Cheaper than reconstructing the
// file on every read; the checksum lets Phase 6's catalog updater
// detect changes.
type ImportedDefinitionRepo struct {
	s *Store
}

func NewImportedDefinitionRepo(s *Store) *ImportedDefinitionRepo {
	return &ImportedDefinitionRepo{s: s}
}

// Create stores a new imported definition. id must be unique; the
// checksum is computed from Content if not already set.
func (r *ImportedDefinitionRepo) Create(in model.ImportedDefinition) error {
	if in.CreatedAt.IsZero() {
		in.CreatedAt = time.Now().UTC()
	}
	in.UpdatedAt = time.Now().UTC()
	if in.Checksum == "" {
		sum := sha256.Sum256([]byte(in.Content))
		in.Checksum = hex.EncodeToString(sum[:])
	}

	_, err := r.s.DB.ExecContext(context.Background(), `
		INSERT INTO imported_definitions(
			id, version, content, checksum, created_at, updated_at
		) VALUES (?,?,?,?,?,?)
	`,
		in.ID, in.Version, in.Content, in.Checksum,
		in.CreatedAt.Format(time.RFC3339), in.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("%w: id=%s", ErrDuplicate, in.ID)
		}
		return fmt.Errorf("insert imported: %w", err)
	}
	return nil
}

// Get returns the raw imported definition by id, or ErrNotFound.
func (r *ImportedDefinitionRepo) Get(id string) (model.ImportedDefinition, error) {
	row := r.s.DB.QueryRowContext(context.Background(),
		`SELECT id, version, content, checksum, created_at, updated_at FROM imported_definitions WHERE id = ?`,
		id,
	)
	return scanImported(row)
}

// List returns every imported definition, sorted by id.
func (r *ImportedDefinitionRepo) List() ([]model.ImportedDefinition, error) {
	rows, err := r.s.DB.QueryContext(context.Background(),
		`SELECT id, version, content, checksum, created_at, updated_at FROM imported_definitions ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list imported: %w", err)
	}
	defer rows.Close()

	var out []model.ImportedDefinition
	for rows.Next() {
		d, err := scanImported(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Delete removes a definition by id. ErrNotFound if missing.
func (r *ImportedDefinitionRepo) Delete(id string) error {
	res, err := r.s.DB.ExecContext(context.Background(),
		`DELETE FROM imported_definitions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete imported: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanImported(s scanner) (model.ImportedDefinition, error) {
	var d model.ImportedDefinition
	var created, updated string
	if err := s.Scan(&d.ID, &d.Version, &d.Content, &d.Checksum, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.ImportedDefinition{}, ErrNotFound
		}
		return model.ImportedDefinition{}, fmt.Errorf("scan imported: %w", err)
	}
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		d.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, updated); err == nil {
		d.UpdatedAt = t
	}
	return d, nil
}
