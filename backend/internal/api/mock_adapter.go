package api

import (
    "context"
    "time"

    "github.com/local/easysearch/backend/internal/indexer"
    "github.com/local/easysearch/backend/internal/model"
)

// mockAdapter is a built-in fake indexer used in Phase 2 (and the dev
// smoke tests) before the catalog-driven selection lands in Phase 4.
// It returns three deterministic results for any keyword and never
// touches the network.
type mockAdapter struct{}

func newMockAdapter() *mockAdapter { return &mockAdapter{} }

func (m *mockAdapter) ID() string   { return "mock" }
func (m *mockAdapter) Name() string { return "示例索引器" }

func (m *mockAdapter) Test(_ context.Context) indexer.TestResult {
    return indexer.TestResult{OK: true, StatusCode: 200, ResultCount: 3, DurationMs: 5}
}

func (m *mockAdapter) Search(_ context.Context, q model.SearchQuery) ([]model.SearchResult, error) {
    // 5ms delay to feel like a real network call.
    time.Sleep(5 * time.Millisecond)
    var size int64 = 8_400_000_000
    seeders := 326
    published := time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)
    return []model.SearchResult{
        {
            ID:          "mock-1",
            Title:       "示例电影 " + q.Keyword,
            Category:    "movie",
            SizeBytes:   &size,
            Seeders:     &seeders,
            PublishedAt: &published,
            MagnetURL:   "magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567&dn=" + q.Keyword,
            DetailURL:   "https://example.com/torrent/1",
            IndexerID:   "mock",
            IndexerName: m.Name(),
        },
        {
            ID:          "mock-2",
            Title:       "示例剧集 " + q.Keyword,
            Category:    "tv",
            SizeBytes:   &size,
            Seeders:     &seeders,
            PublishedAt: &published,
            TorrentURL:  "https://example.com/torrent/2.torrent",
            DetailURL:   "https://example.com/torrent/2",
            IndexerID:   "mock",
            IndexerName: m.Name(),
        },
        {
            ID:          "mock-3",
            Title:       "示例软件 " + q.Keyword,
            Category:    "software",
            SizeBytes:   &size,
            Seeders:     &seeders,
            PublishedAt: &published,
            DirectURL:   "https://example.com/download/3.zip",
            DetailURL:   "https://example.com/torrent/3",
            IndexerID:   "mock",
            IndexerName: m.Name(),
        },
    }, nil
}

var _ indexer.IndexerAdapter = (*mockAdapter)(nil)
