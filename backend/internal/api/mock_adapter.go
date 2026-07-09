package api

import (
    "context"
    "fmt"
    "time"

    "github.com/local/easysearch/backend/internal/indexer"
    "github.com/local/easysearch/backend/internal/model"
    "github.com/local/easysearch/backend/internal/search"
)

// mockAdapter is a built-in fake indexer used in Phase 2/3 (and the
// dev smoke tests) before the catalog-driven selection lands in Phase 4.
// It returns three deterministic results for any keyword and never
// touches the network. The SetDelay controls latency; the SetResults
// hook lets tests inject overlapping results to exercise dedup.
type mockAdapter struct {
    id      string
    name    string
    delay   time.Duration
    fail    bool
    results []model.SearchResult
}

func newMockAdapter() *mockAdapter { return &mockAdapter{id: "mock", name: "示例索引器"} }

// NewMockIndexers returns three demo indexers for Phase 3 demos and
// integration tests. Their results deliberately overlap on the first
// item so the dedup path is exercised.
func NewMockIndexers() []search.IndexerJob {
    return []search.IndexerJob{
        {Adapter: newSlowMock("alpha", "示例 A", 5*time.Millisecond, false), Name: "示例 A"},
        {Adapter: newSlowMock("beta", "示例 B", 20*time.Millisecond, false), Name: "示例 B"},
        {Adapter: newSlowMock("gamma", "示例 C", 80*time.Millisecond, true), Name: "示例 C"},
    }
}

func newSlowMock(id, name string, delay time.Duration, fail bool) *mockAdapter {
    return &mockAdapter{id: id, name: name, delay: delay, fail: fail}
}

func (m *mockAdapter) ID() string   { return m.id }
func (m *mockAdapter) Name() string { return m.name }

func (m *mockAdapter) Test(_ context.Context) indexer.TestResult {
    if m.fail {
        return indexer.TestResult{OK: false, ErrorCode: "INDEXER_HTTP_ERROR", ErrorMessage: "demo: always fails"}
    }
    return indexer.TestResult{OK: true, StatusCode: 200, ResultCount: 3, DurationMs: 5}
}

func (m *mockAdapter) Search(_ context.Context, q model.SearchQuery) ([]model.SearchResult, error) {
    if m.delay > 0 {
        time.Sleep(m.delay)
    }
    if m.fail {
        return nil, fmt.Errorf("demo: timeout")
    }
    if m.results != nil {
        return m.results, nil
    }
    var size int64 = 8_400_000_000
    seeders := 326
    published := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
    // A shared infohash across indexers to exercise strong dedup.
    sharedMagnet := "magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567&dn=" + q.Keyword
    return []model.SearchResult{
        {
            ID:          fmt.Sprintf("%s-1", m.id),
            Title:       "示例电影 " + q.Keyword,
            Category:    "movie",
            SizeBytes:   &size,
            Seeders:     &seeders,
            PublishedAt: &published,
            MagnetURL:   sharedMagnet,
            DetailURL:   "https://example.com/" + m.id + "/1",
            IndexerID:   m.id,
            IndexerName: m.name,
        },
        {
            ID:          fmt.Sprintf("%s-2", m.id),
            Title:       "示例剧集 " + q.Keyword,
            Category:    "tv",
            SizeBytes:   &size,
            Seeders:     &seeders,
            PublishedAt: &published,
            TorrentURL:  "https://example.com/" + m.id + "/2.torrent",
            DetailURL:   "https://example.com/" + m.id + "/2",
            IndexerID:   m.id,
            IndexerName: m.name,
        },
        {
            ID:          fmt.Sprintf("%s-3", m.id),
            Title:       "示例软件 " + q.Keyword,
            Category:    "software",
            SizeBytes:   &size,
            Seeders:     &seeders,
            PublishedAt: &published,
            DirectURL:   "https://example.com/" + m.id + "/3.zip",
            DetailURL:   "https://example.com/" + m.id + "/3",
            IndexerID:   m.id,
            IndexerName: m.name,
        },
    }, nil
}

var _ indexer.IndexerAdapter = (*mockAdapter)(nil)
