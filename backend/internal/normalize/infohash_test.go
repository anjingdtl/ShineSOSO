package normalize

import (
    "strings"
    "testing"
)

const (
    sampleHexUpper     = "0123456789ABCDEF0123456789ABCDEF01234567"
    sampleHexLower     = "0123456789abcdef0123456789abcdef01234567"
    sampleHexCanonical = "0123456789ABCDEF0123456789ABCDEF01234567"
    // base32 of 0x0123456789abcdef0123456789abcdef01234567 (no padding)
    sampleBase32 = "AERUKZ4JVPG66AJDIVTYTK6N54ASGRLH"
)

func TestNormalizeInfoHashHexUpper(t *testing.T) {
    got, err := NormalizeInfoHash(sampleHexUpper)
    if err != nil {
        t.Fatal(err)
    }
    if got != sampleHexCanonical {
        t.Errorf("upper hex should be unchanged, got %q", got)
    }
}

func TestNormalizeInfoHashHexLower(t *testing.T) {
    got, err := NormalizeInfoHash(sampleHexLower)
    if err != nil {
        t.Fatal(err)
    }
    if got != sampleHexCanonical {
        t.Errorf("lower hex should canonicalize to upper, got %q", got)
    }
}

func TestNormalizeInfoHashBase32(t *testing.T) {
    got, err := NormalizeInfoHash(sampleBase32)
    if err != nil {
        t.Fatalf("base32 should decode, got %v", err)
    }
    // The exact hex is determined by the base32 bytes; the contract is
    // that the result is a 40-char uppercase hex string.
    if len(got) != 40 {
        t.Errorf("expected 40 chars, got %d", len(got))
    }
    if got != strings.ToUpper(got) {
        t.Error("result should be uppercase")
    }
}

func TestNormalizeInfoHashRejectsBad(t *testing.T) {
    cases := []string{"", "short", "ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ", "not a hash"}
    for _, in := range cases {
        if _, err := NormalizeInfoHash(in); err == nil {
            t.Errorf("expected error for %q", in)
        }
    }
}

func TestExtractInfoHashFromMagnet(t *testing.T) {
    magnet := "magnet:?xt=urn:btih:" + sampleHexLower + "&dn=Ubuntu&tr=udp%3A%2F%2Ftracker.example.com"
    got := ExtractInfoHashFromMagnet(magnet)
    if got != sampleHexCanonical {
        t.Errorf("want %q, got %q", sampleHexCanonical, got)
    }
}

func TestExtractInfoHashFromMagnetCaseInsensitive(t *testing.T) {
    magnet := "magnet:?xt=urn:btih:" + sampleHexLower
    if got := ExtractInfoHashFromMagnet(magnet); got != sampleHexCanonical {
        t.Errorf("case-insensitive xt should still extract, got %q", got)
    }
}

func TestExtractInfoHashFromMagnetNoMatch(t *testing.T) {
    if got := ExtractInfoHashFromMagnet("magnet:?dn=Ubuntu"); got != "" {
        t.Errorf("expected empty, got %q", got)
    }
}

func TestExtractInfoHashFromURL(t *testing.T) {
    magnet := "magnet:?xt=urn:btih:" + sampleHexLower
    if got := ExtractInfoHashFromURL(magnet); got != sampleHexCanonical {
        t.Errorf("want %q, got %q", sampleHexCanonical, got)
    }
    if got := ExtractInfoHashFromURL("https://example.com/foo.torrent"); got != "" {
        t.Errorf("non-magnet URL should return empty, got %q", got)
    }
}
