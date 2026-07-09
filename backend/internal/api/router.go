package api

import (
    "log/slog"
    "net/http"

    "github.com/go-chi/chi/v5"
)

// ServerDeps bundles the dependencies a Router needs. Phase 1 only wires
// SystemHandler; later phases add indexerHandler, searchHandler, etc.
type ServerDeps struct {
    Logger        *slog.Logger
    System        *SystemHandler
    // Future: Indexer *IndexerHandler, Search *SearchHandler, Catalog *CatalogHandler
}

// NewRouter returns a chi.Mux with /api/v1 mounted, plus a /healthz alias.
// In dev mode the router also answers CORS preflight from any origin (Phase
// 1: not enabled; the Vite dev proxy handles same-origin).
func NewRouter(deps ServerDeps) http.Handler {
    r := chi.NewRouter()

    r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })

    r.Route("/api/v1", func(r chi.Router) {
        r.Get("/system/status", deps.System.GetStatus)
    })

    return r
}
