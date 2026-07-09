package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/local/easysearch/backend/internal/catalog"
	"github.com/local/easysearch/backend/internal/indexer"
	"github.com/local/easysearch/backend/internal/model"
	"github.com/local/easysearch/backend/internal/store"
)

// newTestHandler wires up a handler with the catalog + store + http
// client. Returns the handler and a cleanup func.
func newTestHandler(t *testing.T) (*IndexerHandler, *catalog.Catalog, *store.IndexerRepo) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	repo := store.NewIndexerRepo(s)
	c := catalog.New(repo)
	for _, d := range catalog.BuiltinDefinitions() {
		c.RegisterDefinition(d)
	}
	if err := c.Refresh(); err != nil {
		t.Fatalf("catalog refresh: %v", err)
	}
	client := indexer.NewClient()
	h := &IndexerHandler{
		Logger:     slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Catalog:    c,
		Repo:       repo,
		HTTPClient: client,
	}
	return h, c, repo
}

func mustCreateInstalled(t *testing.T, repo *store.IndexerRepo, defID, name string) model.InstalledIndexer {
	t.Helper()
	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	in := model.InstalledIndexer{
		ID:                uuid.NewString(),
		DefinitionID:      defID,
		Name:              name,
		Enabled:           true,
		BaseURL:           "https://example.com",
		DefinitionVersion: "1.0.0",
		Status:            "unknown",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := repo.Create(in); err != nil {
		t.Fatalf("create installed: %v", err)
	}
	return in
}

func do(t *testing.T, h http.HandlerFunc, method, url string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, url, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// PathValue extraction in the handler uses stdlib http.Request's
	// PathValue, which reads from a side-table. Set it via SetPathValue.
	if strings.Contains(url, "/test") {
		parts := strings.Split(strings.TrimPrefix(url, "/api/v1/indexers/"), "/")
		if len(parts) >= 1 {
			req.SetPathValue("id", parts[0])
		}
	} else if strings.HasPrefix(url, "/api/v1/indexers/") {
		req.SetPathValue("id", strings.TrimPrefix(url, "/api/v1/indexers/"))
	}

	rr := httptest.NewRecorder()
	h(rr, req)
	return rr
}

func TestIndexerHandlerListEmpty(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rr := do(t, h.List, "GET", "/api/v1/indexers", nil)
	if rr.Code != 200 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp listIndexersResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Items) != 0 {
		t.Errorf("items=%d", len(resp.Items))
	}
}

func TestIndexerHandlerCreateGet(t *testing.T) {
	h, c, repo := newTestHandler(t)

	rr := do(t, h.Create, "POST", "/api/v1/indexers", map[string]any{
		"definitionId": "demo-alpha",
		"baseUrl":      "https://example.com",
	})
	if rr.Code != 201 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var created model.InstalledIndexer
	json.NewDecoder(rr.Body).Decode(&created)
	if created.DefinitionID != "demo-alpha" {
		t.Errorf("definitionId=%s", created.DefinitionID)
	}
	if !created.Enabled {
		t.Error("should be enabled by default")
	}

	rr = do(t, h.Get, "GET", "/api/v1/indexers/"+created.ID, nil)
	if rr.Code != 200 {
		t.Fatalf("get status=%d", rr.Code)
	}
	if err := c.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	enabled := c.Enabled()
	if len(enabled) != 1 || enabled[0].ID != created.ID {
		t.Errorf("catalog did not pick up new indexer: %v", enabled)
	}
	_ = repo
}

func TestIndexerHandlerRejectsUnsafeURL(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rr := do(t, h.Create, "POST", "/api/v1/indexers", map[string]any{
		"definitionId": "demo-alpha",
		"baseUrl":      "http://127.0.0.1/x",
	})
	if rr.Code != 400 {
		t.Fatalf("expected 400 for unsafe URL, got %d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "UNSAFE_INDEXER_URL") {
		t.Errorf("body missing UNSAFE_INDEXER_URL: %s", rr.Body.String())
	}
}

func TestIndexerHandlerCreateUnknownDefinition(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rr := do(t, h.Create, "POST", "/api/v1/indexers", map[string]any{
		"definitionId": "no-such-thing",
		"baseUrl":      "https://example.com",
	})
	if rr.Code != 400 {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestIndexerHandlerUpdateToggle(t *testing.T) {
	h, _, repo := newTestHandler(t)
	inst := mustCreateInstalled(t, repo, "demo-alpha", "alpha")
	if err := repo.Update(inst); err != nil {
		t.Fatal(err)
	}
	enabled := false
	rr := do(t, h.Update, "PATCH", "/api/v1/indexers/"+inst.ID, map[string]any{"enabled": enabled})
	if rr.Code != 200 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var out model.InstalledIndexer
	json.NewDecoder(rr.Body).Decode(&out)
	if out.Enabled {
		t.Error("expected disabled")
	}
}

func TestIndexerHandlerDelete(t *testing.T) {
	h, _, repo := newTestHandler(t)
	inst := mustCreateInstalled(t, repo, "demo-alpha", "alpha")
	rr := do(t, h.Delete, "DELETE", "/api/v1/indexers/"+inst.ID, nil)
	if rr.Code != 204 {
		t.Fatalf("status=%d", rr.Code)
	}
	rr = do(t, h.Get, "GET", "/api/v1/indexers/"+inst.ID, nil)
	if rr.Code != 404 {
		t.Fatalf("expected 404 after delete, got %d", rr.Code)
	}
}

func TestIndexerHandlerTestHealthy(t *testing.T) {
	h, _, repo := newTestHandler(t)
	inst := mustCreateInstalled(t, repo, "demo-alpha", "alpha")
	rr := do(t, h.Test, "POST", "/api/v1/indexers/"+inst.ID+"/test", nil)
	if rr.Code != 200 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp testIndexerResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if !resp.OK {
		t.Errorf("expected ok=true: %+v", resp)
	}
	// After a successful test the persisted status must be healthy.
	got, _ := repo.Get(inst.ID)
	if got.Status != "healthy" {
		t.Errorf("status=%s", got.Status)
	}
	events, _ := repo.ListHealthEvents(inst.ID, 10)
	if len(events) != 1 || events[0].Status != "healthy" {
		t.Errorf("events=%+v", events)
	}
}

func TestIndexerHandlerTestFailing(t *testing.T) {
	h, _, repo := newTestHandler(t)
	inst := mustCreateInstalled(t, repo, "demo-gamma", "gamma")
	rr := do(t, h.Test, "POST", "/api/v1/indexers/"+inst.ID+"/test", nil)
	if rr.Code != 200 {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp testIndexerResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.OK {
		t.Errorf("expected ok=false: %+v", resp)
	}
	got, _ := repo.Get(inst.ID)
	if got.Status != "error" {
		t.Errorf("status=%s", got.Status)
	}
	if got.LastError == "" {
		t.Error("expected last_error set")
	}
}

func TestIndexerHandlerListCatalog(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rr := do(t, h.ListCatalog, "GET", "/api/v1/indexer-catalog", nil)
	if rr.Code != 200 {
		t.Fatalf("status=%d", rr.Code)
	}
	var resp listCatalogResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Items) != 3 {
		t.Errorf("catalog items=%d want 3", len(resp.Items))
	}
}

// Silence unused-import warnings when individual subtests above change.
var (
	_ = io.Discard
)