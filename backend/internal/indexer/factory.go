package indexer

import (
	"fmt"
	"sync"
)

// Protocol identifies which adapter family an IndexerDefinition belongs to.
type Protocol string

const (
	ProtocolDeclarative Protocol = "declarative"
	ProtocolTorznab     Protocol = "torznab"
)

// Registry maps Protocol -> AdapterFactory. It is a process-global
// singleton; the indexer engine looks up factories here when it
// instantiates adapters from catalog entries.
type Registry struct {
	mu        sync.RWMutex
	factories map[Protocol]AdapterFactory
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{factories: map[Protocol]AdapterFactory{}}
}

// Register binds a factory to a protocol. Re-registering replaces.
func (r *Registry) Register(p Protocol, f AdapterFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[p] = f
}

// Get returns the factory for a protocol, or an error if unknown.
func (r *Registry) Get(p Protocol) (AdapterFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[p]
	if !ok {
		return nil, fmt.Errorf("indexer: no factory for protocol %q", p)
	}
	return f, nil
}

// Default is the process-wide registry. Adapters register themselves
// in init() functions and the engine reads from Default.
var Default = NewRegistry()
