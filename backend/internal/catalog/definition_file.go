// Package catalog — Phase 5 YAML loader.
//
// A YAML indexer definition (spec-o1.md §13) is parsed into
// model.IndexerDefinition. The loader is deliberately strict: unknown
// fields raise an error so a typo in a YAML file is never silently
// ignored.
package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"

	"gopkg.in/yaml.v3"

	"github.com/local/easysearch/backend/internal/model"
)

// MaxDefinitionBytes caps a single YAML file at 512 KB (spec §13.8).
const MaxDefinitionBytes = 512 * 1024

// LoadDefinition parses YAML bytes into a model.IndexerDefinition.
//
// Rejects:
//   - oversize input (> MaxDefinitionBytes)
//   - YAML syntax errors
//   - unknown fields (strict mode)
func LoadDefinition(data []byte, filename string) (model.IndexerDefinition, error) {
	if len(data) > MaxDefinitionBytes {
		return model.IndexerDefinition{}, fmt.Errorf(
			"%w: definition %q is %d bytes (max %d)",
			ErrDefinitionTooLarge, filename, len(data), MaxDefinitionBytes,
		)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var def model.IndexerDefinition
	if err := dec.Decode(&def); err != nil {
		if errors.Is(err, io.EOF) {
			return model.IndexerDefinition{}, fmt.Errorf(
				"%w: %q is empty", ErrInvalidYAML, filename,
			)
		}
		return model.IndexerDefinition{}, fmt.Errorf(
			"%w: %q: %v", ErrInvalidYAML, filename, err,
		)
	}
	if def.ID == "" {
		return model.IndexerDefinition{}, fmt.Errorf(
			"%w: %q is missing id", ErrInvalidYAML, filename,
		)
	}
	return def, nil
}

// LoadDir reads *.yml / *.yaml files from an fs.FS and returns the
// parsed definitions plus per-file errors. The map is keyed by
// definition id; later files with duplicate ids overwrite earlier ones.
func LoadDir(fsys fs.FS, dir string) (map[string]model.IndexerDefinition, []error) {
	defs := map[string]model.IndexerDefinition{}
	var errs []error

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		errs = append(errs, fmt.Errorf("readdir %s: %w", dir, err))
		return defs, errs
	}

	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		ext := path.Ext(ent.Name())
		if ext != ".yml" && ext != ".yaml" {
			continue
		}
		full := path.Join(dir, ent.Name())
		raw, err := fs.ReadFile(fsys, full)
		if err != nil {
			errs = append(errs, fmt.Errorf("read %s: %w", full, err))
			continue
		}
		def, err := LoadDefinition(raw, full)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		defs[def.ID] = def
	}
	return defs, errs
}

// Sentinel errors callers can errors.Is() against.
var (
	ErrDefinitionTooLarge = errors.New("definition too large")
	ErrInvalidYAML        = errors.New("invalid yaml")
)
