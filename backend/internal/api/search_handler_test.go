package api

import (
    "bufio"
    "encoding/json"
    "io"
    "log/slog"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func newTestSearchHandler() *SearchHandler {
    return NewSearchHandler(slog.New(slog.NewJSONHandler(io.Discard, nil)))
}

func TestCreateSessionRejectsEmptyKeyword(t *testing.T) {
    h := newTestSearchHandler()
    body := strings.NewReader(`{"keyword":""}`)
    rr := httptest.NewRecorder()
    h.CreateSession(rr, httptest.NewRequest("POST", "/api/v1/search/sessions", body))
    if rr.Code != http.StatusBadRequest {
        t.Fatalf("status want 400, got %d", rr.Code)
    }
    var env ErrorBody
    if err := json.NewDecoder(rr.Body).Decode(&env); err != nil {
        t.Fatal(err)
    }
    if env.Error.Code != "EMPTY_KEYWORD" {
        t.Errorf("want EMPTY_KEYWORD, got %s", env.Error.Code)
    }
}

func TestCreateSessionRejectsTooLongKeyword(t *testing.T) {
    h := newTestSearchHandler()
    long := strings.Repeat("a", 201)
    body := strings.NewReader(`{"keyword":"` + long + `"}`)
    rr := httptest.NewRecorder()
    h.CreateSession(rr, httptest.NewRequest("POST", "/api/v1/search/sessions", body))
    if rr.Code != http.StatusBadRequest {
        t.Fatalf("status want 400, got %d", rr.Code)
    }
}

func TestCreateSessionSucceedsAndStreamsEvents(t *testing.T) {
    h := newTestSearchHandler()

    rr := httptest.NewRecorder()
    body := strings.NewReader(`{"keyword":"hello","category":"movie","sort":"relevance"}`)
    h.CreateSession(rr, httptest.NewRequest("POST", "/api/v1/search/sessions", body))
    if rr.Code != http.StatusCreated {
        t.Fatalf("create status want 201, got %d body=%s", rr.Code, rr.Body.String())
    }
    var resp createSearchResponse
    if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
        t.Fatal(err)
    }
    if resp.SessionID == "" {
        t.Fatal("sessionId should be set")
    }
    if !strings.HasSuffix(resp.StreamURL, "/events") {
        t.Fatalf("streamURL should end with /events, got %q", resp.StreamURL)
    }

    // Drive the SSE handler. The full test takes ~30ms.
    sseRR := httptest.NewRecorder()
    req := httptest.NewRequest("GET", resp.StreamURL, nil)
    req.SetPathValue("sessionId", resp.SessionID)
    h.StreamEvents(sseRR, req)
    if sseRR.Code != http.StatusOK {
        t.Fatalf("sse status want 200, got %d", sseRR.Code)
    }
    if ct := sseRR.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
        t.Errorf("content-type want event-stream, got %q", ct)
    }
    events := parseSSE(t, sseRR.Body.String())
    mustHaveEvent(t, events, "session_started")
    mustHaveEvent(t, events, "indexer_started")
    mustHaveEvent(t, events, "indexer_completed")
    mustHaveEvent(t, events, "results_merged")
    mustHaveEvent(t, events, "session_completed")
}

func TestCreateSessionNormalizesCategoryAndSort(t *testing.T) {
    h := newTestSearchHandler()
    body := strings.NewReader(`{"keyword":"x","category":"Movies","sort":"Weird"}`)
    rr := httptest.NewRecorder()
    h.CreateSession(rr, httptest.NewRequest("POST", "/api/v1/search/sessions", body))
    if rr.Code != http.StatusCreated {
        t.Fatalf("status %d", rr.Code)
    }
    var resp createSearchResponse
    json.NewDecoder(rr.Body).Decode(&resp)
    h.mu.Lock()
    s := h.sessions[resp.SessionID]
    h.mu.Unlock()
    if s.Query.Category != "all" { // "Movies" -> all
        t.Errorf("category normalize: want all got %q", s.Query.Category)
    }
    if s.Query.Sort != "relevance" {
        t.Errorf("sort normalize: want relevance got %q", s.Query.Sort)
    }
}

func TestCancelUnknownSession(t *testing.T) {
    h := newTestSearchHandler()
    rr := httptest.NewRecorder()
    req := httptest.NewRequest("POST", "/api/v1/search/sessions/nope/cancel", nil)
    req.SetPathValue("sessionId", "nope")
    h.CancelSession(rr, req)
    if rr.Code != http.StatusNotFound {
        t.Fatalf("want 404, got %d", rr.Code)
    }
}

func TestStreamUnknownSession(t *testing.T) {
    h := newTestSearchHandler()
    rr := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/api/v1/search/sessions/nope/events", nil)
    req.SetPathValue("sessionId", "nope")
    h.StreamEvents(rr, req)
    if rr.Code != http.StatusNotFound {
        t.Fatalf("want 404, got %d", rr.Code)
    }
}

type sseEvent struct {
    event string
    data  string
}

func parseSSE(t *testing.T, raw string) []sseEvent {
    t.Helper()
    var out []sseEvent
    s := bufio.NewScanner(strings.NewReader(raw))
    var cur sseEvent
    for s.Scan() {
        line := s.Text()
        switch {
        case strings.HasPrefix(line, "event: "):
            cur.event = strings.TrimPrefix(line, "event: ")
        case strings.HasPrefix(line, "data: "):
            cur.data = strings.TrimPrefix(line, "data: ")
        case line == "" && cur.event != "":
            out = append(out, cur)
            cur = sseEvent{}
        }
    }
    if cur.event != "" {
        out = append(out, cur)
    }
    return out
}

func mustHaveEvent(t *testing.T, evs []sseEvent, name string) {
    t.Helper()
    for _, e := range evs {
        if e.event == name {
            return
        }
    }
    var got []string
    for _, e := range evs {
        got = append(got, e.event)
    }
    t.Errorf("expected SSE event %q, got %v", name, got)
}
