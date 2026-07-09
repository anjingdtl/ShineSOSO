package search

import (
    "context"
    "io"
    "log/slog"
    "testing"
    "time"

    "github.com/local/easysearch/backend/internal/indexer"
    "github.com/local/easysearch/backend/internal/model"
)

type stubAdapter struct {
    id      string
    delay   time.Duration
    err     error
    results []model.SearchResult
}

func (s *stubAdapter) ID() string { return s.id }
func (s *stubAdapter) Test(_ context.Context) indexer.TestResult {
    return indexer.TestResult{OK: true}
}
func (s *stubAdapter) Search(ctx context.Context, _ model.SearchQuery) ([]model.SearchResult, error) {
    select {
    case <-time.After(s.delay):
    case <-ctx.Done():
        return nil, ctx.Err()
    }
    return s.results, s.err
}

func TestOrchestratorSingleIndexerSuccess(t *testing.T) {
    a := &stubAdapter{
        id: "a1",
        results: []model.SearchResult{
            {ID: "r1", Title: "Result 1", Category: "movie"},
        },
    }
    o := NewOrchestrator(Config{PerIndexerTimeout: time.Second}, []IndexerJob{{Adapter: a, Name: "stub"}}, slog.New(slog.NewJSONHandler(io.Discard, nil)))
    s := o.NewSession("s1", model.SearchQuery{Keyword: "test"})
    o.Run(context.Background(), s)

    var got []EventType
    var completed IndexerCompletedData
    for e := range s.Events {
        got = append(got, e.Type)
        if e.Type == EventIndexerCompleted {
            completed = e.Data.(IndexerCompletedData)
        }
    }
    if len(got) < 2 {
        t.Fatalf("expected at least 2 events, got %d: %v", len(got), got)
    }
    if got[0] != EventSessionStarted {
        t.Errorf("first event should be session_started, got %s", got[0])
    }
    if completed.Status != "success" {
        t.Errorf("want status success, got %s", completed.Status)
    }
    if completed.ResultCount != 1 {
        t.Errorf("want 1 result, got %d", completed.ResultCount)
    }
    if got[len(got)-1] != EventSessionCompleted {
        t.Errorf("last event should be session_completed, got %s", got[len(got)-1])
    }
}

func TestOrchestratorSingleIndexerEmpty(t *testing.T) {
    a := &stubAdapter{id: "a1"}
    o := NewOrchestrator(Config{}, []IndexerJob{{Adapter: a, Name: "empty"}}, nil)
    s := o.NewSession("s1", model.SearchQuery{Keyword: "test"})
    o.Run(context.Background(), s)
    var completed IndexerCompletedData
    for e := range s.Events {
        if e.Type == EventIndexerCompleted {
            completed = e.Data.(IndexerCompletedData)
        }
    }
    if completed.Status != "empty" {
        t.Errorf("want status empty, got %s", completed.Status)
    }
}

func TestOrchestratorTimeoutEmitsTimeout(t *testing.T) {
    a := &stubAdapter{id: "slow", delay: 200 * time.Millisecond}
    o := NewOrchestrator(Config{PerIndexerTimeout: 20 * time.Millisecond}, []IndexerJob{{Adapter: a, Name: "slow"}}, nil)
    s := o.NewSession("s1", model.SearchQuery{Keyword: "test"})
    o.Run(context.Background(), s)
    var completed IndexerCompletedData
    for e := range s.Events {
        if e.Type == EventIndexerCompleted {
            completed = e.Data.(IndexerCompletedData)
        }
    }
    if completed.Status != "timeout" {
        t.Errorf("want status timeout, got %s", completed.Status)
    }
}

func TestOrchestratorNoIndexers(t *testing.T) {
    o := NewOrchestrator(Config{}, nil, nil)
    s := o.NewSession("s1", model.SearchQuery{Keyword: "test"})
    o.Run(context.Background(), s)
    var types []EventType
    for e := range s.Events {
        types = append(types, e.Type)
    }
    if len(types) != 2 {
        t.Fatalf("expected 2 events (started+completed), got %v", types)
    }
}

func TestOrchestratorErrorEmitsFailedAndCompleted(t *testing.T) {
    a := &stubAdapter{id: "bad", err: context.DeadlineExceeded}
    o := NewOrchestrator(Config{PerIndexerTimeout: time.Second}, []IndexerJob{{Adapter: a, Name: "bad"}}, nil)
    s := o.NewSession("s1", model.SearchQuery{Keyword: "test"})
    o.Run(context.Background(), s)
    var failed, completed IndexerCompletedData
    var sawFailed bool
    for e := range s.Events {
        switch e.Type {
        case EventIndexerFailed:
            sawFailed = true
        case EventIndexerCompleted:
            completed = e.Data.(IndexerCompletedData)
        }
        _ = failed
    }
    if !sawFailed {
        t.Error("expected indexer_failed event")
    }
    if completed.Status != "error" {
        t.Errorf("want status error, got %s", completed.Status)
    }
}
