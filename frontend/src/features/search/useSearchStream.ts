import { useCallback, useEffect, useRef, useState } from 'react';
import type { IndexerStatus, SearchResult } from '../../types';

export type IndexerLiveStatus = {
    indexerId: string;
    indexerName: string;
    status: IndexerStatus | 'pending';
    resultCount: number;
    durationMs: number;
};

export type SearchState = {
    status: 'idle' | 'creating' | 'streaming' | 'completed' | 'cancelled' | 'error';
    sessionId: string | null;
    streamUrl: string | null;
    keyword: string;
    category: string;
    sort: string;
    results: SearchResult[];
    indexers: Record<string, IndexerLiveStatus>;
    rawCount: number;
    mergedCount: number;
    totalMs: number;
    errorCode: string | null;
    errorMessage: string | null;
};

const initialState: SearchState = {
    status: 'idle',
    sessionId: null,
    streamUrl: null,
    keyword: '',
    category: 'all',
    sort: 'relevance',
    results: [],
    indexers: {},
    rawCount: 0,
    mergedCount: 0,
    totalMs: 0,
    errorCode: null,
    errorMessage: null,
};

export function useSearchStream(): {
    state: SearchState;
    startSearch: (req: { keyword: string; category: string; sort: string }) => Promise<void>;
    cancel: () => Promise<void>;
    reset: () => void;
} {
    const [state, setState] = useState<SearchState>(initialState);
    const esRef = useRef<EventSource | null>(null);

    const closeStream = useCallback(() => {
        if (esRef.current) {
            esRef.current.close();
            esRef.current = null;
        }
    }, []);

    const startSearch = useCallback(async (req: { keyword: string; category: string; sort: string }) => {
        closeStream();
        setState({ ...initialState, keyword: req.keyword, category: req.category, sort: req.sort, status: 'creating' });
        try {
            const res = await fetch('/api/v1/search/sessions', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(req),
            });
            if (!res.ok) {
                const err = (await res.json()) as { error?: { code: string; message: string } };
                setState((s) => ({
                    ...s,
                    status: 'error',
                    errorCode: err.error?.code ?? 'INTERNAL_ERROR',
                    errorMessage: err.error?.message ?? `HTTP ${res.status}`,
                }));
                return;
            }
            const created = (await res.json()) as { sessionId: string; streamUrl: string };
            setState((s) => ({ ...s, sessionId: created.sessionId, streamUrl: created.streamUrl, status: 'streaming' }));

            const es = new EventSource(created.streamUrl);
            esRef.current = es;

            const handle = (eventName: string, data: unknown) => {
                setState((s) => applyEvent(s, eventName, data as Record<string, unknown>));
            };

            for (const ev of [
                'session_started',
                'indexer_started',
                'indexer_completed',
                'indexer_failed',
                'results_merged',
                'session_completed',
                'session_cancelled',
            ]) {
                es.addEventListener(ev, (e: MessageEvent) => {
                    try {
                        const data = JSON.parse(e.data) as Record<string, unknown>;
                        handle(ev, data);
                    } catch (err) {
                        // ignore parse errors
                    }
                });
            }
            es.onerror = () => {
                // EventSource auto-reconnects; we let the orchestrator's
                // session_completed / session_cancelled drive the final state.
            };
        } catch (err) {
            setState((s) => ({
                ...s,
                status: 'error',
                errorCode: 'NETWORK_ERROR',
                errorMessage: err instanceof Error ? err.message : String(err),
            }));
        }
    }, [closeStream]);

    const cancel = useCallback(async () => {
        if (!state.sessionId) return;
        try {
            await fetch(`/api/v1/search/sessions/${state.sessionId}/cancel`, { method: 'POST' });
        } catch {
            // best-effort
        }
    }, [state.sessionId]);

    const reset = useCallback(() => {
        closeStream();
        setState(initialState);
    }, [closeStream]);

    useEffect(() => {
        return () => {
            closeStream();
        };
    }, [closeStream]);

    return { state, startSearch, cancel, reset };
}

function applyEvent(state: SearchState, type: string, data: Record<string, unknown>): SearchState {
    switch (type) {
        case 'session_started': {
            return state;
        }
        case 'indexer_started': {
            const id = String(data.indexerId ?? '');
            return {
                ...state,
                indexers: {
                    ...state.indexers,
                    [id]: {
                        indexerId: id,
                        indexerName: String(data.indexerName ?? id),
                        status: 'running',
                        resultCount: 0,
                        durationMs: 0,
                    },
                },
            };
        }
        case 'indexer_completed': {
            const id = String(data.indexerId ?? '');
            return {
                ...state,
                indexers: {
                    ...state.indexers,
                    [id]: {
                        ...(state.indexers[id] ?? {
                            indexerId: id,
                            indexerName: id,
                            resultCount: 0,
                            durationMs: 0,
                        }),
                        status: (data.status as IndexerStatus) ?? 'success',
                        resultCount: Number(data.resultCount ?? 0),
                        durationMs: Number(data.durationMs ?? 0),
                    },
                },
            };
        }
        case 'indexer_failed': {
            const id = String(data.indexerId ?? '');
            return {
                ...state,
                indexers: {
                    ...state.indexers,
                    [id]: {
                        ...(state.indexers[id] ?? {
                            indexerId: id,
                            indexerName: id,
                            resultCount: 0,
                            durationMs: 0,
                        }),
                        status: 'error',
                    },
                },
            };
        }
        case 'results_merged': {
            return {
                ...state,
                rawCount: Number(data.rawCount ?? state.rawCount),
                mergedCount: Number(data.mergedCount ?? state.mergedCount),
            };
        }
        case 'session_completed': {
            return {
                ...state,
                status: 'completed',
                totalMs: Number(data.totalMs ?? 0),
            };
        }
        case 'session_cancelled': {
            return { ...state, status: 'cancelled' };
        }
        default:
            return state;
    }
}
