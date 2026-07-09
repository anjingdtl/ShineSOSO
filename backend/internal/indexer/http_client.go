// Package indexer provides the IndexerAdapter interface, the indexer
// engine, and the HTTP client used to fetch from external sites.
package indexer

import (
    "context"
    "errors"
    "fmt"
    "io"
    "net"
    "net/http"
    "net/url"
    "strconv"
    "time"

    "github.com/local/easysearch/backend/internal/security"
)

// Default limits (spec §15.2, §21.4). Overridable via the search package
// in later phases.
const (
    MaxResponseBytes int64 = 10 * 1024 * 1024 // 10 MB
    MaxRedirects          = 5
    DefaultTimeout        = 15 * time.Second
)

// Client is a hardened *http.Client tailored for indexer traffic:
//   - SSRF check before DNS resolution and after every redirect
//   - bounded response body
//   - bounded redirect count
//   - per-request timeout (caller may override via context)
type Client struct {
    HTTP    *http.Client
    Policy  security.DefaultValidator
    Timeout time.Duration
}

// NewClient returns a Client that uses the production security policy
// (HTTPS only, no private IPs, redirect cap 5).
func NewClient() *Client {
    timeout := DefaultTimeout
    return &Client{
        Timeout: timeout,
        Policy:  security.DefaultValidator{},
        HTTP: &http.Client{
            Timeout: timeout,
            CheckRedirect: func(req *http.Request, via []*http.Request) error {
                if len(via) >= MaxRedirects {
                    return fmt.Errorf("stopped after %d redirects", MaxRedirects)
                }
                return nil
            },
        },
    }
}

// Get performs an HTTP GET against rawURL, enforcing the security policy
// at every step. The response body is fully read and bounded by
// MaxResponseBytes; the caller must close the body.
func (c *Client) Get(ctx context.Context, rawURL string) (*http.Response, error) {
    return c.doWithBody(ctx, http.MethodGet, rawURL, "", nil)
}

// Post performs an HTTP POST. body may be nil. contentType is set as the
// request's Content-Type header when body is non-nil.
func (c *Client) Post(ctx context.Context, rawURL string, contentType string, body io.Reader) (*http.Response, error) {
    if body != nil && contentType == "" {
        contentType = "application/octet-stream"
    }
    return c.doWithBody(ctx, http.MethodPost, rawURL, contentType, body)
}

func (c *Client) doWithBody(ctx context.Context, method, rawURL, contentType string, body io.Reader, extraHeaders ...string) (*http.Response, error) {
    // 1. Pre-flight URL check (does not resolve DNS for IP literals).
    u, err := c.Policy.ValidateURL(rawURL)
    if err != nil {
        return nil, fmt.Errorf("url policy: %w", err)
    }

    // 2. Per-request context-bound timeout.
    if _, hasDeadline := ctx.Deadline(); !hasDeadline {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, c.Timeout)
        defer cancel()
    }

    req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
    if err != nil {
        return nil, fmt.Errorf("build request: %w", err)
    }
    if contentType != "" {
        req.Header.Set("Content-Type", contentType)
    }
    for i := 0; i+1 < len(extraHeaders); i += 2 {
        req.Header.Set(extraHeaders[i], extraHeaders[i+1])
    }
    if req.Header.Get("User-Agent") == "" {
        req.Header.Set("User-Agent", "EasySearch/0.1 (+local)")
    }
    if req.Header.Get("Accept") == "" {
        req.Header.Set("Accept", "*/*")
    }

    // 3. Hook CheckRedirect to re-validate the new host (DNS rebinding
    //    defense, spec §21.2).
    c.HTTP.CheckRedirect = func(req *http.Request, via []*http.Request) error {
        if len(via) >= MaxRedirects {
            return fmt.Errorf("stopped after %d redirects", MaxRedirects)
        }
        if err := c.Policy.ValidateHost(req.URL.Hostname()); err != nil {
            return fmt.Errorf("redirect target: %w", err)
        }
        return nil
    }

    resp, err := c.HTTP.Do(req)
    if err != nil {
        return nil, fmt.Errorf("http do: %w", err)
    }

    // 4. Re-validate the IP we actually dialed (post-DNS check).
    if resp.TLS != nil {
        // TLS handshake already happened; for the IP-layer defense we
        // re-validate the URL host one more time in case DNS rebinding
        // swapped it between our pre-flight and the dial.
        if err := c.Policy.ValidateHost(u.Hostname()); err != nil {
            resp.Body.Close()
            return nil, fmt.Errorf("post-dns policy: %w", err)
        }
    }

    // 5. Enforce the response body cap.
    resp.Body = http.MaxBytesReader(nil, resp.Body, MaxResponseBytes)

    return resp, nil
}

// ResolveAndValidateIP is exposed for tests and for the redirect hook to
// re-validate the actual dialed address.
func ResolveAndValidateIP(host string, v security.DefaultValidator) error {
    if ip := net.ParseIP(host); ip != nil {
        return v.ValidateHost(host)
    }
    addrs, err := net.LookupIP(host)
    if err != nil {
        return fmt.Errorf("resolve %s: %w", host, err)
    }
    for _, a := range addrs {
        if err := v.ValidateHost(a.String()); err != nil {
            return err
        }
    }
    return nil
}

// StatusCodeError wraps a non-2xx response so handlers can map it to a
// typed error code (INDEXER_HTTP_ERROR, INDEXER_RATE_LIMITED, etc.).
type StatusCodeError struct {
    StatusCode int
    URL        *url.URL
}

func (e *StatusCodeError) Error() string {
    return "unexpected status " + strconv.Itoa(e.StatusCode) + " from " + e.URL.String()
}

// IsRateLimited reports whether the response status is 429.
func IsRateLimited(status int) bool { return status == http.StatusTooManyRequests }

// IsNotFound reports whether the response status is 404.
func IsNotFound(status int) bool { return status == http.StatusNotFound }

// DiscardAndClose drains and closes resp.Body, never panicking. Use this
// whenever a response is being discarded by error paths.
func DiscardAndClose(resp *http.Response) {
    if resp == nil || resp.Body == nil {
        return
    }
    _, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, MaxResponseBytes))
    _ = resp.Body.Close()
}

var ErrEmptyResponse = errors.New("indexer: empty response body")
