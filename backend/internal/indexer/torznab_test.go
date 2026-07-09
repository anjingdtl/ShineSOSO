package indexer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/local/easysearch/backend/internal/model"
	"github.com/local/easysearch/backend/internal/security"
)

// torznabFixtureXML is a tiny but representative Torznab feed.
const torznabFixtureXML = `<?xml version="1.0" encoding="UTF-8"?>
<rss xmlns:torznab="http://torznab.com/schemas/2015/feed">
  <channel>
    <title>Test Indexer</title>
    <item>
      <title>Ubuntu 24.04 Desktop</title>
      <guid>ubuntu-2404-desktop</guid>
      <link>https://example.com/details/ubuntu-2404</link>
      <pubDate>Thu, 09 Jul 2026 12:00:00 +0000</pubDate>
      <category>5000</category>
      <enclosure url="https://example.com/dl/ubuntu.torrent" length="0" type="application/x-bittorrent" />
      <torznab:attr name="size" value="5368709120" />
      <torznab:attr name="seeders" value="42" />
      <torznab:attr name="peers" value="50" />
      <torznab:attr name="infohash" value="0123456789abcdef0123456789abcdef01234567" />
    </item>
    <item>
      <title>Big Buck Bunny 1080p</title>
      <guid>bbb-1080p</guid>
      <link>https://example.com/details/bbb</link>
      <pubDate>Wed, 08 Jul 2026 11:00:00 +0000</pubDate>
      <category>2000</category>
      <enclosure url="https://example.com/dl/bbb.torrent" length="0" type="application/x-bittorrent" />
      <torznab:attr name="size" value="2147483648" />
      <torznab:attr name="seeders" value="120" />
      <torznab:attr name="peers" value="125" />
      <torznab:attr name="magneturl" value="magnet:?xt=urn:btih:DEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF&amp;dn=bbb" />
    </item>
    <item>
      <title>Sample Audiobook</title>
      <guid>audiobook-1</guid>
      <link>https://example.com/details/audiobook</link>
      <pubDate>Tue, 07 Jul 2026 09:00:00 +0000</pubDate>
      <category>3000</category>
      <torznab:attr name="size" value="524288000" />
      <torznab:attr name="seeders" value="3" />
      <torznab:attr name="infohash" value="AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA1AAAAAA" />
    </item>
  </channel>
</rss>`

// newTestTorznabServer returns an httptest.Server returning the fixture
// XML for any path. It also captures the request URL so callers can
// assert that the adapter built the right query string.
func newTestTorznabServer(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	captured := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.String()
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(torznabFixtureXML))
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

// testClient returns a hardened HTTP client that accepts plain HTTP and
// loopback (so the in-process test server is reachable). Tests of real
// code paths still pass through SSRF for non-loopback, non-HTTP cases.
func testClient() *Client {
	return &Client{
		Timeout: 5 * time.Second,
		Policy:  security.DefaultValidator{AllowHTTP: true, AllowLoopback: true},
		HTTP: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func TestTorznabFactory_Create_andAdapterID(t *testing.T) {
	def := model.IndexerDefinition{Protocol: "torznab", Result: model.ResultDefinition{Format: "torznab"}}
	inst := model.InstalledIndexer{ID: "x1", BaseURL: "http://localhost:0"}
	a, err := NewTorznabFactory().Create(def, inst, testClient())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID() != "x1" {
		t.Errorf("adapter ID mismatch: %s", a.ID())
	}
}

func TestTorznabFactory_RejectsBadProtocol(t *testing.T) {
	def := model.IndexerDefinition{Protocol: "declarative"}
	_, err := NewTorznabFactory().Create(def, model.InstalledIndexer{BaseURL: "http://localhost:0"}, testClient())
	if err == nil {
		t.Fatal("protocol mismatch should fail")
	}
}

func TestTorznab_buildURL_appliesSpec14Defaults(t *testing.T) {
	def := model.IndexerDefinition{Protocol: "torznab"}
	inst := model.InstalledIndexer{BaseURL: "https://example.com"}
	a := &torznabAdapter{def: def, installed: inst}
	u, err := a.buildURL("matrix", "", "2000", "1")
	if err != nil {
		t.Fatalf("buildURL: %v", err)
	}
	if u.Path != "/api" {
		t.Errorf("path=%q want /api", u.Path)
	}
	q := u.Query()
	if q.Get("t") != "search" {
		t.Errorf("t=%q want search", q.Get("t"))
	}
	if q.Get("q") != "matrix" {
		t.Errorf("q=%q want matrix", q.Get("q"))
	}
	if q.Get("cat") != "2000" {
		t.Errorf("cat=%q want 2000", q.Get("cat"))
	}
}

func TestTorznab_buildURL_appliesUserOverrides(t *testing.T) {
	def := model.IndexerDefinition{
		Protocol: "torznab",
		Search: model.SearchDefinition{
			Path: "/torznab/api",
			Query: map[string]string{
				"t":       "tvsearch",
				"q":       "{{ .Query.Keyword }}",
				"cat":     "{{ .Query.CategoryID }}",
				"apikey":  "redacted",
			},
		},
	}
	inst := model.InstalledIndexer{BaseURL: "https://example.com"}
	a := &torznabAdapter{def: def, installed: inst}
	u, err := a.buildURL("matrix", "", "5000", "1")
	if err != nil {
		t.Fatalf("buildURL: %v", err)
	}
	if u.Path != "/torznab/api" {
		t.Errorf("path=%q want /torznab/api", u.Path)
	}
	q := u.Query()
	if q.Get("t") != "tvsearch" {
		t.Errorf("t=%q want tvsearch", q.Get("t"))
	}
	if q.Get("q") != "matrix" {
		t.Errorf("q=%q want matrix", q.Get("q"))
	}
	if q.Get("cat") != "5000" {
		t.Errorf("cat=%q want 5000", q.Get("cat"))
	}
	if q.Get("apikey") != "redacted" {
		t.Errorf("apikey not preserved")
	}
}

func TestTorznab_Search_returnsParsedResults(t *testing.T) {
	srv, captured := newTestTorznabServer(t)
	def := model.IndexerDefinition{Protocol: "torznab"}
	a, err := NewTorznabFactory().Create(def, model.InstalledIndexer{
		ID: "test-torznab",
		Name: "Test Torznab",
		BaseURL: srv.URL,
	}, testClient())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	results, err := a.Search(context.Background(), model.SearchQuery{Keyword: "ubuntu"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	if !strings.Contains(*captured, "t=search") || !strings.Contains(*captured, "q=ubuntu") {
		t.Errorf("URL=%q missing t=search&q=ubuntu", *captured)
	}

	// Item 1: infohash provided, no magneturl -> magnet auto-constructed.
	r1 := results[0]
	if r1.Title != "Ubuntu 24.04 Desktop" {
		t.Errorf("title=%q", r1.Title)
	}
	if r1.MagnetURL == "" || !strings.Contains(r1.MagnetURL, "urn:btih:") {
		t.Errorf("r1 magnet missing or wrong: %q", r1.MagnetURL)
	}
	if r1.InfoHash != "0123456789ABCDEF0123456789ABCDEF01234567" {
		t.Errorf("r1 infohash=%q", r1.InfoHash)
	}
	if r1.SizeBytes == nil || *r1.SizeBytes != 5368709120 {
		t.Errorf("r1 size=%v", r1.SizeBytes)
	}
	if r1.Seeders == nil || *r1.Seeders != 42 {
		t.Errorf("r1 seeders=%v", r1.Seeders)
	}
	if r1.PublishedAt == nil {
		t.Errorf("r1 publishedAt missing")
	}

	// Item 2: explicit magneturl present.
	r2 := results[1]
	if !strings.Contains(r2.MagnetURL, "DEADBEEF") {
		t.Errorf("r2 magnet=%q", r2.MagnetURL)
	}

	// Item 3: no enclosure, no magnet -> falls through to detail URL only.
	r3 := results[2]
	if r3.MagnetURL == "" {
		t.Errorf("r3 should still have magnet from infohash")
	}
	if r3.DetailURL == "" {
		t.Errorf("r3 detail URL missing")
	}
}

func TestTorznab_Test_returnsOKAgainstFixture(t *testing.T) {
	srv, _ := newTestTorznabServer(t)
	a, err := NewTorznabFactory().Create(model.IndexerDefinition{Protocol: "torznab"},
		model.InstalledIndexer{ID: "x", BaseURL: srv.URL}, testClient())
	if err != nil {
		t.Fatal(err)
	}
	res := a.Test(context.Background())
	if !res.OK {
		t.Fatalf("Test not OK: %+v", res)
	}
	if res.StatusCode != 200 {
		t.Errorf("status=%d", res.StatusCode)
	}
}

func TestTorznab_parseTorznabDate_handlesCommonFormats(t *testing.T) {
	cases := []string{
		"Thu, 09 Jul 2026 12:00:00 +0000",
		"Wed, 08 Jul 2026 11:00:00 GMT",
		"Tue, 07 Jul 2026 09:00:00 -0700",
	}
	for _, in := range cases {
		got, err := parseTorznabDate(in)
		if err != nil {
			t.Errorf("parseTorznabDate(%q): %v", in, err)
			continue
		}
		if got.Year() != 2026 {
			t.Errorf("parseTorznabDate(%q) year=%d", in, got.Year())
		}
	}
}