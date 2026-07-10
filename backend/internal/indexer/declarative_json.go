// Package indexer — declarative JSON adapter.
//
// Companion to declarative.go. Walks a JSON response body using path
// expressions declared in def.Fields, where the field whose map key is
// "row" provides the row anchor. Path syntax:
//   - "results[*]"          : expand each value in `results`
//   - "results[*].field"    : chain into each row's `field`
//   - "title"               : leaf, read scalar
//
// Field values are typed via the FieldDefinition.Value string (which the
// HTML pipeline uses for "text"/"html"/"attr"); here it's repurposed as
// the kind: "string", "int", "size", "infohash", or "magnet_url". The
// normalize package handles the size / infohash conversions.
package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/local/easysearch/backend/internal/model"
	"github.com/local/easysearch/backend/internal/normalize"
)

// runJSON parses a JSON body and applies the field paths declared in
// def.Fields, using the `row` anchor to find row blocks.
func (a *declarativeAdapter) runJSON(_ context.Context, body []byte, def model.ResultDefinition) ([]model.SearchResult, error) {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("declarative json: %w", err)
	}
	rowPath := ""
	if f, ok := def.Fields["row"]; ok {
		rowPath = f.Selector
	}
	if rowPath == "" {
		return nil, fmt.Errorf("declarative json: no `row` anchor field declared")
	}
	rows := jsonRowsAtPath(doc, rowPath)
	out := make([]model.SearchResult, 0, len(rows))
	for idx, row := range rows {
		rec := a.buildJSONResult(row, def.Fields, idx)
		if rec.Title == "" {
			continue
		}
		rec.IndexerID = a.installed.ID
		rec.IndexerName = a.installed.Name
		rec.NormalizedTitle = strings.ToLower(strings.TrimSpace(rec.Title))
		out = append(out, rec)
	}
	return out, nil
}

// jsonRowsAtPath walks `path` from the root, expanding any "[*]"
// segment into its map children. Returns the resulting set of rows
// (each a map[string]any). The brief's notation is the only supported
// shape — identifiers separated by ".", with "[*]" suffix to enumerate
// a level.
func jsonRowsAtPath(doc any, path string) []map[string]any {
	out := []map[string]any{}
	if path == "" {
		if m, ok := doc.(map[string]any); ok {
			return []map[string]any{m}
		}
		return out
	}
	segs := splitJSONPath(path)
	cur := []any{doc}
	for _, seg := range segs {
		nex := []any{}
		for _, n := range cur {
			m, ok := n.(map[string]any)
			if !ok {
				continue
			}
			if seg.wildcard {
				for _, v := range m {
					switch vv := v.(type) {
					case map[string]any:
						nex = append(nex, vv)
					case []any:
						for _, x := range vv {
							if xm, ok := x.(map[string]any); ok {
								nex = append(nex, xm)
							}
						}
					}
				}
				continue
			}
			if v, ok := m[seg.name]; ok {
				nex = append(nex, v)
			}
		}
		cur = nex
	}
	for _, n := range cur {
		if m, ok := n.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// jsonPathSeg is a single segment of a JSON path expression.
type jsonPathSeg struct {
	name     string
	wildcard bool
}

// splitJSONPath parses an identifier path like "results[*].foo" into
// segments. Segments with an "[*]" suffix are wildcard (enumerate).
func splitJSONPath(path string) []jsonPathSeg {
	parts := strings.Split(path, ".")
	segs := make([]jsonPathSeg, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "[*]") {
			segs = append(segs, jsonPathSeg{
				name:     strings.TrimSuffix(p, "[*]"),
				wildcard: true,
			})
			continue
		}
		segs = append(segs, jsonPathSeg{name: p})
	}
	return segs
}

// buildJSONResult walks each non-row field's selector against the row
// map and applies the field type, populating the matching SearchResult
// fields.
func (a *declarativeAdapter) buildJSONResult(row map[string]any, fields map[string]model.FieldDefinition, idx int) model.SearchResult {
	res := model.SearchResult{
		IndexerID:   a.installed.ID,
		IndexerName: a.installed.Name,
		ID:          fmt.Sprintf("%s:json:%d", a.installed.ID, idx),
	}
	for name, f := range fields {
		if name == "row" {
			continue
		}
		segs := splitJSONPath(f.Selector)
		if len(segs) == 0 {
			continue
		}
		v, ok := jsonLookup(row, segs)
		if !ok || v == nil {
			continue
		}
		applyJSONTyped(&res, name, f.Value, v)
	}
	// Reconstruct a magnet from an extracted info hash when no
	// magnet_url field was provided (mirrors the HTML pipeline's
	// fallback in extractRow).
	if res.MagnetURL == "" && res.InfoHash != "" {
		res.MagnetURL = "magnet:?xt=urn:btih:" + strings.ToLower(res.InfoHash)
	}
	return res
}

// jsonLookup descends a row map along the given segments and returns
// the first non-nil value found. Returns (nil, false) on miss.
func jsonLookup(root any, segs []jsonPathSeg) (any, bool) {
	cur := root
	for _, s := range segs {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[s.name]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

// applyJSONTyped maps a typed JSON value into a SearchResult field by
// the FieldDefinition.Value string. Falls back to "string" when unset.
func applyJSONTyped(res *model.SearchResult, name, kind string, val any) {
	switch kind {
	case "int":
		n, ok := jsonToInt64(val)
		if !ok {
			return
		}
		i := int(n)
		switch name {
		case "seeders":
			res.Seeders = &i
		case "leechers":
			res.Leechers = &i
		case "downloads":
			res.Downloads = &i
		}
	case "size":
		n, ok := jsonToInt64(val)
		if !ok {
			return
		}
		res.SizeBytes = &n
	case "infohash":
		s, ok := val.(string)
		if !ok {
			return
		}
		h, err := normalize.NormalizeInfoHash(s)
		if err != nil {
			return
		}
		res.InfoHash = h
	case "magnet_url":
		s, ok := val.(string)
		if !ok {
			return
		}
		res.MagnetURL = s
	case "string", "":
		s := jsonScalarToString(val)
		switch name {
		case "title":
			res.Title = s
		case "category":
			res.Category = s
		}
	}
}

// jsonScalarToString turns a JSON-decoded value into its printable
// representation. encoding/json decodes numbers as float64, so we
// format integers without trailing decimals.
func jsonScalarToString(val any) string {
	switch v := val.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// jsonToInt64 coerces a JSON value to int64. Accepts float64 (the
// common JSON numeric decode), int, int64, and a base-10 string.
func jsonToInt64(val any) (int64, bool) {
	switch v := val.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}
