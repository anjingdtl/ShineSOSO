package api

import (
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// SystemHandler reports service-level health and version metadata (spec §19.12).
// It is intentionally dependency-free so the endpoint can serve even before
// the database is initialized.
type SystemHandler struct {
	StartTime time.Time
	Version   string
	Logger    *slog.Logger
	// UIHeartbeat is touched by the embedded web UI. The desktop launcher
	// uses it to stop the local server after the last browser window closes.
	UIHeartbeat *atomic.Int64
}

// Heartbeat is intentionally tiny: it is a same-origin local signal, not an
// authentication mechanism. It gives the desktop host a reliable UI liveness
// signal without tying the server lifetime to a browser process name.
func (h *SystemHandler) Heartbeat(w http.ResponseWriter, _ *http.Request) {
	if h != nil && h.UIHeartbeat != nil {
		h.UIHeartbeat.Store(time.Now().UnixNano())
	}
	w.WriteHeader(http.StatusNoContent)
}

type systemStatusResponse struct {
	Version           string `json:"version"`
	UptimeMs          int64  `json:"uptimeMs"`
	DBStatus          string `json:"dbStatus"`
	BindHost          string `json:"bindHost"`
	ListenPort        int    `json:"listenPort"`
	DataDir           string `json:"dataDir"`
	StartedAt         string `json:"startedAt"`
	DefinitionVersion string `json:"definitionVersion,omitempty"`
	InstalledIndexers int    `json:"installedIndexers"`
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
