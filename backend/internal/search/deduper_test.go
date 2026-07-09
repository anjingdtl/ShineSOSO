package search

import (
    "testing"

    "github.com/local/easysearch/backend/internal/model"
)

func TestDedupSingleNoop(t *testing.T) {
    d := NewDeduper()
    rs := []model.SearchResult{{ID: "1", Title: "x", IndexerID: "a"}}
    out := d.Dedup(rs)
    if len(out) != 1 {
        t.Fatalf("expected 1, got %d", len(out))
    }
}

func TestDedupStrongByInfoHash(t *testing.T) {
    d := NewDeduper()
    rs := []model.SearchResult{
        {ID: "1", Title: "Foo 2024 1080p", InfoHash: "AAAA", IndexerID: "a", IndexerName: "A"},
        {ID: "2", Title: "Foo.2024.1080p", InfoHash: "AAAA", IndexerID: "b", IndexerName: "B"},
    }
    out := d.Dedup(rs)
    if len(out) != 1 {
        t.Fatalf("expected 1 merged result, got %d", len(out))
    }
    if len(out[0].Sources) != 2 {
        t.Errorf("expected 2 sources, got %d", len(out[0].Sources))
    }
}

func TestDedupStrongByMagnet(t *testing.T) {
    d := NewDeduper()
    validHash := "0123456789ABCDEF0123456789ABCDEF01234567"
    magnet := "magnet:?xt=urn:btih:" + validHash + "&dn=x"
    rs := []model.SearchResult{
        {ID: "1", Title: "Alpha", MagnetURL: magnet, IndexerID: "a"},
        {ID: "2", Title: "Beta",  MagnetURL: magnet, IndexerID: "b"},
    }
    out := d.Dedup(rs)
    if len(out) != 1 {
        t.Fatalf("expected 1, got %d", len(out))
    }
    if len(out[0].Sources) != 2 {
        t.Errorf("expected 2 sources, got %d", len(out[0].Sources))
    }
}

func TestDedupWeakByTitleAndSize(t *testing.T) {
    d := NewDeduper()
    sz := int64(1_000_000_000)
    rs := []model.SearchResult{
        {ID: "1", Title: "Foo Bar 2024 1080p", SizeBytes: &sz, IndexerID: "a"},
        {ID: "2", Title: "Foo.Bar.2024.1080p", SizeBytes: &sz, IndexerID: "b"},
    }
    out := d.Dedup(rs)
    if len(out) != 1 {
        t.Fatalf("expected 1 (weak merge), got %d: %+v", len(out), out)
    }
}

func TestDedupWeakSizeConflict(t *testing.T) {
    d := NewDeduper()
    sz1 := int64(1_000_000_000)
    sz2 := int64(1_200_000_000) // 20% diff, exceeds 2% threshold
    rs := []model.SearchResult{
        {ID: "1", Title: "Same Title 2024 1080p", SizeBytes: &sz1, IndexerID: "a"},
        {ID: "2", Title: "Same Title 2024 1080p", SizeBytes: &sz2, IndexerID: "b"},
    }
    out := d.Dedup(rs)
    if len(out) != 2 {
        t.Fatalf("expected 2 (size conflict blocks merge), got %d", len(out))
    }
}

func TestDedupWeakResolutionConflict(t *testing.T) {
    d := NewDeduper()
    sz := int64(1_000_000_000)
    rs := []model.SearchResult{
        {ID: "1", Title: "Movie 2024 1080p", SizeBytes: &sz, IndexerID: "a"},
        {ID: "2", Title: "Movie 2024 720p",  SizeBytes: &sz, IndexerID: "b"},
    }
    out := d.Dedup(rs)
    if len(out) != 2 {
        t.Fatalf("expected 2 (resolution conflict), got %d", len(out))
    }
}

func TestDedupPreservesMaxSeeders(t *testing.T) {
    d := NewDeduper()
    sz := int64(1_000_000_000)
    a := 50
    b := 200
    rs := []model.SearchResult{
        {ID: "1", Title: "X 2024", SizeBytes: &sz, Seeders: &a, InfoHash: "Z", IndexerID: "a"},
        {ID: "2", Title: "X 2024", SizeBytes: &sz, Seeders: &b, InfoHash: "Z", IndexerID: "b"},
    }
    out := d.Dedup(rs)
    if len(out) != 1 {
        t.Fatalf("expected 1, got %d", len(out))
    }
    if out[0].Seeders == nil || *out[0].Seeders != 200 {
        t.Errorf("seeders should be max(200), got %v", out[0].Seeders)
    }
}

func TestTitleSimilarityIdentical(t *testing.T) {
    if got := titleSimilarity("foo bar", "foo bar"); got != 1.0 {
        t.Errorf("identical strings should score 1.0, got %v", got)
    }
}

func TestTitleSimilarityDisjoint(t *testing.T) {
    if got := titleSimilarity("alpha beta", "gamma delta"); got != 0.0 {
        t.Errorf("disjoint strings should score 0.0, got %v", got)
    }
}

func TestTitleSimilarityPartial(t *testing.T) {
    // "foo bar" vs "foo baz" — 1 of 3 tokens overlap => 1/3
    got := titleSimilarity("foo bar", "foo baz")
    if got < 0.3 || got > 0.4 {
        t.Errorf("partial overlap should be ~0.33, got %v", got)
    }
}

func TestExtractYearAndResolution(t *testing.T) {
    if got := extractYear("Foo 2024 1080p"); got != "2024" {
        t.Errorf("year: want 2024, got %q", got)
    }
    if got := extractResolution("Foo 2024 1080p"); got != "1080p" {
        t.Errorf("res: want 1080p, got %q", got)
    }
    if got := extractYear("Foo 4K"); got != "" {
        t.Errorf("year: want empty, got %q", got)
    }
}

func TestNormalizeURL(t *testing.T) {
    if normalizeURL("HTTPS://Example.com/A?b=1#x") != "https://example.com/a?b=1" {
        t.Error("normalizeURL should lowercase + strip fragment")
    }
}

func TestMergeIntoPicksLongestTitle(t *testing.T) {
    a := &model.SearchResult{Title: "Short"}
    b := model.SearchResult{Title: "Much Longer Title"}
    mergeInto(a, b)
    if a.Title != "Much Longer Title" {
        t.Errorf("title should be the longer one, got %q", a.Title)
    }
}

func TestMergeIntoPreservesFirstDownload(t *testing.T) {
    a := &model.SearchResult{MagnetURL: "magnet:?xt=urn:btih:AAAA"}
    b := model.SearchResult{TorrentURL: "https://x/t.torrent"}
    mergeInto(a, b)
    if a.MagnetURL != "magnet:?xt=urn:btih:AAAA" {
        t.Error("mergeInto should not overwrite a's magnet with b's torrent")
    }
    if a.TorrentURL != "https://x/t.torrent" {
        t.Error("mergeInto should fill b's torrentURL into a")
    }
}
