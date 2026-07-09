package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/local/easysearch/backend/internal/catalog"
)

// CatalogUpdateHandler exposes the Phase 6 catalog-update endpoint
// (spec §26.3, POST /api/v1/indexer-catalog/update). The handler is
// responsible only for translating HTTP into an Updater.Fetch call;
// policy lives in the catalog package.
type CatalogUpdateHandler struct {
	Logger  *slog.Logger
	Updater *catalog.Updater
}

// CatalogUpdateResponse mirrors catalog.UpdateReport so the JSON tags
// stay stable even if the catalog package grows new internal fields.
type CatalogUpdateResponse struct {
	Before  string                       `json:"before"`
	After   string                       `json:"after"`
	Source  string                       `json:"source"`
	Changed []catalog.ChangedDefinition  `json:"changed"`
	Added   []string                     `json:"added"`
	Removed []string                     `json:"removed"`
}

// Update POST /api/v1/indexer-catalog/update
//
// On success: 200 with the report describing what changed.
// On failure: 502 with the underlying catalog error mapped to
// CATALOG_UPDATE_FAILED (spec §22.2).
func (h *CatalogUpdateHandler) Update(w http.ResponseWriter, r *http.Request) {
	if h.Updater == nil {
		WriteError(w, h.Logger, http.StatusServiceUnavailable, ErrorPayload{
			Code:    "CATALOG_UPDATE_DISABLED",
			Message: "catalog updater is not configured",
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*1_000_000_000) // 60s
	defer cancel()

	report, err := h.Updater.Fetch(ctx)
	if err != nil {
		WriteError(w, h.Logger, http.StatusBadGateway, ErrorPayload{
			Code:    "CATALOG_UPDATE_FAILED",
			Message: err.Error(),
		})
		return
	}
	resp := CatalogUpdateResponse{
		Before:  report.Before,
		After:   report.After,
		Source:  report.Source,
		Changed: report.Changed,
		Added:   report.Added,
		Removed: report.Removed,
	}
	WriteJSON(w, http.StatusOK, resp)
}

// CatalogStatus GET /api/v1/indexer-catalog/status
//
// Returns the currently-active manifest version + source so the UI can
// label it ("embedded v2026.07.1" / "remote v2026.08.3"). Cheap; hits
// no network.
func (h *CatalogUpdateHandler) Status(w http.ResponseWriter, _ *http.Request) {
	type statusResp struct {
		Version string `json:"version"`
		Source  string `json:"source"`
	}
	if h.Updater == nil {
		WriteJSON(w, http.StatusOK, statusResp{Version: "", Source: "disabled"})
		return
	}
	WriteJSON(w, http.StatusOK, statusResp{
		Version: h.Updater.ActiveVersion(),
		Source:  h.Updater.ActiveSource(),
	})
}

// Ensure we don't break the import chain if the catalog package adds new
// report fields later.
var _ = json.Marshal