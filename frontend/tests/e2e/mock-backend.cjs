// mock-backend.js — minimal Express-less HTTP server that emulates the
// easysearch backend AND serves the built frontend from
// ../backend/internal/webembed/web/ so the Playwright E2E test can
// exercise the real React UI against a deterministic backend.
//
// Endpoints implemented:
//
//   GET  /api/v1/system/status          -> { version, uptimeMs }
//   GET  /api/v1/indexers               -> { items: [...] }
//   POST /api/v1/indexers               -> creates demo-alpha
//   POST /api/v1/search/sessions        -> creates a session and returns streamUrl
//   GET  /api/v1/search/sessions/:id/events  -> SSE stream
//   POST /api/v1/search/sessions/:id/cancel -> cancels
//
// All other paths fall through to serving the built frontend (with
// SPA fallback to index.html for client-side routes).
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const { randomUUID } = require('node:crypto');

const PORT = Number(process.env.MOCK_PORT || 18766);
const VERSION = '0.4.0-test';
// Path is computed from process.cwd() which Playwright sets to the
// frontend/ directory. From there, ../backend/internal/webembed/web/
// reaches the vite build output.
const WEB_ROOT = path.resolve(process.cwd(), '..', 'backend', 'internal', 'webembed', 'web');

function send(res, status, body, headers) {
    res.writeHead(status, { 'Content-Type': 'application/json', ...headers });
    res.end(typeof body === 'string' ? body : JSON.stringify(body));
}

function contentTypeFor(p) {
    const ext = path.extname(p).toLowerCase();
    const map = {
        '.html': 'text/html; charset=utf-8',
        '.js': 'application/javascript; charset=utf-8',
        '.mjs': 'application/javascript; charset=utf-8',
        '.css': 'text/css; charset=utf-8',
        '.svg': 'image/svg+xml',
        '.png': 'image/png',
        '.jpg': 'image/jpeg',
        '.json': 'application/json',
        '.ico': 'image/x-icon',
    };
    return map[ext] || 'application/octet-stream';
}

function serveStatic(req, res) {
    const urlPath = decodeURIComponent(req.url.split('?')[0]);
    let p = path.join(WEB_ROOT, urlPath);
    if (!p.startsWith(WEB_ROOT)) {
        res.writeHead(403);
        res.end();
        return;
    }
    fs.stat(p, (err, stat) => {
        if (!err && stat.isFile()) {
            const stream = fs.createReadStream(p);
            res.writeHead(200, { 'Content-Type': contentTypeFor(p) });
            stream.pipe(res);
            return;
        }
        // SPA fallback: serve index.html for unknown routes so React
        // Router can take over.
        const idx = path.join(WEB_ROOT, 'index.html');
        fs.readFile(idx, (e, data) => {
            if (e) {
                res.writeHead(500);
                res.end('index.html missing — run `npm run build` in frontend/');
                return;
            }
            res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
            res.end(data);
        });
    });
}

const indexers = [];
let sessionCounter = 0;

const server = http.createServer((req, res) => {
    const { method, url } = req;

    // CORS for safety; Vite proxy usually strips origin but be defensive.
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PATCH, DELETE, OPTIONS');
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

    if (method === 'OPTIONS') {
        res.writeHead(204);
        res.end();
        return;
    }

    if (url === '/api/v1/system/status') {
        return send(res, 200, {
            version: VERSION,
            uptimeMs: 1234,
            bindHost: '127.0.0.1',
            listenPort: PORT,
            dataDir: '/tmp/easysearch-test',
            startedAt: new Date().toISOString(),
            installedIndexers: indexers.length,
        });
    }

    if (url === '/api/v1/indexers' && method === 'GET') {
        return send(res, 200, { items: indexers });
    }

    if (url === '/api/v1/indexers' && method === 'POST') {
        let body = '';
        req.on('data', (c) => (body += c));
        req.on('end', () => {
            const parsed = body ? JSON.parse(body) : {};
            const id = 'inst-' + randomUUID().slice(0, 8);
            const newIx = {
                id,
                definitionId: parsed.definitionId || 'demo-alpha',
                name: parsed.definitionId || 'demo-alpha',
                baseUrl: 'https://mock.invalid',
                enabled: true,
                status: 'healthy',
                definitionVersion: '1.0.0',
                createdAt: new Date().toISOString(),
                updatedAt: new Date().toISOString(),
            };
            indexers.push(newIx);
            send(res, 200, newIx);
        });
        return;
    }

    let m;
    if ((m = url.match(/^\/api\/v1\/search\/sessions\/([^/]+)\/events$/)) && method === 'GET') {
        const sessionId = m[1];
        res.writeHead(200, {
            'Content-Type': 'text/event-stream',
            'Cache-Control': 'no-cache',
            'Connection': 'keep-alive',
        });
        const write = (event, data) => {
            res.write(`event: ${event}\n`);
            res.write(`data: ${JSON.stringify(data)}\n\n`);
        };
        write('session_started', { sessionId, indexerCount: indexers.length });
        setTimeout(() => {
            for (const ix of indexers) {
                write('indexer_started', { indexerId: ix.id, indexerName: ix.name });
                write('indexer_result', {
                    indexerId: ix.id,
                    results: [{
                        id: 'mock-r1',
                        title: 'Mock Movie ' + Date.now(),
                        magnetUrl: 'magnet:?xt=urn:btih:abcdef1234567890abcdef1234567890abcdef12',
                        sizeBytes: 8_400_000_000,
                        seeders: 326,
                        publishedAt: new Date().toISOString(),
                        indexerId: ix.id,
                        indexerName: ix.name,
                    }],
                });
                write('indexer_completed', {
                    indexerId: ix.id,
                    status: 'success',
                    resultCount: 1,
                    durationMs: 50,
                });
            }
            write('results_merged', { rawCount: indexers.length, mergedCount: indexers.length });
            write('session_completed', { totalMs: 200 });
            res.end();
        }, 100);
        return;
    }

    if ((m = url.match(/^\/api\/v1\/search\/sessions\/([^/]+)\/cancel$/)) && method === 'POST') {
        return send(res, 200, { cancelled: true });
    }

    if (url === '/api/v1/search/sessions' && method === 'POST') {
        const id = 'sess-' + (++sessionCounter);
        return send(res, 200, {
            sessionId: id,
            streamUrl: `/api/v1/search/sessions/${id}/events`,
        });
    }

    // Everything else: serve the built frontend.
    return serveStatic(req, res);
});

server.listen(PORT, '127.0.0.1', () => {
    console.log(`[mock-backend] listening on http://127.0.0.1:${PORT}`);
    console.log(`[mock-backend] serving web from ${WEB_ROOT}`);
});