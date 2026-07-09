package indexer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/local/easysearch/backend/internal/model"
)

// TestMain registers an HTTP client that allows loopback + http so
// every test in this package can talk to httptest servers on
// 127.0.0.1 without per-test setup.
func TestMain(m *testing.M) {
	c := NewClient()
	c.Policy.AllowHTTP = true
	c.Policy.AllowLoopback = true
	setClient(c)
	m.Run()
	setClient(nil)
}

// declarativeFixtureHTML is a small but complete search-results HTML
// page used by the declarative adapter tests.
const declarativeFixtureHTML = `<html><body>
<table>
  <tr class="row">
    <td class="title"><a href="/dl/1.torrent">示例电影 1</a></td>
    <td class="size">8.4 GB</td>
    <td class="seeders">326</td>
    <td class="date">2026-07-09</td>
    <td class="magnet">magnet:?xt=urn:btih:abcdef1234567890abcdef1234567890abcdef12</td>
    <td class="infohash">0123456789abcdef0123456789abcdef01234567</td>
  </tr>
  <tr class="row">
    <td class="title"><a href="/dl/2.torrent">示例剧集 2</a></td>
    <td class="size">1.2 GiB</td>
    <td class="seeders">12</td>
    <td class="date">2026-06-01</td>
    <td class="magnet">magnet:?xt=urn:btih:1111222233334444555566667777888899990000</td>
    <td class="infohash">1111222233334444555566667777888899990000</td>
  </tr>
</table>
</body></html>`

func sampleDefinition() model.IndexerDefinition {
	return model.IndexerDefinition{
		Schema:   1,
		ID:       "fixture",
		Name:     "Fixture",
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
				"title":        {Selector: "td.title a", Filters: []string{"trim"}},
				"detail_url":   {Selector: "td.title a", Attribute: "href", ResolveURL: true},
				"size":         {Selector: "td.size", Filters: []string{"trim", "parse_size"}},
				"seeders":      {Selector: "td.seeders", Filters: []string{"trim", "parse_int"}},
				"published_at": {Selector: "td.date", Filters: []string{"trim", "parse_date"}, DateLayouts: []string{"2006-01-02"}},
				"magnet_url":   {Selector: "td.magnet"},
				"info_hash":    {Selector: "td.infohash", Filters: []string{"trim", "extract_info_hash"}},
			},
		},
	}
}

func withMockServer(t *testing.T, html string) (*httptest.Server, model.InstalledIndexer, model.IndexerDefinition) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/probe") {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<html><body>ok</body></html>"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(html))
	}))
	// Register an HTTP client that allows loopback + http so tests can
	// talk to httptest servers on 127.0.0.1 (which is plain HTTP).
	// TestMain already sets a default; this is a no-op for now.
	def := sampleDefinition()
	inst := model.InstalledIndexer{
		ID:       "installed-1",
		Name:     "Test Installed",
		BaseURL:  srv.URL,
		Enabled:  true,
	}
	return srv, inst, def
}

func TestDeclarativeAdapter_Search_ParsesHTML(t *testing.T) {
	srv, inst, def := withMockServer(t, declarativeFixtureHTML)
	defer srv.Close()

	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	results, err := a.Search(context.Background(), model.SearchQuery{Keyword: "matrix"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	// First row: title, size, seeders, date, magnet, infohash
	r0 := results[0]
	if !strings.Contains(r0.Title, "示例电影 1") {
		t.Errorf("title: got %q", r0.Title)
	}
	if r0.SizeBytes == nil || *r0.SizeBytes == 0 {
		t.Errorf("size: got %v", r0.SizeBytes)
	}
	if r0.Seeders == nil || *r0.Seeders != 326 {
		t.Errorf("seeders: got %v", r0.Seeders)
	}
	if !strings.HasPrefix(r0.MagnetURL, "magnet:?xt=urn:btih:") {
		t.Errorf("magnet: got %q", r0.MagnetURL)
	}
	if r0.IndexerID != "installed-1" {
		t.Errorf("indexerID: got %q", r0.IndexerID)
	}
	if r0.IndexerName != "Test Installed" {
		t.Errorf("indexerName: got %q", r0.IndexerName)
	}
}

func TestDeclarativeAdapter_Search_FailsOnUnsupportedFormat(t *testing.T) {
	_, inst, def := withMockServer(t, declarativeFixtureHTML)
	def.Result.Format = "json"
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	_, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err == nil {
		t.Error("expected ErrFormatUnsupported for json")
	}
}

func TestDeclarativeAdapter_Test_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal", http.StatusInternalServerError)
	}))
	defer srv.Close()
	def := sampleDefinition()
	inst := model.InstalledIndexer{ID: "x", BaseURL: srv.URL}
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	res := a.Test(context.Background())
	if res.OK {
		t.Errorf("expected Test to fail on 500, got %+v", res)
	}
	if res.StatusCode != 500 {
		t.Errorf("expected StatusCode 500, got %d", res.StatusCode)
	}
}

func TestDeclarativeAdapter_Test_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 200 but empty body
		_, _ = w.Write([]byte(""))
	}))
	defer srv.Close()
	def := sampleDefinition()
	inst := model.InstalledIndexer{ID: "x", BaseURL: srv.URL}
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	res := a.Test(context.Background())
	if res.OK {
		t.Errorf("expected Test to fail on empty body, got %+v", res)
	}
	if res.ErrorCode != "EMPTY" {
		t.Errorf("expected ErrorCode EMPTY, got %q", res.ErrorCode)
	}
}

func TestDeclarativeAdapter_Test_OK(t *testing.T) {
	srv, inst, def := withMockServer(t, declarativeFixtureHTML)
	defer srv.Close()
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	res := a.Test(context.Background())
	if !res.OK {
		t.Errorf("expected OK, got %+v", res)
	}
}

func TestDeclarativeAdapter_BuildURL_UnsupportedMethod(t *testing.T) {
	_, inst, def := withMockServer(t, declarativeFixtureHTML)
	def.Search.Method = "DELETE"
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	if _, err := a.buildURL("x", "", "", "1"); err == nil {
		t.Error("expected error for unsupported method")
	}
}

func TestDeclarativeAdapter_BuildURL_BadBaseURL(t *testing.T) {
	_, inst, def := withMockServer(t, declarativeFixtureHTML)
	inst.BaseURL = "://not a url"
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	if _, err := a.buildURL("x", "", "", "1"); err == nil {
		t.Error("expected error for bad base URL")
	}
}

func TestDeclarativeAdapter_BuildURL_BadPath(t *testing.T) {
	_, inst, def := withMockServer(t, declarativeFixtureHTML)
	def.Search.Path = "://not a path"
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	if _, err := a.buildURL("x", "", "", "1"); err == nil {
		t.Error("expected error for bad path")
	}
}

func TestDeclarativeAdapter_BuildURL_BadTemplate(t *testing.T) {
	_, inst, def := withMockServer(t, declarativeFixtureHTML)
	def.Search.Query = map[string]string{"q": "{{ .Env.HOME }}"}
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	if _, err := a.buildURL("x", "", "", "1"); err == nil {
		t.Error("expected error for forbidden template var")
	}
}

func TestDeclarativeAdapter_BuildURL_POST_NoQuery(t *testing.T) {
	_, inst, def := withMockServer(t, declarativeFixtureHTML)
	def.Search.Method = "POST"
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	u, err := a.buildURL("x", "", "", "1")
	if err != nil {
		t.Fatalf("buildURL: %v", err)
	}
	if u.RawQuery != "" {
		t.Errorf("POST should not have RawQuery, got %q", u.RawQuery)
	}
}

func TestDeclarativeAdapter_Fetch_NoClient(t *testing.T) {
	_, inst, def := withMockServer(t, declarativeFixtureHTML)
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	// Temporarily clear the global client (TestMain restores it after m.Run).
	prev := currentClient()
	setClient(nil)
	defer setClient(prev)
	if _, err := a.fetch(context.Background(), "http://example.com"); err == nil {
		t.Error("expected error when client is nil")
	}
}

func TestDeclarativeAdapter_Fetch_RespectsTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	_, inst, def := withMockServer(t, declarativeFixtureHTML)
	inst.BaseURL = srv.URL
	// Force a sub-second per-request timeout; the server sleeps 2s so the
	// client must abort and return an error.
	def.Search.TimeoutSeconds = 1
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err := a.fetch(ctx, srv.URL)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestDeclarativeAdapter_Search_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, inst, def := withMockServer(t, declarativeFixtureHTML)
	inst.BaseURL = srv.URL
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	_, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
	_, ok := err.(*StatusCodeError)
	if !ok {
		t.Errorf("expected StatusCodeError, got %T: %v", err, err)
	}
}

func TestDeclarativeAdapter_ExtractRow_RequiredMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// empty body but 200 — extractRow should reject if required fields missing
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer srv.Close()
	_, inst, def := withMockServer(t, declarativeFixtureHTML)
	def.Result.Fields["title"] = model.FieldDefinition{Selector: "td.title a", Required: true}
	inst.BaseURL = srv.URL
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	results, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results when required field missing, got %d", len(results))
	}
}

func TestDeclarativeAdapter_Search_NoRowSelector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Without a row selector the body itself is treated as a single
		// row. Title selector looks for td.title a — put one inside body
		// so extractRow finds the title.
		_, _ = w.Write([]byte(`<html><body><table><tr><td class="title"><a href="/dl/1.torrent">only title</a></td></tr></table></body></html>`))
	}))
	defer srv.Close()
	_, inst, def := withMockServer(t, declarativeFixtureHTML)
	def.Result.Rows.Selector = ""
	inst.BaseURL = srv.URL
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}
	results, err := a.Search(context.Background(), model.SearchQuery{Keyword: "x"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// With no row selector we treat the body as a single row.
	if len(results) != 1 {
		t.Errorf("expected 1 result for body-as-row, got %d", len(results))
	}
}