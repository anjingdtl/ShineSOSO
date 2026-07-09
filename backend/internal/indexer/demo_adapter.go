package indexer

import (
	"context"
	"fmt"
	"time"

	"github.com/local/easysearch/backend/internal/model"
)

// demoAdapter is the deterministic mock used by built-in definitions.
// It mirrors the shape of the Phase 3 mockAdapter so the dedup / ranking
// pipelines can be exercised end-to-end against real catalog entries.
type demoAdapter struct {
	id         string
	name       string
	delay      time.Duration
	fail       bool
}

func newDemoAdapter(defID, displayName string) *demoAdapter {
	a := &demoAdapter{id: defID, name: displayName}
	switch defID {
	case "demo-alpha":
		a.delay = 5 * time.Millisecond
	case "demo-beta":
		a.delay = 20 * time.Millisecond
	case "demo-gamma":
		a.delay = 80 * time.Millisecond
		a.fail = true
	}
	return a
}

func (a *demoAdapter) ID() string   { return a.id }
func (a *demoAdapter) Name() string { return a.name }

func (a *demoAdapter) Test(_ context.Context) TestResult {
	if a.fail {
		return TestResult{OK: false, ErrorCode: "INDEXER_HTTP_ERROR", ErrorMessage: "demo: always fails"}
	}
	return TestResult{OK: true, StatusCode: 200, ResultCount: 3, DurationMs: 5}
}

func (a *demoAdapter) Search(_ context.Context, q model.SearchQuery) ([]model.SearchResult, error) {
	if a.delay > 0 {
		time.Sleep(a.delay)
	}
	if a.fail {
		return nil, fmt.Errorf("demo: timeout")
	}
	var size int64 = 8_400_000_000
	seeders := 326
	published := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
	sharedMagnet := "magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567&dn=" + q.Keyword
	return []model.SearchResult{
		{
			ID:          fmt.Sprintf("%s-1", a.id),
			Title:       "示例电影 " + q.Keyword,
			Category:    "movie",
			SizeBytes:   &size,
			Seeders:     &seeders,
			PublishedAt: &published,
			MagnetURL:   sharedMagnet,
			DetailURL:   "https://example.com/" + a.id + "/1",
			IndexerID:   a.id,
			IndexerName: a.name,
		},
		{
			ID:          fmt.Sprintf("%s-2", a.id),
			Title:       "示例剧集 " + q.Keyword,
			Category:    "tv",
			SizeBytes:   &size,
			Seeders:     &seeders,
			PublishedAt: &published,
			TorrentURL:  "https://example.com/" + a.id + "/2.torrent",
			DetailURL:   "https://example.com/" + a.id + "/2",
			IndexerID:   a.id,
			IndexerName: a.name,
		},
		{
			ID:          fmt.Sprintf("%s-3", a.id),
			Title:       "示例软件 " + q.Keyword,
			Category:    "software",
			SizeBytes:   &size,
			Seeders:     &seeders,
			PublishedAt: &published,
			DirectURL:   "https://example.com/" + a.id + "/3.zip",
			DetailURL:   "https://example.com/" + a.id + "/3",
			IndexerID:   a.id,
			IndexerName: a.name,
		},
	}, nil
}