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

// TestDeclarativeXML_Search verifies the XML declarative adapter walks
// `rss.channel.item[*]` to find rows and assigns typed leaf values to
// the matching SearchResult fields. Same path contract as the JSON
// test, with `.` separated element names and "[*]" for the row level.
//
// Adapted to model.ResultDefinition / model.FieldDefinition + the
// existing declarative schema (no model.DeclarativeResponse additions).
func TestDeclarativeXML_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<rss>
  <channel>
    <item>
      <title>Bravo</title>
      <size>2048</size>
      <seeders>5</seeders>
      <infohash>bbcdef0123456789abcdef0123456789abcdef01</infohash>
      <download>magnet:?xt=urn:btih:bbcdef0123456789abcdef0123456789abcdef01</download>
    </item>
  </channel>
</rss>`))
	}))
	defer srv.Close()

	def := model.IndexerDefinition{
		ID:   "demo-xml",
		Name: "Demo XML",
		Type: "public",
		Result: model.ResultDefinition{
			Format: "xml",
			Fields: map[string]model.FieldDefinition{
				"row":      {Selector: "rss.channel.item[*]"},
				"title":    {Selector: "title", Value: "string"},
				"size":     {Selector: "size", Value: "size"},
				"seeders":  {Selector: "seeders", Value: "int"},
				"infohash": {Selector: "infohash", Value: "infohash"},
				"magnet":   {Selector: "download", Value: "magnet_url"},
			},
		},
	}

	inst := model.InstalledIndexer{ID: def.ID, Name: def.Name, BaseURL: srv.URL}
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}

	// Tests like TestDeclarativeFactory_Create mutate the package-level
	// client via NewClient() (policy rejects http); install one that
	// allows loopback + http for the httptest server.
	c := &Client{
		Timeout: 5 * time.Second,
		Policy:  security.DefaultValidator{AllowHTTP: true, AllowLoopback: true},
		HTTP:    &http.Client{Timeout: 5 * time.Second},
	}
	prev := currentClient()
	setClient(c)
	defer setClient(prev)

	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "bravo"})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
	if !strings.Contains(res[0].MagnetURL, "btih:bbcdef0123456789abcdef0123456789abcdef01") {
		t.Fatalf("missing magnet: %+v", res[0])
	}
	if res[0].Seeders == nil || *res[0].Seeders != 5 {
		t.Fatalf("seeders mismatch: %+v", res[0].Seeders)
	}
	if res[0].Title != "Bravo" {
		t.Fatalf("title mismatch: %q", res[0].Title)
	}
}
