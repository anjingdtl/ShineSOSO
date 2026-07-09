import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SearchPage } from '../../src/pages/SearchPage';
import { IndexerPage } from '../../src/pages/IndexerPage';
import { SystemStatusBar } from '../../src/features/SystemStatusBar';

function wrap(node: React.ReactNode): React.ReactNode {
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    return (
        <QueryClientProvider client={qc}>
            <MemoryRouter>{node}</MemoryRouter>
        </QueryClientProvider>
    );
}

describe('SearchPage', () => {
    it('shows the empty state when no indexers are added', () => {
        render(wrap(<SearchPage />) as React.ReactElement);
        expect(screen.getByText('搜索')).toBeInTheDocument();
        expect(screen.getByText('尚未添加索引器')).toBeInTheDocument();
    });
});

describe('IndexerPage', () => {
    it('shows the empty state when no indexers are installed', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            status: 200,
            json: async () => ({ items: [] }),
        } as Response);
        render(wrap(<IndexerPage />) as React.ReactElement);
        expect(screen.getByText('索引器')).toBeInTheDocument();
        await waitFor(() => {
            expect(screen.getByText(/尚无索引器/)).toBeInTheDocument();
        });
    });
});

describe('SystemStatusBar', () => {
    beforeEach(() => {
        vi.restoreAllMocks();
    });

    it('renders loading state initially', () => {
        vi.spyOn(globalThis, 'fetch').mockReturnValue(new Promise(() => {}));
        render(wrap(<SystemStatusBar />) as React.ReactElement);
        // initial state shows "…" (loading) — confirmed by absence of version text
        expect(screen.queryByText(/v0\./)).not.toBeInTheDocument();
    });

    it('renders version and uptime when status fetch succeeds', async () => {
        vi.spyOn(globalThis, 'fetch').mockResolvedValue({
            ok: true,
            status: 200,
            statusText: 'OK',
            json: async () => ({
                version: '0.1.0',
                uptimeMs: 4321,
                dbStatus: 'unknown',
                startedAt: '2026-07-09T00:00:00Z',
                installedIndexers: 0,
            }),
        } as Response);
        render(wrap(<SystemStatusBar />) as React.ReactElement);
        await waitFor(() => {
            expect(screen.getByText(/v0\.1\.0/)).toBeInTheDocument();
        });
    });

    it('shows offline pill on error', async () => {
        vi.spyOn(globalThis, 'fetch').mockRejectedValue(new Error('network down'));
        render(wrap(<SystemStatusBar />) as React.ReactElement);
        await waitFor(() => {
            expect(screen.getByText('离线')).toBeInTheDocument();
        });
    });
});
