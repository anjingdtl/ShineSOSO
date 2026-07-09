// Package builtin embeds the shipped catalog (manifest.json +
// definitions/*.yml) at compile time. Phase 6 introduces this so the
// runtime updater (catalog/updater.go) has a stable source of truth to
// fall back to when an online update fails.
//
// The embed.FS exposes three trees:
//   - manifest.json              — top-level metadata
//   - definitions/*.yml          — indexer YAML files
//   - signatures/                — reserved for future signature files (empty in v1)
package builtin

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed manifest.json
//go:embed definitions/*.yml
//go:embed signatures/*
var fsys embed.FS

// Manifest mirrors the JSON structure on disk.
type Manifest struct {
	Schema      int               `json:"schema"`
	Version     string            `json:"version"`
	GeneratedAt string            `json:"generatedAt"`
	Definitions []ManifestEntry   `json:"definitions"`
}

// ManifestEntry is one row in manifest.json's `definitions` array.
type ManifestEntry struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	File    string `json:"file"`
	SHA256  string `json:"sha256"`
}

// ReadManifest parses the embedded manifest.json.
func ReadManifest() (Manifest, error) {
	var m Manifest
	raw, err := fsys.ReadFile("manifest.json")
	if err != nil {
		return m, fmt.Errorf("read manifest: %w", err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Schema != 1 {
		return m, fmt.Errorf("unsupported manifest schema %d", m.Schema)
	}
	return m, nil
}

// DefinitionYAML returns the raw YAML bytes for one definition by ID.
func DefinitionYAML(id string) ([]byte, error) {
	m, err := ReadManifest()
	if err != nil {
		return nil, err
	}
	for _, e := range m.Definitions {
		if e.ID == id {
			return fsys.ReadFile(e.File)
		}
	}
	return nil, fmt.Errorf("builtin catalog: id %q not in manifest", id)
}

// AllYAMLs returns every definition's YAML in the order they appear in
// the manifest. Used by the updater to roll back to a known-good state.
func AllYAMLs() (map[string][]byte, error) {
	m, err := ReadManifest()
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(m.Definitions))
	for _, e := range m.Definitions {
		b, err := fsys.ReadFile(e.File)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.File, err)
		}
		out[e.ID] = b
	}
	return out, nil
}

// VerifyChecksum returns true if the YAML bytes for `id` match the
// sha256 declared in manifest.json. Useful in tests and as a final
// guard before the updater activates a downloaded file.
func VerifyChecksum(id string) (bool, error) {
	m, err := ReadManifest()
	if err != nil {
		return false, err
	}
	for _, e := range m.Definitions {
		if e.ID == id {
			b, err := fsys.ReadFile(e.File)
			if err != nil {
				return false, err
			}
			sum := sha256.Sum256(b)
			return hex.EncodeToString(sum[:]) == e.SHA256, nil
		}
	}
	return false, fmt.Errorf("builtin catalog: id %q not in manifest", id)
}

// FileCount returns how many embedded files exist (for diagnostics).
func FileCount() int {
	count := 0
	_ = fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		count++
		return nil
	})
	return count
}

// SortedIDs returns every definition ID in lexicographic order. Useful
// for stable test output and catalog listings.
func SortedIDs() ([]string, error) {
	m, err := ReadManifest()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(m.Definitions))
	for _, e := range m.Definitions {
		ids = append(ids, e.ID)
	}
	sort.Strings(ids)
	return ids, nil
}