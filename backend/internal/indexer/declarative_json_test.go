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

// TestDeclarativeJSON_Search verifies that the JSON declarative adapter
// walks `results[*]` to find rows, reads bare field names against each
// row, and assigns typed values (string/size/int/infohash/magnet_url)
// to the matching SearchResult fields.
//
// Note: this test follows the brief's path-syntax contract
// (rows[*] / bare names + a `row` anchor field whose path is the row
// selector) and uses model.ResultDefinition / model.FieldDefinition,
// which is the existing declarative schema. The mapping from
// DeclarativeField.Type to internal handling uses FieldDefinition.Value.
func TestDeclarativeJSON_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "results": [
		    {"title":"Alpha","size":1024,"seeders":3,"infohash":"abcdef0123456789abcdef0123456789abcdef01","download":"magnet:?xt=urn:btih:abcdef0123456789abcdef0123456789abcdef01"}
		  ]
		}`))
	}))
	defer srv.Close()

	def := model.IndexerDefinition{
		ID:   "demo-json",
		Name: "Demo JSON",
		Type: "public",
		Result: model.ResultDefinition{
			Format: "json",
			Fields: map[string]model.FieldDefinition{
				"row":        {Selector: "results[*]"},
				"title":      {Selector: "title", Value: "string"},
				"size":       {Selector: "size", Value: "size"},
				"seeders":    {Selector: "seeders", Value: "int"},
				"infohash":   {Selector: "infohash", Value: "infohash"},
				"magnet":     {Selector: "download", Value: "magnet_url"},
				"detail_url": {Selector: "title", Value: "url_template", Template: "https://example.org/item/{{ value }}"},
			},
		},
	}

	inst := model.InstalledIndexer{ID: def.ID, Name: def.Name, BaseURL: srv.URL}
	a := &declarativeAdapter{id: def.ID, def: def, installed: inst}

	// Tests like TestDeclarativeFactory_Create mutate the package-level
	// client via NewClient() (which returns a policy that rejects http);
	// install one that allows our loopback httptest server.
	c := &Client{
		Timeout: 5 * time.Second,
		Policy:  security.DefaultValidator{AllowHTTP: true, AllowLoopback: true},
		HTTP:    &http.Client{Timeout: 5 * time.Second},
	}
	prev := currentClient()
	setClient(c)
	defer setClient(prev)

	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "alpha"})
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
	if !strings.Contains(res[0].MagnetURL, "btih:abcdef0123456789abcdef0123456789abcdef01") {
		t.Fatalf("missing magnet: %+v", res[0])
	}
	if res[0].Seeders == nil || *res[0].Seeders != 3 {
		t.Fatalf("seeders=nil or wrong: %+v", res[0].Seeders)
	}
	if res[0].Title != "Alpha" {
		t.Fatalf("title mismatch: %q", res[0].Title)
	}
	if res[0].DetailURL != "https://example.org/item/Alpha" {
		t.Fatalf("detail URL mismatch: %q", res[0].DetailURL)
	}
}
