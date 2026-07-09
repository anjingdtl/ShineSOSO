package indexer

import (
	"testing"

	"github.com/local/easysearch/backend/internal/model"
)

func TestTorznabFactory_RejectsProtocolMismatch(t *testing.T) {
	f := torznabFactory{}
	def := model.IndexerDefinition{ID: "x", Protocol: "html"}
	_, err := f.Create(def, model.InstalledIndexer{BaseURL: "https://example.com"}, NewClient())
	if err == nil {
		t.Error("expected protocol mismatch error")
	}
}

func TestTorznabFactory_RejectsEmptyBaseURL(t *testing.T) {
	f := torznabFactory{}
	def := model.IndexerDefinition{ID: "x", Protocol: "torznab"}
	_, err := f.Create(def, model.InstalledIndexer{BaseURL: ""}, NewClient())
	if err == nil {
		t.Error("expected empty BaseURL error")
	}
}

func TestTorznabFactory_RejectsNilClient(t *testing.T) {
	f := torznabFactory{}
	def := model.IndexerDefinition{ID: "x", Protocol: "torznab"}
	_, err := f.Create(def, model.InstalledIndexer{BaseURL: "https://example.com"}, nil)
	if err == nil {
		t.Error("expected nil client error")
	}
}

func TestTorznabFactory_OK(t *testing.T) {
	f := torznabFactory{}
	def := model.IndexerDefinition{ID: "x", Protocol: "torznab"}
	a, err := f.Create(def, model.InstalledIndexer{ID: "i1", BaseURL: "https://example.com"}, NewClient())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID() != "i1" {
		t.Errorf("ID want i1 got %q", a.ID())
	}
}

func TestTorznabAdapter_BuildURL_PostRejected(t *testing.T) {
	def := model.IndexerDefinition{
		ID: "x", Protocol: "torznab",
		Search: model.SearchDefinition{Method: "POST"},
	}
	a := &torznabAdapter{def: def, installed: model.InstalledIndexer{BaseURL: "https://e.x"}}
	if _, err := a.buildURL("x", "", "", "1"); err == nil {
		t.Error("expected POST to be rejected")
	}
}

func TestTorznabAdapter_BuildURL_BadBase(t *testing.T) {
	def := model.IndexerDefinition{ID: "x", Protocol: "torznab"}
	a := &torznabAdapter{def: def, installed: model.InstalledIndexer{BaseURL: "://bad"}}
	if _, err := a.buildURL("x", "", "", "1"); err == nil {
		t.Error("expected bad base URL error")
	}
}

func TestTorznabAdapter_BuildURL_BadPath(t *testing.T) {
	def := model.IndexerDefinition{
		ID: "x", Protocol: "torznab",
		Search: model.SearchDefinition{Path: "://bad"},
	}
	a := &torznabAdapter{def: def, installed: model.InstalledIndexer{BaseURL: "https://e.x"}}
	if _, err := a.buildURL("x", "", "", "1"); err == nil {
		t.Error("expected bad path error")
	}
}