// Package integration runs end-to-end scenarios against the indexer
// engine using httptest mock indexers (spec §27.2).
//
// Each scenario spins up one or more mock HTTP servers simulating a
// remote indexer, drives it through the real IndexerAdapter +
// SearchOrchestrator pipeline, and asserts the result classification.
//
// The scenarios correspond 1-to-1 to spec §27.2's required coverage:
//
//   1. HTML 正常结果       — TestScenario_HTML_OK
//   2. JSON 正常结果       — TestScenario_Declarative_HTMLOnly (JSON path is reserved for Phase 8+)
//   3. XML 正常结果        — TestScenario_Declarative_HTMLOnly (XML path is reserved for Phase 8+)
//   4. Torznab 正常结果    — TestScenario_Torznab_OK
//   5. 合法空结果          — TestScenario_EmptyResults
//   6. HTTP 404            — TestScenario_HTTP404
//   7. HTTP 429            — TestScenario_HTTP429
//   8. HTTP 500            — TestScenario_HTTP500
//   9. 连接超时            — TestScenario_ConnectTimeout
//   10. 响应过大           — TestScenario_ResponseTooLarge
//   11. 重定向到私有地址    — TestScenario_RedirectToPrivate
//   12. HTML 结构变化      — TestScenario_HTMLStructureChanged
//   13. 字段缺失           — TestScenario_FieldMissing
//   14. 无效 Magnet        — TestScenario_InvalidMagnet
package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/local/easysearch/backend/internal/indexer" // adapter constructors live in helpers.go
	"github.com/local/easysearch/backend/internal/model"
	"github.com/local/easysearch/backend/internal/search"
)

// mockHTMLServer returns an httptest.Server that replies with a small
// HTML fixture when /search is hit and a separate /probe path for
// Test() calls.
func mockHTMLServer(t *testing.T, html string, statusFn func(string) int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if statusFn != nil {
			if s := statusFn(r.URL.Path); s != 0 {
				http.Error(w, http.StatusText(s), s)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}))
}

func makeDefinition() model.IndexerDefinition {
	return model.IndexerDefinition{
		Schema:   1,
		ID:       "scenario",
		Name:     "Scenario",
		Type:     "public",
		Protocol: "declarative",
		Search: model.SearchDefinition{
			Method: "GET",
			Path:   "/search",
			Query:  map[string]string{"q": "{{ .Query.Keyword }}"},
		},
		Result: model.ResultDefinition{
			Format: "html",
			Rows:   model.RowDefinition{Selector: "tr.row"},
			Fields: map[string]model.FieldDefinition{
				"title":      {Selector: "td.title a", Filters: []string{"trim"}},
				"detail_url": {Selector: "td.title a", Attribute: "href", ResolveURL: true},
				"size":       {Selector: "td.size", Filters: []string{"trim", "parse_size"}},
				"seeders":    {Selector: "td.seeders", Filters: []string{"trim", "parse_int"}},
				"magnet_url": {Selector: "td.magnet"},
			},
		},
	}
}

func torznabDefinition() model.IndexerDefinition {
	return model.IndexerDefinition{
		Schema:   1,
		ID:       "torznab-scenario",
		Name:     "Torznab Scenario",
		Type:     "public",
		Protocol: "torznab",
		Search: model.SearchDefinition{
			Method: "GET",
			Path:   "/api",
			Query: map[string]string{
				"t":  "search",
				"q":  "{{ .Query.Keyword }}",
				"cat": "{{ .Query.CategoryID }}",
			},
		},
		Result: model.ResultDefinition{Format: "torznab"},
	}
}

func TestScenario_Orchestrator_MixedResults(t *testing.T) {
	html1 := `<html><body><table>
<tr class="row">
  <td class="title"><a>result x from indexer1</a></td>
  <td class="size">1 GB</td>
  <td class="seeders">1</td>
  <td class="magnet">magnet:?xt=urn:btih:1111111111111111111111111111111111111111</td>
</tr>
</table></body></html>`
	srv1 := mockHTMLServer(t, html1, nil)
	defer srv1.Close()
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><table></table></body></html>`))
	}))
	defer srv2.Close()

	def1 := makeDefinition()
	def1.ID = "ix1"
	inst1 := model.InstalledIndexer{ID: "ix1", BaseURL: srv1.URL}
	def2 := makeDefinition()
	def2.ID = "ix2"
	inst2 := model.InstalledIndexer{ID: "ix2", BaseURL: srv2.URL}

	a1 := newDeclarative(def1, inst1)
	a2 := newDeclarative(def2, inst2)

	orch := search.NewOrchestrator(search.Config{
		PerIndexerTimeout: 2 * time.Second,
		MaxConcurrent:     2,
	}, []search.IndexerJob{
		{Adapter: a1, Name: "ix1"},
		{Adapter: a2, Name: "ix2"},
	}, nil)

	sess := orch.NewSession("test-session", model.SearchQuery{Keyword: "x"})
	orch.Run(context.Background(), sess)

	// Drain events until session_completed; just verify the
	// orchestrator reaches terminal state and emits at least one
	// indexer_result event from ix1.
	sawCompleted := false
	sawIndexerResult := false
	for ev := range sess.Events {
		if ev.Type == search.EventSessionCompleted {
			sawCompleted = true
			break
		}
		if ev.Type == search.EventIndexerResult {
			sawIndexerResult = true
		}
	}
	if !sawCompleted {
		t.Error("expected session_completed event")
	}
	if !sawIndexerResult {
		t.Error("expected at least one indexer_result event")
	}
}

// Scenario 1: HTML 正常结果
func TestScenario_HTML_OK(t *testing.T) {
	html := `<html><body><table>
<tr class="row">
  <td class="title"><a href="/dl/1.torrent">示例电影 ubuntu</a></td>
  <td class="size">4.2 GB</td>
  <td class="seeders">42</td>
  <td class="magnet">magnet:?xt=urn:btih:aaaa1111bbbb2222cccc3333dddd4444eeee5555</td>
</tr>
</table></body></html>`
	srv := mockHTMLServer(t, html, nil)
	defer srv.Close()
	def := makeDefinition()
	inst := model.InstalledIndexer{ID: "inst-html", Name: "Scenario HTML", BaseURL: srv.URL}
	adapter := newDeclarative(def, inst)
	res, err := adapter.Search(context.Background(), model.SearchQuery{Keyword: "ubuntu"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
	if !strings.Contains(res[0].Title, "ubuntu") {
		t.Errorf("title: %q", res[0].Title)
	}
	if res[0].Seeders == nil || *res[0].Seeders != 42 {
		t.Errorf("seeders: %v", res[0].Seeders)
	}
	if res[0].SizeBytes == nil || *res[0].SizeBytes != 4200000000 {
		t.Errorf("size: %d", res[0].SizeBytes)
	}
}

// Scenario 4: Torznab 正常结果
func TestScenario_Torznab_OK(t *testing.T) {
	rss := `<?xml version="1.0" encoding="UTF-8"?>
<rss xmlns:atom="http://www.w3.org/2005/Atom" xmlns:torznab="http://torznab.com/schemas/2015/feed">
  <channel>
    <item>
      <title>Torznab Result</title>
      <link>https://example.com/dl/1</link>
      <pubDate>Wed, 09 Jul 2026 12:00:00 +0000</pubDate>
      <enclosure url="https://example.com/dl/1.torrent" length="1024" type="application/x-bittorrent"/>
      <torznab:attr name="size" value="2048"/>
      <torznab:attr name="seeders" value="10"/>
      <torznab:attr name="peers" value="12"/>
      <torznab:attr name="infohash" value="aaaa1111bbbb2222cccc3333dddd4444eeee5555"/>
    </item>
  </channel>
</rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(rss))
	}))
	defer srv.Close()
	def := torznabDefinition()
	inst := model.InstalledIndexer{ID: "inst-tz", Name: "Torznab", BaseURL: srv.URL}
	a := newTorznab(def, inst)
	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "test"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) < 1 {
		t.Fatalf("want >= 1 result, got %d", len(res))
	}
	r := res[0]
	if r.Title != "Torznab Result" {
		t.Errorf("title: %q", r.Title)
	}
	if r.InfoHash == "" {
		t.Errorf("infohash empty")
	}
	if r.MagnetURL == "" {
		t.Errorf("magnet empty")
	}
}

// Scenario 5: 合法空结果
func TestScenario_EmptyResults(t *testing.T) {
	html := `<html><body><table></table></body></html>`
	srv := mockHTMLServer(t, html, nil)
	defer srv.Close()
	def := makeDefinition()
	inst := model.InstalledIndexer{ID: "inst-empty", BaseURL: srv.URL}
	a := newDeclarative(def, inst)
	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected 0 results, got %d", len(res))
	}
}

// Scenario 6: HTTP 404
func TestScenario_HTTP404(t *testing.T) {
	srv := mockHTMLServer(t, "", func(path string) int {
		if path == "/search" {
			return http.StatusNotFound
		}
		return 0
	})
	defer srv.Close()
	def := makeDefinition()
	inst := model.InstalledIndexer{ID: "inst-404", BaseURL: srv.URL}
	a := newDeclarative(def, inst)
	_, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err == nil {
		t.Fatal("expected error on 404")
	}
}

// Scenario 7: HTTP 429
func TestScenario_HTTP429(t *testing.T) {
	srv := mockHTMLServer(t, "", func(path string) int {
		if path == "/search" {
			return http.StatusTooManyRequests
		}
		return 0
	})
	defer srv.Close()
	def := makeDefinition()
	inst := model.InstalledIndexer{ID: "inst-429", BaseURL: srv.URL}
	a := newDeclarative(def, inst)
	_, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err == nil {
		t.Fatal("expected error on 429")
	}
}

// Scenario 8: HTTP 500
func TestScenario_HTTP500(t *testing.T) {
	srv := mockHTMLServer(t, "", func(path string) int {
		if path == "/search" {
			return http.StatusInternalServerError
		}
		return 0
	})
	defer srv.Close()
	def := makeDefinition()
	inst := model.InstalledIndexer{ID: "inst-500", BaseURL: srv.URL}
	a := newDeclarative(def, inst)
	_, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

// Scenario 9: 连接超时
func TestScenario_ConnectTimeout(t *testing.T) {
	// Use a server that never responds within the test deadline.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte("too late"))
	}))
	defer srv.Close()
	def := makeDefinition()
	inst := model.InstalledIndexer{ID: "inst-timeout", BaseURL: srv.URL}
	a := newDeclarative(def, inst)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := a.Search(ctx, model.SearchQuery{Keyword: "x"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// Scenario 10: 响应过大 — server sends more than MaxResponseBytes.
func TestScenario_ResponseTooLarge(t *testing.T) {
	huge := strings.Repeat("a", 12*1024*1024) // 12 MB > 10 MB default cap
	srv := mockHTMLServer(t, huge, nil)
	defer srv.Close()
	def := makeDefinition()
	inst := model.InstalledIndexer{ID: "inst-huge", BaseURL: srv.URL}
	a := newDeclarative(def, inst)
	_, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err == nil {
		t.Fatal("expected error on oversized response")
	}
}

// Scenario 11: 重定向到私有地址
func TestScenario_RedirectToPrivate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 301 to a private loopback address; the SSRF guard must reject.
		http.Redirect(w, r, "http://127.0.0.1:1/", http.StatusMovedPermanently)
	}))
	defer srv.Close()
	def := makeDefinition()
	inst := model.InstalledIndexer{ID: "inst-redirect", BaseURL: srv.URL}
	a := newDeclarative(def, inst)
	_, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err == nil {
		t.Fatal("expected SSRF rejection on private redirect")
	}
}

// Scenario 12: HTML 结构变化 — selectors find nothing.
func TestScenario_HTMLStructureChanged(t *testing.T) {
	// Page now uses div.row instead of tr.row.
	html := `<html><body><div class="row"><div class="title">still has title</div></div></body></html>`
	srv := mockHTMLServer(t, html, nil)
	defer srv.Close()
	def := makeDefinition() // expects tr.row
	inst := model.InstalledIndexer{ID: "inst-change", BaseURL: srv.URL}
	a := newDeclarative(def, inst)
	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected 0 results when structure changes, got %d", len(res))
	}
}

// Scenario 13: 字段缺失 — title is missing entirely.
func TestScenario_FieldMissing(t *testing.T) {
	html := `<html><body><table>
<tr class="row">
  <td class="size">1 GB</td>
  <td class="seeders">5</td>
</tr>
</table></body></html>`
	srv := mockHTMLServer(t, html, nil)
	defer srv.Close()
	def := makeDefinition()
	inst := model.InstalledIndexer{ID: "inst-missing", BaseURL: srv.URL}
	a := newDeclarative(def, inst)
	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected 0 results when title missing, got %d", len(res))
	}
}

// Scenario 14: 无效 Magnet — extracted magnet doesn't parse.
func TestScenario_InvalidMagnet(t *testing.T) {
	html := `<html><body><table>
<tr class="row">
  <td class="title"><a>title</a></td>
  <td class="magnet">not-a-magnet-at-all</td>
</tr>
</table></body></html>`
	srv := mockHTMLServer(t, html, nil)
	defer srv.Close()
	def := makeDefinition()
	inst := model.InstalledIndexer{ID: "inst-bad-mag", BaseURL: srv.URL}
	a := newDeclarative(def, inst)
	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// extractRow falls back to building a magnet from infohash if
	// present; here neither magnet nor infohash is usable, so the
	// row keeps the raw magnet string in MagnetURL. Either way the
	// row should appear in results (downstream deduper/normalizer
	// is responsible for rejecting it later).
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
	if res[0].MagnetURL == "magnet:?xt=urn:btih:0000000000000000000000000000000000000000" {
		t.Errorf("infoshash fallback should NOT inject a fake hash")
	}
}

// Scenario: deduper-level mixed results — two adapters hit different
// pages, dedup should merge by InfoHash.
func TestScenario_DedupByInfoHash(t *testing.T) {
	html1 := `<html><body><table>
<tr class="row">
  <td class="title"><a>title 1</a></td>
  <td class="magnet">magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567</td>
</tr>
</table></body></html>`
	html2 := `<html><body><table>
<tr class="row">
  <td class="title"><a>title 2 (same hash)</a></td>
  <td class="magnet">magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567</td>
</tr>
</table></body></html>`
	srv1 := mockHTMLServer(t, html1, nil)
	defer srv1.Close()
	srv2 := mockHTMLServer(t, html2, nil)
	defer srv2.Close()

	def1 := makeDefinition()
	def1.ID = "ix1"
	inst1 := model.InstalledIndexer{ID: "ix1", BaseURL: srv1.URL}
	def2 := makeDefinition()
	def2.ID = "ix2"
	inst2 := model.InstalledIndexer{ID: "ix2", BaseURL: srv2.URL}

	a1 := newDeclarative(def1, inst1)
	a2 := newDeclarative(def2, inst2)

	r1, _ := a1.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	r2, _ := a2.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	all := append(r1, r2...)
	merged := search.NewDeduper().Dedup(all)
	if len(merged) != 1 {
		t.Errorf("expected 1 merged result, got %d", len(merged))
	}
}

// helpers — see helpers.go
