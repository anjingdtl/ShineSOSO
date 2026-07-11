// Package model holds the data structures shared across packages.
// These mirror spec-o1.md §11 and are intentionally side-effect free so
// the storage, search, and API layers can all import them safely.
package model

import "time"

// IndexerDefinition describes a built-in or imported indexer schema.
// It is loaded from YAML and is immutable for the lifetime of a process.
type IndexerDefinition struct {
	Schema      int                  `json:"schema" yaml:"schema"`
	ID          string               `json:"id" yaml:"id"`
	Name        string               `json:"name" yaml:"name"`
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string               `json:"version" yaml:"version"`
	Language    string               `json:"language,omitempty" yaml:"language,omitempty"`
	Type        string               `json:"type" yaml:"type"`         // "public" only in v1
	Protocol    string               `json:"protocol" yaml:"protocol"` // "declarative" | "torznab"
	Categories  map[string][]string  `json:"categories,omitempty" yaml:"categories,omitempty"`
	Links       []string             `json:"links,omitempty" yaml:"links,omitempty"`
	Search      SearchDefinition     `json:"search" yaml:"search"`
	Result      ResultDefinition     `json:"result" yaml:"response"`
	RateLimit   *RateLimitDefinition `json:"rateLimit,omitempty" yaml:"rateLimit,omitempty"`
}

// SearchDefinition is the request shape for the indexer's search endpoint.
type SearchDefinition struct {
	Method         string            `json:"method" yaml:"method"` // "GET" | "POST"
	Path           string            `json:"path" yaml:"path"`
	Query          map[string]string `json:"query,omitempty" yaml:"query,omitempty"`
	Headers        map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body           string            `json:"body,omitempty" yaml:"body,omitempty"`
	TimeoutSeconds int               `json:"timeoutSeconds,omitempty" yaml:"timeoutSeconds,omitempty"`
}

// ResultDefinition describes how to parse the response (Phase 5 YAML).
type ResultDefinition struct {
	Format string                     `json:"format" yaml:"format"` // "html" | "json" | "xml" | "torznab"
	Rows   RowDefinition              `json:"rows" yaml:"rows"`
	Fields map[string]FieldDefinition `json:"fields" yaml:"fields"`
}

// RowDefinition is the outer container selector. Empty for non-row formats.
type RowDefinition struct {
	Selector string `json:"selector,omitempty" yaml:"selector,omitempty"`
}

// FieldDefinition is one extracted field. Phase 5 expands this; in
// Phase 2 the indexer engine reads it but we only assert non-nil.
type FieldDefinition struct {
	Selector    string   `json:"selector,omitempty" yaml:"selector,omitempty"`
	Value       string   `json:"value,omitempty" yaml:"value,omitempty"`
	Attribute   string   `json:"attribute,omitempty" yaml:"attribute,omitempty"`
	Required    bool     `json:"required,omitempty" yaml:"required,omitempty"`
	Filters     []string `json:"filters,omitempty" yaml:"filters,omitempty"`
	DateLayouts []string `json:"dateLayouts,omitempty" yaml:"dateLayouts,omitempty"`
	ResolveURL  bool     `json:"resolveUrl,omitempty" yaml:"resolveUrl,omitempty"`
	// Template builds a URL from one extracted JSON scalar. Only the
	// literal {{ value }} placeholder is permitted by the validator.
	Template string `json:"template,omitempty" yaml:"template,omitempty"`
}

// RateLimitDefinition throttles requests (spec §21.x).
type RateLimitDefinition struct {
	RequestsPerMinute int `json:"requestsPerMinute,omitempty" yaml:"requestsPerMinute,omitempty"`
}

// InstalledIndexer is a user's enabled/disabled instance of an
// IndexerDefinition. Persisted in SQLite; mutated by the API.
type InstalledIndexer struct {
	ID                string     `json:"id"`
	DefinitionID      string     `json:"definitionId"`
	Name              string     `json:"name"`
	Enabled           bool       `json:"enabled"`
	BaseURL           string     `json:"baseUrl"`
	DefinitionVersion string     `json:"definitionVersion"`
	Status            string     `json:"status"` // IndexerHealth
	LastCheckedAt     *time.Time `json:"lastCheckedAt,omitempty"`
	LastSuccessAt     *time.Time `json:"lastSuccessAt,omitempty"`
	LastError         string     `json:"lastError,omitempty"`
	ResponseTimeMs    int64      `json:"responseTimeMs,omitempty"`
	ConsecutiveFails  int        `json:"consecutiveFails,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}
