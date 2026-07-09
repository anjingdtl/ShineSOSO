package security

import (
    "errors"
    "testing"
)

func TestValidateURL_HTTPS(t *testing.T) {
    v := DefaultValidator{}
    // ValidateURL also calls net.LookupIP for hostnames; use a literal
    // public IP (TEST-NET-2, 198.51.100.0/24) so the resolver isn't needed.
    u, err := v.ValidateURL("https://198.51.100.1/foo")
    if err != nil {
        t.Fatalf("https public IP should pass, got %v", err)
    }
    if u.Scheme != "https" {
        t.Fatalf("scheme want https got %q", u.Scheme)
    }
}

func TestValidateURL_RejectsHTTPByDefault(t *testing.T) {
    v := DefaultValidator{}
    _, err := v.ValidateURL("http://example.com/foo")
    if !errors.Is(err, ErrUnapprovedScheme) {
        t.Fatalf("want ErrUnapprovedScheme, got %v", err)
    }
}

func TestValidateURL_AllowsHTTPWhenOptIn(t *testing.T) {
    v := DefaultValidator{AllowHTTP: true}
    // 198.51.100.0/24 is reserved for documentation (TEST-NET-2) and
    // not a private IP, so it passes the host check.
    if _, err := v.ValidateURL("http://198.51.100.1/foo"); err != nil {
        t.Fatalf("AllowHTTP should accept http, got %v", err)
    }
}

func TestValidateURL_RejectsPrivateIPs(t *testing.T) {
    v := DefaultValidator{}
    cases := []string{
        "https://127.0.0.1/foo",
        "https://10.0.0.1/foo",
        "https://192.168.1.1/foo",
        "https://172.16.0.1/foo",
        "https://169.254.169.254/latest/meta-data",
        "https://[::1]/foo",
    }
    for _, raw := range cases {
        if _, err := v.ValidateURL(raw); err == nil {
            t.Errorf("expected private-IP rejection for %s, got nil", raw)
        }
    }
}

func TestValidateURL_RejectsBadSchemes(t *testing.T) {
    v := DefaultValidator{AllowHTTP: true}
    cases := []string{
        "file:///etc/passwd",
        "ftp://example.com/foo",
        "gopher://example.com/foo",
    }
    for _, raw := range cases {
        if _, err := v.ValidateURL(raw); err == nil {
            t.Errorf("expected scheme rejection for %s, got nil", raw)
        }
    }
}

func TestValidateURL_RejectsNonAbsolute(t *testing.T) {
    v := DefaultValidator{}
    if _, err := v.ValidateURL("/relative/path"); err == nil {
        t.Fatal("expected error for relative url")
    }
}

func TestValidateURL_RejectsEmpty(t *testing.T) {
    v := DefaultValidator{}
    if _, err := v.ValidateURL(""); err == nil {
        t.Fatal("expected error for empty url")
    }
}

func TestValidateHost_Blocks(t *testing.T) {
    v := DefaultValidator{}
    bad := []string{"127.0.0.1", "10.0.0.5", "192.168.0.1", "::1", "169.254.169.254"}
    for _, h := range bad {
        if err := v.ValidateHost(h); err == nil {
            t.Errorf("expected %s blocked, got nil", h)
        }
    }
}

func TestIsCloudMetadataHost(t *testing.T) {
    cases := []struct {
        host string
        want bool
    }{
        {"169.254.169.254", true},
        {"metadata.google.internal", true},
        {"example.com", false},
        {"1.1.1.1", false},
    }
    for _, tc := range cases {
        if got := isCloudMetadataHost(tc.host); got != tc.want {
            t.Errorf("isCloudMetadataHost(%q) = %v, want %v", tc.host, got, tc.want)
        }
    }
}
