package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/local/easysearch/backend/internal/prowlarr"
)

type ProwlarrHandler struct {
	Logger  *slog.Logger
	Manager *prowlarr.Manager
}

func (h *ProwlarrHandler) Status(w http.ResponseWriter, _ *http.Request) {
	if h == nil || h.Manager == nil {
		WriteJSON(w, http.StatusOK, prowlarr.Status{State: "unavailable", Message: "当前运行版本未包含 Prowlarr 引擎"})
		return
	}
	WriteJSON(w, http.StatusOK, h.Manager.Status())
}

func (h *ProwlarrHandler) Discover(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Query string `json:"query"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INVALID_REQUEST", Message: "请求体不是合法 JSON"})
		return
	}
	ctx, cancel := contextWithTimeout(r, 20*time.Second)
	defer cancel()
	items, err := h.Manager.Discover(ctx, strings.TrimSpace(in.Query))
	if err != nil {
		WriteError(w, h.Logger, http.StatusServiceUnavailable, ErrorPayload{Code: "PROWLARR_UNAVAILABLE", Message: err.Error()})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *ProwlarrHandler) Add(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SchemaID string `json:"schemaId"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.SchemaID) == "" {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "INVALID_REQUEST", Message: "缺少 schemaId"})
		return
	}
	ctx, cancel := contextWithTimeout(r, 30*time.Second)
	defer cancel()
	item, err := h.Manager.AddAndTest(ctx, strings.TrimSpace(in.SchemaID))
	if err != nil {
		WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{Code: "PROWLARR_ADD_FAILED", Message: err.Error()})
		return
	}
	WriteJSON(w, http.StatusCreated, item)
}

func contextWithTimeout(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), timeout)
}
