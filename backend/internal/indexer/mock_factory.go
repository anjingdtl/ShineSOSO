package indexer

import (
	"github.com/local/easysearch/backend/internal/model"
)

// mockFactory builds the demo adapters used by built-in definitions.
// Phase 4 uses this for the three "demo-*" definitions; Phase 5/6 will
// replace it with the real declarative and torznab factories.
type mockFactory struct{}

func (mockFactory) Create(def model.IndexerDefinition, installed model.InstalledIndexer, _ *Client) (IndexerAdapter, error) {
	return newDemoAdapter(def.ID, installed.Name), nil
}

func init() {
	Default.Register(Protocol("mock"), mockFactory{})
}