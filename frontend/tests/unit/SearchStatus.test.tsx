import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { SearchStatus } from '../../src/features/search/SearchStatus';
import type { SearchState } from '../../src/features/search/useSearchStream';

function makeState(overrides: Partial<SearchState> = {}): SearchState {
    return {
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
        ...overrides,
    };
}

describe('SearchStatus', () => {
    it('renders nothing when idle and no indexers', () => {
        const { container } = render(<SearchStatus state={makeState()} />);
        expect(container.firstChild).toBeNull();
    });

    it('shows 搜索中 while streaming', () => {
        render(<SearchStatus state={makeState({ status: 'streaming' })} />);
        expect(screen.getByText('搜索中…')).toBeInTheDocument();
    });

    it('shows completion time when completed', () => {
        render(<SearchStatus state={makeState({ status: 'completed', totalMs: 1234 })} />);
        expect(screen.getByText(/1234 ms/)).toBeInTheDocument();
    });

    it('shows cancellation message', () => {
        render(<SearchStatus state={makeState({ status: 'cancelled' })} />);
        expect(screen.getByText(/已取消/)).toBeInTheDocument();
    });

    it('shows error message', () => {
        render(
            <SearchStatus
                state={makeState({ status: 'error', errorCode: 'TIMEOUT', errorMessage: 'took too long' })}
            />
        );
        expect(screen.getByText(/took too long/)).toBeInTheDocument();
    });

    it('falls back to errorCode when errorMessage is null', () => {
        render(
            <SearchStatus state={makeState({ status: 'error', errorCode: 'NETWORK_ERROR' })} />
        );
        expect(screen.getByText(/NETWORK_ERROR/)).toBeInTheDocument();
    });

    it('counts indexers by status', () => {
        const state = makeState({
            status: 'streaming',
            indexers: {
                a: { indexerId: 'a', indexerName: 'A', status: 'success', resultCount: 1, durationMs: 100 },
                b: { indexerId: 'b', indexerName: 'B', status: 'running', resultCount: 0, durationMs: 0 },
                c: { indexerId: 'c', indexerName: 'C', status: 'error', resultCount: 0, durationMs: 0 },
                d: { indexerId: 'd', indexerName: 'D', status: 'timeout', resultCount: 0, durationMs: 15000 },
                e: { indexerId: 'e', indexerName: 'E', status: 'empty', resultCount: 0, durationMs: 50 },
            },
        });
        render(<SearchStatus state={state} />);
        expect(screen.getByText(/1 成功/)).toBeInTheDocument();
        expect(screen.getByText(/1 搜索中/)).toBeInTheDocument();
        expect(screen.getByText(/1 失败/)).toBeInTheDocument();
        expect(screen.getByText(/1 超时/)).toBeInTheDocument();
        expect(screen.getByText(/1 空结果/)).toBeInTheDocument();
    });

    it('shows raw and merged counts', () => {
        render(
            <SearchStatus state={makeState({ status: 'completed', rawCount: 100, mergedCount: 60 })} />
        );
        expect(screen.getByText(/原始结果 100/)).toBeInTheDocument();
        expect(screen.getByText(/合并后 60/)).toBeInTheDocument();
    });
});