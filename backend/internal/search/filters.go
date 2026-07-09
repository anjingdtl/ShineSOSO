package search

import (
    "github.com/local/easysearch/backend/internal/model"
)

// Filters applies the user-selected constraints (spec §6.5).
type Filters struct{}

func NewFilters() *Filters { return &Filters{} }

// Apply returns the subset of results that pass the query's filters.
// The input slice is not mutated.
func (f *Filters) Apply(q model.SearchQuery, in []model.SearchResult) []model.SearchResult {
    if q.MinSizeBytes == nil && q.MaxSizeBytes == nil &&
        q.MinSeeders == nil && q.PublishedAfter == nil &&
        len(q.IndexerIDs) == 0 {
        return in
    }
    indexerFilter := make(map[string]struct{}, len(q.IndexerIDs))
    for _, id := range q.IndexerIDs {
        indexerFilter[id] = struct{}{}
    }
    out := make([]model.SearchResult, 0, len(in))
    for _, r := range in {
        if len(indexerFilter) > 0 {
            if _, ok := indexerFilter[r.IndexerID]; !ok {
                continue
            }
        }
        if q.MinSizeBytes != nil && r.SizeBytes != nil && *r.SizeBytes < *q.MinSizeBytes {
            continue
        }
        if q.MaxSizeBytes != nil && r.SizeBytes != nil && *r.SizeBytes > *q.MaxSizeBytes {
            continue
        }
        if q.MinSeeders != nil && r.Seeders != nil && *r.Seeders < *q.MinSeeders {
            continue
        }
        if q.PublishedAfter != nil && r.PublishedAt != nil && r.PublishedAt.Before(*q.PublishedAfter) {
            continue
        }
        out = append(out, r)
    }
    return out
}
