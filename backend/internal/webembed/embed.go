// Package webembed embeds the built frontend into the Go binary.
//
// At build time the Vite output (frontend/dist or backend/web, depending
// on outDir) is expected to live under backend/web. In dev mode (when
// the directory is missing or only contains a placeholder), the package
// exposes a fallback that serves a minimal HTML page from the API
// origin. Production builds always include the real assets.
package webembed

import (
    "embed"
    "errors"
    "io/fs"
    "log/slog"
    "net/http"
    "strings"
)

//go:embed all:web
var content embed.FS

// FS returns the embedded filesystem rooted at the frontend's index.html.
// If the embed is empty (dev mode without a build), the returned FS will
// be a single-folder placeholder; callers should detect that and serve
// a redirect to the Vite dev server.
func FS() fs.FS {
    sub, err := fs.Sub(content, "web")
    if err != nil {
        // This only happens if the embed path is empty, which `go:embed`
        // would have rejected at compile time. Return the parent so the
        // caller still has a usable (empty) FS.
        return content
    }
    return sub
}

// HasBuild reports whether a real frontend build is present in the embed.
// Returns false in dev (e.g. when `backend/web/` has not been populated
// by `npm run build`).
func HasBuild() bool {
    f, err := content.Open("web/index.html")
    if err != nil {
        return false
    }
    _ = f.Close()
    return true
}

// Handler returns an http.Handler that serves the embedded frontend.
// Routes unknown to /api fall through to the SPA shell (index.html), so
// the React Router can take over for client-side navigation.
//
// If no build is present, the handler writes a development placeholder
// page explaining the situation; this keeps the API self-explanatory
// during local development without requiring a Vite dev server.
func Handler(logger *slog.Logger) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !HasBuild() {
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            w.WriteHeader(http.StatusOK)
            _, _ = w.Write([]byte(devPlaceholderHTML))
            return
        }
        sub := FS()
        // 1. Try static file
        path := strings.TrimPrefix(r.URL.Path, "/")
        if path == "" {
            path = "index.html"
        }
        f, err := sub.Open(path)
        if err == nil {
            defer f.Close()
            info, err := f.Stat()
            if err == nil && !info.IsDir() {
                http.ServeContent(w, r, path, info.ModTime(), readSeeker(f))
                return
            }
        } else if !errors.Is(err, fs.ErrNotExist) {
            if logger != nil {
                logger.Warn("embed open failed", "path", path, "err", err)
            }
        }
        // 2. SPA fallback: serve index.html
        indexFile, err := sub.Open("index.html")
        if err != nil {
            if logger != nil {
                logger.Error("index.html missing in embed", "err", err)
            }
            http.Error(w, "frontend not available", http.StatusInternalServerError)
            return
        }
        defer indexFile.Close()
        info, err := indexFile.Stat()
        if err != nil {
            http.Error(w, "frontend not available", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        http.ServeContent(w, r, "index.html", info.ModTime(), readSeeker(indexFile))
    })
}

const devPlaceholderHTML = `<!doctype html>
<html lang="zh-CN">
<head>
    <meta charset="utf-8" />
    <title>EasySearch (dev)</title>
    <style>
        body { font: 14px -apple-system, "Segoe UI", "PingFang SC", sans-serif;
               background: #0f1115; color: #e5e7eb; padding: 40px; max-width: 720px; margin: 0 auto; }
        code { background: #181b22; padding: 2px 6px; border-radius: 4px; }
        h1 { font-size: 20px; }
        .hint { color: #9ca3af; }
    </style>
</head>
<body>
    <h1>EasySearch &mdash; 开发模式占位页</h1>
    <p class="hint">前端构建未找到。请先在前端目录运行 <code>npm run build</code>，或启动 Vite 开发服务器：</p>
    <pre><code>cd frontend
npm run dev   # http://127.0.0.1:5173</code></pre>
    <p class="hint">API 端点仍然可用，例如 <code id="api">/api/v1/system/status</code>。</p>
    <script>document.getElementById('api').href = location.origin + '/api/v1/system/status';</script>
</body>
</html>
`
