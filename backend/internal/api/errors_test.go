package api

import (
    "encoding/json"
    "io"
    "net/http/httptest"
    "testing"
)

func TestWriteErrorEnvelopeShape(t *testing.T) {
    rr := httptest.NewRecorder()
    WriteError(rr, nil, 404, ErrorPayload{
        Code:    "INDEXER_NOT_FOUND",
        Message: "索引器未找到",
        Details: map[string]any{"indexerId": "x"},
    })
    if rr.Code != 404 {
        t.Fatalf("status want 404 got %d", rr.Code)
    }
    if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
        t.Fatalf("content-type want json got %q", ct)
    }
    var got ErrorBody
    if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
        t.Fatal(err)
    }
    if got.Error.Code != "INDEXER_NOT_FOUND" {
        t.Fatalf("code wrong: %+v", got)
    }
    if got.Error.Details["indexerId"] != "x" {
        t.Fatalf("details wrong: %+v", got)
    }
}

func TestWriteJSON(t *testing.T) {
    rr := httptest.NewRecorder()
    WriteJSON(rr, 200, map[string]any{"ok": true})
    if rr.Code != 200 {
        t.Fatalf("status want 200 got %d", rr.Code)
    }
    b, _ := io.ReadAll(rr.Body)
    if string(b) != "{\"ok\":true}\n" {
        t.Fatalf("body want %q got %q", "{\"ok\":true}\n", string(b))
    }
}
