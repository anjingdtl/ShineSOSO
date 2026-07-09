import { test, expect } from '@playwright/test';

test.describe('EasySearch happy path', () => {
    test('status endpoint reachable', async ({ request }) => {
        const status = await request.get('/api/v1/system/status');
        expect(status.ok()).toBeTruthy();
        const body = await status.json();
        expect(body.version).toMatch(/^\d+\.\d+\.\d+/);
    });

    test('home page loads with search input', async ({ page }) => {
        await page.goto('/');
        // Wait for React to mount — look for the rendered search input
        // (placeholder contains 输入关键词) or the app title.
        await expect(page.locator('input[placeholder*="关键词"], .app-title').first()).toBeVisible({ timeout: 10_000 });
    });

    test('indexer page is reachable', async ({ page }) => {
        await page.goto('/indexers');
        await expect(page).toHaveURL(/\/indexers/);
        // The page should render without a server error and expose
        // navigation links.
        await expect(page.locator('.app-nav').first()).toBeVisible({ timeout: 10_000 });
    });

    test('home shows empty-state when no indexers added', async ({ page }) => {
        await page.goto('/');
        // Empty-state copy: "尚未添加索引器".
        await expect(page.locator('text=尚未添加索引器').first()).toBeVisible({ timeout: 10_000 });
    });

    test('mock indexer POST then GET roundtrip', async ({ request }) => {
        const create = await request.post('/api/v1/indexers', {
            data: { definitionId: 'demo-alpha' },
        });
        expect(create.ok()).toBeTruthy();
        const created = await create.json();
        expect(created.id).toBeTruthy();

        const list = await request.get('/api/v1/indexers');
        expect(list.ok()).toBeTruthy();
        const { items }: { items: Array<{ id: string }> } = await list.json();
        expect(items.length).toBeGreaterThanOrEqual(1);
        expect(items.some((i) => i.id === created.id)).toBe(true);
    });

    test('search session creates and SSE returns completed', async ({ request }) => {
        const create = await request.post('/api/v1/search/sessions', {
            data: { keyword: 'matrix', category: 'all' },
        });
        expect(create.ok()).toBeTruthy();
        const { sessionId, streamUrl } = await create.json();
        expect(sessionId).toBeTruthy();
        expect(streamUrl).toContain(sessionId);

        // Read SSE stream and look for session_completed within 5s.
        const resp = await request.get(streamUrl);
        expect(resp.ok()).toBeTruthy();
        const body = await resp.body();
        const text = body.toString('utf-8');
        expect(text).toContain('session_started');
        expect(text).toContain('session_completed');
        expect(text).toContain('magnet:?xt=urn:btih:');
    });

    test('cancel endpoint returns 200', async ({ request }) => {
        const create = await request.post('/api/v1/search/sessions', {
            data: { keyword: 'x', category: 'all' },
        });
        const { sessionId } = await create.json();
        const cancel = await request.post(`/api/v1/search/sessions/${sessionId}/cancel`);
        expect(cancel.ok()).toBeTruthy();
    });
});