package api

import (
    "io"
    "log/slog"
    "net/http"
    "net/http/httptest"
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
    if rr.Code != 404 {
        t.Fatalf("status want 404 got %d", rr.Code)
    }
}
