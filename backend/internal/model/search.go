package model

import "time"

// SearchQuery is the validated, normalized input to a search.
type SearchQuery struct {
    Keyword        string     `json:"keyword"`
    Category       string     `json:"category"` // "all" or a Category enum
    MinSizeBytes   *int64     `json:"minSizeBytes,omitempty"`
    MaxSizeBytes   *int64     `json:"maxSizeBytes,omitempty"`
    MinSeeders     *int       `json:"minSeeders,omitempty"`
    PublishedAfter *time.Time `json:"publishedAfter,omitempty"`
    IndexerIDs     []string   `json:"indexerIds,omitempty"`
    Sort           string     `json:"sort"` // SortMode
}

// SearchSession tracks one user-initiated search across multiple indexers.
type SearchSession struct {
    ID              string     `json:"id"`
    Query           SearchQuery `json:"query"`
    Status          string     `json:"status"` // pending | running | completed | cancelled
    StartedAt       time.Time  `json:"startedAt"`
    FinishedAt      *time.Time `json:"finishedAt,omitempty"`
    TotalIndexers   int        `json:"totalIndexers"`
    CompletedCount  int        `json:"completedCount"`
    RawResultCount  int        `json:"rawResultCount"`
    MergedCount     int        `json:"mergedCount"`
}
