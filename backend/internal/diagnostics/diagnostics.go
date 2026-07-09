// Package diagnostics builds a sanitized diagnostic ZIP for the
// GET /api/v1/system/diagnostics endpoint (spec §25.3, Phase 7-6).
//
// Privacy contract (must hold):
//   - No complete magnet links (magnet:?xt=urn:btih:<hash>...).
//   - No search keywords.
//   - No download URLs or torrent file contents.
//   - No user file paths outside the data dir.
//   - Health events keep status/duration/error code; error messages
//     are trimmed to 200 chars and have magnets stripped.
//
// The package is dependency-free apart from the standard library so it
// can be tested without the rest of the backend.
package diagnostics

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// HealthSummary is a sanitized snapshot of one installed indexer's health.
// We intentionally do NOT include base_url or definition_id verbatim if
// they could contain query strings with credentials — but the YAML
// loader already rejects any URL with a userinfo segment, so by the time
// data reaches this struct it is safe.
type HealthSummary struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Status           string `json:"status"`
	Enabled          bool   `json:"enabled"`
	ResponseTimeMs   int64  `json:"responseTimeMs,omitempty"`
	ConsecutiveFails int    `json:"consecutiveFails"`
	LastErrorCode    string `json:"lastErrorCode,omitempty"`
	LastErrorAt      string `json:"lastErrorAt,omitempty"`
}

// CatalogStatus summarises the active catalog for diagnostics.
type CatalogStatus struct {
	Source  string `json:"source"`
	Version string `json:"version"`
}

// Snapshot is the data the handler feeds into Build.
type Snapshot struct {
	Version           string
	StartedAt         time.Time
	BuildGoOS         string
	BuildGoArch       string
	DataDir           string
	LogDir            string
	DBPath            string
	SchemaVersion     int
	DefinitionVersion string
	InstalledIndexers []HealthSummary
	Catalog           CatalogStatus
}

// MaxLogsBytes caps the total size of inlined logs (spec §25.3:
// sanitized logs, no magnets/keywords). 5 MB across all rotated files
// combined, summed tail-first.
const MaxLogsBytes = 5 * 1024 * 1024

// MaxErrorMessageLen trims any error message that ends up in the
// diagnostic. Magnets are also stripped from these strings.
const MaxErrorMessageLen = 200

// Build writes a diagnostic ZIP into w containing:
//
//	README.txt          — human-readable index
//	version.txt         — app version + build target
//	os.txt              — OS / arch / Go version
//	uptime.txt          — uptime + start time
//	schema.txt          — schema version + definition version
//	indexers.json       — sanitized health summary
//	catalog.json        — catalog source + version
//	logs/<file>.log     — recent sanitized logs (tail, capped)
//
// Returns the SHA-256 (hex) of the bytes written for caller logging.
func Build(w io.Writer, snap Snapshot, logFiles []string) (string, error) {
	if snap.Version == "" {
		return "", fmt.Errorf("diagnostics: empty Version")
	}
	if snap.StartedAt.IsZero() {
		return "", fmt.Errorf("diagnostics: zero StartedAt")
	}

	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)

	if err := writeText(zw, "README.txt", readmeBody(snap)); err != nil {
		return "", err
	}
	if err := writeText(zw, "version.txt", versionBody(snap)); err != nil {
		return "", err
	}
	if err := writeText(zw, "os.txt", osBody()); err != nil {
		return "", err
	}
	if err := writeText(zw, "uptime.txt", uptimeBody(snap)); err != nil {
		return "", err
	}
	if err := writeText(zw, "schema.txt", schemaBody(snap)); err != nil {
		return "", err
	}
	if err := writeJSON(zw, "indexers.json", snap.InstalledIndexers); err != nil {
		return "", err
	}
	if err := writeJSON(zw, "catalog.json", snap.Catalog); err != nil {
		return "", err
	}
	if err := writeLogs(zw, snap.LogDir, logFiles); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("diagnostics: close zip: %w", err)
	}

	out := buf.Bytes()
	sum := sha256.Sum256(out)
	if _, err := w.Write(out); err != nil {
		return "", fmt.Errorf("diagnostics: write zip: %w", err)
	}
	return hex.EncodeToString(sum[:]), nil
}

func writeText(zw *zip.Writer, name, body string) error {
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return fmt.Errorf("diagnostics: create %s: %w", name, err)
	}
	if _, err := io.WriteString(w, body); err != nil {
		return fmt.Errorf("diagnostics: write %s: %w", name, err)
	}
	return nil
}

func writeJSON(zw *zip.Writer, name string, v any) error {
	hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return fmt.Errorf("diagnostics: create %s: %w", name, err)
	}
	// The standard library is sufficient for our flat structs; if the
	// shape grows we can switch to encoding/json with an indent.
	if err := encodeJSON(w, v); err != nil {
		return fmt.Errorf("diagnostics: encode %s: %w", name, err)
	}
	return nil
}

func readmeBody(snap Snapshot) string {
	lines := []string{
		"EasySearch diagnostic bundle",
		"===========================",
		"",
		"This archive contains a sanitized snapshot of the running",
		"EasySearch instance, intended for bug reports.",
		"",
		"Files:",
		"  version.txt    — application version + build target",
		"  os.txt         — operating system + Go runtime",
		"  uptime.txt     — process uptime + start time",
		"  schema.txt     — database schema version + indexer definition version",
		"  indexers.json  — installed indexer status summary (no URLs, no errors)",
		"  catalog.json   — active catalog source and version",
		"  logs/*.log     — recent log files (magnet links and keywords redacted)",
		"",
		"Privacy: this bundle NEVER contains magnet links, search",
		"keywords, downloaded content, or files outside the EasySearch",
		"data directory.",
	}
	if snap.DataDir != "" {
		lines = append(lines, "", fmt.Sprintf("Data directory: %s", snap.DataDir))
	}
	return strings.Join(lines, "\n") + "\n"
}

func versionBody(snap Snapshot) string {
	return fmt.Sprintf("version=%s\ngoos=%s\ngoarch=%s\n",
		snap.Version, snap.BuildGoOS, snap.BuildGoArch)
}

func osBody() string {
	return fmt.Sprintf("goos=%s\ngoarch=%s\ngo_version=%s\nnum_cpu=%d\n",
		runtime.GOOS, runtime.GOARCH, runtime.Version(), runtime.NumCPU())
}

func uptimeBody(snap Snapshot) string {
	uptime := time.Since(snap.StartedAt).Truncate(time.Second)
	return fmt.Sprintf("started_at=%s\nuptime=%s\n",
		snap.StartedAt.UTC().Format(time.RFC3339), uptime)
}

func schemaBody(snap Snapshot) string {
	return fmt.Sprintf("db_schema_version=%d\ndefinition_version=%s\n",
		snap.SchemaVersion, snap.DefinitionVersion)
}

// writeLogs tacks up to MaxLogsBytes of recent logs into the zip, with
// each input file mapped to logs/<basename>.
//
// We pick files in the order given (the caller is expected to pass them
// most-recent-first). Each file's tail is read up to its share of the
// budget; once the budget is exhausted we stop.
//
// All log contents are run through SanitizeLine before being written.
func writeLogs(zw *zip.Writer, logDir string, logFiles []string) error {
	remaining := MaxLogsBytes
	if len(logFiles) == 0 || remaining <= 0 {
		return nil
	}
	share := remaining / len(logFiles)
	if share < 1024 {
		share = 1024
	}
	for _, f := range logFiles {
		if remaining <= 0 {
			break
		}
		budget := share
		if budget > remaining {
			budget = remaining
		}
		tail, err := tailFile(f, int(budget))
		if err != nil {
			// A missing or unreadable log file is not fatal for
			// diagnostics; record a stub so the user knows.
			tail = []byte(fmt.Sprintf("(failed to read %s: %v)\n", f, err))
		}
		cleaned := SanitizeBytes(tail)
		name := "logs/" + filepath.Base(f)
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()}
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return fmt.Errorf("diagnostics: create %s: %w", name, err)
		}
		if _, err := w.Write(cleaned); err != nil {
			return fmt.Errorf("diagnostics: write %s: %w", name, err)
		}
		remaining -= len(cleaned)
	}
	return nil
}

// tailFile returns the last n bytes of path. If the file is smaller
// than n, the whole content is returned.
func tailFile(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()
	if size <= int64(n) {
		return io.ReadAll(f)
	}
	if _, err := f.Seek(size-int64(n), io.SeekStart); err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	read, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	return buf[:read], nil
}

// SortedLogFiles returns the list of rotated log files in dir, newest
// first, suitable for passing to Build.
func SortedLogFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	type pair struct {
		path    string
		modTime time.Time
	}
	var files []pair
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".log") {
			continue
		}
		full := filepath.Join(dir, name)
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, pair{path: full, modTime: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.path)
	}
	return out, nil
}