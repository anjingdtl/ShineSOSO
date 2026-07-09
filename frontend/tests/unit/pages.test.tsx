import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { SearchPage } from '../../src/pages/SearchPage';
import { IndexerPage } from '../../src/pages/IndexerPage';

function wrap(node: React.ReactNode): React.ReactNode {
    return <MemoryRouter>{node}</MemoryRouter>;
}

describe('SearchPage', () => {
    it('shows the empty state when no indexers are added', () => {
        render(wrap(<SearchPage />) as React.ReactElement);
        expect(screen.getByText('搜索')).toBeInTheDocument();
        expect(screen.getByText('尚未添加索引器')).toBeInTheDocument();
    });
});

describe('IndexerPage', () => {
    it('shows the empty state when no indexers are installed', () => {
        render(wrap(<IndexerPage />) as React.ReactElement);
        expect(screen.getByText('索引器')).toBeInTheDocument();
        expect(screen.getByText('尚无索引器')).toBeInTheDocument();
    });
});
