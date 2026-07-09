package search

import (
    "context"
    "log/slog"
    "sync"
    "time"

    "github.com/local/easysearch/backend/internal/indexer"
    "github.com/local/easysearch/backend/internal/model"
)

// Default limits (spec §15.2).
const (
    DefaultPerIndexerTimeout = 15 * time.Second
    DefaultTotalTimeout      = 30 * time.Second
    DefaultMaxConcurrent     = 6
    DefaultPerIndexerResults = 100
    DefaultMaxRawResults     = 1000
)

// Config controls orchestrator behavior. Zero values are replaced with
// the defaults above.
type Config struct {
    PerIndexerTimeout time.Duration
    TotalTimeout      time.Duration
    MaxConcurrent     int
    PerIndexerResults int
    MaxRawResults     int
}

func (c Config) withDefaults() Config {
    if c.PerIndexerTimeout <= 0 {
        c.PerIndexerTimeout = DefaultPerIndexerTimeout
    }
    if c.TotalTimeout <= 0 {
        c.TotalTimeout = DefaultTotalTimeout
    }
    if c.MaxConcurrent <= 0 {
        c.MaxConcurrent = DefaultMaxConcurrent
    }
    if c.PerIndexerResults <= 0 {
        c.PerIndexerResults = DefaultPerIndexerResults
    }
    if c.MaxRawResults <= 0 {
        c.MaxRawResults = DefaultMaxRawResults
    }
    return c
}

// IndexerJob pairs an adapter with its display name for status events.
type IndexerJob struct {
    Adapter indexer.IndexerAdapter
    Name    string
}

// Session holds the live state of a single search.
type Session struct {
    ID        string
    Query     model.SearchQuery
    StartedAt time.Time
    Events    chan Event
    cancel    context.CancelFunc
}

// Run executes the search and emits events on session.Events. The
// channel is closed when the session finishes (success, cancel, or
// total-timeout). Run blocks until the session is done; callers should
// invoke it in a goroutine and consume from session.Events.
func (o *Orchestrator) Run(ctx context.Context, s *Session) {
    cfg := o.cfg.withDefaults()
    octx, cancel := context.WithTimeout(ctx, cfg.TotalTimeout)
    s.cancel = cancel
    defer func() {
        cancel()
        close(s.Events)
    }()

    startedAt := time.Now()
    o.logger.Info("search session started",
        "session", s.ID,
        "indexers", len(o.jobs),
        "keyword", s.Query.Keyword,
    )

    s.Events <- Event{
        Type:      EventSessionStarted,
        SessionID: s.ID,
        Timestamp: startedAt,
        Data: SessionStartedData{
            SessionID:     s.ID,
            TotalIndexers: len(o.jobs),
        },
    }

    if len(o.jobs) == 0 {
        s.Events <- Event{
            Type:      EventSessionCompleted,
            SessionID: s.ID,
            Timestamp: time.Now(),
            Data: SessionCompletedData{TotalMs: 0, MergedCount: 0, RawCount: 0},
        }
        return
    }

    // Concurrency cap
    sem := make(chan struct{}, cfg.MaxConcurrent)
    var wg sync.WaitGroup
    var rawCount, mergedCount int
    var mu sync.Mutex

    for _, job := range o.jobs {
        job := job
        select {
        case <-octx.Done():
            // total timeout / cancel: stop dispatching
            goto waitAll
        case sem <- struct{}{}:
        }
        wg.Add(1)
        go func() {
            defer wg.Done()
            defer func() { <-sem }()
            o.runOne(octx, s, job, cfg, &rawCount, &mergedCount, &mu)
        }()
    }
waitAll:
    wg.Wait()

    // Emit session_cancelled if the parent context was cancelled.
    if octx.Err() != nil {
        s.Events <- Event{
            Type:      EventSessionCancelled,
            SessionID: s.ID,
            Timestamp: time.Now(),
            Data:      SessionCancelledData{SessionID: s.ID},
        }
        return
    }

    s.Events <- Event{
        Type:      EventSessionCompleted,
        SessionID: s.ID,
        Timestamp: time.Now(),
        Data: SessionCompletedData{
            TotalMs:     time.Since(startedAt).Milliseconds(),
            MergedCount: mergedCount,
            RawCount:    rawCount,
        },
    }
}

// Orchestrator fans out a search to multiple indexers and emits events.
type Orchestrator struct {
    cfg    Config
    jobs   []IndexerJob
    logger *slog.Logger
}

// New returns a ready-to-Run orchestrator. Use Run to start the search
// in a goroutine.
func (o *Orchestrator) NewSession(id string, q model.SearchQuery) *Session {
    return &Session{
        ID:        id,
        Query:     q,
        StartedAt: time.Now(),
        Events:    make(chan Event, 64),
    }
}

// Cancel terminates a running session. Safe to call from any goroutine.
func (s *Session) Cancel() {
    if s.cancel != nil {
        s.cancel()
    }
}

func (o *Orchestrator) runOne(
    ctx context.Context,
    s *Session,
    job IndexerJob,
    cfg Config,
    rawCount, mergedCount *int,
    mu *sync.Mutex,
) {
    startedAt := time.Now()
    s.Events <- Event{
        Type:      EventIndexerStarted,
        SessionID: s.ID,
        Timestamp: startedAt,
        Data: IndexerStartedData{
            IndexerID:   job.Adapter.ID(),
            IndexerName: job.Name,
        },
    }

    // Per-indexer timeout
    ictx, cancel := context.WithTimeout(ctx, cfg.PerIndexerTimeout)
    defer cancel()

    results, err := job.Adapter.Search(ictx, s.Query)
    if err != nil {
        if ictx.Err() != nil {
            s.Events <- Event{
                Type:      EventIndexerCompleted,
                SessionID: s.ID,
                Timestamp: time.Now(),
                Data: IndexerCompletedData{
                    IndexerID:   job.Adapter.ID(),
                    Status:      "timeout",
                    ResultCount: 0,
                    DurationMs:  time.Since(startedAt).Milliseconds(),
                },
            }
        } else {
            s.Events <- Event{
                Type:      EventIndexerFailed,
                SessionID: s.ID,
                Timestamp: time.Now(),
                Data: IndexerFailedData{
                    IndexerID: job.Adapter.ID(),
                    Code:      "INDEXER_NETWORK_ERROR",
                    Message:   err.Error(),
                },
            }
            s.Events <- Event{
                Type:      EventIndexerCompleted,
                SessionID: s.ID,
                Timestamp: time.Now(),
                Data: IndexerCompletedData{
                    IndexerID:   job.Adapter.ID(),
                    Status:      "error",
                    ResultCount: 0,
                    DurationMs:  time.Since(startedAt).Milliseconds(),
                },
            }
        }
        return
    }
    if len(results) > cfg.PerIndexerResults {
        results = results[:cfg.PerIndexerResults]
    }

    mu.Lock()
    *rawCount += len(results)
    *mergedCount += len(results) // Phase 3 will dedup here
    raw, merged := *rawCount, *mergedCount
    mu.Unlock()

    s.Events <- Event{
        Type:      EventIndexerCompleted,
        SessionID: s.ID,
        Timestamp: time.Now(),
        Data: IndexerCompletedData{
            IndexerID:   job.Adapter.ID(),
            Status:      mapStatus(results),
            ResultCount: len(results),
            DurationMs:  time.Since(startedAt).Milliseconds(),
        },
    }
    s.Events <- Event{
        Type:      EventResultsMerged,
        SessionID: s.ID,
        Timestamp: time.Now(),
        Data:      ResultsMergedData{MergedCount: merged, RawCount: raw},
    }
}

func mapStatus(results []model.SearchResult) string {
    if len(results) == 0 {
        return "empty"
    }
    return "success"
}

// NewOrchestrator builds an orchestrator for one Run. Use it once per
// session to keep the lifetime obvious.
func NewOrchestrator(cfg Config, jobs []IndexerJob, logger *slog.Logger) *Orchestrator {
    if logger == nil {
        logger = slog.Default()
    }
    if len(jobs) == 0 {
        // avoid surprising nil later
        jobs = []IndexerJob{}
    }
    return &Orchestrator{cfg: cfg, jobs: jobs, logger: logger}
}
