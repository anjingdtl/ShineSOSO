package diagnostics

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSanitizeLine_StripsMagnet(t *testing.T) {
	in := `caller msg="got result" magnet="magnet:?xt=urn:btih:abcdef1234567890abcdef1234567890abcdef12&dn=ubuntu.iso"`
	out := string(SanitizeLine(in))
	if strings.Contains(out, "abcdef1234567890abcdef1234567890abcdef12") {
		t.Fatalf("magnet infohash leaked: %q", out)
	}
	if !strings.Contains(out, "<redacted>") {
		t.Fatalf("missing redacted marker: %q", out)
	}
}

func TestSanitizeLine_StripsBareInfoHash(t *testing.T) {
	in := `infohash=abcdef1234567890abcdef1234567890abcdef12 fetched`
	out := string(SanitizeLine(in))
	if strings.Contains(out, "abcdef1234567890abcdef1234567890abcdef12") {
		t.Fatalf("bare 40-hex leaked: %q", out)
	}
	if !strings.Contains(out, "<btih-redacted>") {
		t.Fatalf("expected <btih-redacted>, got %q", out)
	}
}

func TestSanitizeLine_StripsBase32BTIH(t *testing.T) {
	in := `infohash=ABCDEFGHIJKLMNOPQRSTUVWXYZ234567 fetched`
	out := string(SanitizeLine(in))
	if strings.Contains(out, "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567") {
		t.Fatalf("base32 BTIH leaked: %q", out)
	}
}

func TestSanitizeLine_StripsCredentialedURL(t *testing.T) {
	in := `fetching https://user:hunter2@example.com/path`
	out := string(SanitizeLine(in))
	if strings.Contains(out, "hunter2") {
		t.Fatalf("credential leaked: %q", out)
	}
	if !strings.Contains(out, "[redacted-url-with-creds]") {
		t.Fatalf("expected creds redaction, got %q", out)
	}
}

func TestSanitizeLine_StripsSearchKeywords(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"keyword= quoted", `indexer started keyword="ubuntu 22.04"`, "keyword=<redacted>"},
		{"keyword= unquoted", `indexer started keyword=ubuntu22`, "keyword=<redacted>"},
		{"q= quoted", `GET /search?q="debian"`, "q=<redacted>"},
		{"query= unquoted", `indexer query=archlinux`, "query=<redacted>"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := string(SanitizeLine(tc.in))
			if !strings.Contains(out, tc.want) {
				t.Fatalf("got %q, want substring %q", out, tc.want)
			}
		})
	}
}

func TestSanitizeLine_PreservesUnrelatedText(t *testing.T) {
	in := `2026/07/09 12:00:00 INFO search session_started session=abc123 timeout=15s`
	out := string(SanitizeLine(in))
	if out != in {
		t.Fatalf("clean line was modified: %q -> %q", in, out)
	}
}

func TestSanitizeErrorMessage_TrimsAndStrips(t *testing.T) {
	msg := "fetch failed: " + strings.Repeat("a", 300) + ` url=magnet:?xt=urn:btih:` +
		"abcdef1234567890abcdef1234567890abcdef12"
	got := SanitizeErrorMessage(msg)
	if len(got) > MaxErrorMessageLen+10 { // tolerate a few bytes of "…"
		t.Fatalf("message not trimmed: len=%d", len(got))
	}
	if strings.Contains(got, "abcdef1234567890abcdef1234567890abcdef12") {
		t.Fatalf("magnet hash leaked: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix: %q", got)
	}
}

func TestSanitizeBytes_HandlesLinesAndTrailing(t *testing.T) {
	in := []byte("line one\nline two\nno terminator")
	out := string(SanitizeBytes(in))
	if out != "line one\nline two\nno terminator" {
		t.Fatalf("clean input modified: %q", out)
	}
	mixed := []byte(`a magnet=?xt=urn:btih:abcdef1234567890abcdef1234567890abcdef12 b`)
	out = string(SanitizeBytes(mixed))
	if strings.Contains(out, "abcdef1234567890abcdef1234567890abcdef12") {
		t.Fatalf("btih leaked: %q", out)
	}
}

func TestBuild_ProducesValidZipWithExpectedFiles(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(logDir, "easysearch.log")
	if err := os.WriteFile(logPath, []byte("first line\nsecond magnet=?xt=urn:btih:abc line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap := Snapshot{
		Version:           "0.4.0",
		StartedAt:         time.Now().Add(-30 * time.Second),
		BuildGoOS:         runtime.GOOS,
		BuildGoArch:       runtime.GOARCH,
		DataDir:           dir,
		LogDir:            logDir,
		DBPath:            filepath.Join(dir, "easysearch.db"),
		SchemaVersion:     1,
		DefinitionVersion: "2026.07.1",
		InstalledIndexers: []HealthSummary{{
			ID: "id1", Name: "MockHTML", Status: "healthy",
			Enabled: true, ResponseTimeMs: 123, ConsecutiveFails: 0,
		}},
		Catalog: CatalogStatus{Source: "embedded", Version: "2026.07.1"},
	}
	logFiles, err := SortedLogFiles(logDir)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	sum, err := Build(&out, snap, logFiles)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(sum) != 64 {
		t.Fatalf("sha256 hex length = %d, want 64", len(sum))
	}
	r, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	names := map[string]bool{}
	for _, f := range r.File {
		names[f.Name] = true
	}
	wantFiles := []string{
		"README.txt", "version.txt", "os.txt", "uptime.txt",
		"schema.txt", "indexers.json", "catalog.json",
		"logs/easysearch.log",
	}
	for _, w := range wantFiles {
		if !names[w] {
			t.Errorf("zip missing %s", w)
		}
	}
	// Validate schema.txt content.
	for _, f := range r.File {
		if f.Name != "schema.txt" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "db_schema_version=1") {
			t.Errorf("schema.txt missing db_schema_version=1: %s", body)
		}
	}
	// Validate indexers.json content.
	for _, f := range r.File {
		if f.Name != "indexers.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		var got []HealthSummary
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("indexers.json unmarshal: %v", err)
		}
		if len(got) != 1 || got[0].ID != "id1" {
			t.Errorf("indexers.json wrong: %+v", got)
		}
	}
	// Validate log sanitization.
	for _, f := range r.File {
		if f.Name != "logs/easysearch.log" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "magnet=?xt=urn:btih:abcdef") {
			t.Errorf("log contained unsanitized magnet: %s", body)
		}
	}
}

func TestBuild_RejectsBadSnapshot(t *testing.T) {
	cases := []struct {
		name string
		snap Snapshot
	}{
		{"empty version", Snapshot{StartedAt: time.Now()}},
		{"zero started at", Snapshot{Version: "0.4.0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if _, err := Build(&out, tc.snap, nil); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestBuild_NoLogFilesIsOK(t *testing.T) {
	snap := Snapshot{
		Version:   "0.4.0",
		StartedAt: time.Now(),
		BuildGoOS: runtime.GOOS,
	}
	var out bytes.Buffer
	if _, err := Build(&out, snap, nil); err != nil {
		t.Fatalf("Build with no logs: %v", err)
	}
}

func TestTailFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.log")
	if err := os.WriteFile(p, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := tailFile(p, 5)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "world" {
		t.Fatalf("got %q, want %q", out, "world")
	}
}

func TestSortedLogFiles_NewestFirst(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "easysearch-2026-07-08.log")
	now := filepath.Join(dir, "easysearch.log")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	older := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(old, older, older); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(now, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := SortedLogFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d files, want 2: %v", len(got), got)
	}
	if filepath.Base(got[0]) != "easysearch.log" {
		t.Errorf("expected newest first, got %v", got)
	}
}

func TestSortedLogFiles_MissingDirReturnsNil(t *testing.T) {
	got, err := SortedLogFiles(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestSortedLogFiles_IgnoresNonLog(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "easysearch.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "easysearch.log"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := SortedLogFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 log file, got %d: %v", len(got), got)
	}
}