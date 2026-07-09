package api

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "net/http"
    "strings"
    "sync"
    "time"

    "github.com/google/uuid"

    "github.com/local/easysearch/backend/internal/indexer"
    "github.com/local/easysearch/backend/internal/model"
    "github.com/local/easysearch/backend/internal/search"
)

// SearchHandler hosts the POST /sessions and GET /sessions/{id}/events
// endpoints. It also serves /sessions/{id}/cancel.
//
// Phase 2 supports a single hard-coded mock indexer; Phase 4 wires in
// the catalog-driven selection.
type SearchHandler struct {
    Logger *slog.Logger

    mu       sync.Mutex
    sessions map[string]*search.Session
    cancels  map[string]context.CancelFunc
}

func NewSearchHandler(logger *slog.Logger) *SearchHandler {
    return &SearchHandler{
        Logger:   logger,
        sessions: map[string]*search.Session{},
        cancels:  map[string]context.CancelFunc{},
    }
}

type createSearchRequest struct {
    Keyword  string                  `json:"keyword"`
    Category string                  `json:"category"`
    Sort     string                  `json:"sort"`
    Filters  map[string]any          `json:"filters"`
}

type createSearchResponse struct {
    SessionID string `json:"sessionId"`
    StreamURL string `json:"streamUrl"`
}

func (h *SearchHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
    var req createSearchRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{
            Code:    "INVALID_REQUEST",
            Message: "请求体不是合法 JSON",
        })
        return
    }
    if strings.TrimSpace(req.Keyword) == "" {
        WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{
            Code:    "EMPTY_KEYWORD",
            Message: "关键词不能为空",
        })
        return
    }
    if len([]rune(req.Keyword)) > 200 {
        WriteError(w, h.Logger, http.StatusBadRequest, ErrorPayload{
            Code:    "INVALID_REQUEST",
            Message: "关键词长度不能超过 200",
        })
        return
    }

    sid := uuid.NewString()
    q := model.SearchQuery{
        Keyword:  strings.TrimSpace(req.Keyword),
        Category: normalizeCategory(req.Category),
        Sort:     normalizeSort(req.Sort),
    }

    // Phase 3: use the 3-mock demo fleet. Phase 4 will replace this with
    // catalog-driven selection.
    jobs := NewMockIndexers()

    o := search.NewOrchestrator(search.Config{
        PerIndexerTimeout: 15 * time.Second,
        TotalTimeout:      30 * time.Second,
        MaxConcurrent:     6,
    }, jobs, h.Logger)

    s := o.NewSession(sid, q)

    h.mu.Lock()
    h.sessions[sid] = s
    h.mu.Unlock()

    ctx, cancel := context.WithCancel(context.Background())
    h.mu.Lock()
    h.cancels[sid] = cancel
    h.mu.Unlock()

    go o.Run(ctx, s)

    WriteJSON(w, http.StatusCreated, createSearchResponse{
        SessionID: sid,
        StreamURL: fmt.Sprintf("/api/v1/search/sessions/%s/events", sid),
    })
}

func (h *SearchHandler) StreamEvents(w http.ResponseWriter, r *http.Request) {
    sid := r.PathValue("sessionId")
    h.mu.Lock()
    s, ok := h.sessions[sid]
    h.mu.Unlock()
    if !ok {
        WriteError(w, h.Logger, http.StatusNotFound, ErrorPayload{
            Code:    "INDEXER_NOT_FOUND",
            Message: "会话不存在或已结束",
        })
        return
    }

    flusher, ok := w.(http.Flusher)
    if !ok {
        WriteError(w, h.Logger, http.StatusInternalServerError, ErrorPayload{
            Code:    "INTERNAL_ERROR",
            Message: "服务器不支持流式响应",
        })
        return
    }

    w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
    w.Header().Set("Cache-Control", "no-cache, no-store")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")
    w.WriteHeader(http.StatusOK)
    flusher.Flush()

    // Keepalive ping every 15s so corporate proxies don't kill the stream.
    ticker := time.NewTicker(15 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-r.Context().Done():
            return
        case ev, ok := <-s.Events:
            if !ok {
                return
            }
            writeSSE(w, ev.Type, ev.Data)
            flusher.Flush()
            if ev.Type == search.EventSessionCompleted || ev.Type == search.EventSessionCancelled {
                h.cleanup(sid)
                return
            }
        case <-ticker.C:
            fmt.Fprintf(w, ": keepalive\n\n")
            flusher.Flush()
        }
    }
}

func (h *SearchHandler) CancelSession(w http.ResponseWriter, r *http.Request) {
    sid := r.PathValue("sessionId")
    h.mu.Lock()
    cancel, ok := h.cancels[sid]
    h.mu.Unlock()
    if !ok {
        WriteError(w, h.Logger, http.StatusNotFound, ErrorPayload{
            Code:    "INDEXER_NOT_FOUND",
            Message: "会话不存在或已结束",
        })
        return
    }
    cancel()
    w.WriteHeader(http.StatusNoContent)
}

func (h *SearchHandler) cleanup(sid string) {
    h.mu.Lock()
    delete(h.sessions, sid)
    delete(h.cancels, sid)
    h.mu.Unlock()
}

func writeSSE(w http.ResponseWriter, eventType search.EventType, data any) {
    b, err := json.Marshal(data)
    if err != nil {
        // The data shapes are all struct types; marshal should not fail.
        // If it does, emit an error event instead of crashing the stream.
        b, _ = json.Marshal(map[string]string{"error": "marshal failed"})
    }
    fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(b))
}

func normalizeCategory(c string) string {
    switch c {
    case "movie", "tv", "music", "game", "software", "book", "anime", "other":
        return c
    default:
        return "all"
    }
}

func normalizeSort(s string) string {
    switch s {
    case "seeders", "publishedAt", "sizeDesc", "sizeAsc":
        return s
    default:
        return "relevance"
    }
}

// Avoid unused-import noise in Phase 2; indexer is referenced by the mock.
var _ = indexer.NewClient
