// helpers.go — small adapter-construction wrappers used by the
// integration tests. They go through the public registry / factory
// surface so the tests don't depend on package internals that may
// move between releases.
package integration

import (
	"github.com/local/easysearch/backend/internal/indexer"
	"github.com/local/easysearch/backend/internal/model"
)

// testClient returns an indexer.Client configured for httptest loopback
// servers (plain HTTP, 127.0.0.1).
func testClient() *indexer.Client {
	c := indexer.NewClient()
	c.Policy.AllowHTTP = true
	c.Policy.AllowLoopback = true
	return c
}

func newDeclarative(def model.IndexerDefinition, inst model.InstalledIndexer) indexer.IndexerAdapter {
	a, err := indexer.NewDeclarativeFactory().Create(def, inst, testClient())
	if err != nil {
		panic(err)
	}
	return a
}

func newTorznab(def model.IndexerDefinition, inst model.InstalledIndexer) indexer.IndexerAdapter {
	a, err := indexer.NewTorznabFactory().Create(def, inst, testClient())
	if err != nil {
		panic(err)
	}
	return a
}