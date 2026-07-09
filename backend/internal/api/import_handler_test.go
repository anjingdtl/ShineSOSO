package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/local/easysearch/backend/internal/catalog"
	"github.com/local/easysearch/backend/internal/store"
)

const goodHTMLYAML = `
schema: 1
id: my-fixture
name: My Fixture
version: 1.0.0
description: A test import
type: public
protocol: declarative

links:
  - https://198.51.100.1/

categories:
  movie:
    - "1"

search:
  method: GET
  path: /search
  query:
    q: "{{ .Query.Keyword }}"
  timeoutSeconds: 10

response:
  format: html
  rows:
    selector: "tr"
  fields:
    title:
      selector: "a.t"
      value: text
      required: true
`

const missingTitleYAML = `
schema: 1
id: bad-id
name: ""
version: 1.0.0
type: public
protocol: declarative
links:
  - https://198.51.100.1/
response:
  format: html
  fields: {}
`

const privateIPYAML = `
schema: 1
id: bad-host
name: Bad
version: 1.0.0
type: public
protocol: declarative
links:
  - https://127.0.0.1/
response:
  format: html
  fields:
    title: { selector: "a", value: text, required: true }
`

func newImportHandler(t *testing.T) (*ImportHandler, *catalog.Catalog) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	idxRepo := store.NewIndexerRepo(s)
	cat := catalog.New(idxRepo)
	impRepo := store.NewImportedDefinitionRepo(s)
	return &ImportHandler{
		Logger:     slog.Default(),
		Repo:       impRepo,
		Catalog:    cat,
		HTTPClient: nil,
	}, cat
}

func decodeImport(t *testing.T, body []byte) ImportResponse {
	t.Helper()
	var out ImportResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}
	return out
}

func TestImport_validYAMLPersists(t *testing.T) {
	h, cat := newImportHandler(t)

	body, _ := json.Marshal(map[string]any{"yaml": goodHTMLYAML})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/indexer-catalog/import", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Import(rr, req.WithContext(context.Background()))

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d, body: %s", rr.Code, rr.Body.String())
	}
	resp := decodeImport(t, rr.Body.Bytes())
	if !resp.Valid {
		t.Fatalf("expected valid; body: %s", rr.Body.String())
	}
	if !resp.Persisted {
		t.Errorf("not persisted")
	}
	// Should be registered with the catalog now.
	if _, ok := cat.GetDefinition("my-fixture"); !ok {
		t.Errorf("definition not registered in catalog")
	}
}

func TestImport_rejectsInvalidYAML(t *testing.T) {
	h, _ := newImportHandler(t)
	body, _ := json.Marshal(map[string]any{"yaml": "this : is : not : yaml"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/indexer-catalog/import", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Import(rr, req.WithContext(context.Background()))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "INVALID_YAML") {
		t.Errorf("missing error code: %s", rr.Body.String())
	}
}

func TestImport_rejectsMissingTitle(t *testing.T) {
	h, _ := newImportHandler(t)
	body, _ := json.Marshal(map[string]any{"yaml": missingTitleYAML})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/indexer-catalog/import", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Import(rr, req.WithContext(context.Background()))

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 envelope, got %d", rr.Code)
	}
	resp := decodeImport(t, rr.Body.Bytes())
	if resp.Valid {
		t.Fatalf("expected invalid; got %#v", resp)
	}
	if len(resp.Errors) == 0 || resp.Errors[0].Code != catalog.CodeNameMissing {
		t.Errorf("want NAME_MISSING, got %#v", resp.Errors)
	}
}

func TestImport_rejectsPrivateIP(t *testing.T) {
	h, _ := newImportHandler(t)
	body, _ := json.Marshal(map[string]any{"yaml": privateIPYAML})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/indexer-catalog/import", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Import(rr, req.WithContext(context.Background()))

	rrBody := rr.Body.String()
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 envelope, got %d", rr.Code)
	}
	resp := decodeImport(t, rr.Body.Bytes())
	if resp.Valid {
		t.Fatalf("expected invalid; got %#v", resp)
	}
	if len(resp.Errors) == 0 || resp.Errors[0].Code != catalog.CodeLinkUnsafe {
		t.Errorf("want LINK_UNSAFE, got %#v", resp.Errors)
	}
	_ = rrBody
}

func TestImport_rejectsOversize(t *testing.T) {
	h, _ := newImportHandler(t)
	big := strings.Repeat("a ", catalog.MaxDefinitionBytes/2+100)
	body, _ := json.Marshal(map[string]any{"yaml": big})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/indexer-catalog/import", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.Import(rr, req.WithContext(context.Background()))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d (%s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "DEFINITION_TOO_LARGE") {
		t.Errorf("missing code: %s", rr.Body.String())
	}
}
