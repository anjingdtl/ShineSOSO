// Package catalog owns the in-memory list of installed indexers plus
// their lookup-by-id. The catalog is refreshed from the SQLite repo on
// startup and after every mutation; queries read from memory.
package catalog

import (
	"fmt"
	"sort"
	"sync"

	"github.com/local/easysearch/backend/internal/indexer"
	"github.com/local/easysearch/backend/internal/model"
	"github.com/local/easysearch/backend/internal/search"
	"github.com/local/easysearch/backend/internal/store"
)

// Catalog is the read-optimized view of installed_indexers.
type Catalog struct {
	repo *store.IndexerRepo

	mu      sync.RWMutex
	byID    map[string]model.InstalledIndexer
	enabled []model.InstalledIndexer
	defs    map[string]model.IndexerDefinition // keyed by DefinitionID
}

// New returns an empty catalog. Call Refresh to load from disk.
func New(repo *store.IndexerRepo) *Catalog {
	return &Catalog{
		repo: repo,
		byID: map[string]model.InstalledIndexer{},
		defs: map[string]model.IndexerDefinition{},
	}
}

// RegisterDefinition adds (or replaces) an IndexerDefinition keyed by ID.
func (c *Catalog) RegisterDefinition(d model.IndexerDefinition) {
	c.mu.Lock()
	c.defs[d.ID] = d
	c.mu.Unlock()
}

// GetDefinition returns a definition by ID.
func (c *Catalog) GetDefinition(id string) (model.IndexerDefinition, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	d, ok := c.defs[id]
	return d, ok
}

// Definitions returns all registered definitions, sorted by ID.
func (c *Catalog) Definitions() []model.IndexerDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.IndexerDefinition, 0, len(c.defs))
	for _, d := range c.defs {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Refresh reloads from the repo. Cheap; called after every mutation.
func (c *Catalog) Refresh() error {
	all, err := c.repo.List()
	if err != nil {
		return fmt.Errorf("catalog refresh: %w", err)
	}
	enabled, err := c.repo.ListEnabled()
	if err != nil {
		return fmt.Errorf("catalog enabled: %w", err)
	}
	c.mu.Lock()
	c.byID = make(map[string]model.InstalledIndexer, len(all))
	for _, in := range all {
		c.byID[in.ID] = in
	}
	c.enabled = enabled
	c.mu.Unlock()
	return nil
}

// Get returns one installed indexer.
func (c *Catalog) Get(id string) (model.InstalledIndexer, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	in, ok := c.byID[id]
	return in, ok
}

// Enabled returns the enabled indexers, sorted by created_at (stable).
func (c *Catalog) Enabled() []model.InstalledIndexer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]model.InstalledIndexer, len(c.enabled))
	copy(out, c.enabled)
	return out
}

// Jobs builds the orchestrator jobs for enabled indexers that have a
// registered definition. Unknown / unbuiltable indexers are skipped.
func (c *Catalog) Jobs(client *indexer.Client) ([]search.IndexerJob, error) {
	enabled := c.Enabled()
	out := make([]search.IndexerJob, 0, len(enabled))
	for _, in := range enabled {
		def, ok := c.GetDefinition(in.DefinitionID)
		if !ok {
			continue // unknown definition; skip silently
		}
		factory, err := indexer.Default.Get(indexer.Protocol(def.Protocol))
		if err != nil {
			continue // no factory for this protocol
		}
		adapter, err := factory.Create(def, in, client)
		if err != nil {
			return nil, fmt.Errorf("create adapter for %s: %w", in.Name, err)
		}
		out = append(out, search.IndexerJob{Adapter: adapter, Name: in.Name})
	}
	return out, nil
}