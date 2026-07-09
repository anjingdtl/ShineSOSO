package api

import (
    "log/slog"
    "net/http"

    "github.com/go-chi/chi/v5"

    "github.com/local/easysearch/backend/internal/webembed"
)

// ServerDeps bundles the dependencies a Router needs. Phase 1 only wires
// SystemHandler; later phases add indexerHandler, searchHandler, etc.
type ServerDeps struct {
    Logger  *slog.Logger
    System  *SystemHandler
    Search  *SearchHandler
    Indexer *IndexerHandler
    Import  *ImportHandler
}

// NewRouter returns a chi.Mux with /api/v1 mounted, plus a /healthz alias.
// Non-API requests fall through to the embedded frontend (SPA + assets).
func NewRouter(deps ServerDeps) http.Handler {
    r := chi.NewRouter()

    r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("ok"))
    })

    r.Route("/api/v1", func(r chi.Router) {
        r.Get("/system/status", deps.System.GetStatus)
        if deps.Search != nil {
            r.Post("/search/sessions", deps.Search.CreateSession)
            r.Get("/search/sessions/{sessionId}/events", deps.Search.StreamEvents)
            r.Post("/search/sessions/{sessionId}/cancel", deps.Search.CancelSession)
        }
        if deps.Indexer != nil {
            r.Get("/indexers", deps.Indexer.List)
            r.Post("/indexers", deps.Indexer.Create)
            r.Get("/indexers/{id}", deps.Indexer.Get)
            r.Patch("/indexers/{id}", deps.Indexer.Update)
            r.Delete("/indexers/{id}", deps.Indexer.Delete)
            r.Post("/indexers/{id}/test", deps.Indexer.Test)
            r.Get("/indexer-catalog", deps.Indexer.ListCatalog)
        }
        if deps.Import != nil {
            r.Post("/indexer-catalog/import", deps.Import.Import)
            r.Get("/indexer-catalog/imported", deps.Import.ListImported)
        }
    })

    // Static + SPA fallback. chi routes /api/v1/... explicitly above;
    // everything else (including unknown /api/v1/... paths) falls through
    // to NotFound which serves the embedded SPA shell.
    r.NotFound(webembed.Handler(deps.Logger).ServeHTTP)

    return r
}
