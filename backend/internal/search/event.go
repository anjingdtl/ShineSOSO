// Package search orchestrates concurrent indexer queries and emits a
// stream of events the SSE handler forwards to the browser.
package search

import (
    "time"

    "github.com/local/easysearch/backend/internal/model"
)

// EventType is the discriminant of Event.
type EventType string

const (
    EventSessionStarted   EventType = "session_started"
    EventIndexerStarted   EventType = "indexer_started"
    EventIndexerResult    EventType = "indexer_result"
    EventIndexerCompleted EventType = "indexer_completed"
    EventIndexerFailed    EventType = "indexer_failed"
    EventResultsMerged    EventType = "results_merged"
    EventSessionCompleted EventType = "session_completed"
    EventSessionCancelled EventType = "session_cancelled"
)

// Event is the unit of work produced by the orchestrator and consumed by
// the SSE handler (spec §15.4). The Data field is type-erased to keep
// the package importable without circular dependencies; callers assert
// the concrete shape.
type Event struct {
    Type      EventType
    SessionID string
    Timestamp time.Time
    Data      any
}

// IndexerStartedData is emitted when a single indexer begins its search.
type IndexerStartedData struct {
    IndexerID   string `json:"indexerId"`
    IndexerName string `json:"indexerName"`
}

// IndexerResultData is emitted when an indexer has produced at least
// one raw result (incremental).
type IndexerResultData struct {
    IndexerID   string `json:"indexerId"`
    ResultCount int    `json:"resultCount"`
}

// IndexerCompletedData is the terminal event for one indexer.
type IndexerCompletedData struct {
    IndexerID   string `json:"indexerId"`
    Status      string `json:"status"` // IndexerStatus
    ResultCount int    `json:"resultCount"`
    DurationMs  int64  `json:"durationMs"`
}

// IndexerFailedData reports a hard failure; the result stream is unaffected.
type IndexerFailedData struct {
    IndexerID   string `json:"indexerId"`
    Code        string `json:"code"`
    Message     string `json:"message"`
}

// ResultsMergedData is emitted after an indexer completes; carries the
// running totals of raw vs. deduped results.
type ResultsMergedData struct {
    MergedCount int `json:"mergedCount"`
    RawCount    int `json:"rawCount"`
}

// SessionCompletedData is the final terminal event.
type SessionCompletedData struct {
    TotalMs     int64 `json:"totalMs"`
    MergedCount int   `json:"mergedCount"`
    RawCount    int   `json:"rawCount"`
}

// SessionStartedData reports the plan at the start of a session.
type SessionStartedData struct {
    SessionID     string `json:"sessionId"`
    TotalIndexers int    `json:"totalIndexers"`
}

// SessionCancelledData reports cancellation.
type SessionCancelledData struct {
    SessionID string `json:"sessionId"`
}

// InitialResultBatch is the result payload sent in a single SSE event
// (kept small: max 50 results per event to keep the wire friendly).
const MaxResultsPerEvent = 50

// ResultBatch is the wire format for incremental results.
type ResultBatch struct {
    SessionID string                 `json:"sessionId"`
    IndexerID string                 `json:"indexerId"`
    Results   []model.SearchResult   `json:"results"`
}
