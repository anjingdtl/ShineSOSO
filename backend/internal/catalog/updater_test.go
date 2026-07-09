package catalog_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/local/easysearch/backend/internal/catalog"
	"github.com/local/easysearch/backend/internal/catalog/builtin"
	"github.com/local/easysearch/backend/internal/model"
	"github.com/local/easysearch/backend/internal/store"
)

func newTestRepo(t *testing.T) *store.IndexerRepo {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return store.NewIndexerRepo(st)
}

func writeManifest(t *testing.T, dir string, entries []map[string]string, version string) {
	t.Helper()
	m := builtin.Manifest{
		Schema: 1, Version: version,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, e := range entries {
		m.Definitions = append(m.Definitions, builtin.ManifestEntry{
			ID: e["id"], Version: e["version"],
			File: e["file"], SHA256: e["sha256"],
		})
	}
	raw, _ := json.MarshalIndent(&m, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestUpdater_ActivateEmbedded_loadsAndRegisters(t *testing.T) {
	repo := newTestRepo(t)
	cat := catalog.New(repo)
	u := catalog.NewUpdater(cat, catalog.UpdaterConfig{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	report, err := u.ActivateEmbedded()
	if err != nil && !strings.Contains(err.Error(), "unchanged") {
		t.Fatalf("ActivateEmbedded: %v", err)
	}
	if report == nil || report.After == "" {
		t.Errorf("empty report")
	}
	// The catalog should now contain at least the two embedded YAMLs.
	defs := cat.Definitions()
	seen := map[string]bool{}
	for _, d := range defs {
		seen[d.ID] = true
	}
	if !seen["example-public-html"] || !seen["example-torznab"] {
		t.Errorf("missing built-ins: %v", seen)
	}
}

func TestUpdater_Activate_rejectsChecksumMismatch(t *testing.T) {
	repo := newTestRepo(t)
	cat := catalog.New(repo)
	u := catalog.NewUpdater(cat, catalog.UpdaterConfig{})

	dir := t.TempDir()
	ymlPath := filepath.Join(dir, "definitions", "broken.yml")
	if err := os.MkdirAll(filepath.Dir(ymlPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := []byte("schema: 1\nid: broken\nname: Broken\nversion: 1.0.0\ntype: public\nprotocol: declarative\nlinks: [\"https://example.com/\"]\nsearch: {method: GET, path: /, query: {}}\nresponse: {format: html, rows: {selector: li}, fields: {title: {selector: a, value: text, required: true}}}\n")
	if err := os.WriteFile(ymlPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, dir, []map[string]string{
		{"id": "broken", "version": "1.0.0", "file": "definitions/broken.yml", "sha256": "deadbeef"},
	}, "2026.07.2")

	_, err := u.Activate(os.DirFS(dir), ".")
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch, got %v", err)
	}
}

func TestUpdater_Activate_rejectsInvalidYAML(t *testing.T) {
	repo := newTestRepo(t)
	cat := catalog.New(repo)
	u := catalog.NewUpdater(cat, catalog.UpdaterConfig{})

	dir := t.TempDir()
	ymlPath := filepath.Join(dir, "definitions", "incomplete.yml")
	if err := os.MkdirAll(filepath.Dir(ymlPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Missing required name field -> Validate fails.
	raw := []byte("schema: 1\nid: incomplete\nversion: 1.0.0\ntype: public\nprotocol: declarative\nlinks: [\"https://example.com/\"]\nsearch: {method: GET, path: /, query: {}}\nresponse: {format: html, rows: {selector: li}, fields: {title: {selector: a, value: text, required: true}}}\n")
	if err := os.WriteFile(ymlPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, dir, []map[string]string{
		{"id": "incomplete", "version": "1.0.0", "file": "definitions/incomplete.yml", "sha256": sha256Hex(raw)},
	}, "2026.07.3")

	_, err := u.Activate(os.DirFS(dir), ".")
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestUpdater_Activate_diffCalculatesCorrectly(t *testing.T) {
	repo := newTestRepo(t)
	cat := catalog.New(repo)
	u := catalog.NewUpdater(cat, catalog.UpdaterConfig{})

	dir := t.TempDir()
	rawAlpha := []byte("schema: 1\nid: alpha\nname: Alpha\nversion: 1.0.0\ntype: public\nprotocol: declarative\nlinks: [\"https://example.com/\"]\nsearch: {method: GET, path: /, query: {}}\nresponse: {format: html, rows: {selector: li}, fields: {title: {selector: a, value: text, required: true}}}\n")
	rawBeta := []byte("schema: 1\nid: beta\nname: Beta\nversion: 1.0.0\ntype: public\nprotocol: declarative\nlinks: [\"https://example.com/\"]\nsearch: {method: GET, path: /, query: {}}\nresponse: {format: html, rows: {selector: li}, fields: {title: {selector: a, value: text, required: true}}}\n")
	if err := os.MkdirAll(filepath.Join(dir, "definitions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "definitions", "alpha.yml"), rawAlpha, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "definitions", "beta.yml"), rawBeta, 0o644); err != nil {
		t.Fatal(err)
	}

	writeManifest(t, dir, []map[string]string{
		{"id": "alpha", "version": "1.0.0", "file": "definitions/alpha.yml", "sha256": sha256Hex(rawAlpha)},
	}, "v1")
	r1, err := u.Activate(os.DirFS(dir), ".")
	if err != nil {
		t.Fatalf("activate 1: %v", err)
	}
	if r1.Added[0] != "alpha" {
		t.Errorf("added should be [alpha], got %v", r1.Added)
	}

	// Replace manifest with [alpha, beta]. alpha is unchanged, beta is new.
	writeManifest(t, dir, []map[string]string{
		{"id": "alpha", "version": "1.0.0", "file": "definitions/alpha.yml", "sha256": sha256Hex(rawAlpha)},
		{"id": "beta", "version": "1.0.0", "file": "definitions/beta.yml", "sha256": sha256Hex(rawBeta)},
	}, "v2")
	r2, err := u.Activate(os.DirFS(dir), ".")
	if err != nil {
		t.Fatalf("activate 2: %v", err)
	}
	if len(r2.Added) != 1 || r2.Added[0] != "beta" {
		t.Errorf("added should be [beta], got %v", r2.Added)
	}
	if len(r2.Changed) != 0 {
		t.Errorf("alpha unchanged: changed should be empty, got %v", r2.Changed)
	}

	// Now change alpha's bytes (and version) and re-activate.
	rawAlpha2 := []byte("schema: 1\nid: alpha\nname: Alpha v2\nversion: 1.0.1\ntype: public\nprotocol: declarative\nlinks: [\"https://example.com/\"]\nsearch: {method: GET, path: /, query: {}}\nresponse: {format: html, rows: {selector: li}, fields: {title: {selector: a, value: text, required: true}}}\n")
	if err := os.WriteFile(filepath.Join(dir, "definitions", "alpha.yml"), rawAlpha2, 0o644); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, dir, []map[string]string{
		{"id": "alpha", "version": "1.0.1", "file": "definitions/alpha.yml", "sha256": sha256Hex(rawAlpha2)},
		{"id": "beta", "version": "1.0.0", "file": "definitions/beta.yml", "sha256": sha256Hex(rawBeta)},
	}, "v3")
	r3, err := u.Activate(os.DirFS(dir), ".")
	if err != nil {
		t.Fatalf("activate 3: %v", err)
	}
	if len(r3.Changed) != 1 || r3.Changed[0].ID != "alpha" {
		t.Errorf("changed should be [alpha], got %v", r3.Changed)
	}
	if r3.Changed[0].NewVersion != "1.0.1" {
		t.Errorf("new version=%q", r3.Changed[0].NewVersion)
	}
}

func TestUpdater_Fetch_downloadsAndActivates(t *testing.T) {
	// Serve a manifest + its referenced YAMLs from httptest.
	rawA := []byte("schema: 1\nid: remote-a\nname: Remote A\nversion: 2.0.0\ntype: public\nprotocol: declarative\nlinks: [\"https://example.com/\"]\nsearch: {method: GET, path: /, query: {}}\nresponse: {format: html, rows: {selector: li}, fields: {title: {selector: a, value: text, required: true}}}\n")
	rawB := []byte("schema: 1\nid: remote-b\nname: Remote B\nversion: 1.5.0\ntype: public\nprotocol: declarative\nlinks: [\"https://example.com/\"]\nsearch: {method: GET, path: /, query: {}}\nresponse: {format: html, rows: {selector: li}, fields: {title: {selector: a, value: text, required: true}}}\n")
	mfst := builtin.Manifest{
		Schema: 1, Version: "2030.01.1",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Definitions: []builtin.ManifestEntry{
			{ID: "remote-a", Version: "2.0.0", File: "definitions/remote-a.yml", SHA256: sha256Hex(rawA)},
			{ID: "remote-b", Version: "1.5.0", File: "definitions/remote-b.yml", SHA256: sha256Hex(rawB)},
		},
	}
	mfstRaw, _ := json.Marshal(&mfst)

	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(mfstRaw)
	})
	mux.HandleFunc("/definitions/remote-a.yml", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(rawA)
	})
	mux.HandleFunc("/definitions/remote-b.yml", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(rawB)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	repo := newTestRepo(t)
	cat := catalog.New(repo)
	cacheDir := t.TempDir()
	u := catalog.NewUpdater(cat, catalog.UpdaterConfig{
		ManifestURL: srv.URL + "/manifest.json",
		CacheDir:    cacheDir,
	})

	report, err := u.Fetch(t.Context())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if report.After != "2030.01.1" {
		t.Errorf("after=%q", report.After)
	}
	if len(cat.Definitions()) == 0 {
		t.Errorf("catalog empty after fetch")
	}

	// Both files should now be on disk under cacheDir.
	if _, err := os.Stat(filepath.Join(cacheDir, "manifest.json")); err != nil {
		t.Errorf("manifest.json not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "definitions", "remote-a.yml")); err != nil {
		t.Errorf("remote-a.yml not written: %v", err)
	}
}

func TestUpdater_Fetch_rejectsBadChecksum(t *testing.T) {
	rawA := []byte("schema: 1\nid: remote-a\nname: Remote A\nversion: 2.0.0\ntype: public\nprotocol: declarative\nlinks: [\"https://example.com/\"]\nsearch: {method: GET, path: /, query: {}}\nresponse: {format: html, rows: {selector: li}, fields: {title: {selector: a, value: text, required: true}}}\n")
	mfst := builtin.Manifest{
		Schema: 1, Version: "2030.01.2",
		Definitions: []builtin.ManifestEntry{
			{ID: "remote-a", Version: "2.0.0", File: "definitions/remote-a.yml", SHA256: "wrongchecksum"},
		},
	}
	mfstRaw, _ := json.Marshal(&mfst)
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(mfstRaw)
	})
	mux.HandleFunc("/definitions/remote-a.yml", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(rawA)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	repo := newTestRepo(t)
	cat := catalog.New(repo)
	u := catalog.NewUpdater(cat, catalog.UpdaterConfig{
		ManifestURL: srv.URL + "/manifest.json",
		CacheDir:    t.TempDir(),
	})
	_, err := u.Fetch(t.Context())
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch error, got %v", err)
	}
}

func TestUpdater_Fetch_rejectsDefinitionValidationFailure(t *testing.T) {
	rawA := []byte("schema: 1\nid: remote-a\nversion: 1.0.0\ntype: public\nprotocol: declarative\nlinks: [\"https://example.com/\"]\nsearch: {method: GET, path: /, query: {}}\nresponse: {format: html, rows: {selector: li}, fields: {title: {selector: a, value: text, required: true}}}\n") // missing name
	mfst := builtin.Manifest{
		Schema: 1, Version: "2030.01.3",
		Definitions: []builtin.ManifestEntry{
			{ID: "remote-a", Version: "1.0.0", File: "definitions/remote-a.yml", SHA256: sha256Hex(rawA)},
		},
	}
	mfstRaw, _ := json.Marshal(&mfst)
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(mfstRaw)
	})
	mux.HandleFunc("/definitions/remote-a.yml", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(rawA)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	repo := newTestRepo(t)
	cat := catalog.New(repo)
	u := catalog.NewUpdater(cat, catalog.UpdaterConfig{
		ManifestURL: srv.URL + "/manifest.json",
		CacheDir:    t.TempDir(),
	})
	_, err := u.Fetch(t.Context())
	if err == nil || !strings.Contains(err.Error(), "validate") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

// Use model to keep the import line alive in case the file grows.
var _ = model.InstalledIndexer{}