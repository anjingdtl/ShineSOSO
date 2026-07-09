package api

import (
    "encoding/json"
    "io"
    "log/slog"
    "net/http/httptest"
    "testing"
    "time"
)

func TestSystemHandlerGetStatus(t *testing.T) {
    h := &SystemHandler{
        StartTime: time.Now().Add(-2 * time.Second),
        Version:   "0.1.0",
        Logger:    slog.New(slog.NewJSONHandler(io.Discard, nil)),
    }
    rr := httptest.NewRecorder()
    h.GetStatus(rr, httptest.NewRequest("GET", "/api/v1/system/status", nil))

    if rr.Code != 200 {
        t.Fatalf("status want 200 got %d", rr.Code)
    }
    var got systemStatusResponse
    if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
        t.Fatal(err)
    }
    if got.Version != "0.1.0" {
        t.Fatalf("version want 0.1.0 got %q", got.Version)
    }
    if got.UptimeMs < 1000 {
        t.Fatalf("uptime should be >= 1000ms, got %d", got.UptimeMs)
    }
    if got.DBStatus != "unknown" {
        t.Fatalf("dbStatus want unknown got %q", got.DBStatus)
    }
    if got.StartedAt == "" {
        t.Fatal("startedAt should be set")
    }
}

func TestSystemHandlerNilSafety(t *testing.T) {
    var h *SystemHandler
    rr := httptest.NewRecorder()
    h.GetStatus(rr, httptest.NewRequest("GET", "/api/v1/system/status", nil))
    if rr.Code != 500 {
        t.Fatalf("nil handler should 500, got %d", rr.Code)
    }
}
