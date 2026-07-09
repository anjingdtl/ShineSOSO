package search

import (
    "math"
    "sort"
    "strings"
    "time"

    "github.com/local/easysearch/backend/internal/model"
)

// Ranker scores and sorts results per spec §18.
//
//   score = text_match  * 0.45
//         + seed        * 0.20
//         + freshness   * 0.15
//         + source      * 0.10
//         + completeness* 0.10
//
// Sort is stable; ties break by publishedAt desc, then seeders desc,
// then title asc.
type Ranker struct {
    Now func() time.Time
}

func NewRanker() *Ranker { return &Ranker{Now: time.Now} }

// Rank mutates results in place, assigning Score and sorting them.
func (r *Ranker) Rank(query model.SearchQuery, results []model.SearchResult) {
    if r.Now == nil {
        r.Now = time.Now
    }
    now := r.Now()
    for i := range results {
        results[i].Score = r.score(query, results[i], now)
    }
    sort.SliceStable(results, func(i, j int) bool {
        if results[i].Score != results[j].Score {
            return results[i].Score > results[j].Score
        }
        // Tie-breakers.
        ai, aj := results[i].PublishedAt, results[j].PublishedAt
        if ai != nil && aj != nil && !ai.Equal(*aj) {
            return ai.After(*aj)
        }
        si, sj := results[i].Seeders, results[j].Seeders
        if si != nil && sj != nil && *si != *sj {
            return *si > *sj
        }
        return strings.ToLower(results[i].Title) < strings.ToLower(results[j].Title)
    })
}

func (r *Ranker) score(q model.SearchQuery, res model.SearchResult, now time.Time) float64 {
    return textMatchScore(q.Keyword, res.Title)*0.45 +
        seedScore(res.Seeders)*0.20 +
        freshnessScore(res.PublishedAt, now)*0.15 +
        sourceScore(res.Sources)*0.10 +
        completenessScore(res)*0.10
}

// textMatchScore returns 1.0 for an exact (case-insensitive, whitespace-
// normalized) match, lower for partial word matches, and ~0 for no
// overlap. The implementation is intentionally simple: it measures
// token overlap between keyword and title.
func textMatchScore(keyword, title string) float64 {
    kw := strings.ToLower(strings.TrimSpace(keyword))
    if kw == "" {
        return 0
    }
    titleLower := strings.ToLower(title)
    if strings.Contains(titleLower, kw) {
        return 1.0
    }
    kwTokens := strings.Fields(kw)
    if len(kwTokens) == 0 {
        return 0
    }
    titleTokens := map[string]struct{}{}
    for _, t := range strings.Fields(titleLower) {
        titleTokens[t] = struct{}{}
    }
    matched := 0
    for _, t := range kwTokens {
        if _, ok := titleTokens[t]; ok {
            matched++
        }
    }
    return float64(matched) / float64(len(kwTokens))
}

// seedScore: log-normalized, in [0, 1]. 1 seed -> 0, 10 -> ~0.5, 1000 -> 1.
func seedScore(seeders *int) float64 {
    if seeders == nil || *seeders <= 0 {
        return 0
    }
    return math.Min(1, math.Log10(float64(*seeders)+1)/3)
}

// freshnessScore: linear decay from 1.0 at "now" to 0 at 365 days.
func freshnessScore(published *time.Time, now time.Time) float64 {
    if published == nil {
        return 0
    }
    age := now.Sub(*published)
    if age < 0 {
        age = 0
    }
    if age > 365*24*time.Hour {
        return 0
    }
    return 1 - float64(age)/(365*24*float64(time.Hour))
}

// sourceScore: more sources -> higher score, capped at 5.
func sourceScore(sources []model.ResultSource) float64 {
    if len(sources) == 0 {
        return 0
    }
    return math.Min(1, float64(len(sources))/5)
}

// completenessScore: fraction of important fields that are present.
func completenessScore(r model.SearchResult) float64 {
    total := 4
    present := 0
    if r.SizeBytes != nil {
        present++
    }
    if r.Seeders != nil {
        present++
    }
    if r.PublishedAt != nil {
        present++
    }
    if r.MagnetURL != "" || r.TorrentURL != "" || r.DirectURL != "" {
        present++
    }
    return float64(present) / float64(total)
}
