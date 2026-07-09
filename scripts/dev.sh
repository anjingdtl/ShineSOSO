#!/usr/bin/env bash
# Start the Go backend and Vite dev server side-by-side.
# The Go server writes .port under $EASYSEARCH_DATA_DIR; the Vite proxy
# reads it on startup.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DATA_DIR="${EASYSEARCH_DATA_DIR:-$ROOT/backend/.devdata}"
PORT="${EASYSEARCH_PORT:-0}"

mkdir -p "$DATA_DIR/logs"

cleanup() {
    if [[ -n "${BACKEND_PID:-}" ]] && kill -0 "$BACKEND_PID" 2>/dev/null; then
        echo "stopping backend (pid=$BACKEND_PID)"
        kill "$BACKEND_PID" 2>/dev/null || true
        wait "$BACKEND_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT INT TERM

echo "[dev] starting backend on random port (data dir: $DATA_DIR) ..."
EASYSEARCH_DATA_DIR="$DATA_DIR" \
    go -C "$ROOT" run ./backend/cmd/easysearch \
    --port "$PORT" --no-browser \
    > "$DATA_DIR/logs/backend.log" 2>&1 &
BACKEND_PID=$!

# Wait for the .port file to appear (backend writes it before serving).
for i in {1..30}; do
    if [[ -f "$DATA_DIR/.port" ]]; then break; fi
    sleep 0.2
done
if [[ ! -f "$DATA_DIR/.port" ]]; then
    echo "[dev] backend did not write .port file in time" >&2
    cat "$DATA_DIR/logs/backend.log" >&2
    exit 1
fi
echo "[dev] backend listening on port $(cat "$DATA_DIR/.port")"

echo "[dev] starting vite dev server ..."
cd "$ROOT/frontend"
exec npm run dev
