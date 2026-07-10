// Package indexer — declarative XML adapter.
//
// Companion to declarative.go. Streams the XML body using
// encoding/xml.Decoder, tracking the open-element stack until it
// matches the path expression declared as the `row` anchor (e.g.
// "rss.channel.item[*]"). Each matched row's children are read as a
// name→text map; subsequent field declarations use bare element names
// (with the same "string" / "int" / "size" / "infohash" / "magnet_url"
// kind convention as the JSON adapter) to assign each typed value.
//
// This implementation targets the flat fixture shape used in tests:
// no nested elements of the same name within a row. Nested elements
// inside leaf text (e.g. <title>Hello <em>world</em></title>) are
// flattened into a single accumulated string.
package indexer

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/local/easysearch/backend/internal/model"
	"github.com/local/easysearch/backend/internal/normalize"
)

// runXML parses an XML body and extracts rows defined by the `row`
// anchor in def.Fields.
func (a *declarativeAdapter) runXML(body []byte, def model.ResultDefinition) ([]model.SearchResult, error) {
	f, ok := def.Fields["row"]
	if !ok {
		return nil, fmt.Errorf("declarative xml: no `row` anchor field declared")
	}
	rows, err := xmlRowsAtPath(bytes.NewReader(body), f.Selector)
	if err != nil {
		return nil, fmt.Errorf("declarative xml: %w", err)
	}
	out := make([]model.SearchResult, 0, len(rows))
	for idx, row := range rows {
		rec := a.buildXMLResult(row, def.Fields, idx)
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

// xmlRowsAtPath streams XML tokens and returns every row whose
// enclosing element stack matches the path. The path is a "."-separated
// element name list; a trailing "[*]" on the last segment indicates
// the row level (and is otherwise a no-op for matching since we
// enumerate all matching elements, not just one).
func xmlRowsAtPath(r io.Reader, anchor string) ([]map[string]string, error) {
	segs := splitXMLPath(anchor)
	if len(segs) == 0 {
		return nil, fmt.Errorf("declarative xml: empty row anchor path")
	}
	stack := []string{}
	dec := xml.NewDecoder(r)
	rows := []map[string]string{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch tt := tok.(type) {
		case xml.StartElement:
			stack = append(stack, tt.Name.Local)
			if stackMatches(stack, segs) {
				row, err := readXMLRow(dec, tt.Name.Local)
				if err != nil {
					return nil, err
				}
				rows = append(rows, row)
				// readXMLRow consumed up to and including the
				// matching EndElement; pop manually.
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return rows, nil
}

// splitXMLPath parses "rss.channel.item[*]" into ["rss","channel","item"].
// Wildcard suffix is stripped.
func splitXMLPath(path string) []string {
	parts := strings.Split(path, ".")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, strings.TrimSuffix(p, "[*]"))
	}
	return out
}

// stackMatches reports whether the current open-element stack equals
// the desired path exactly. Using equality (rather than prefix) keeps
// the walker unambiguous about which row level we're at.
func stackMatches(stack, want []string) bool {
	if len(stack) != len(want) {
		return false
	}
	for i := range stack {
		if stack[i] != want[i] {
			return false
		}
	}
	return true
}

// readXMLRow reads a row element's direct children, capturing each
// child's text into row[name]. Stops at the matching EndElement.
func readXMLRow(dec *xml.Decoder, startName string) (map[string]string, error) {
	row := map[string]string{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return row, err
		}
		switch tt := tok.(type) {
		case xml.StartElement:
			name := tt.Name.Local
			s, err := readElementText(dec, name)
			if err != nil {
				return row, err
			}
			row[name] = s
		case xml.EndElement:
			if tt.Name.Local == startName {
				return row, nil
			}
		}
	}
}

// readElementText reads tokens until the matching EndElement for
// `name`, accumulating CharData. Nested StartElements are ignored;
// their matching EndElement just passes through. The brief's flat
// fixture doesn't exercise nesting, but a single level of nesting
// (e.g. <title>The <b>best</b></title>) is handled correctly.
func readElementText(dec *xml.Decoder, name string) (string, error) {
	var buf strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return strings.TrimSpace(buf.String()), err
		}
		switch tt := tok.(type) {
		case xml.StartElement:
			// Nested elements: drop; their closing tag falls to the
			// next EndElement branch, which is a no-op unless it
			// matches `name`.
		case xml.EndElement:
			if tt.Name.Local == name {
				return strings.TrimSpace(buf.String()), nil
			}
		case xml.CharData:
			buf.Write(tt)
		}
	}
}

// buildXMLResult maps a row's child text into a SearchResult. Field
// names use bare element names matching the row map keys; values are
// coerced by FieldDefinition.Value (the field kind).
func (a *declarativeAdapter) buildXMLResult(row map[string]string, fields map[string]model.FieldDefinition, idx int) model.SearchResult {
	res := model.SearchResult{
		IndexerID:   a.installed.ID,
		IndexerName: a.installed.Name,
		ID:          fmt.Sprintf("%s:xml:%d", a.installed.ID, idx),
	}
	for name, f := range fields {
		if name == "row" {
			continue
		}
		s, ok := row[f.Selector]
		if !ok {
			continue
		}
		applyXMLTyped(&res, name, f.Value, s)
	}
	if res.MagnetURL == "" && res.InfoHash != "" {
		res.MagnetURL = "magnet:?xt=urn:btih:" + strings.ToLower(res.InfoHash)
	}
	return res
}

// applyXMLTyped parses a string value into the appropriate numeric /
// hash type and assigns it to the matching SearchResult field.
func applyXMLTyped(res *model.SearchResult, name, kind, val string) {
	switch kind {
	case "int":
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
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
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return
		}
		res.SizeBytes = &n
	case "infohash":
		h, err := normalize.NormalizeInfoHash(val)
		if err != nil {
			return
		}
		res.InfoHash = h
	case "magnet_url":
		res.MagnetURL = val
	case "string", "":
		switch name {
		case "title":
			res.Title = val
		case "category":
			res.Category = val
		}
	}
}
