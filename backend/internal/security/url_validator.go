// Package security centralizes network and URL policy enforcement
// (spec §21). All outbound HTTP requests MUST go through ValidateURL
// before any DNS lookup, and again after every redirect.
package security

import (
    "errors"
    "fmt"
    "net"
    "net/url"
	"strings"
)

// Sentinel errors returned by ValidateURL. Callers may translate them
// into UNSAFE_INDEXER_URL (spec §22.2) error responses.
var (
    ErrUnapprovedScheme   = errors.New("security: scheme must be https (or http when explicitly allowed)")
    ErrPrivateHost        = errors.New("security: host resolves to a private, loopback, or link-local address")
    ErrNonAbsoluteURL     = errors.New("security: url must be absolute")
    ErrCloudMetadataHost  = errors.New("security: host is a cloud metadata service address")
    ErrBareHostname       = errors.New("security: url must include a host")
)

// DefaultValidator is the production policy: HTTPS only, no private IPs.
type DefaultValidator struct {
    AllowHTTP     bool // when true, http:// is permitted (for built-in indexers that opt in)
    AllowLoopback bool // tests-only: lets httptest loopback servers through
}

// ValidateURL checks the URL's scheme and that its host is not a
// private/loopback/link-local/cloud-metadata address. It does NOT
// perform DNS resolution; callers must resolve and call ValidateHost
// again to defend against DNS rebinding (spec §21.2).
func (v DefaultValidator) ValidateURL(raw string) (*url.URL, error) {
    if raw == "" {
        return nil, fmt.Errorf("%w", ErrBareHostname)
    }
    u, err := url.Parse(raw)
    if err != nil {
        return nil, fmt.Errorf("parse url: %w", err)
    }
    if !u.IsAbs() {
        return nil, fmt.Errorf("%w: %s", ErrNonAbsoluteURL, raw)
    }
    scheme := strings.ToLower(u.Scheme)
    switch scheme {
    case "https":
        // always allowed
    case "http":
        if !v.AllowHTTP {
            return nil, fmt.Errorf("%w", ErrUnapprovedScheme)
        }
    default:
        return nil, fmt.Errorf("%w: %s", ErrUnapprovedScheme, scheme)
    }
    host := u.Hostname()
    if host == "" {
        return nil, fmt.Errorf("%w", ErrBareHostname)
    }
    if err := v.validateHostLiteral(host); err != nil {
        return nil, err
    }
    return u, nil
}

// ValidateHost checks a host (literal IP or resolved address) against
// the private-IP blocklist. Called by HTTP clients after every DNS
// resolution and redirect.
func (v DefaultValidator) ValidateHost(host string) error {
    return v.validateHostLiteral(host)
}

func (v DefaultValidator) validateHostLiteral(host string) error {
    // Cloud metadata services (AWS, GCP, Azure, OCI, Alibaba).
    if isCloudMetadataHost(host) {
        return fmt.Errorf("%w: %s", ErrCloudMetadataHost, host)
    }
    // If host is a literal IP, validate it directly.
    if ip := net.ParseIP(host); ip != nil {
        if isBlockedIP(ip, v.AllowLoopback) {
            return fmt.Errorf("%w: %s", ErrPrivateHost, host)
        }
        return nil
    }
    // Hostname; resolve it. If any returned IP is blocked, reject.
    ips, err := net.LookupIP(host)
    if err != nil {
        return fmt.Errorf("resolve host %s: %w", host, err)
    }
    if len(ips) == 0 {
        return fmt.Errorf("%w: no addresses for %s", ErrPrivateHost, host)
    }
    for _, ip := range ips {
        if isBlockedIP(ip, v.AllowLoopback) {
            return fmt.Errorf("%w: %s resolves to %s", ErrPrivateHost, host, ip)
        }
    }
    return nil
}

// isBlockedIP returns true for any address that must not be dialed by
// indexer clients (spec §21.2). When allowLoopback is true, loopback
// addresses (127.0.0.0/8, ::1) pass through — used only by tests.
func isBlockedIP(ip net.IP, allowLoopback bool) bool {
    if !allowLoopback {
        if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
            ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
            return true
        }
    } else {
        // Still block link-local + multicast + unspecified even in tests;
        // only loopback is opt-in.
        if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
            ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
            return true
        }
    }
    if ip.IsPrivate() {
        return true
    }
    // 169.254.0.0/16 is link-local and caught by IsLinkLocalUnicast, but
    // call it out explicitly for clarity on the cloud-metadata overlap.
    if v4 := ip.To4(); v4 != nil {
        switch {
        case v4[0] == 10:
            return true
        case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
            return true
        case v4[0] == 192 && v4[1] == 168:
            return true
        case v4[0] == 169 && v4[1] == 254:
            return true
        case v4[0] == 127:
            if allowLoopback {
                return false
            }
            return true
        case v4[0] == 0:
            return true
        }
    }
    return false
}

// isCloudMetadataHost returns true for the well-known cloud metadata
// service hostnames, which may resolve to public IPs in some clouds
// and must be blocked regardless.
func isCloudMetadataHost(host string) bool {
    h := strings.ToLower(host)
    switch h {
    case "169.254.169.254", "fd00:ec2::254", "metadata.google.internal",
        "metadata.azure.com", "100.100.100.200":
        return true
    }
    return false
}
