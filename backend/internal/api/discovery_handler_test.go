package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverySearchPrefersStructuredCatalogAndExpandsChineseTerms(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/catalog":
			_ = json.NewEncoder(w).Encode([]remoteIndexerDefinition{{
				ID: "movie-source", Name: "Cinema Tracker", Type: "public", Protocol: "torrent",
				Language: "en-US", Description: "Public MOVIES and film tracker", Links: []string{"https://movies.example/"},
			}})
		case "/html":
			_, _ = w.Write([]byte(`<html><body></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	h := &DiscoveryHandler{CatalogURL: srv.URL + "/catalog", SearchURL: srv.URL + "/html", HTTP: srv.Client()}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/indexer-discovery/search", bytes.NewBufferString(`{"query":"电影"}`))
	rr := httptest.NewRecorder()
	h.Search(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Items []discoveryCandidate `json:"items"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || response.Items[0].Name != "Cinema Tracker" || response.Items[0].Source != "Prowlarr 结构化目录" {
		t.Fatalf("unexpected discovery results: %+v", response.Items)
	}
}

func TestDiscoverySearchRanksExactNameAboveDescriptionOnly(t *testing.T) {
	terms := []string{"nyaa"}
	exact := scoreRemoteDefinition(remoteIndexerDefinition{Name: "Nyaa", Description: "anime"}, terms)
	descriptionOnly := scoreRemoteDefinition(remoteIndexerDefinition{Name: "Other", Description: "Nyaa compatible"}, terms)
	if exact <= descriptionOnly {
		t.Fatalf("exact=%d description=%d", exact, descriptionOnly)
	}
}
