import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ResultCard } from '../../src/features/search/ResultCard';
import type { SearchResult } from '../../src/types';

function makeResult(overrides: Partial<SearchResult> = {}): SearchResult {
    return {
        id: 'r1',
        title: '示例电影 matrix',
        indexerId: 'ix1',
        indexerName: 'Mock A',
        category: 'movie',
        sizeBytes: 8_400_000_000,
        seeders: 326,
        publishedAt: new Date().toISOString(),
        magnetUrl: 'magnet:?xt=urn:btih:0000000000000000000000000000000000000000',
        sources: [],
        ...overrides,
    };
}

beforeEach(() => {
    // Reset clipboard mock between tests.
    Object.assign(navigator, {
        clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
    });
});

describe('ResultCard', () => {
    it('renders title and indexer name', () => {
        render(<ResultCard result={makeResult()} />);
        expect(screen.getByText('示例电影 matrix')).toBeInTheDocument();
        expect(screen.getByText(/Mock A/)).toBeInTheDocument();
    });

    it('labels magnet links as 磁力', () => {
        render(<ResultCard result={makeResult()} />);
        const card = screen.getByRole('article');
        expect(card).toHaveAttribute('data-kind', 'magnet');
        expect(screen.getByText('磁力')).toBeInTheDocument();
    });

    it('labels torrent links as 种子', () => {
        render(
            <ResultCard
                result={makeResult({
                    magnetUrl: undefined,
                    torrentUrl: 'https://example.com/x.torrent',
                })}
            />
        );
        expect(screen.getByText('种子')).toBeInTheDocument();
    });

    it('labels direct links as 直链', () => {
        render(
            <ResultCard
                result={makeResult({
                    magnetUrl: undefined,
                    torrentUrl: undefined,
                    directUrl: 'https://example.com/x.zip',
                })}
            />
        );
        expect(screen.getByText('直链')).toBeInTheDocument();
    });

    it('formats size with formatBytes', () => {
        render(<ResultCard result={makeResult({ sizeBytes: 1024 * 1024 })} />);
        expect(screen.getByText(/1\.0 MB/)).toBeInTheDocument();
    });

    it('renders seeders count', () => {
        render(<ResultCard result={makeResult({ seeders: 326 })} />);
        expect(screen.getByText(/326/)).toBeInTheDocument();
    });

    it('copies primary URL on click', async () => {
        const writeText = vi.fn().mockResolvedValue(undefined);
        Object.assign(navigator, { clipboard: { writeText } });
        render(<ResultCard result={makeResult()} />);
        const copyBtn = screen.getByRole('button', { name: /复制/ });
        await userEvent.click(copyBtn);
        expect(writeText).toHaveBeenCalledWith(expect.stringContaining('magnet:'));
        await waitFor(() => {
            expect(screen.getByText('已复制')).toBeInTheDocument();
        });
    });

    it('shows error label when clipboard fails', async () => {
        const writeText = vi.fn().mockRejectedValue(new Error('denied'));
        Object.assign(navigator, { clipboard: { writeText } });
        render(<ResultCard result={makeResult()} />);
        await userEvent.click(screen.getByRole('button', { name: /复制/ }));
        await waitFor(() => {
            expect(screen.getByText('复制失败')).toBeInTheDocument();
        });
    });

    it('disables copy button when no URL', () => {
        render(
            <ResultCard
                result={makeResult({
                    magnetUrl: undefined,
                    torrentUrl: undefined,
                    directUrl: undefined,
                    detailUrl: undefined,
                })}
            />
        );
        const copyBtn = screen.getByRole('button', { name: /复制/ });
        expect(copyBtn).toBeDisabled();
    });

    it('renders detail link when detailUrl is separate from primary', () => {
        render(
            <ResultCard
                result={makeResult({
                    magnetUrl: 'magnet:?xt=urn:btih:abc',
                    detailUrl: 'https://example.com/d/1',
                })}
            />
        );
        expect(screen.getByRole('button', { name: /详情/ })).toBeInTheDocument();
    });

    it('hides detail link when detailUrl equals primary', () => {
        render(
            <ResultCard
                result={makeResult({
                    magnetUrl: 'magnet:?xt=urn:btih:abc',
                    detailUrl: 'magnet:?xt=urn:btih:abc',
                })}
            />
        );
        expect(screen.queryByRole('button', { name: /详情/ })).not.toBeInTheDocument();
    });
});