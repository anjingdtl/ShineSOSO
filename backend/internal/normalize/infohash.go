package normalize

import (
    "encoding/hex"
    "fmt"
    "net/url"
    "regexp"
    "strings"
)

// MagnetInfoHashRE extracts the BTIH v1 urn from a magnet link.
var magnetInfoHashRE = regexp.MustCompile(`(?i)xt=urn:btih:([a-zA-Z0-9]+)`)

// NormalizeInfoHash returns the canonical 40-character uppercase hex
// representation of a BitTorrent v1 info hash. Accepts:
//   - 40-char hex (any case)
//   - 32-char base32 (BitTorrent "short" hash, RFC 4648 without padding)
// Returns an error for anything else.
func NormalizeInfoHash(s string) (string, error) {
    s = strings.TrimSpace(s)
    if s == "" {
        return "", fmt.Errorf("normalize: empty info hash")
    }
    if len(s) == 40 {
        b, err := hex.DecodeString(strings.ToLower(s))
        if err != nil {
            return "", fmt.Errorf("normalize: bad hex info hash: %w", err)
        }
        if len(b) != 20 {
            return "", fmt.Errorf("normalize: hex info hash must be 20 bytes, got %d", len(b))
        }
        return strings.ToUpper(s), nil
    }
    if len(s) == 32 {
        // base32 (no padding). Use hex.Decode works for binary; base32 needs encoding/base32.
        return base32ToUpperHex(s)
    }
    return "", fmt.Errorf("normalize: info hash must be 40 hex or 32 base32 chars, got %d", len(s))
}

func base32ToUpperHex(s string) (string, error) {
    // Add padding to a multiple of 8 if needed.
    padded := s
    for len(padded)%8 != 0 {
        padded += "="
    }
    decoded, err := decodeBase32(padded)
    if err != nil {
        return "", fmt.Errorf("normalize: bad base32 info hash: %w", err)
    }
    if len(decoded) != 20 {
        return "", fmt.Errorf("normalize: base32 info hash must decode to 20 bytes, got %d", len(decoded))
    }
    return strings.ToUpper(hex.EncodeToString(decoded)), nil
}

// ExtractInfoHashFromMagnet returns the canonical info hash from a
// magnet URI's xt=urn:btih: parameter, or "" if none is found.
func ExtractInfoHashFromMagnet(magnet string) string {
    m := magnetInfoHashRE.FindStringSubmatch(magnet)
    if len(m) != 2 {
        return ""
    }
    if h, err := NormalizeInfoHash(m[1]); err == nil {
        return h
    }
    return ""
}

// ExtractInfoHashFromURL returns the canonical info hash if the URL is
// a magnet link, or "" otherwise. The input is accepted as both raw
// magnet URIs and URL-escaped forms inside a query string.
func ExtractInfoHashFromURL(raw string) string {
    raw = strings.TrimSpace(raw)
    if strings.HasPrefix(strings.ToLower(raw), "magnet:?") {
        return ExtractInfoHashFromMagnet(raw)
    }
    // Possibly an http(s) URL with magnet=… in the query; cheap check.
    if u, err := url.Parse(raw); err == nil {
        if m := u.Query().Get("magnet"); m != "" {
            return ExtractInfoHashFromMagnet(m)
        }
    }
    return ""
}
