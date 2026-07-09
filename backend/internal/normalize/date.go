package normalize

import (
    "fmt"
    "strings"
    "time"
)

// Default date layouts to try when the caller has none. The spec says
// (spec §16.3) that the indexer definition may declare its own layouts;
// these are the fallbacks.
var defaultDateLayouts = []string{
    time.RFC3339,
    time.RFC1123Z,
    time.RFC1123,
    "2006-01-02T15:04:05",
    "2006-01-02 15:04:05",
    "2006-01-02 15:04",
    "2006-01-02",
    "01-02-2006",
    "02 Jan 2006 15:04",
    "02 Jan 2006",
    "Jan 2, 2006 3:04 PM",
    "Jan 2, 2006",
}

// ParseDate attempts to parse s as a timestamp using layouts in order.
// The returned time is always in UTC; the spec forbids substituting
// "now" when parsing fails.
func ParseDate(s string, layouts ...string) (time.Time, error) {
    s = strings.TrimSpace(s)
    if s == "" {
        return time.Time{}, fmt.Errorf("normalize: empty date")
    }
    if len(layouts) == 0 {
        layouts = defaultDateLayouts
    }
    for _, layout := range layouts {
        if t, err := time.Parse(layout, s); err == nil {
            return t.UTC(), nil
        }
    }
    return time.Time{}, fmt.Errorf("normalize: no layout matched %q", s)
}

// ParseUnixSeconds converts a unix timestamp in seconds to a UTC time.Time.
func ParseUnixSeconds(secs int64) time.Time {
    return time.Unix(secs, 0).UTC()
}
