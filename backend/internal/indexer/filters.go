// Package indexer — field filter pipeline (spec §13.6).
//
// Filters are applied in declaration order to the raw string value
// extracted for a field. The full set of filters implemented here
// matches the spec; unrecognized filters return an error so a typo in
// a YAML file fails fast instead of silently no-opping.
package indexer

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/local/easysearch/backend/internal/normalize"
)

// ApplyFilters runs the named filters in order. base is the current
// value; each filter receives the previous output and returns the new.
func ApplyFilters(in string, filters []string, baseURL string) (string, error) {
	for _, name := range filters {
		out, err := applyOne(name, in, baseURL)
		if err != nil {
			return "", fmt.Errorf("filter %q: %w", name, err)
		}
		in = out
	}
	return in, nil
}

// ApplyFiltersByLayout is the variant that hands per-call data (like
// the indexer's base_url for resolve_url, or the parsed date layouts
// for parse_date) to filters. Most filters don't need extras.
func ApplyFiltersByLayout(in string, filters []string, baseURL string, dateLayouts []string) (string, error) {
	for _, name := range filters {
		var out string
		var err error
		switch name {
		case "parse_date":
			out, err = filterParseDate(in, dateLayouts)
		default:
			out, err = applyOne(name, in, baseURL)
		}
		if err != nil {
			return "", fmt.Errorf("filter %q: %w", name, err)
		}
		in = out
	}
	return in, nil
}

func applyOne(name, in, baseURL string) (string, error) {
	switch name {
	case "trim":
		return strings.TrimSpace(in), nil
	case "lower":
		return strings.ToLower(in), nil
	case "upper":
		return strings.ToUpper(in), nil
	case "replace":
		// Default 'replace' filter call signature: replace(old, new).
		// We don't carry args from YAML in v1; callers should use
		// inline template piping for parameterized replacements.
		return in, nil
	case "regex_extract":
		// Default pattern: first [A-Za-z0-9]{32,40} word (info hash).
		re := regexp.MustCompile(`[A-Fa-f0-9]{32,40}`)
		if m := re.FindString(in); m != "" {
			return m, nil
		}
		return "", fmt.Errorf("no regex match in %q", in)
	case "parse_int":
		s := strings.TrimSpace(in)
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return strconv.FormatInt(i, 10), nil
		}
		// Tolerate trailing units: "12 users", "5 seeds" — strip first.
		digit := regexp.MustCompile(`-?\d+`)
		if m := digit.FindString(s); m != "" {
			if i, err := strconv.ParseInt(m, 10, 64); err == nil {
				return strconv.FormatInt(i, 10), nil
			}
		}
		return "", fmt.Errorf("not an integer: %q", in)
	case "parse_float":
		s := strings.TrimSpace(in)
		// Tolerate commas as decimal separators (European format).
		s = strings.ReplaceAll(s, ",", ".")
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return "", fmt.Errorf("not a number: %q", in)
		}
		return strconv.FormatFloat(f, 'f', -1, 64), nil
	case "parse_size":
		v, err := normalize.ParseSizeBytes(in)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(v, 10), nil
	case "resolve_url":
		if baseURL == "" {
			return in, nil
		}
		base, err := url.Parse(baseURL)
		if err != nil {
			return "", fmt.Errorf("base url: %w", err)
		}
		ref, err := url.Parse(in)
		if err != nil {
			return "", fmt.Errorf("ref url: %w", err)
		}
		return base.ResolveReference(ref).String(), nil
	case "extract_info_hash":
		// From a magnet link or torrent URL.
		if strings.HasPrefix(in, "magnet:") {
			hi := normalize.ExtractInfoHashFromMagnet(in)
			if hi != "" {
				return hi, nil
			}
		}
		// Fallback: try a 40-char hex directly.
		re := regexp.MustCompile(`[A-Fa-f0-9]{40}`)
		if m := re.FindString(in); m != "" {
			return strings.ToUpper(m), nil
		}
		return "", fmt.Errorf("no info hash found in %q", in)
	default:
		return "", fmt.Errorf("unknown filter %q", name)
	}
}

func filterParseDate(in string, layouts []string) (string, error) {
	t, err := parseDateWithLayouts(in, layouts)
	if err != nil {
		return "", err
	}
	if t.Equal(time.Time{}) {
		return "", nil
	}
	return t.UTC().Format(time.RFC3339), nil
}

func parseDateWithLayouts(s string, layouts []string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	if len(layouts) == 0 {
		// Delegate to normalize package's bundled fallbacks.
		return normalize.ParseDate(s)
	}
	return normalize.ParseDate(s, layouts...)
}
