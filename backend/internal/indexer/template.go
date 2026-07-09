// Package indexer — restricted template engine for YAML indexer definitions.
//
// Spec §13.7: only the listed variables are allowed. Anything else in a
// {{ ... }} expression is rejected at parse time so a YAML file can
// never reach into the runtime environment or execute code.
package indexer

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"text/template"
)

// RenderTemplate is the public entry point. `data` supplies query.* and
// indexer.base_url; any other field is ignored.
func RenderTemplate(tmpl string, data TemplateData) (string, error) {
	if !strings.Contains(tmpl, "{{") {
		return tmpl, nil
	}
	t, err := template.New("ymltpl").
		Funcs(safeFuncs).
		Option("missingkey=zero").
		Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse %q: %w", tmpl, err)
	}

	// Pre-walk every {{ ... }} and reject vars not on the allow list.
	if err := rejectForbiddenVars(tmpl); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute %q: %w", tmpl, err)
	}
	return buf.String(), nil
}

// TemplateData is the typed view exposed to YAML templates. Field
// names use Go convention so they map naturally to text/template
// (which always interprets a bare identifier as a func name first).
type TemplateData struct {
	Query   QueryData
	Indexer IndexerData
}

// QueryData mirrors the SearchQuery the orchestrator hands every indexer.
// Only these four are bound to real data; the rest of SearchQuery
// (limits, filters, signal) don't reach the YAML layer.
type QueryData struct {
	Keyword    string
	Category   string
	CategoryID string
	Page       string
}

// IndexerData currently exposes only base_url; future phases may add
// rate-limit headers, etc.
type IndexerData struct {
	BaseURL string
}

// safeFuncs is the allow-list of template functions exposed to YAML.
// urlencode, trim, upper, lower, replace and toupper are spec §13.6
// filter names — exposed here so simple uses (e.g. {{ query.keyword |
// urlencode }}) work without a separate field config.
var safeFuncs = template.FuncMap{
	"urlencode": func(s string) string { return url.QueryEscape(s) },
	"trim":      strings.TrimSpace,
	"lower":     strings.ToLower,
	"upper":     strings.ToUpper,
	"replace":   func(old, new, s string) string { return strings.ReplaceAll(s, old, new) },
}

// templateAllowed is mirrored from catalog.isAllowedTemplate; the
// adapter layer doesn't import catalog (would be a cycle in some
// package layouts) so the rule is duplicated here. Keep in sync.
var templateAllowed = map[string]bool{
	".Query.Keyword":     true,
	".Query.Category":    true,
	".Query.CategoryID":  true,
	".Query.Page":        true,
	".Indexer.BaseURL":   true,
}

func rejectForbiddenVars(tmpl string) error {
	for i := 0; i+1 < len(tmpl); {
		if tmpl[i] != '{' || tmpl[i+1] != '{' {
			i++
			continue
		}
		end := strings.Index(tmpl[i+2:], "}}")
		if end < 0 {
			return fmt.Errorf("unterminated template expression")
		}
		expr := strings.TrimSpace(tmpl[i+2 : i+2+end])
		varIdent := expr
		if idx := strings.IndexAny(expr, " |"); idx >= 0 {
			varIdent = strings.TrimSpace(expr[:idx])
		}
		if varIdent == "" {
			i += 2 + end + 2
			continue
		}
		if !templateAllowed[varIdent] {
			return fmt.Errorf("variable %q is not allowed (only .Query.* and .Indexer.BaseURL)", varIdent)
		}
		i += 2 + end + 2
	}
	return nil
}
