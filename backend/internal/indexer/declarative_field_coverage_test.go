package indexer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/local/easysearch/backend/internal/model"
)

func mustGoquery(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	return doc
}

// TestDeclarativeAdapter_CategoryAndDirect exercises the category /
// direct_url / torrent_url / leechers / downloads / published_at
// branches in assignField.
func TestDeclarativeAdapter_CategoryAndDirect(t *testing.T) {
	html := `<html><body>
<table>
  <tr class="row">
    <td class="title"><a href="/dl/1.torrent">title</a></td>
    <td class="cat">movie</td>
    <td class="size">1024</td>
    <td class="seeders">5</td>
    <td class="leechers">2</td>
    <td class="downloads">100</td>
    <td class="date">2026-07-09</td>
    <td class="direct">https://files.example.com/x.zip</td>
    <td class="torrent">https://files.example.com/x.torrent</td>
  </tr>
</table>
</body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	def := sampleDefinition()
	// Override fields to exercise assignField branches.
	def.Result.Fields = map[string]model.FieldDefinition{
		"title":        {Selector: "td.title a", Filters: []string{"trim"}},
		"category":     {Selector: "td.cat"},
		"size":         {Selector: "td.size", Filters: []string{"trim", "parse_int"}},
		"seeders":      {Selector: "td.seeders", Filters: []string{"trim", "parse_int"}},
		"leechers":     {Selector: "td.leechers", Filters: []string{"trim", "parse_int"}},
		"downloads":    {Selector: "td.downloads", Filters: []string{"trim", "parse_int"}},
		"published_at": {Selector: "td.date", Filters: []string{"trim", "parse_date"}, DateLayouts: []string{"2006-01-02"}},
		"direct_url":   {Selector: "td.direct"},
		"torrent_url":  {Selector: "td.torrent"},
	}
	inst := model.InstalledIndexer{ID: "x", Name: "X", BaseURL: srv.URL}
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "k"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
	r := res[0]
	if r.Category != "movie" {
		t.Errorf("category: %q", r.Category)
	}
	if r.SizeBytes == nil || *r.SizeBytes != 1024 {
		t.Errorf("size: %v", r.SizeBytes)
	}
	if r.Seeders == nil || *r.Seeders != 5 {
		t.Errorf("seeders: %v", r.Seeders)
	}
	if r.Leechers == nil || *r.Leechers != 2 {
		t.Errorf("leechers: %v", r.Leechers)
	}
	if r.Downloads == nil || *r.Downloads != 100 {
		t.Errorf("downloads: %v", r.Downloads)
	}
	if r.PublishedAt == nil {
		t.Errorf("published_at is nil")
	}
	if r.DirectURL != "https://files.example.com/x.zip" {
		t.Errorf("direct: %q", r.DirectURL)
	}
	if r.TorrentURL != "https://files.example.com/x.torrent" {
		t.Errorf("torrent: %q", r.TorrentURL)
	}
}

// TestDeclarativeAdapter_InfoHashImpliesMagnet exercises the "if only
// info_hash was extracted, build a magnet" branch.
func TestDeclarativeAdapter_InfoHashImpliesMagnet(t *testing.T) {
	html := `<html><body>
<table>
  <tr class="row">
    <td class="title"><a>only title</a></td>
    <td class="infohash">ABCDEF1234567890ABCDEF1234567890ABCDEF12</td>
  </tr>
</table></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	def := sampleDefinition()
	def.Result.Fields = map[string]model.FieldDefinition{
		"title":     {Selector: "td.title a"},
		"info_hash": {Selector: "td.infohash", Filters: []string{"trim", "extract_info_hash"}},
	}
	inst := model.InstalledIndexer{ID: "x", BaseURL: srv.URL}
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "k"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
	if !strings.HasPrefix(res[0].MagnetURL, "magnet:?xt=urn:btih:") {
		t.Errorf("info_hash should imply magnet, got %q", res[0].MagnetURL)
	}
}

// TestDeclarativeAdapter_RequiredFilterFail exercises the "required
// field failed filter" branch in extractRow.
func TestDeclarativeAdapter_RequiredFilterFail(t *testing.T) {
	html := `<html><body>
<table>
  <tr class="row">
    <td class="title">x</td>
    <td class="seeders">not-a-number</td>
  </tr>
</table></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()
	def := sampleDefinition()
	def.Result.Fields = map[string]model.FieldDefinition{
		"title":    {Selector: "td.title"},
		"seeders":  {Selector: "td.seeders", Filters: []string{"parse_int"}, Required: true},
	}
	inst := model.InstalledIndexer{ID: "x", BaseURL: srv.URL}
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "k"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("required filter fail should reject row, got %d results", len(res))
	}
}

// TestDeclarativeFactory_Create exercises the declarative factory.
func TestDeclarativeFactory_Create(t *testing.T) {
	f := declarativeFactory{}
	def := model.IndexerDefinition{
		ID: "x", Protocol: "declarative",
		Search:  model.SearchDefinition{Path: "/"},
		Result:  model.ResultDefinition{Format: "html"},
	}
	a, err := f.Create(def, model.InstalledIndexer{ID: "i", BaseURL: "https://e.x"}, NewClient())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID() != "i" {
		t.Errorf("ID want i got %q", a.ID())
	}
}

// TestDeclarativeAdapter_ReadField_Attr covers the attr branch of
// readField.
func TestDeclarativeAdapter_ReadField_Attr(t *testing.T) {
	doc := mustGoquery(t, `<html><body><div class="row"><a href="/dl/x" class="link">x</a></div></body></html>`)
	row := doc.Find("div.row").First()
	got := readField(row, model.FieldDefinition{Selector: "a.link", Attribute: "href", Value: "attr"})
	if got != "/dl/x" {
		t.Errorf("attr read want /dl/x got %q", got)
	}
	got = readField(row, model.FieldDefinition{Selector: "a.link", Value: "text"})
	if got != "x" {
		t.Errorf("text read want x got %q", got)
	}
	got = readField(row, model.FieldDefinition{Selector: "a.link"})
	if got == "" {
		t.Errorf("default html read should not be empty")
	}
	got = readField(row, model.FieldDefinition{Selector: "missing"})
	if got != "" {
		t.Errorf("missing selector should return empty, got %q", got)
	}
	got = readField(row, model.FieldDefinition{Selector: ""})
	if got != "" {
		t.Errorf("empty selector should return empty, got %q", got)
	}
}