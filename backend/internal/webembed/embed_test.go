package webembed

import (
    "io"
    "log/slog"
    "net/http/httptest"
    "strings"
    "testing"
)

func discardLogger() *slog.Logger {
    return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestFSReturnsUsableFS(t *testing.T) {
    sub := FS()
    f, err := sub.Open("index.html")
    if err != nil {
        t.Fatalf("FS().Open(index.html): %v", err)
    }
    defer f.Close()
    info, err := f.Stat()
    if err != nil {
        t.Fatal(err)
    }
    if info.IsDir() {
        t.Fatal("index.html should be a file, not a directory")
    }
}

func TestHasBuildTrue(t *testing.T) {
    // After `npm run build` (or after committing the placeholder), the
    // embedded FS always contains an index.html.
    if !HasBuild() {
        t.Fatal("HasBuild should be true when index.html is in the embed")
    }
}

func TestHandlerServesIndexForRoot(t *testing.T) {
    h := Handler(discardLogger())
    rr := httptest.NewRecorder()
    h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
    if rr.Code != 200 {
        t.Fatalf("status want 200 got %d", rr.Code)
    }
    body := rr.Body.String()
    if !strings.Contains(body, "<html") {
        t.Fatalf("body should contain <html>, got %q", body[:min(80, len(body))])
    }
}

func TestHandlerFallsBackToIndexForSpaPath(t *testing.T) {
    h := Handler(discardLogger())
    rr := httptest.NewRecorder()
    h.ServeHTTP(rr, httptest.NewRequest("GET", "/indexers/abc", nil))
    if rr.Code != 200 {
        t.Fatalf("SPA fallback status want 200 got %d", rr.Code)
    }
    if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
        t.Fatalf("SPA fallback should return text/html, got %q", ct)
    }
}

func TestReadSeekerRoundTrip(t *testing.T) {
    sub := FS()
    f, err := sub.Open("index.html")
    if err != nil {
        t.Fatal(err)
    }
    defer f.Close()
    rs := readSeeker(f)
    b, err := io.ReadAll(rs)
    if err != nil {
        t.Fatal(err)
    }
    if len(b) == 0 {
        t.Fatal("readSeeker returned empty content")
    }
    if _, err := rs.Seek(0, 0); err != nil {
        t.Fatalf("readSeeker should support Seek: %v", err)
    }
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
