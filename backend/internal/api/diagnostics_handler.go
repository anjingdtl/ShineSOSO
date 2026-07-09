package api

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"github.com/local/easysearch/backend/internal/diagnostics"
	"github.com/local/easysearch/backend/internal/store"
)

// CatalogStatusProvider is the minimal interface a DiagnosticsHandler
// needs from the catalog layer. Returning (source, version) keeps the
// API package decoupled from the concrete catalog type.
type CatalogStatusProvider interface {
	ActiveSource() string
	ActiveVersion() string
}

// RepoSummaryProvider is the minimal interface the diagnostics handler
// needs from the indexer repo.
type RepoSummaryProvider interface {
	DiagnosticsSummary() ([]store.DiagnosticsRow, error)
}

// StoreVersionProvider is the minimal interface for fetching the
// highest applied schema migration.
type StoreVersionProvider interface {
	SchemaVersion() (int, error)
}

// DiagnosticsHandler serves GET /api/v1/system/diagnostics by bundling
// a sanitized snapshot into a ZIP (spec §25.3, Phase 7-6).
//
// The handler is intentionally tolerant: if a subsystem fails (e.g.
// DB is corrupt or the catalog is unloaded) it still produces a bundle
// with whatever data is available and surfaces the failure in the log.
type DiagnosticsHandler struct {
	StartTime time.Time
	Version   string
	DataDir   string
	LogDir    string
	DBPath    string
	BuildOS   string // optional override; defaults to runtime.GOOS
	BuildArch string // optional override; defaults to runtime.GOARCH
	Store     StoreVersionProvider
	Repo      RepoSummaryProvider
	Catalog   CatalogStatusProvider
	Logger    *slog.Logger
}

// Get serves a ZIP bundle with Content-Disposition: attachment so the
// browser triggers a download. The ZIP's SHA-256 is included as a
// response header for traceability.
func (h *DiagnosticsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		WriteError(w, nil, http.StatusInternalServerError, ErrorPayload{
			Code:    "INTERNAL_ERROR",
			Message: "diagnostics handler not initialized",
		})
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		WriteError(w, h.Logger, http.StatusMethodNotAllowed, ErrorPayload{
			Code:    "INVALID_REQUEST",
			Message: "method not allowed",
		})
		return
	}

	snap := h.buildSnapshot()
	logFiles, logErr := diagnostics.SortedLogFiles(h.LogDir)
	if logErr != nil && h.Logger != nil {
		h.Logger.Warn("diagnostics: list logs", "err", logErr)
	}

	var buf bytes.Buffer
	sum, err := diagnostics.Build(&buf, snap, logFiles)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("diagnostics: build", "err", err)
		}
		WriteError(w, h.Logger, http.StatusInternalServerError, ErrorPayload{
			Code:    "INTERNAL_ERROR",
			Message: "diagnostics bundle failed",
		})
		return
	}

	name := fmt.Sprintf("easysearch-diagnostics-%s.zip",
		time.Now().UTC().Format("20060102T150405Z"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("X-Diagnostics-SHA256", sum)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())

	if h.Logger != nil {
		h.Logger.Info("diagnostics bundle served",
			"bytes", buf.Len(),
			"sha256", sum,
			"indexer_count", len(snap.InstalledIndexers),
		)
	}
}

func (h *DiagnosticsHandler) buildSnapshot() diagnostics.Snapshot {
	os, arch := h.BuildOS, h.BuildArch
	if os == "" {
		os = runtime.GOOS
	}
	if arch == "" {
		arch = runtime.GOARCH
	}

	snap := diagnostics.Snapshot{
		Version:   h.Version,
		StartedAt: h.StartTime,
		BuildGoOS: os,
		BuildGoArch: arch,
		DataDir:   h.DataDir,
		LogDir:    h.LogDir,
		DBPath:    h.DBPath,
	}

	if h.Store != nil {
		if v, err := h.Store.SchemaVersion(); err == nil {
			snap.SchemaVersion = v
		} else if h.Logger != nil {
			h.Logger.Warn("diagnostics: schema version", "err", err)
		}
	}
	if h.Catalog != nil {
		snap.Catalog = diagnostics.CatalogStatus{
			Source:  h.Catalog.ActiveSource(),
			Version: h.Catalog.ActiveVersion(),
		}
	}
	if h.Repo != nil {
		if rows, err := h.Repo.DiagnosticsSummary(); err == nil {
			snap.InstalledIndexers = make([]diagnostics.HealthSummary, 0, len(rows))
			for _, row := range rows {
				snap.InstalledIndexers = append(snap.InstalledIndexers, diagnostics.HealthSummary{
					ID:               row.ID,
					Name:             row.Name,
					Status:           row.Status,
					Enabled:          row.Enabled,
					ResponseTimeMs:   row.ResponseTimeMs,
					ConsecutiveFails: row.ConsecutiveFails,
					LastErrorCode:    row.LastErrorCode,
					LastErrorAt:      row.LastErrorAt,
				})
			}
		} else if h.Logger != nil {
			h.Logger.Warn("diagnostics: indexer summary", "err", err)
		}
	}
	// Definition version flows from the catalog (it's a catalog-level
	// field in Phase 6). If the catalog provider doesn't expose it,
	// leave blank rather than fabricating one.
	return snap
}

// DefaultLogDir returns <dataDir>/logs — the location logging.New
// writes to. Centralised so handlers and tests use the same path.
func DefaultLogDir(dataDir string) string {
	return filepath.Join(dataDir, "logs")
}