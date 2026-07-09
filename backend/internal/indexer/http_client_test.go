package indexer

import (
    "context"
    "net/http"
    "net/url"
    "testing"
    "time"
)

func TestClientRejectsPrivateHost(t *testing.T) {
    c := NewClient()
    _, err := c.Get(context.Background(), "http://127.0.0.1/foo")
    if err == nil {
        t.Fatal("expected error for private host")
    }
}

func TestClientRejectsHTTPByDefault(t *testing.T) {
    c := NewClient()
    _, err := c.Get(context.Background(), "http://198.51.100.1/foo")
    if err == nil {
        t.Fatal("expected error for http scheme")
    }
}

func TestIsRateLimitedAndNotFound(t *testing.T) {
    if !IsRateLimited(429) {
        t.Error("429 should be rate-limited")
    }
    if IsRateLimited(503) {
        t.Error("503 should not be rate-limited")
    }
    if !IsNotFound(404) {
        t.Error("404 should be not-found")
    }
    if IsNotFound(500) {
        t.Error("500 should not be not-found")
    }
}

func TestStatusCodeErrorMessage(t *testing.T) {
    u, _ := url.Parse("https://example.com/foo")
    e := &StatusCodeError{StatusCode: 503, URL: u}
    if e.Error() == "" {
        t.Fatal("error message should be non-empty")
    }
}

func TestDiscardAndCloseNilSafe(t *testing.T) {
    DiscardAndClose(nil)
    DiscardAndClose(&http.Response{Body: nil})
    // should not panic
}

func TestClientTimeoutHonorsContext(t *testing.T) {
    c := NewClient()
    c.Timeout = 50 * time.Millisecond
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
    defer cancel()
    _, err := c.Get(ctx, "https://198.51.100.1/foo")
    if err == nil {
        t.Fatal("expected timeout error")
    }
}
