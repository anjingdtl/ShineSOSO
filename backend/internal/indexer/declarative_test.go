package indexer

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/local/easysearch/backend/internal/model"
	"github.com/local/easysearch/backend/internal/security"
)

func TestRenderTemplate_substitutesAllowedVars(t *testing.T) {
	data := TemplateData{
		Query:   QueryData{Keyword: "matrix", Page: "2"},
		Indexer: IndexerData{BaseURL: "https://example.com"},
	}
	out, err := RenderTemplate("https://example.com/search?q={{ .Query.Keyword }}&p={{ .Query.Page }}&base={{ .Indexer.BaseURL }}", data)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(out, "q=matrix") || !strings.Contains(out, "p=2") || !strings.Contains(out, "base=https://example.com") {
		t.Errorf("rendered wrong: %q", out)
	}
}

func TestRenderTemplate_rejectsForbiddenVar(t *testing.T) {
	data := TemplateData{Query: QueryData{Keyword: "x"}}
	_, err := RenderTemplate("{{ .Env.HOME }}", data)
	if err == nil {
		t.Fatalf("forbidden var must fail")
	}
}

func TestApplyFilters_pipeline(t *testing.T) {
	tests := []struct {
		in    string
		filt  []string
		base  string
		want  string
	}{
		{"  hello  ", []string{"trim"}, "", "hello"},
		{"Hello", []string{"upper"}, "", "HELLO"},
		{"1.2 GB", []string{"trim", "parse_size"}, "", "1200000000"}, // decimal SI
		{"1.2 GiB", []string{"trim", "parse_size"}, "", "1288490188"}, // binary IEC
		{"42 seeds", []string{"trim", "parse_int"}, "", "42"},
		{"/detail/1", []string{"resolve_url"}, "https://example.com/page", "https://example.com/detail/1"},
	}
	for _, tc := range tests {
		got, err := ApplyFilters(tc.in, tc.filt, tc.base)
		if err != nil {
			t.Errorf("ApplyFilters(%q, %v): %v", tc.in, tc.filt, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ApplyFilters(%q, %v) = %q, want %q", tc.in, tc.filt, got, tc.want)
		}
	}
}

func TestApplyFilters_parseDate(t *testing.T) {
	got, err := ApplyFiltersByLayout("2026-07-09 12:00", []string{"parse_date"}, "", []string{"2006-01-02 15:04"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "2026-07-09T12:00:00") {
		t.Errorf("got %q", got)
	}
}

func TestApplyFilters_extractInfoHash(t *testing.T) {
	got, err := ApplyFilters("magnet:?xt=urn:btih:00112233445566778899AABBCCDDEEFF00112233&dn=x", []string{"extract_info_hash"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "00112233445566778899AABBCCDDEEFF00112233" {
		t.Errorf("got %q", got)
	}
}

func TestApplyFilters_unknownFilterFails(t *testing.T) {
	_, err := ApplyFilters("x", []string{"nope"}, "")
	if err == nil || !strings.Contains(err.Error(), "unknown filter") {
		t.Fatalf("want unknown filter error, got %v", err)
	}
}

const fixtureHTML = `
<html><body>
<table>
  <tr class="r"><td><a class="t" href="/d/1">First</a></td><td class="sz">1.2 GB</td></tr>
  <tr class="r"><td><a class="t" href="/d/2">Second</a></td><td class="sz">512 MB</td></tr>
</table>
</body></html>
`

func sampleDecl() (model.IndexerDefinition, model.InstalledIndexer) {
	def := model.IndexerDefinition{
		ID: "fixture", Type: "public", Protocol: "declarative",
		Links: []string{"https://example.com"},
		Search: model.SearchDefinition{
			Method: "GET", Path: "/search",
			Query:  map[string]string{"q": "{{ .Query.Keyword }}"},
			Headers: map[string]string{"Accept": "text/html"},
			TimeoutSeconds: 5,
		},
		Result: model.ResultDefinition{
			Format: "html",
			Rows:   model.RowDefinition{Selector: "tr.r"},
			Fields: map[string]model.FieldDefinition{
				"title":      {Selector: "a.t", Value: "text", Required: true},
				"detail_url": {Selector: "a.t", Value: "attr", Attribute: "href", ResolveURL: true},
				"size":       {Selector: "td.sz", Value: "text", Filters: []string{"trim", "parse_size"}},
			},
		},
	}
	installed := model.InstalledIndexer{ID: "id1", Name: "Fixture", BaseURL: "https://example.com"}
	return def, installed
}

func TestExtractRow_parsesTwoRows(t *testing.T) {
	def, installed := sampleDecl()
	a := &declarativeAdapter{id: installed.ID, def: def, installed: installed}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(fixtureHTML))
	if err != nil {
		t.Fatal(err)
	}
	out := []model.SearchResult{}
	doc.Find(def.Result.Rows.Selector).Each(func(_ int, row *goquery.Selection) {
		res, ok := a.extractRow(row, installed.ID, installed.Name)
		if ok {
			out = append(out, res)
		}
	})
	if len(out) != 2 {
		t.Fatalf("want 2 rows, got %d (%+v)", len(out), out)
	}
	if out[0].Title != "First" || out[1].Title != "Second" {
		t.Errorf("titles wrong: %q / %q", out[0].Title, out[1].Title)
	}
	if out[0].SizeBytes == nil || *out[0].SizeBytes != 1_200_000_000 {
		t.Errorf("row0 size: %v", out[0].SizeBytes)
	}
	if out[1].SizeBytes == nil || *out[1].SizeBytes != 512_000_000 {
		t.Errorf("row1 size: %v", out[1].SizeBytes)
	}
	if !strings.HasPrefix(out[0].DetailURL, "https://example.com/d/") {
		t.Errorf("row0 detail_url not resolved: %q", out[0].DetailURL)
	}
}

func TestFactory_acceptsHTMLJSONXML(t *testing.T) {
	// Post-Phase-8: the declarative factory owns html|json|xml (and the
	// empty string, which Search() treats like html). Anything else —
	// notably "torznab" — is rejected so callers route to the right
	// factory.
	for _, fmt_ := range []string{"", "html", "json", "xml"} {
		def, installed := sampleDecl()
		def.Result.Format = fmt_
		if _, err := NewDeclarativeFactory().Create(def, installed, NewClient()); err != nil {
			t.Errorf("format %q: want success, got %v", fmt_, err)
		}
	}
}

func TestFactory_rejectsNonDeclarativeFormat(t *testing.T) {
	def, installed := sampleDecl()
	def.Result.Format = "torznab"
	_, err := NewDeclarativeFactory().Create(def, installed, NewClient())
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("want format error for torznab, got %v", err)
	}
}

// TestBuildURL_composesPathQueryAndBase verifies the URL builder
// without ever touching the network — the SSRF guard would otherwise
// reject any local test server. End-to-end over the network is covered
// by the Phase 5 smoke test.
func TestBuildURL_composesPathQueryAndBase(t *testing.T) {
	def, installed := sampleDecl()
	installed.BaseURL = "https://example.com"
	a := &declarativeAdapter{id: installed.ID, def: def, installed: installed}
	u, err := a.buildURL("matrix", "all", "1", "2")
	if err != nil {
		t.Fatalf("build url: %v", err)
	}
	if u.Host != "example.com" {
		t.Errorf("host: %q", u.Host)
	}
	if u.Path != "/search" {
		t.Errorf("path: %q", u.Path)
	}
	q := u.Query()
	if q.Get("q") != "matrix" {
		t.Errorf("query q: %q", q.Get("q"))
	}
}

// kept import happy
var _ = fmt.Println
var _ = context.Background
var _ = http.StatusOK
var _ = time.Now
var _ = httptest.NewServer
var _ = security.DefaultValidator{}
