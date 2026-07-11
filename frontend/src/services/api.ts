// Thin fetch wrapper that points at the same origin as the current page.
// In production the frontend is served by the Go binary on 127.0.0.1:0;
// in dev the Vite proxy forwards /api to the Go backend on the port
// declared in $EASYSEARCH_DATA_DIR/.port.

import type { DiscoveryCandidate, ImportResponse, IndexerDefinition, IndexerTestResult, InstalledIndexer, ProwlarrCandidate, ProwlarrInstalledIndexer, ProwlarrStatus, SystemStatus } from '../types';

export class ApiError extends Error {
    constructor(
        public readonly status: number,
        public readonly code: string,
        message: string,
    ) {
        super(message);
        this.name = 'ApiError';
    }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const res = await fetch(path, {
        ...init,
        headers: {
            Accept: 'application/json',
            ...(init?.headers ?? {}),
        },
    });
    if (!res.ok) {
        let code = 'INTERNAL_ERROR';
        let message = res.statusText;
        try {
            const body = (await res.json()) as { error?: { code: string; message: string } };
            if (body.error) {
                code = body.error.code;
                message = body.error.message;
            }
        } catch {
            // non-JSON body; use the statusText fallback
        }
        throw new ApiError(res.status, code, message);
    }
    if (res.status === 204) {
        return undefined as unknown as T;
    }
    return (await res.json()) as T;
}

export interface CreateIndexerRequest {
    definitionId: string;
    baseUrl: string;
    name?: string;
    enabled?: boolean;
    testBeforeEnable?: boolean;
}

export interface UpdateIndexerRequest {
    enabled?: boolean;
    baseUrl?: string;
    name?: string;
}

export const api = {
    getSystemStatus(): Promise<SystemStatus> {
        return request<SystemStatus>('/api/v1/system/status');
    },

    listIndexers(): Promise<{ items: InstalledIndexer[] }> {
        return request<{ items: InstalledIndexer[] }>('/api/v1/indexers');
    },
    getIndexer(id: string): Promise<InstalledIndexer> {
        return request<InstalledIndexer>(`/api/v1/indexers/${encodeURIComponent(id)}`);
    },
    createIndexer(req: CreateIndexerRequest): Promise<InstalledIndexer> {
        return request<InstalledIndexer>('/api/v1/indexers', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(req),
        });
    },
    updateIndexer(id: string, req: UpdateIndexerRequest): Promise<InstalledIndexer> {
        return request<InstalledIndexer>(`/api/v1/indexers/${encodeURIComponent(id)}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(req),
        });
    },
    deleteIndexer(id: string): Promise<void> {
        return request<void>(`/api/v1/indexers/${encodeURIComponent(id)}`, {
            method: 'DELETE',
        });
    },
    testIndexer(id: string): Promise<IndexerTestResult> {
        return request<IndexerTestResult>(`/api/v1/indexers/${encodeURIComponent(id)}/test`, {
            method: 'POST',
        });
    },
    listCatalog(): Promise<{ items: IndexerDefinition[] }> {
        return request<{ items: IndexerDefinition[] }>('/api/v1/indexer-catalog');
    },
    discoverIndexers(query: string): Promise<{ items: DiscoveryCandidate[] }> { return request('/api/v1/indexer-discovery/search',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({query})}); },
    probeIndexer(url: string): Promise<{ baseUrl: string }> { return request('/api/v1/indexer-discovery/probe',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({url})}); },
    getProwlarrStatus(): Promise<ProwlarrStatus> { return request('/api/v1/prowlarr/status'); },
    listProwlarrIndexers(): Promise<{ items: ProwlarrInstalledIndexer[] }> { return request('/api/v1/prowlarr/indexers'); },
    discoverProwlarrIndexers(query: string): Promise<{ items: ProwlarrCandidate[] }> { return request('/api/v1/prowlarr/discover',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({query})}); },
    addProwlarrIndexer(schemaId: string): Promise<ProwlarrCandidate> { return request('/api/v1/prowlarr/indexers',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({schemaId})}); },
    importDefinition(yaml: string, filename: string, withTest = true): Promise<ImportResponse> {
        return request<ImportResponse>('/api/v1/indexer-catalog/import', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ yaml, filename, test: withTest }),
        });
    },
};
