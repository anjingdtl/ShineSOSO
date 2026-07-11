package search

import (
	"testing"
	"time"

	"github.com/local/easysearch/backend/internal/model"
)

func TestFiltersEmptyPassthrough(t *testing.T) {
	f := NewFilters()
	in := []model.SearchResult{{Title: "x"}, {Title: "y"}}
	if got := f.Apply(model.SearchQuery{}, in); len(got) != 2 {
		t.Errorf("empty query should pass through, got %d", len(got))
	}
}

func TestFiltersByIndexerID(t *testing.T) {
	f := NewFilters()
	in := []model.SearchResult{
		{Title: "x", IndexerID: "a"},
		{Title: "y", IndexerID: "b"},
	}
	out := f.Apply(model.SearchQuery{IndexerIDs: []string{"a"}}, in)
	if len(out) != 1 || out[0].IndexerID != "a" {
		t.Errorf("expected only 'a', got %+v", out)
	}
}

func TestFiltersRemoveKeywordMismatches(t *testing.T) {
	f := NewFilters()
	got := f.Apply(model.SearchQuery{Keyword: "The Matrix"}, []model.SearchResult{
		{Title: "The.Matrix.1999.1080p"},
		{Title: "Unrelated blockbuster"},
	})
	if len(got) != 1 || got[0].Title != "The.Matrix.1999.1080p" {
		t.Fatalf("unexpected keyword filter output: %+v", got)
	}
}

func TestFiltersByMinSize(t *testing.T) {
	f := NewFilters()
	min := int64(1_000)
	in := []model.SearchResult{
		{Title: "small", SizeBytes: ptrInt64(500)},
		{Title: "big", SizeBytes: ptrInt64(2_000)},
	}
	out := f.Apply(model.SearchQuery{MinSizeBytes: &min}, in)
	if len(out) != 1 || out[0].Title != "big" {
		t.Errorf("expected only 'big', got %+v", out)
	}
}

func TestFiltersByMaxSize(t *testing.T) {
	f := NewFilters()
	max := int64(1_000)
	in := []model.SearchResult{
		{Title: "small", SizeBytes: ptrInt64(500)},
		{Title: "big", SizeBytes: ptrInt64(2_000)},
	}
	out := f.Apply(model.SearchQuery{MaxSizeBytes: &max}, in)
	if len(out) != 1 || out[0].Title != "small" {
		t.Errorf("expected only 'small', got %+v", out)
	}
}

func TestFiltersByMinSeeders(t *testing.T) {
	f := NewFilters()
	min := 50
	in := []model.SearchResult{
		{Title: "low", Seeders: ptrInt(10)},
		{Title: "high", Seeders: ptrInt(100)},
	}
	out := f.Apply(model.SearchQuery{MinSeeders: &min}, in)
	if len(out) != 1 || out[0].Title != "high" {
		t.Errorf("expected only 'high', got %+v", out)
	}
}

func TestFiltersByPublishedAfter(t *testing.T) {
	f := NewFilters()
	cutoff := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	old := cutoff.AddDate(0, -1, 0)
	new := cutoff.AddDate(0, 1, 0)
	in := []model.SearchResult{
		{Title: "old", PublishedAt: &old},
		{Title: "new", PublishedAt: &new},
	}
	out := f.Apply(model.SearchQuery{PublishedAfter: &cutoff}, in)
	if len(out) != 1 || out[0].Title != "new" {
		t.Errorf("expected only 'new', got %+v", out)
	}
}

func TestFiltersMissingSizeIsKept(t *testing.T) {
	// A result with no size should not be filtered out by min/max.
	f := NewFilters()
	min := int64(1_000_000_000)
	in := []model.SearchResult{
		{Title: "no-size"},
	}
	out := f.Apply(model.SearchQuery{MinSizeBytes: &min}, in)
	if len(out) != 1 {
		t.Errorf("missing size should be kept, got %d", len(out))
	}
}

func ptrInt64(v int64) *int64 { return &v }
func ptrInt(v int) *int       { return &v }
