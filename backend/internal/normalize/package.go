// Package normalize centralizes all the field-level transformation
// rules that produce uniform SearchResult values regardless of which
// indexer the data came from (spec §16).
//
// Functions are pure and side-effect free; callers compose them.
package normalize

import (
    "regexp"
    "strings"

    "golang.org/x/text/unicode/norm"
)

// titleWhitespace collapses runs of whitespace and trims the result.
var titleWhitespace = regexp.MustCompile(`\s+`)

// punctuationStriphers are characters that the spec says to treat as
// word boundaries when building the dedup key.
const punctStriphers = "._-"

// NormalizeTitle returns the canonical form used for dedup (spec §16.1).
//
//   - NFKC unicode normalization
//   - lowercase
//   - punctuation striphers (.,_-) become spaces
//   - whitespace collapsed
//   - leading/trailing whitespace and punctuation removed
//
// The display title is left untouched; only the dedup key is normalized.
func NormalizeTitle(s string) string {
    s = norm.NFKC.String(s)
    s = strings.ToLower(s)
    var b strings.Builder
    b.Grow(len(s))
    for _, r := range s {
        switch r {
        case '.', '_', '-':
            b.WriteByte(' ')
        default:
            b.WriteRune(r)
        }
    }
    out := titleWhitespace.ReplaceAllString(b.String(), " ")
    return strings.TrimSpace(out)
}
