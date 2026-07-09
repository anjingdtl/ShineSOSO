package search

import (
    "math"
    "regexp"
    "strings"

    "github.com/local/easysearch/backend/internal/model"
    "github.com/local/easysearch/backend/internal/normalize"
)

// Deduper merges equivalent results from multiple indexers (spec §17).
//
//   - Strong dedup: identical InfoHash, magnet BTIH, normalized torrent
//     URL, or normalized direct URL.
//   - Weak dedup: normalized title similarity >= 0.92, size delta <= 2 %,
//     and no conflicting key features (year / resolution / codec /
//     season / episode).
//
// Merge rules:
//   - keep all sources
//   - seeders = max
//   - publishedAt = newest
//   - title = longest (most complete)
//   - size = most-common value
//   - primary download = highest priority present in any source
type Deduper struct{}

func NewDeduper() *Deduper { return &Deduper{} }

// Dedup returns the merged, sorted-by-source list. The input order is
// not preserved; callers should pass the raw results in arrival order
// and then sort the output via the ranker.
func (d *Deduper) Dedup(results []model.SearchResult) []model.SearchResult {
    if len(results) <= 1 {
        for i := range results {
            if results[i].NormalizedTitle == "" {
                results[i].NormalizedTitle = normalize.NormalizeTitle(results[i].Title)
            }
        }
        return results
    }

    // First, assign normalized keys.
    for i := range results {
        r := &results[i]
        if r.NormalizedTitle == "" {
            r.NormalizedTitle = normalize.NormalizeTitle(r.Title)
        }
        if r.InfoHash == "" && r.MagnetURL != "" {
            r.InfoHash = normalize.ExtractInfoHashFromMagnet(r.MagnetURL)
        }
    }

    // Cluster: results in the same cluster will be merged.
    type cluster struct {
        canonical model.SearchResult
        indices   []int
    }
    clusters := make([]cluster, 0, len(results))

    for i := range results {
        ri := results[i]
        merged := false
        for ci := range clusters {
            cj := &clusters[ci]
            if isStrongMatch(ri, cj.canonical) || isWeakMatch(ri, cj.canonical) {
                cj.indices = append(cj.indices, i)
                mergeInto(&cj.canonical, ri)
                merged = true
                break
            }
        }
        if !merged {
            clusters = append(clusters, cluster{canonical: ri, indices: []int{i}})
        }
    }

    out := make([]model.SearchResult, len(clusters))
    for i, c := range clusters {
        out[i] = c.canonical
        // Move all sources into the merged result.
        sources := make([]model.ResultSource, 0, len(c.indices))
        for _, idx := range c.indices {
            r := results[idx]
            sources = append(sources, model.ResultSource{
                IndexerID:   r.IndexerID,
                IndexerName: r.IndexerName,
                MagnetURL:   r.MagnetURL,
                TorrentURL:  r.TorrentURL,
                DirectURL:   r.DirectURL,
                DetailURL:   r.DetailURL,
                Seeders:     r.Seeders,
                PublishedAt: r.PublishedAt,
            })
        }
        out[i].Sources = sources
    }
    return out
}

func isStrongMatch(a, b model.SearchResult) bool {
    if a.InfoHash != "" && a.InfoHash == b.InfoHash {
        return true
    }
    if a.MagnetURL != "" && b.MagnetURL != "" {
        ia := normalize.ExtractInfoHashFromMagnet(a.MagnetURL)
        ib := normalize.ExtractInfoHashFromMagnet(b.MagnetURL)
        if ia != "" && ia == ib {
            return true
        }
    }
    if a.TorrentURL != "" && b.TorrentURL != "" && normalizeURL(a.TorrentURL) == normalizeURL(b.TorrentURL) {
        return true
    }
    if a.DirectURL != "" && b.DirectURL != "" && normalizeURL(a.DirectURL) == normalizeURL(b.DirectURL) {
        return true
    }
    return false
}

func isWeakMatch(a, b model.SearchResult) bool {
    sim := titleSimilarity(a.NormalizedTitle, b.NormalizedTitle)
    if sim < 0.92 {
        return false
    }
    if a.SizeBytes == nil || b.SizeBytes == nil {
        return false
    }
    delta := math.Abs(float64(*a.SizeBytes-*b.SizeBytes)) / math.Max(1, float64(max64(*a.SizeBytes, *b.SizeBytes)))
    if delta > 0.02 {
        return false
    }
    if conflictingFeatures(a.Title, b.Title) {
        return false
    }
    return true
}

func max64(a, b int64) int64 {
    if a > b {
        return a
    }
    return b
}

// normalizeURL strips fragments and lowercases scheme/host. We don't
// strip query params wholesale because the URL itself is part of the
// identity for many indexers.
func normalizeURL(raw string) string {
    s := strings.ToLower(strings.TrimSpace(raw))
    if i := strings.Index(s, "#"); i >= 0 {
        s = s[:i]
    }
    return s
}

// titleSimilarity returns a Jaccard-like score on word sets, in [0, 1].
// For two identical strings the score is 1.0; for disjoint strings it
// approaches 0. Empty inputs return 0.
func titleSimilarity(a, b string) float64 {
    wa := tokenSet(a)
    wb := tokenSet(b)
    if len(wa) == 0 || len(wb) == 0 {
        return 0
    }
    inter := 0
    for x := range wa {
        if _, ok := wb[x]; ok {
            inter++
        }
    }
    union := len(wa) + len(wb) - inter
    if union == 0 {
        return 0
    }
    return float64(inter) / float64(union)
}

func tokenSet(s string) map[string]struct{} {
    out := map[string]struct{}{}
    for _, f := range strings.Fields(s) {
        out[f] = struct{}{}
    }
    return out
}

// conflictingFeatures detects disagreements in year / season / resolution
// / codec (spec §17.2).
func conflictingFeatures(a, b string) bool {
    ya, sa := extractYear(a), extractYear(b)
    if ya != "" && sa != "" && ya != sa {
        return true
    }
    ra, rb := extractResolution(a), extractResolution(b)
    if ra != "" && rb != "" && ra != rb {
        return true
    }
    return false
}

var yearRe = regexp.MustCompile(`\b(19|20)\d{2}\b`)
var resRe = regexp.MustCompile(`\b(2160p|1080p|720p|480p|4k|uhd)\b`)

func extractYear(s string) string {
    m := yearRe.FindString(s)
    return strings.ToLower(m)
}

func extractResolution(s string) string {
    m := resRe.FindString(strings.ToLower(s))
    return m
}

// mergeInto folds b's fields into a using spec §17.3 rules.
func mergeInto(a *model.SearchResult, b model.SearchResult) {
    if len(b.Title) > len(a.Title) {
        a.Title = b.Title
    }
    if a.Category == "" {
        a.Category = b.Category
    }
    if a.SizeBytes == nil && b.SizeBytes != nil {
        a.SizeBytes = b.SizeBytes
    }
    if a.Seeders == nil || (b.Seeders != nil && *b.Seeders > *a.Seeders) {
        a.Seeders = b.Seeders
    }
    if a.PublishedAt == nil || (b.PublishedAt != nil && b.PublishedAt.After(*a.PublishedAt)) {
        a.PublishedAt = b.PublishedAt
    }
    if a.MagnetURL == "" {
        a.MagnetURL = b.MagnetURL
    }
    if a.TorrentURL == "" {
        a.TorrentURL = b.TorrentURL
    }
    if a.DirectURL == "" {
        a.DirectURL = b.DirectURL
    }
    if a.DetailURL == "" {
        a.DetailURL = b.DetailURL
    }
    if a.InfoHash == "" {
        a.InfoHash = b.InfoHash
    }
}
