package indexer

import (
    "context"
    "time"

    "github.com/local/easysearch/backend/internal/model"
)

// TestResult is the outcome of a health probe (Indexer's Test method).
type TestResult struct {
    OK           bool          `json:"ok"`
    StatusCode   int           `json:"statusCode,omitempty"`
    DurationMs   int64         `json:"durationMs"`
    ResultCount  int           `json:"resultCount,omitempty"`
    ErrorCode    string        `json:"errorCode,omitempty"`
    ErrorMessage string        `json:"errorMessage,omitempty"`
}

// IndexerAdapter is the contract every indexer must implement (spec §12).
//
// Implementations are expected to be safe for concurrent use. They MUST
// honor ctx cancellation and respect their configured timeout.
type IndexerAdapter interface {
    // ID returns the stable id of the InstalledIndexer this adapter serves.
    ID() string

    // Test performs a health probe (no user query). Used on add, on user
    // demand, and on the background health loop.
    Test(ctx context.Context) TestResult

    // Search runs a single user query and returns the raw results before
    // dedup/sort. Errors are returned to the orchestrator, which maps
    // them to SSE events; the adapter should not crash the caller.
    Search(ctx context.Context, query model.SearchQuery) ([]model.SearchResult, error)
}

// AdapterFactory turns an (IndexerDefinition, InstalledIndexer) pair into
// a ready-to-use IndexerAdapter. Concrete factories live in adapter
// implementations (declarative.go, torznab.go) and are registered in the
// global factory registry in factory.go.
type AdapterFactory interface {
    Create(def model.IndexerDefinition, installed model.InstalledIndexer, client *Client) (IndexerAdapter, error)
}

// HealthCheckInterval is the default gap between automatic probes.
const HealthCheckInterval = 12 * time.Hour
