# EasySearch

A local-only resource search tool for public indexers.

## Status

Phase 1 — skeleton. See [`docs/superpowers/plans/2026-07-09-easysearch-mvp.md`](docs/superpowers/plans/2026-07-09-easysearch-mvp.md) for the full 7-phase implementation plan.

## What it is

A Windows-shipping desktop app (single Go executable with embedded React WebUI) that lets a local user:

1. Add public indexers with one click.
2. Search across them concurrently.
3. See unified, deduplicated, ranked results.
4. Click "Copy" / "Open" on any download entry (magnet / torrent / direct).

Not in scope (see `spec-o1.md` §3.2 for the full list): private site auth, RSS, downloader management, Sonarr/Radarr sync, captcha bypass, headless browser, telemetry, cloud sync.

## Build (dev)

```bash
go build -o backend/easysearch.exe ./backend/cmd/easysearch
./backend/easysearch.exe --version
# 0.1.0
```

## Build (Windows release)

```powershell
.\scripts\build.ps1
```

Produces a single `easysearch.exe` with the frontend embedded.

## Layout

```
backend/       — Go server (cmd/easysearch + internal/*)
frontend/      — React + Vite WebUI
docs/          — Spec & implementation plan
scripts/       — Build & smoke scripts
```

## Spec & Plan

- `spec-o1.md` — product + technical spec (source of truth)
- `docs/superpowers/plans/2026-07-09-easysearch-mvp.md` — phased implementation plan

## Requirements

- Go 1.24+
- Node 18+ and npm (for frontend dev)
- Windows 10/11 (release target)
