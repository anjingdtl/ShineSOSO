package indexer

import (
	"testing"

	"github.com/local/easysearch/backend/internal/model"
)

func TestMockFactory_Create(t *testing.T) {
	f := mockFactory{}
	def := model.IndexerDefinition{ID: "demo-alpha", Protocol: "mock"}
	inst := model.InstalledIndexer{ID: "x", Name: "Demo Alpha"}
	a, err := f.Create(def, inst, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID() != "demo-alpha" {
		t.Errorf("ID want demo-alpha got %q", a.ID())
	}
}

func TestRegistry_Register_Get(t *testing.T) {
	r := NewRegistry()
	f := mockFactory{}
	r.Register(Protocol("test-mock"), f)
	got, err := r.Get(Protocol("test-mock"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Error("factory should not be nil")
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get(Protocol("does-not-exist"))
	if err == nil {
		t.Error("expected error for unknown protocol")
	}
}

func TestRegistry_RegisterReplaces(t *testing.T) {
	r := NewRegistry()
	r.Register(Protocol("x"), mockFactory{})
	r.Register(Protocol("x"), torznabFactory{})
	f, err := r.Get(Protocol("x"))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// torznabFactory rejects protocol mismatch so we know it replaced mockFactory.
	_, err2 := f.Create(model.IndexerDefinition{ID: "y"}, model.InstalledIndexer{BaseURL: "https://e.x"}, nil)
	if err2 == nil {
		t.Error("expected torznabFactory to fail on protocol mismatch")
	}
}