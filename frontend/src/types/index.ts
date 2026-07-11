// Types shared between frontend and backend.
//
// Mirrors backend/internal/api/system_handler.go and the SSE events
// defined in backend/internal/search/event.go (filled in Phase 2+).

export type IndexerHealth = 'healthy' | 'degraded' | 'unhealthy' | 'unknown' | 'disabled';

export type IndexerStatus = 'pending' | 'running' | 'success' | 'empty' | 'timeout' | 'error' | 'cancelled';

export type Category = 'all' | 'movie' | 'tv' | 'music' | 'game' | 'software' | 'book' | 'anime' | 'other';

export type SortMode = 'relevance' | 'seeders' | 'publishedAt' | 'sizeDesc' | 'sizeAsc';

export interface SystemStatus {
    version: string;
    uptimeMs: number;
    dbStatus: string;
    bindHost?: string;
    listenPort?: number;
    dataDir?: string;
    startedAt: string;
    definitionVersion?: string;
    installedIndexers: number;
}

export interface InstalledIndexer {
    id: string;
    definitionId: string;
    name: string;
    enabled: boolean;
    baseUrl: string;
    definitionVersion?: string;
    status: IndexerHealth;
    lastCheckedAt?: string;
    lastSuccessAt?: string;
    lastError?: string;
    responseTimeMs?: number;
    consecutiveFails?: number;
    createdAt: string;
    updatedAt: string;
}

export interface IndexerDefinition {
    id: string;
    name: string;
    description?: string;
    version: string;
    language?: string;
    type: string;
    protocol: string;
    links?: string[];
    categories?: Record<string, string[]>;
}
export interface DiscoveryCandidate { name: string; url: string; summary?: string }

export interface IndexerTestResult {
    ok: boolean;
    statusCode?: number;
    durationMs: number;
    resultCount?: number;
    errorCode?: string;
    errorMessage?: string;
}

export interface ImportValidationError {
    code: string;
    message: string;
}

export interface ImportResponse {
    id: string;
    valid: boolean;
    errors?: ImportValidationError[];
    definition?: IndexerDefinition;
    installed: boolean;
    installedId?: string;
    test?: IndexerTestResult;
    persisted: boolean;
}

export interface SearchFilters {
    minSizeBytes?: number;
    maxSizeBytes?: number;
    minSeeders?: number;
    publishedAfter?: string;
    indexerIds?: string[];
}

export interface SearchRequest {
    keyword: string;
    category: Category;
    sort: SortMode;
    filters: SearchFilters;
}

export interface SearchSession {
    sessionId: string;
    streamUrl: string;
}

export type SearchEvent =
    | { event: 'session_started'; data: { sessionId: string; totalIndexers: number } }
    | { event: 'indexer_started'; data: { sessionId: string; indexerId: string; indexerName: string } }
    | { event: 'indexer_result'; data: { sessionId: string; indexerId: string; resultCount: number } }
    | { event: 'indexer_completed'; data: { sessionId: string; indexerId: string; status: IndexerStatus; resultCount: number; durationMs: number } }
    | { event: 'indexer_failed'; data: { sessionId: string; indexerId: string; code: string; message: string } }
    | { event: 'results_merged'; data: { sessionId: string; mergedCount: number; rawCount: number } }
    | { event: 'session_completed'; data: { sessionId: string; totalMs: number; mergedCount: number } }
    | { event: 'session_cancelled'; data: { sessionId: string } };

export interface ResultSource {
    indexerId: string;
    indexerName: string;
    magnetUrl?: string;
    torrentUrl?: string;
    directUrl?: string;
    detailUrl?: string;
    seeders?: number;
    publishedAt?: string;
}

export interface SearchResult {
    id: string;
    title: string;
    category: Category;
    sizeBytes?: number;
    seeders?: number;
    leechers?: number;
    downloads?: number;
    publishedAt?: string;
    magnetUrl?: string;
    torrentUrl?: string;
    directUrl?: string;
    detailUrl?: string;
    infoHash?: string;
    indexerId: string;
    indexerName: string;
    sources: ResultSource[];
}
