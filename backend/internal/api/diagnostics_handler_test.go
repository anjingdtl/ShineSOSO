package api

import (
	"archive/zip"
	"bytes"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/local/easysearch/backend/internal/store"
)

// fakeStore implements StoreVersionProvider for tests.
type fakeStore struct {
	version int
	err     error
}

func (f *fakeStore) SchemaVersion() (int, error) { return f.version, f.err }

// fakeRepo implements RepoSummaryProvider for tests.
type fakeRepo struct {
	rows []store.DiagnosticsRow
	err  error
}

func (f *fakeRepo) DiagnosticsSummary() ([]store.DiagnosticsRow, error) {
	return f.rows, f.err
}

// fakeCatalog implements CatalogStatusProvider for tests.
type fakeCatalog struct {
	source  string
	version string
}

func (f *fakeCatalog) ActiveSource() string  { return f.source }
func (f *fakeCatalog) ActiveVersion() string { return f.version }

func newTestDiagnosticsHandler(t *testing.T) (*DiagnosticsHandler, string) {
	t.Helper()
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "easysearch.log"),
		[]byte("INFO boot version=0.4.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &DiagnosticsHandler{
		StartTime: time.Now().Add(-time.Minute),
		Version:   "0.4.0",
		DataDir:   dir,
		LogDir:    logDir,
		DBPath:    filepath.Join(dir, "easysearch.db"),
		BuildOS:   "windows",
		BuildArch: "amd64",
		Store:     &fakeStore{version: 1},
		Repo: &fakeRepo{rows: []store.DiagnosticsRow{{
			ID: "idx1", Name: "MockHTML", Status: "healthy",
			Enabled: true, ResponseTimeMs: 88, ConsecutiveFails: 0,
		}}},
		Catalog: &fakeCatalog{source: "embedded", version: "2026.07.1"},
		Logger:  slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	return h, dir
}

func TestDiagnosticsHandlerReturnsZip(t *testing.T) {
	h, _ := newTestDiagnosticsHandler(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/system/diagnostics", nil)
	h.Get(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status want 200 got %d body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type want application/zip got %q", ct)
	}
	if cd := rr.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition missing attachment: %q", cd)
	}
	if sha := rr.Header().Get("X-Diagnostics-SHA256"); len(sha) != 64 {
		t.Errorf("X-Diagnostics-SHA256 len want 64 got %d", len(sha))
	}

	r, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	names := map[string]bool{}
	for _, f := range r.File {
		names[f.Name] = true
	}
	for _, want := range []string{"version.txt", "schema.txt", "indexers.json", "logs/easysearch.log"} {
		if !names[want] {
			t.Errorf("zip missing %s", want)
		}
	}
}

func TestDiagnosticsHandlerRejectsNonGET(t *testing.T) {
	h, _ := newTestDiagnosticsHandler(t)
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest("POST", "/api/v1/system/diagnostics", nil))
	if rr.Code != 405 {
		t.Fatalf("want 405 got %d", rr.Code)
	}
	if rr.Header().Get("Allow") != "GET" {
		t.Errorf("Allow header missing or wrong: %q", rr.Header().Get("Allow"))
	}
}

func TestDiagnosticsHandlerNilSafety(t *testing.T) {
	var h *DiagnosticsHandler
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest("GET", "/api/v1/system/diagnostics", nil))
	if rr.Code != 500 {
		t.Fatalf("nil handler should 500, got %d", rr.Code)
	}
}

func TestDiagnosticsHandlerSurvivesPartialFailures(t *testing.T) {
	h, _ := newTestDiagnosticsHandler(t)
	h.Store = &fakeStore{err: errFake}
	h.Repo = &fakeRepo{err: errFake}
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest("GET", "/api/v1/system/diagnostics", nil))
	if rr.Code != 200 {
		t.Fatalf("expected 200 with degraded data, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestDiagnosticsHandlerMissingLogDir(t *testing.T) {
	h, _ := newTestDiagnosticsHandler(t)
	h.LogDir = filepath.Join(h.DataDir, "no-such-dir")
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest("GET", "/api/v1/system/diagnostics", nil))
	if rr.Code != 200 {
		t.Fatalf("missing log dir should still produce a zip, got %d", rr.Code)
	}
}

// errFake is a sentinel used by the partial-failures test.
var errFake = errFakeType{}

type errFakeType struct{}

func (errFakeType) Error() string { return "fake error for tests" }