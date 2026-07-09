package api

import (
    "log/slog"
    "net/http"
    "time"
)

// SystemHandler reports service-level health and version metadata (spec §19.12).
// It is intentionally dependency-free so the endpoint can serve even before
// the database is initialized.
type SystemHandler struct {
    StartTime time.Time
    Version   string
    Logger    *slog.Logger
}

type systemStatusResponse struct {
    Version     string `json:"version"`
    UptimeMs    int64  `json:"uptimeMs"`
    DBStatus    string `json:"dbStatus"`
    BindHost    string `json:"bindHost"`
    ListenPort  int    `json:"listenPort"`
    DataDir     string `json:"dataDir"`
    StartedAt   string `json:"startedAt"`
    DefinitionVersion string `json:"definitionVersion,omitempty"`
    InstalledIndexers int `json:"installedIndexers"`
}

func (h *SystemHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
    if h == nil || h.Logger == nil {
        WriteError(w, nil, http.StatusInternalServerError, ErrorPayload{
            Code:    "INTERNAL_ERROR",
            Message: "system handler not initialized",
        })
        return
    }
    WriteJSON(w, http.StatusOK, systemStatusResponse{
        Version:   h.Version,
        UptimeMs:  time.Since(h.StartTime).Milliseconds(),
        DBStatus:  "unknown", // populated in Phase 4 after SQLite lands
        StartedAt: h.StartTime.UTC().Format(time.RFC3339),
    })
}
