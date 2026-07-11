package search

import (
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/local/easysearch/backend/internal/model"
)

// Ranker scores and sorts results per spec §18.
//
//   score = text_match  * 0.65
//         + seed        * 0.12
//         + freshness   * 0.08
//         + source      * 0.08
//         + completeness* 0.07
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
	return textMatchScore(q.Keyword, res.Title)*0.65 +
		seedScore(res.Seeders)*0.12 +
		freshnessScore(res.PublishedAt, now)*0.08 +
		sourceScore(res.Sources)*0.08 +
		completenessScore(res)*0.07
}

// textMatchScore returns 1.0 for an exact (case-insensitive, whitespace-
// normalized) match, handles common release punctuation and Chinese
// substrings, discounts partial matches, and avoids short-token false positives.
func textMatchScore(keyword, title string) float64 {
	kw := normalizeSearchText(keyword)
	if kw == "" {
		return 0
	}
	titleNorm := normalizeSearchText(title)
	if titleNorm == kw {
		return 1
	}
	if containsHan(kw) && strings.Contains(titleNorm, kw) {
		return .98
	}
	if strings.Contains(" "+titleNorm+" ", " "+kw+" ") {
		return .98
	}
	kwTokens := uniqueMatchTokens(strings.Fields(kw))
	if len(kwTokens) == 0 {
		return 0
	}
	titleTokens := strings.Fields(titleNorm)
	titleSet := map[string]bool{}
	for _, t := range titleTokens {
		titleSet[t] = true
	}
	matched := 0
	for _, token := range kwTokens {
		if titleSet[token] {
			matched++
			continue
		}
		if utf8RuneCount(token) >= 3 {
			for _, candidate := range titleTokens {
				if strings.HasPrefix(candidate, token) || strings.HasPrefix(token, candidate) {
					matched++
					break
				}
			}
		}
	}
	coverage := float64(matched) / float64(len(kwTokens))
	if coverage == 1 {
		return .92
	}
	return coverage * .72
}

func normalizeSearchText(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}
func containsHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
func utf8RuneCount(s string) int { return len([]rune(s)) }
func uniqueMatchTokens(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
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
