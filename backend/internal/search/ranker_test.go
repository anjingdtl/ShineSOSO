package search

import (
	"testing"
	"time"

	"github.com/local/easysearch/backend/internal/model"
)

func TestTextMatchScoreExact(t *testing.T) {
	if got := textMatchScore("ubuntu", "Ubuntu 22.04 LTS"); got < 0.95 {
		t.Errorf("whole-token match should be high, got %v", got)
	}
}

func TestTextMatchScorePartial(t *testing.T) {
	if got := textMatchScore("ubuntu lts", "Ubuntu 22.04"); got < 0.3 || got > 0.5 {
		t.Errorf("partial match should be discounted, got %v", got)
	}
}

func TestTextMatchScoreNormalizesReleasePunctuation(t *testing.T) {
	if got := textMatchScore("The Matrix 1999", "The.Matrix.1999.1080p.BluRay"); got < .9 {
		t.Fatalf("punctuated release title should match, got %v", got)
	}
}

func TestTextMatchScoreAvoidsSubstringFalsePositive(t *testing.T) {
	if got := textMatchScore("cat", "Educational documentary"); got != 0 {
		t.Fatalf("short token must not match inside another word, got %v", got)
	}
}

func TestTextMatchScoreSupportsChineseSubstring(t *testing.T) {
	if got := textMatchScore("流浪地球", "流浪地球 2 2160p"); got < .95 {
		t.Fatalf("Chinese phrase should match, got %v", got)
	}
}

func TestTextMatchScoreNoMatch(t *testing.T) {
	if got := textMatchScore("xxx", "ubuntu 22.04"); got != 0.0 {
		t.Errorf("no match, want 0, got %v", got)
	}
}

func TestSeedScore(t *testing.T) {
	zero := 0
	one := 1
	hundred := 100
	if got := seedScore(&zero); got != 0 {
		t.Errorf("0 seeders should score 0, got %v", got)
	}
	if got := seedScore(&one); got > 0.2 {
		t.Errorf("1 seeder should score <= 0.2, got %v", got)
	}
	if got := seedScore(&hundred); got < 0.5 {
		t.Errorf("100 seeders should score >= 0.5, got %v", got)
	}
}

func TestFreshnessScore(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	if got := freshnessScore(&now, now); got != 1.0 {
		t.Errorf("now should score 1.0, got %v", got)
	}
	old := now.AddDate(-1, 0, 0)
	if got := freshnessScore(&old, now); got != 0.0 {
		t.Errorf("1-year-old should score 0, got %v", got)
	}
	halfYear := now.AddDate(0, -6, 0)
	if got := freshnessScore(&halfYear, now); got < 0.4 || got > 0.6 {
		t.Errorf("half-year should score ~0.5, got %v", got)
	}
}

func TestSourceScore(t *testing.T) {
	if got := sourceScore(nil); got != 0 {
		t.Errorf("no sources should score 0")
	}
	if got := sourceScore(make([]model.ResultSource, 1)); got <= 0 {
		t.Errorf("1 source should score > 0")
	}
	if got := sourceScore(make([]model.ResultSource, 5)); got != 1.0 {
		t.Errorf("5 sources should cap at 1.0, got %v", got)
	}
	if got := sourceScore(make([]model.ResultSource, 10)); got != 1.0 {
		t.Errorf("10 sources should cap at 1.0, got %v", got)
	}
}

func TestRankerStableTieBreaker(t *testing.T) {
	r := NewRanker()
	r.Now = func() time.Time { return time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC) }
	pub := r.Now()
	seeders := 100
	sz := int64(1_000_000)
	in := []model.SearchResult{
		{Title: "Z", PublishedAt: &pub, Seeders: &seeders, SizeBytes: &sz},
		{Title: "A", PublishedAt: &pub, Seeders: &seeders, SizeBytes: &sz},
	}
	r.Rank(model.SearchQuery{Keyword: ""}, in)
	if in[0].Title != "A" {
		t.Errorf("tie-breaker should put A first, got %s", in[0].Title)
	}
}

func TestRankerRespectsScore(t *testing.T) {
	r := NewRanker()
	r.Now = func() time.Time { return time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC) }
	pub := r.Now()
	s100 := 100
	s10 := 10
	sz := int64(1_000_000)
	in := []model.SearchResult{
		{Title: "Low", PublishedAt: &pub, Seeders: &s10, SizeBytes: &sz},
		{Title: "High", PublishedAt: &pub, Seeders: &s100, SizeBytes: &sz},
	}
	r.Rank(model.SearchQuery{Keyword: ""}, in)
	if in[0].Title != "High" {
		t.Errorf("High should be first, got %s", in[0].Title)
	}
}
