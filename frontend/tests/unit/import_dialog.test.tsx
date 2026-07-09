import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ImportDialog } from '../../src/features/ImportDialog';

function mockFetchOnce(body: unknown, status = 200): ReturnType<typeof vi.fn> {
    return vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce({
        ok: status >= 200 && status < 300,
        status,
        statusText: 'OK',
        json: async () => body,
    } as Response);
}

describe('ImportDialog', () => {
    beforeEach(() => {
        vi.restoreAllMocks();
    });

    it('submits YAML and renders validation errors', async () => {
        mockFetchOnce({
            id: 'x',
            valid: false,
            errors: [{ code: 'NAME_MISSING', message: 'name is required' }],
            installed: false,
            persisted: false,
        });
        const onClose = vi.fn();
        render(<ImportDialog onClose={onClose} onInstalled={vi.fn()} />);
        const textarea = screen.getByPlaceholderText(/schema: 1/);
        await userEvent.type(textarea, 'foo: bar');
        await userEvent.click(screen.getByRole('button', { name: /校验/ }));
        await waitFor(() => {
            expect(screen.getByText('NAME_MISSING')).toBeInTheDocument();
        });
    });

    it('shows install button on successful validation', async () => {
        mockFetchOnce({
            id: 'my-foo',
            valid: true,
            errors: [],
            installed: false,
            persisted: true,
            definition: {
                id: 'my-foo',
                name: 'Foo',
                version: '1.0.0',
                type: 'public',
                protocol: 'declarative',
            },
            test: { ok: true, durationMs: 50, statusCode: 200 },
        });
        render(<ImportDialog onClose={vi.fn()} onInstalled={vi.fn()} />);
        const textarea = screen.getByPlaceholderText(/schema: 1/);
        await userEvent.type(textarea, 'schema: 1\n');
        await userEvent.click(screen.getByRole('button', { name: /校验/ }));
        await waitFor(() => {
            expect(screen.getByText(/YAML 校验通过/)).toBeInTheDocument();
        });
        expect(screen.getByRole('button', { name: /启用并保存/ })).toBeInTheDocument();
    });

    it('closes on backdrop button', async () => {
        const onClose = vi.fn();
        render(<ImportDialog onClose={onClose} onInstalled={vi.fn()} />);
        await userEvent.click(screen.getByRole('button', { name: /关闭/ }));
        expect(onClose).toHaveBeenCalled();
    });
});
