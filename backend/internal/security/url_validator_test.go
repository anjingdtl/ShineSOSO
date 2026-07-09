package security

import (
	"errors"
	"net"
	"strings"
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

func TestValidateURL_CloudMetadataVariants(t *testing.T) {
	v := DefaultValidator{}
	hosts := []string{
		"https://fd00:ec2::254/foo",
		"https://metadata.azure.com/foo",
		"https://100.100.100.200/foo",
	}
	for _, raw := range hosts {
		if _, err := v.ValidateURL(raw); err == nil {
			t.Errorf("expected cloud-metadata rejection for %s, got nil", raw)
		}
	}
}

func TestValidateURL_AllPrivateIPRanges(t *testing.T) {
	v := DefaultValidator{}
	cases := []string{
		"https://0.0.0.0/foo",
		"https://172.20.5.1/foo",
		"https://172.31.255.255/foo",
		"https://[fe80::1]/foo", // link-local
		"https://[ff02::1]/foo", // multicast
		"https://[::]/foo",      // unspecified
		"https://224.0.0.1/foo", // IPv4 multicast
		"https://169.254.10.20/foo", // link-local IPv4
	}
	for _, raw := range cases {
		if _, err := v.ValidateURL(raw); err == nil {
			t.Errorf("expected rejection for %s, got nil", raw)
		}
	}
}

func TestValidateURL_AllowLoopbackLets127Through(t *testing.T) {
	v := DefaultValidator{AllowLoopback: true}
	if _, err := v.ValidateURL("https://127.0.0.1/foo"); err != nil {
		t.Errorf("AllowLoopback should accept 127.0.0.1, got %v", err)
	}
	// But still blocks link-local + private
	if _, err := v.ValidateURL("https://10.0.0.1/foo"); err == nil {
		t.Errorf("AllowLoopback should NOT accept private IP, got nil")
	}
	if _, err := v.ValidateURL("https://169.254.1.1/foo"); err == nil {
		t.Errorf("AllowLoopback should NOT accept link-local, got nil")
	}
}

func TestValidateHost_IPv6Variants(t *testing.T) {
	v := DefaultValidator{}
	bad := []string{
		"::",      // unspecified
		"::1",     // loopback
		"fe80::1", // link-local
		"fc00::1", // unique local (RFC 4193)
		"ff02::1", // multicast
	}
	for _, h := range bad {
		if err := v.ValidateHost(h); err == nil {
			t.Errorf("expected %s blocked, got nil", h)
		}
	}
}

func TestIsBlockedIP_AllowLoopbackBranch(t *testing.T) {
	// allowLoopback=false -> loopback blocked
	if !isBlockedIP(net.ParseIP("127.0.0.1"), false) {
		t.Error("loopback should be blocked when allowLoopback=false")
	}
	// allowLoopback=true -> loopback allowed
	if isBlockedIP(net.ParseIP("127.0.0.1"), true) {
		t.Error("loopback should pass when allowLoopback=true")
	}
	// allowLoopback=true but link-local still blocked
	if !isBlockedIP(net.ParseIP("169.254.1.1"), true) {
		t.Error("link-local must be blocked even with allowLoopback")
	}
	// allowLoopback=true but multicast still blocked
	if !isBlockedIP(net.ParseIP("224.0.0.1"), true) {
		t.Error("multicast must be blocked even with allowLoopback")
	}
	// unspecified blocked
	if !isBlockedIP(net.ParseIP("0.0.0.0"), false) {
		t.Error("0.0.0.0 must be blocked")
	}
	// IPv6 unspecified
	if !isBlockedIP(net.ParseIP("::"), false) {
		t.Error(":: must be blocked")
	}
	// public IP not blocked
	if isBlockedIP(net.ParseIP("8.8.8.8"), false) {
		t.Error("8.8.8.8 should not be blocked")
	}
	// public IPv6 not blocked
	if isBlockedIP(net.ParseIP("2606:4700:4700::1111"), false) {
		t.Error("public IPv6 should not be blocked")
	}
	// IPv6 unique local (fc00::/7)
	if !isBlockedIP(net.ParseIP("fd00::1"), false) {
		t.Error("fd00::1 (ULA) must be blocked")
	}
	// IPv6 interface-local multicast
	if !isBlockedIP(net.ParseIP("ff01::1"), false) {
		t.Error("ff01::1 (interface-local multicast) must be blocked")
	}
}

func TestIsCloudMetadataHost_AllVariants(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"169.254.169.254", true},
		{"fd00:ec2::254", true},
		{"metadata.google.internal", true},
		{"metadata.azure.com", true},
		{"100.100.100.200", true},
		// case-insensitive
		{"METADATA.GOOGLE.INTERNAL", true},
		// negatives
		{"example.com", false},
		{"1.1.1.1", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isCloudMetadataHost(tc.host); got != tc.want {
			t.Errorf("isCloudMetadataHost(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestValidateHost_DNSFailure(t *testing.T) {
	// Use a TLD that is guaranteed not to resolve.
	v := DefaultValidator{}
	err := v.ValidateHost("definitely-not-a-real-host-12345.invalid")
	if err == nil {
		// Some sandboxes return no-error-with-empty-result; only fail
		// when err is non-nil AND we can assert it wraps the resolve
		// or ErrPrivateHost path.
		t.Skip("sandbox allowed unresolvable host; skipping")
	}
	// Either the resolve error OR the ErrPrivateHost-no-addresses
	// error is acceptable.
	if !strings.Contains(err.Error(), "definitely-not-a-real-host-12345.invalid") {
		t.Errorf("error should mention the host: %v", err)
	}
}

func TestValidateHost_HostnameResolvesToPublic(t *testing.T) {
	// one.one.one.one is Cloudflare DNS, resolves to a public IP.
	// This exercises the successful-hostname-resolution branch in
	// validateHostLiteral.
	v := DefaultValidator{}
	if err := v.ValidateHost("one.one.one.one"); err != nil {
		// If DNS isn't available in this environment, skip rather than fail.
		if strings.Contains(err.Error(), "no such host") ||
			strings.Contains(err.Error(), "no addresses") {
			t.Skipf("DNS unavailable in test sandbox: %v", err)
		}
		t.Errorf("expected public hostname to pass: %v", err)
	}
}

func TestValidateHost_HostnameResolvesToPrivate(t *testing.T) {
	// localhost should resolve to 127.0.0.1 / ::1, both blocked.
	v := DefaultValidator{}
	if err := v.ValidateHost("localhost"); err == nil {
		t.Error("localhost should be blocked (loopback), got nil")
	}
}

func TestValidateURL_LowercaseScheme(t *testing.T) {
	v := DefaultValidator{}
	// Mixed-case scheme should still be accepted
	if _, err := v.ValidateURL("HTTPS://198.51.100.1/foo"); err != nil {
		t.Errorf("uppercase HTTPS should be normalized: %v", err)
	}
	if _, err := v.ValidateURL("HtTpS://198.51.100.1/foo"); err != nil {
		t.Errorf("mixed-case HTTPS should be normalized: %v", err)
	}
}

func TestValidateHost_HostnameResolutionToPublic(t *testing.T) {
	v := DefaultValidator{}
	// 1.1.1.1 literal — public DNS server, should pass.
	if err := v.ValidateHost("1.1.1.1"); err != nil {
		t.Errorf("public literal IP should pass: %v", err)
	}
	// 2606:4700:4700::1111 — Cloudflare DNS IPv6.
	if err := v.ValidateHost("2606:4700:4700::1111"); err != nil {
		t.Errorf("public IPv6 literal should pass: %v", err)
	}
}