package diagnostics

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
)

// MagnetPrefix matches the magnet URI scheme. Captures only the prefix
// so we can replace the rest with a fixed-length placeholder that still
// keeps log structure intact.
const magnetPrefix = "magnet:?xt=urn:btih:"

// MagnetRe matches a magnet link anywhere in a string.
var magnetRe = regexp.MustCompile(`magnet:\?xt=urn:btih:[A-Za-z0-9]+[^\s"']*`)

// InfoHashRe matches a bare 40-char hex or 32-char base32 BTIH outside
// of a magnet URI. We replace these too because they effectively leak
// the same identifier.
var infoHashRe = regexp.MustCompile(`\b[0-9a-fA-F]{40}\b|\b[A-Z2-7]{32}\b`)

// QueryRe is a deliberately conservative regex that matches likely
// search keywords in log messages. The orchestrator logs the keyword
// as `keyword=<value>`. We strip the value, not the key.
var queryRe = regexp.MustCompile(`(?i)(keyword|q|query)=("[^"]*"|'[^']*'|\S+)`)

// URLWithCredsRe matches URLs that embed credentials or tokens. The
// YAML validator already rejects such URLs at definition time, but
// defense in depth: anything that slips through is redacted.
var urlWithCredsRe = regexp.MustCompile(`\b[a-zA-Z][a-zA-Z0-9+.-]*://[^/\s:@]+:[^/\s@]+@[^\s"']+`)

// SanitizeBytes runs SanitizeLine over each newline-terminated segment
// of the input. Useful for log tails where we want to keep line numbers.
func SanitizeBytes(b []byte) []byte {
	// We split on \n so we can preserve line boundaries; long single
	// lines (a JSON dump with embedded URLs) are still sanitized via
	// SanitizeLine.
	var out bytes.Buffer
	out.Grow(len(b))
	start := 0
	for i := 0; i < len(b); i++ {
		if b[i] == '\n' {
			out.Write(SanitizeLine(string(b[start:i])))
			out.WriteByte('\n')
			start = i + 1
		}
	}
	if start < len(b) {
		out.Write(SanitizeLine(string(b[start:])))
	}
	return out.Bytes()
}

// SanitizeLine scrubs one line of free-form text of:
//
//   - magnet links (full URI replaced with magnetPrefix + "<redacted>")
//   - bare 40-hex / 32-base32 infohash values
//   - credentialed URLs
//   - search-keyword log fields
func SanitizeLine(line string) []byte {
	if line == "" {
		return []byte{}
	}
	// Order matters: strip credentials first so a magnet disguised as
	// userinfo doesn't survive the URL pass.
	line = urlWithCredsRe.ReplaceAllString(line, "[redacted-url-with-creds]")
	line = queryRe.ReplaceAllString(line, "$1=<redacted>")
	line = magnetRe.ReplaceAllString(line, magnetPrefix+"<redacted>")
	line = infoHashRe.ReplaceAllString(line, "<btih-redacted>")
	return []byte(line)
}

// SanitizeErrorMessage returns a copy of msg with magnets and credentials
// stripped and the total length capped at MaxErrorMessageLen. If the
// truncated result ends mid-token it is cut on the last word boundary
// followed by an ellipsis.
func SanitizeErrorMessage(msg string) string {
	if msg == "" {
		return ""
	}
	cleaned := string(SanitizeLine(msg))
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) <= MaxErrorMessageLen {
		return cleaned
	}
	cut := cleaned[:MaxErrorMessageLen]
	if i := strings.LastIndexAny(cut, " \t\n\r"); i > MaxErrorMessageLen/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " \t\n\r") + "…"
}

// encodeJSON is a tiny helper around encoding/json so the diagnostics
// package can write flat structs without dragging in extra deps.
func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}