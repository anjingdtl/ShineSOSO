package catalog

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/local/easysearch/backend/internal/model"
	"github.com/local/easysearch/backend/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func sample(name string, enabled bool) model.InstalledIndexer {
	now := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	return model.InstalledIndexer{
		ID:                uuid.NewString(),
		DefinitionID:      "demo-alpha",
		Name:              name,
		Enabled:           enabled,
		BaseURL:           "https://example.com",
		DefinitionVersion: "1.0.0",
		Status:            "unknown",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func TestCatalogRefreshAndGet(t *testing.T) {
	s := openStore(t)
	repo := store.NewIndexerRepo(s)
	c := New(repo)
	if err := c.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := c.Enabled(); len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}

	a := sample("alpha", true)
	b := sample("beta", false)
	if err := repo.Create(a); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if err := repo.Create(b); err != nil {
		t.Fatalf("create b: %v", err)
	}
	if err := c.Refresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if got := c.Enabled(); len(got) != 1 || got[0].Name != "alpha" {
		t.Errorf("enabled=%+v", got)
	}
	got, ok := c.Get(a.ID)
	if !ok || got.Name != "alpha" {
		t.Errorf("get=%v ok=%v", got, ok)
	}
}

func TestCatalogDefinitions(t *testing.T) {
	c := New(nil)
	for _, d := range BuiltinDefinitions() {
		c.RegisterDefinition(d)
	}
	defs := c.Definitions()
	if len(defs) != 3 {
		t.Errorf("expected 3 builtins, got %d", len(defs))
	}
	d, ok := c.GetDefinition("demo-alpha")
	if !ok {
		t.Errorf("demo-alpha missing")
	}
	if d.Protocol != "mock" {
		t.Errorf("protocol=%s", d.Protocol)
	}
}
