package api

import (
    "context"
    "encoding/json"
    "io"
    "log/slog"
    "net/http/httptest"
    "strings"
    "testing"
    "time"

    "github.com/local/easysearch/backend/internal/model"
    "github.com/local/easysearch/backend/internal/search"
)

func ptrInt64(v int64) *int64 { return &v }

// TestMultiIndexerAggregation verifies that 3 mock indexers' results
// are deduped, sorted, and stream over SSE. Two indexers share an
// infohash, so we expect 7 raw results collapsing to 6 merged.
func TestMultiIndexerAggregation(t *testing.T) {
    jobs := NewMockIndexers()
    o := search.NewOrchestrator(search.Config{
        PerIndexerTimeout: 2 * time.Second,
        TotalTimeout:      5 * time.Second,
        MaxConcurrent:     3,
    }, jobs, slog.New(slog.NewJSONHandler(io.Discard, nil)))

    s := o.NewSession("s1", model.SearchQuery{Keyword: "hello", Category: "all", Sort: "relevance"})
    o.Run(context.Background(), s)

    var raw, merged int
    var resultBatches []search.ResultBatch
    var sawIndexers []string
    var sawFailed bool
    for ev := range s.Events {
        switch ev.Type {
        case search.EventResultsMerged:
            d := ev.Data.(search.ResultsMergedData)
            raw = d.RawCount
            merged = d.MergedCount
        case search.EventIndexerResult:
            resultBatches = append(resultBatches, ev.Data.(search.ResultBatch))
        case search.EventIndexerStarted:
            sawIndexers = append(sawIndexers, ev.Data.(search.IndexerStartedData).IndexerID)
        case search.EventIndexerFailed:
            sawFailed = true
        case search.EventSessionCompleted:
            // final event
        }
    }
    if !sawFailed {
        t.Error("expected one indexer to fail (gamma is hard-coded to fail in NewMockIndexers)")
    }
    if raw < 6 {
        t.Errorf("expected at least 6 raw results (2 indexers x 3), got %d", raw)
    }
    // Strong + weak dedup: alpha and beta share an infohash AND share
    // title+size on items 2 and 3, so 6 raw collapses to 3 merged. Plus
    // gamma fails entirely, contributing 0.
    if merged != 3 {
        t.Errorf("merged count should be 3 (all alpha/beta results overlap), got %d", merged)
    }
    // All 3 indexers should have been started.
    if len(sawIndexers) != 3 {
        t.Errorf("expected 3 indexer_started events, got %d (%v)", len(sawIndexers), sawIndexers)
    }
    // Per-result batches should have one result each.
    if len(resultBatches) != merged {
        t.Errorf("expected %d result batches, got %d", merged, len(resultBatches))
    }
    // Results are sorted by score (ranker ran).
    for i := 1; i < len(resultBatches); i++ {
        if len(resultBatches[i-1].Results) == 0 || len(resultBatches[i].Results) == 0 {
            continue
        }
        prev := resultBatches[i-1].Results[0].Score
        cur := resultBatches[i].Results[0].Score
        if cur > prev {
            t.Errorf("results not sorted by score: %f > %f", cur, prev)
        }
    }
}

func TestMultiIndexerEndToEndAPI(t *testing.T) {
    h := NewSearchHandler(slog.New(slog.NewJSONHandler(io.Discard, nil)))

    rr := httptest.NewRecorder()
    body := strings.NewReader(`{"keyword":"e2e","category":"all","sort":"relevance"}`)
    h.CreateSession(rr, httptest.NewRequest("POST", "/api/v1/search/sessions", body))
    if rr.Code != 201 {
        t.Fatalf("create want 201, got %d body=%s", rr.Code, rr.Body.String())
    }
    var resp createSearchResponse
    json.NewDecoder(rr.Body).Decode(&resp)

    sseRR := httptest.NewRecorder()
    req := httptest.NewRequest("GET", resp.StreamURL, nil)
    req.SetPathValue("sessionId", resp.SessionID)
    h.StreamEvents(sseRR, req)

    events := parseSSE(t, sseRR.Body.String())
    mustHaveEvent(t, events, "session_started")
    mustHaveEvent(t, events, "indexer_failed") // gamma always fails
    mustHaveEvent(t, events, "results_merged")
    mustHaveEvent(t, events, "session_completed")
}
