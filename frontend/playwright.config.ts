import { defineConfig, devices } from '@playwright/test';

const PORT = 18766;

export default defineConfig({
    testDir: './tests/e2e',
    timeout: 30_000,
    fullyParallel: false,
    retries: 0,
    reporter: 'list',
    use: {
        baseURL: `http://127.0.0.1:${PORT}`,
        trace: 'off',
        headless: true,
    },
    projects: [
        {
            name: 'chromium',
            use: { ...devices['Desktop Chrome'] },
        },
    ],
    webServer: {
        command: 'node tests/e2e/mock-backend.cjs',
        port: PORT,
        reuseExistingServer: true,
        timeout: 15_000,
    },
});