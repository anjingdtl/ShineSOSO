import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSearchStream } from '../../src/features/search/useSearchStream';

// EventSource mock: each call returns an object whose addEventListener
// captures handlers keyed by event name. Tests then invoke the handlers
// directly to simulate server-sent events.
class MockEventSource {
    static instances: MockEventSource[] = [];
    url: string;
    handlers: Record<string, (e: MessageEvent) => void> = {};
    onerror: (() => void) | null = null;
    closed = false;
    constructor(url: string) {
        this.url = url;
        MockEventSource.instances.push(this);
    }
    addEventListener(name: string, cb: (e: MessageEvent) => void) {
        this.handlers[name] = cb;
    }
    close() {
        this.closed = true;
    }
    fire(name: string, data: unknown) {
        const cb = this.handlers[name];
        if (cb) cb({ data: JSON.stringify(data) } as MessageEvent);
    }
}

beforeEach(() => {
    MockEventSource.instances.length = 0;
    (globalThis as unknown as { EventSource: typeof MockEventSource }).EventSource = MockEventSource;
    (globalThis as unknown as { fetch: typeof fetch }).fetch = vi.fn();
});

describe('useSearchStream', () => {
    it('starts in idle state', () => {
        const { result } = renderHook(() => useSearchStream());
        expect(result.current.state.status).toBe('idle');
        expect(result.current.state.results).toEqual([]);
        expect(result.current.state.indexers).toEqual({});
    });

    it('transitions to streaming on successful POST', async () => {
        const fakeFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        fakeFetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({ sessionId: 'sess-1', streamUrl: '/api/v1/search/sessions/sess-1/events' }),
        });
        const { result } = renderHook(() => useSearchStream());
        await act(async () => {
            await result.current.startSearch({ keyword: 'matrix', category: 'all', sort: 'relevance' });
        });
        expect(result.current.state.sessionId).toBe('sess-1');
        expect(result.current.state.status).toBe('streaming');
        expect(MockEventSource.instances).toHaveLength(1);
    });

    it('captures indexer_started events', async () => {
        const fakeFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        fakeFetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({ sessionId: 's1', streamUrl: '/s1/events' }),
        });
        const { result } = renderHook(() => useSearchStream());
        await act(async () => {
            await result.current.startSearch({ keyword: 'k', category: 'all', sort: 'relevance' });
        });
        const es = MockEventSource.instances[0]!;
        await act(async () => {
            es.fire('indexer_started', { indexerId: 'ix1', indexerName: 'Mock A' });
        });
        expect(result.current.state.indexers['ix1']).toMatchObject({
            status: 'running',
            indexerName: 'Mock A',
        });
    });

    it('appends indexer_result payloads to results', async () => {
        const fakeFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        fakeFetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({ sessionId: 's2', streamUrl: '/s2/events' }),
        });
        const { result } = renderHook(() => useSearchStream());
        await act(async () => {
            await result.current.startSearch({ keyword: 'k', category: 'all', sort: 'relevance' });
        });
        const es = MockEventSource.instances[0]!;
        await act(async () => {
            es.fire('indexer_result', {
                results: [{ id: 'r1', title: 'first' }],
            });
            es.fire('indexer_result', {
                results: [{ id: 'r2', title: 'second' }],
            });
        });
        expect(result.current.state.results.map((r) => r.id)).toEqual(['r1', 'r2']);
    });

    it('marks indexer as error on indexer_failed', async () => {
        const fakeFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        fakeFetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({ sessionId: 's3', streamUrl: '/s3/events' }),
        });
        const { result } = renderHook(() => useSearchStream());
        await act(async () => {
            await result.current.startSearch({ keyword: 'k', category: 'all', sort: 'relevance' });
        });
        const es = MockEventSource.instances[0]!;
        await act(async () => {
            es.fire('indexer_started', { indexerId: 'ix-bad' });
            es.fire('indexer_failed', { indexerId: 'ix-bad', errorCode: 'TIMEOUT' });
        });
        expect(result.current.state.indexers['ix-bad']!.status).toBe('error');
    });

    it('reaches completed on session_completed', async () => {
        const fakeFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        fakeFetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({ sessionId: 's4', streamUrl: '/s4/events' }),
        });
        const { result } = renderHook(() => useSearchStream());
        await act(async () => {
            await result.current.startSearch({ keyword: 'k', category: 'all', sort: 'relevance' });
        });
        const es = MockEventSource.instances[0]!;
        await act(async () => {
            es.fire('session_completed', { totalMs: 1234 });
        });
        expect(result.current.state.status).toBe('completed');
        expect(result.current.state.totalMs).toBe(1234);
    });

    it('handles HTTP error from POST', async () => {
        const fakeFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        fakeFetch.mockResolvedValueOnce({
            ok: false,
            status: 400,
            json: async () => ({ error: { code: 'EMPTY_KEYWORD', message: 'no keyword' } }),
        });
        const { result } = renderHook(() => useSearchStream());
        await act(async () => {
            await result.current.startSearch({ keyword: '', category: 'all', sort: 'relevance' });
        });
        expect(result.current.state.status).toBe('error');
        expect(result.current.state.errorCode).toBe('EMPTY_KEYWORD');
        expect(MockEventSource.instances).toHaveLength(0);
    });

    it('handles network error from POST', async () => {
        const fakeFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        fakeFetch.mockRejectedValueOnce(new Error('network down'));
        const { result } = renderHook(() => useSearchStream());
        await act(async () => {
            await result.current.startSearch({ keyword: 'k', category: 'all', sort: 'relevance' });
        });
        expect(result.current.state.status).toBe('error');
        expect(result.current.state.errorCode).toBe('NETWORK_ERROR');
    });

    it('reset clears state and closes stream', async () => {
        const fakeFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        fakeFetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({ sessionId: 's5', streamUrl: '/s5/events' }),
        });
        const { result } = renderHook(() => useSearchStream());
        await act(async () => {
            await result.current.startSearch({ keyword: 'k', category: 'all', sort: 'relevance' });
        });
        const es = MockEventSource.instances[0]!;
        await act(async () => {
            es.fire('indexer_result', { results: [{ id: 'r1' }] });
        });
        expect(result.current.state.results).toHaveLength(1);
        act(() => {
            result.current.reset();
        });
        expect(result.current.state.status).toBe('idle');
        expect(result.current.state.results).toHaveLength(0);
        expect(es.closed).toBe(true);
    });

    it('updates raw/merged counts on results_merged', async () => {
        const fakeFetch = globalThis.fetch as ReturnType<typeof vi.fn>;
        fakeFetch.mockResolvedValueOnce({
            ok: true,
            json: async () => ({ sessionId: 's6', streamUrl: '/s6/events' }),
        });
        const { result } = renderHook(() => useSearchStream());
        await act(async () => {
            await result.current.startSearch({ keyword: 'k', category: 'all', sort: 'relevance' });
        });
        const es = MockEventSource.instances[0]!;
        await act(async () => {
            es.fire('results_merged', { rawCount: 100, mergedCount: 60 });
        });
        expect(result.current.state.rawCount).toBe(100);
        expect(result.current.state.mergedCount).toBe(60);
    });
});