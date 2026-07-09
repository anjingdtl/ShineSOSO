import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import { readFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

// Vite reads the backend's chosen port from data/.port (written by the Go
// server before it starts listening). The .port file is created under the
// directory pointed to by EASYSEARCH_DATA_DIR, defaulting to the user's
// APPDATA on Windows. The dev proxy then forwards /api requests there.
function readBackendPort(): number {
    const appdata = process.env.APPDATA ?? join(homedir(), '.config');
    const dataDir = process.env.EASYSEARCH_DATA_DIR ?? join(appdata, 'EasySearch', 'data');
    try {
        const raw = readFileSync(join(dataDir, '.port'), 'utf8').trim();
        const port = Number.parseInt(raw, 10);
        if (Number.isFinite(port) && port > 0 && port < 65536) return port;
    } catch {
        // file not present yet; default below
    }
    return 18765; // sane fallback for cold `npm run dev` before Go starts
}

export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, process.cwd(), '');
    const backendPort = readBackendPort();
    const backendTarget = `http://127.0.0.1:${backendPort}`;

    return {
        plugins: [react()],
        server: {
            port: 5173,
            strictPort: false,
            proxy: {
                '/api': {
                    target: backendTarget,
                    changeOrigin: false,
                    ws: false,
                },
            },
        },
        build: {
            // Build directly into the Go server's web/ embed directory so
            // `go:embed all:web` picks it up without a copy step. The path
            // is relative to the frontend/ working directory.
            outDir: '../backend/internal/webembed/web',
            emptyOutDir: true,
            sourcemap: true,
            target: 'es2022',
        },
        test: {
            globals: true,
            environment: 'jsdom',
            setupFiles: ['./tests/setup.ts'],
            exclude: ['**/node_modules/**', '**/dist/**', 'tests/e2e/**'],
        },
        define: {
            __APP_VERSION__: JSON.stringify(env.npm_package_version ?? '0.0.0'),
        },
    };
});
