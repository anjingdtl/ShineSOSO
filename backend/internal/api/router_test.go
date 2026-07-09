package api

import (
    "io"
    "log/slog"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"
)

func newTestRouter() http.Handler {
    return NewRouter(ServerDeps{
        Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)),
        System: &SystemHandler{
            StartTime: time.Now().Add(-time.Second),
            Version:   "test",
            Logger:    slog.New(slog.NewJSONHandler(io.Discard, nil)),
        },
    })
}

func TestRouterHealthz(t *testing.T) {
    rr := httptest.NewRecorder()
    newTestRouter().ServeHTTP(rr, httptest.NewRequest("GET", "/healthz", nil))
    if rr.Code != 200 {
        t.Fatalf("status want 200 got %d", rr.Code)
    }
    if body := rr.Body.String(); body != "ok" {
        t.Fatalf("body want ok got %q", body)
    }
}

func TestRouterStatus(t *testing.T) {
    rr := httptest.NewRecorder()
    newTestRouter().ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/system/status", nil))
    if rr.Code != 200 {
        t.Fatalf("status want 200 got %d", rr.Code)
    }
    if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
        t.Fatalf("content-type want json got %q", ct)
    }
}

func TestRouterUnknownRoute(t *testing.T) {
    rr := httptest.NewRecorder()
    newTestRouter().ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/unknown", nil))
    // /api/v1/unknown is unknown to the API; chi falls through to the
    // catch-all webembed handler which serves the SPA shell. The router
    // returns 200 + the embedded index.html in that case.
    if rr.Code != 200 {
        t.Fatalf("status want 200 (SPA fallback) got %d", rr.Code)
    }
}

func TestRouterServesSpaShell(t *testing.T) {
    rr := httptest.NewRecorder()
    newTestRouter().ServeHTTP(rr, httptest.NewRequest("GET", "/indexers", nil))
    if rr.Code != 200 {
        t.Fatalf("status want 200 got %d", rr.Code)
    }
    if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
        t.Fatalf("SPA shell should be text/html, got %q", ct)
    }
}
