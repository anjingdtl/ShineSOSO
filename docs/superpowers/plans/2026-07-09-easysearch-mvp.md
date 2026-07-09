# EasySearch MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Windows-shipping personal resource search tool (EasySearch) that lets a local user add public indexers, search across them concurrently, and click/copy download entries — delivered as a single Go executable with embedded React WebUI.

**Architecture:** Go 1.24 backend (net/http + chi router, modernc.org/sqlite for pure-Go SQLite, goquery for HTML) wrapped around a clean Search Orchestrator that fans out to pluggable `IndexerAdapter`s. Declarative YAML indexer definitions drive most public sites; a Torznab adapter handles the rest. React 18 + TypeScript + Vite frontend embedded via `embed.FS`, served from a random localhost port.

**Tech Stack:**
- Backend: Go 1.24, `go-chi/chi/v5`, `modernc.org/sqlite`, `github.com/PuerkitoBio/goquery`, `gopkg.in/yaml.v3`, `gopkg.in/natefinch/lumberjack.v2`
- Frontend: React 18, TypeScript 5, Vite 6, React Router 6, TanStack Query v5, Zustand 4, Vitest, Playwright
- Build: PowerShell scripts (Windows) and bash (dev only)
- Tests: Go `testing` + `net/http/httptest`, Vitest + @testing-library/react, Playwright E2E

## Global Constraints (from spec-o1.md)

- **Format:** 4-space indent; Go `PascalCase` exported / `camelCase` internal; TypeScript `camelCase`.
- **API prefix:** `/api/v1`; SSE at `/api/v1/search/sessions/{id}/events`.
- **Listen:** `127.0.0.1` random port; never `0.0.0.0`.
- **DB path:** `%APPDATA%/EasySearch/data/easysearch.db`; auto-migrate; corrupt DB → keep file + recover prompt.
- **Categories (locked):** `all, movie, tv, music, game, software, book, anime, other`.
- **Sort (locked):** `relevance, seeders, publishedAt, sizeDesc, sizeAsc`.
- **Result status:** `pending | running | success | empty | timeout | error | cancelled`.
- **Indexer health:** `healthy | degraded | unhealthy | unknown | disabled`.
- **SSE events:** `session_started | indexer_started | indexer_result | indexer_completed | indexer_failed | results_merged | session_completed | session_cancelled`.
- **Error codes:** `INVALID_REQUEST, EMPTY_KEYWORD, NO_ENABLED_INDEXERS, INDEXER_NOT_FOUND, INDEXER_DISABLED, INDEXER_TIMEOUT, INDEXER_NETWORK_ERROR, INDEXER_HTTP_ERROR, INDEXER_PARSE_ERROR, INDEXER_RATE_LIMITED, INVALID_INDEXER_DEFINITION, UNSAFE_INDEXER_URL, CATALOG_UPDATE_FAILED, SEARCH_CANCELLED, INTERNAL_ERROR`.
- **Security:** block private/loopback/link-local IPs; block `file://`, `ftp://`, `gopher://`; redirect cap 5; max body 10 MB; max YAML 512 KB; re-validate IP after every DNS resolution.
- **Concurrency defaults (overridable via internal settings only):** 6 concurrent indexers, 15 s per-indexer timeout, 30 s total timeout, 100 results/indexer, 1000 raw results/search.
- **Privacy:** never log full magnet links; never persist search results; never long-term-log search keywords.
- **Logs:** 10 MB × 5 files, 30-day auto-clean, default level `info`.
- **No scope creep:** no private sites, no login/Cookie/API Key, no captcha bypass, no FlareSolverr, no headless browser, no Sonarr/Radarr sync, no RSS, no downloader management, no Usenet, no telemetry.
- **Definition of Done:** code + format/lint + unit test + integration test + error path + UI loading/success/empty/error states + DB migration + Windows build + no out-of-scope feature.

## File Structure

```
ShineSOSO/
├── spec-o1.md
├── README.md
├── .gitignore
├── go.mod
├── docs/superpowers/plans/2026-07-09-easysearch-mvp.md
├── backend/
│   ├── cmd/easysearch/main.go
│   ├── internal/
│   │   ├── config/
│   │   ├── logging/
│   │   ├── model/             # indexer.go, result.go, search.go, catalog.go
│   │   ├── storage/           # sqlite.go, migrations.go, *repo.go
│   │   ├── security/          # url_validator.go, network_policy.go, sanitize.go
│   │   ├── normalize/         # title.go, size.go, date.go, infohash.go
│   │   ├── indexer/           # adapter.go, factory.go, declarative.go, torznab.go,
│   │   │                      # request_builder.go, html_parser.go, json_parser.go,
│   │   │                      # xml_parser.go, health.go
│   │   ├── search/            # orchestrator.go, event.go, normalizer.go,
│   │   │                      # deduper.go, ranker.go, filters.go
│   │   ├── catalog/           # loader.go, updater.go, validator.go
│   │   ├── api/               # router.go, *handler.go, errors.go
│   │   └── webembed/embed.go  # go:embed frontend dist
│   ├── indexers/              # built-in catalog (manifest.json + *.yml)
│   ├── testdata/              # fixtures for tests
│   └── web/                   # built frontend (gitignored)
├── frontend/
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   ├── index.html
│   ├── src/
│   │   ├── main.tsx
│   │   ├── app/                 # App.tsx, router.tsx, providers.tsx
│   │   ├── pages/               # SearchPage.tsx, IndexerPage.tsx
│   │   ├── features/search/     # SearchBar, SearchFilters, SearchStatus,
│   │   │                        # ResultList, ResultCard, useSearchStream
│   │   ├── features/indexers/   # InstalledIndexerList, CatalogList, IndexerCard,
│   │   │                        # ImportDialog, TestResultDialog
│   │   ├── services/            # api.ts, search.ts, indexers.ts
│   │   ├── stores/uiStore.ts
│   │   ├── types/index.ts
│   │   └── utils/format.ts
│   └── tests/                   # unit/ + e2e/
├── scripts/                  # dev.sh, dev.ps1, build.ps1, smoke.ps1
└── .github/workflows/ci.yml  # optional
```

## Execution Mode

**Inline execution with phase checkpoints.** Each phase ends with review + commit gate. Subagent-driven burns context on scaffold; TDD discipline is best applied to focused logic (normalization, dedup, security, parsing). Scaffold tasks are batched with explicit review gates.

Conventional Commits, scope `easysearch` or `easysearch-<layer>`. Each task ends with `git commit -m "<type>(scope): <subject>"`.

---

# Phase 1 — Project Skeleton (9 tasks)

**Phase deliverable:** `easysearch.exe` boots, binds random `127.0.0.1` port, auto-opens browser to the (placeholder) WebUI, returns `/api/v1/system/status` JSON. Frontend dev server proxies to Go. Windows build produces a single exe with embedded `web/dist`.

| # | Task | Outcome |
|---|---|---|
| 1 | Repo bootstrap | `go.mod`, `main.go --version`, `.gitignore`, `README.md` |
| 2 | Config + logger | `config.Default()`, rotating slog to `data/logs/easysearch.log` |
| 3 | HTTP server + status endpoint | `chi` router on `127.0.0.1:0`, `GET /api/v1/system/status` returns `{version, uptime}` |
| 4 | Auto-port + browser launcher | Pick free port, write `.port` file, optionally `exec.Command` open browser |
| 5 | Vite + React + TS scaffold | `frontend/` with React 18, TS strict, two empty pages routed at `/` and `/indexers` |
| 6 | Embed frontend | `//go:embed all:web` in `webembed/embed.go`; fallback when `web/` missing in dev |
| 7 | Vite dev proxy | `vite.config.ts` proxies `/api` to `http://127.0.0.1:<port>` reading `.port` |
| 8 | Two-page UI skeleton | `SearchPage` (empty state "尚未添加索引器") + `IndexerPage` (placeholder) |
| 9 | Windows build script | `scripts/build.ps1`: `go build` with `-ldflags "-H windowsgui"`, frontend dist embedded |

**Phase 1 review gate:**
- [ ] `go build` produces `easysearch.exe`; double-click launches silently; browser opens
- [ ] `GET /api/v1/system/status` returns `200` JSON
- [ ] Frontend dev server shows Search and Indexers pages
- [ ] `git log --oneline` shows 9 clean commits
- [ ] `scripts/build.ps1` produces a single exe that boots without Node installed

**Commit template (Phase 1 final):**
```bash
git commit --allow-empty -m "chore(easysearch): phase 1 review gate passed"
```

---

# Phase 2 — Single Indexer Search (10 tasks)

**Phase deliverable:** With one HTML mock indexer, the WebUI can search, see SSE progress, and click "复制" / "打开" on each result. The mock serves fixture HTML via `httptest`.

| # | Task | Outcome |
|---|---|---|
| 10 | Models | `model/indexer.go`, `model/result.go`, `model/search.go` (Go structs matching spec §11) |
| 11 | HTTP client w/ SSRF guard | `indexer.NewClient()` with `security.URLValidator`, per-request timeout, redirect cap 5, post-DNS IP re-check |
| 12 | `IndexerAdapter` interface | `adapter.go` with `Test`, `Search`, `ID`; `factory.go` with registry |
| 13 | HTML parser core | `html_parser.go` with `Selector`, `Attr`, `Text` extraction (uses `goquery`) |
| 14 | Mock indexer server | `testdata/fixtures/html/mock-indexer.go` exposing `/search` returning canned HTML |
| 15 | Mock adapter | `indexer/mock_adapter.go` (test-only build tag) wraps fixtures |
| 16 | `normalizer` package | `normalize/title.go` (lowercase, NFKC, strip punctuation), `size.go` (B/KiB/MiB/... parser), `date.go` (multi-layout parser → UTC), `infohash.go` (40-hex + Base32 BTIH) |
| 17 | `normalizer` tests | TDD: each parser has a table-driven test with edge cases |
| 18 | `SearchOrchestrator` (1 indexer) | `search/orchestrator.go` runs a single indexer, emits SSE events to a `chan event.Event` |
| 19 | SSE handler | `api/search_handler.go` `POST /sessions` creates session, `GET /sessions/{id}/events` streams SSE |
| 20 | ResultCard UI | `ResultCard.tsx` shows title, size, seeders, sources, `复制`/`打开` buttons calling `/api/v1/system/open` |

**Phase 2 review gate:**
- [ ] Unit tests for all 4 normalizers + URL validator pass; coverage ≥ 80% in those packages
- [ ] `httptest` integration: search returns ≥ 1 result within 5 s against mock
- [ ] WebUI shows live progress + result cards
- [ ] Magnet 链接 click → copies to clipboard
- [ ] No panics on cancel, timeout, parse error (each returns SSE `indexer_failed`)

**Commit template:**
```bash
git commit --allow-empty -m "chore(easysearch): phase 2 single-indexer review gate passed"
```

---

# Phase 3 — Multi-Indexer Aggregation (8 tasks)

**Phase deliverable:** Search fans out to N enabled indexers with bounded concurrency, per-indexer timeout, total timeout, live cancel, dedup (strong + weak), score-based sort.

| # | Task | Outcome |
|---|---|---|
| 21 | Concurrency primitive | `errgroup.WithContext` + semaphore channel sized by config |
| 22 | Session store | in-memory `map[sessionID]*Session` with cancel funcs; auto-prune on `done` |
| 23 | Per-indexer timeout context | wrap each `Search(ctx, ...)`; emit `indexer_failed` w/ `INDEXER_TIMEOUT` |
| 24 | Status enum mapping | `Running`/`Success`/`Empty`/`Timeout`/`Error`/`Cancelled`; emit at every transition |
| 25 | `deduper` package | `deduper.go` strong (InfoHash/normalized URL) + weak (title 0.92, size ±2%, key features) |
| 26 | `ranker` package | `ranker.go` `Compute(result, query, sourceCount) float64` per spec §18 |
| 27 | `filters` package | `filters.go` Apply(query, results) (minSize, maxSize, minSeeders, publishedAfter, indexerIDs) |
| 28 | Orchestrator end-to-end | fans out, merges incrementally, emits `results_merged` after each indexer completes |

**Phase 3 review gate:**
- [ ] 5 mock indexers, 1 always-times-out → search returns results from the other 4 with their status
- [ ] Cancel mid-search stops within 500 ms
- [ ] Total wall time ≤ 30 s even with all indexers slow
- [ ] Dedup: 2 indexers returning same InfoHash → 1 card with 2 sources
- [ ] Sort by `seeders`: highest first; ties broken by `publishedAt` desc, then title

**Commit template:**
```bash
git commit --allow-empty -m "chore(easysearch): phase 3 multi-indexer review gate passed"
```

---

# Phase 4 — Indexer Management (10 tasks)

**Phase deliverable:** Full indexer CRUD UI: see installed list, browse built-in catalog, add with one click (auto-test), enable/disable, delete. Health status visible. State persists across restarts.

| # | Task | Outcome |
|---|---|---|
| 29 | SQLite open + migrations | `storage/sqlite.go` opens `%APPDATA%/EasySearch/data/easysearch.db`; runs `migrations.go` (4 tables per spec §20) |
| 30 | Repositories | `indexer_repo.go` (CRUD), `health_repo.go` (insert + 5000/30d prune), `settings_repo.go` |
| 31 | `GET /api/v1/indexers` | list installed w/ last health snapshot |
| 32 | `POST /api/v1/indexers` | add from catalog; runs test; saves as `enabled` if pass, else `disabled` with `last_error` |
| 33 | `PATCH /api/v1/indexers/{id}` | toggle enabled, update base URL |
| 34 | `DELETE /api/v1/indexers/{id}` | remove; catalog definition remains |
| 35 | `POST /api/v1/indexers/{id}/test` | runs `Test()`, writes health event, returns duration + error |
| 36 | `GET /api/v1/indexer-catalog` | list built-in definitions with `installed` flag |
| 37 | Background health loop | `indexer/health.go` scheduler: on start + every 12 h, skip if checked < 30 min ago |
| 38 | IndexerPage UI | 3 sections (Installed, Catalog, Import); cards show health badge, response time, last error |

**Phase 4 review gate:**
- [ ] Add mock indexer → status `healthy` after test → appears in Search
- [ ] Disable → not searched; card shows "已停用"
- [ ] Re-enable → searchable again
- [ ] Delete → removed from list; can re-add from catalog
- [ ] Restart app → state persists
- [ ] Health auto-check fires 12 h after start (verified with clock injection in test)

**Commit template:**
```bash
git commit --allow-empty -m "chore(easysearch): phase 4 indexer-management review gate passed"
```

---

# Phase 5 — YAML Engine (10 tasks)

**Phase deliverable:** Define a new public indexer by writing a YAML file. No Go code change required. `POST /api/v1/indexer-catalog/import` accepts user YAML, validates against spec §13, tests, and saves.

| # | Task | Outcome |
|---|---|---|
| 39 | `catalog/loader.go` | read `indexers/manifest.json` + `definitions/*.yml` from `embed.FS` |
| 40 | YAML struct | `catalog/definition.go` mirrors spec §13 (schema, id, name, links, categories, search, response, fields) |
| 41 | `catalog/validator.go` | check id regex, https, schema version, no local IPs, ≤ 512 KB, etc. (spec §13.8) |
| 42 | `declarative.go` (adapter) | uses `request_builder.go` to compose URL; `html/json/xml_parser.go` to extract fields |
| 43 | Field extractors | HTML `text/attr`, JSON `json_path` (restricted subset), XML `xpath` (restricted) |
| 44 | Filters | `trim/lower/upper/replace/regex_extract/parse_int/parse_float/parse_size/parse_date/resolve_url/extract_info_hash` |
| 45 | Template engine | restricted `text/template` with allow-list vars `query.*`, `indexer.base_url`; no funcs except safe ones |
| 46 | `POST /api/v1/indexer-catalog/import` | multipart upload, validate, test, return decision; user can choose: enabled / disabled / cancel |
| 47 | ImportDialog UI | drop file or browse, show validation errors inline, show test result, 3 actions |
| 48 | Example YAMLs | ship 2 fixtures: `example-public-html.yml` + `example-torznab.yml` |

**Phase 5 review gate:**
- [ ] Add `example-public-html.yml` (mock-backed fixture) → search works
- [ ] Add invalid YAML (private IP, missing title) → rejected with specific error code
- [ ] Add YAML exceeding 512 KB → rejected
- [ ] Add YAML with code-injection attempt in selector → safe (sandboxed)

**Commit template:**
```bash
git commit --allow-empty -m "chore(easysearch): phase 5 yaml-engine review gate passed"
```

---

# Phase 6 — Torznab & Catalog Update (6 tasks)

**Phase deliverable:** Generic Torznab adapter works against any spec-compliant public Torznab endpoint. Catalog update mechanism with manifest + SHA-256 + version fallback.

| # | Task | Outcome |
|---|---|---|
| 49 | `torznab.go` adapter | builds `?t=search&q=...&cat=...`; parses RSS `item` + Torznab attrs |
| 50 | Torznab field mapping | title, link, enclosure, pubDate, size, seeders/peers/grabs, magneturl, infohash |
| 51 | Catalog manifest | `indexers/manifest.json` w/ `{schema, version, generatedAt, definitions[{id, version, file, sha256}]}` |
| 52 | `catalog/updater.go` | fetch manifest from configurable URL; SHA-256 check each yml; atomic swap; rollback on failure |
| 53 | `POST /api/v1/indexer-catalog/update` | triggers update; returns new version + changed list |
| 54 | Definition version tracking | `installed_indexers.definition_version` updated on swap; user enable state preserved |

**Phase 6 review gate:**
- [ ] Torznab mock adapter returns 5 results from RSS XML
- [ ] Catalog update with valid manifest → new yml loaded
- [ ] Catalog update with bad SHA-256 → rejected, no change
- [ ] Catalog update with broken yml → previous version kept

**Commit template:**
```bash
git commit --allow-empty -m "chore(easysearch): phase 6 torznab-and-catalog review gate passed"
```

---

# Phase 7 — Testing & Release (8 tasks)

**Phase deliverable:** Backend ≥ 80% coverage on core packages. Frontend component tests for critical flows. E2E happy path green. Windows single-exe release artifact. User-facing docs.

| # | Task | Outcome |
|---|---|---|
| 55 | Backend unit coverage | `go test -coverprofile` for `normalize/`, `security/`, `search/`, `indexer/` |
| 56 | Backend integration tests | `tests/integration/` exercising full HTTP API via `httptest`; covers spec §27.2 scenarios |
| 57 | Frontend unit tests | Vitest for `useSearchStream`, `format.ts`, `ResultCard`, `SearchStatus` |
| 58 | Frontend E2E | Playwright: launch app, add mock indexer, search, copy magnet |
| 59 | Smoke script | `scripts/smoke.ps1` boots binary, hits `/system/status`, downloads + parses a sample result, exits |
| 60 | Diagnostic export | `GET /api/v1/system/diagnostics` returns ZIP (version, OS, anonymized health, schema version, no magnet/no keywords) |
| 61 | README & user guide | install, first-run, add indexer, search, troubleshooting, FAQ |
| 62 | Release checklist | run all 28 acceptance criteria from spec §28; mark each in CHANGELOG |

**Phase 7 review gate:**
- [ ] `go test ./...` green; coverage report shows ≥ 80% on `normalize`, `security`, `search`, `indexer`
- [ ] `npm test` green for all unit tests
- [ ] `npm run e2e` Playwright happy path passes
- [ ] `scripts/build.ps1` produces a working `easysearch.exe`
- [ ] All spec §28 checkboxes pass

**Final commit:**
```bash
git commit --allow-empty -m "release(easysearch): v0.1.0 MVP"
git tag v0.1.0
```

---

# Total: 62 tasks across 7 phases

This is intentionally a 7-review-gate plan, not 62 review gates. The phase boundaries are where human attention is most valuable; within a phase we keep TDD discipline but batch the work.

## Cross-Phase Conventions

- **No silent cap changes.** If we discover a missing requirement, add a task in the current phase, do not silently drop it.
- **Definition changes require a spec update.** If we need to deviate from `spec-o1.md`, update the spec and reference it in the commit.
- **Test data lives in `backend/testdata/`.** Never commit real magnet links or copyrighted content.
- **All user-visible strings go through a single `frontend/src/i18n/zh-CN.ts` file** (Chinese only for v1; structure supports adding `en-US` later).
- **All user-visible error messages use a centralized `api/errors.go` map** from `error_code → {code, message_zh}`.

## Spec Coverage Map

| Spec § | Covered in |
|---|---|
| §1-5 overview / scope | (this plan, by definition) |
| §6 Search page | Phase 2 + 3 UI tasks (8, 20, 38) |
| §7 Indexer page | Phase 4 task 38 |
| §8 Tech architecture | Phase 1 task 6 (embed) |
| §9 Backend layout | Phase 1+ scaffold |
| §10 Frontend layout | Phase 1 task 5 + phase-specific |
| §11 Data models | Phase 2 task 10 |
| §12 Adapter interface | Phase 2 task 12 |
| §13 YAML spec | Phase 5 |
| §14 Torznab | Phase 6 |
| §15 Search flow | Phase 3 |
| §16 Normalization | Phase 2 task 16 |
| §17 Dedup | Phase 3 task 25 |
| §18 Ranking | Phase 3 task 26 |
| §19 API | Phases 2, 3, 4, 5, 6 |
| §20 SQLite | Phase 4 task 29 |
| §21 Network/security | Phase 1 task 11 + throughout |
| §22 Errors | Phase 1 task 3 + central map |
| §23 Performance | Implicit (concurrency limits in orchestrator) |
| §24 Accessibility | UI tasks specify semantic HTML + Enter-key support |
| §25 Logs/diagnostics | Phase 1 task 2 + Phase 7 task 60 |
| §26 Catalog update | Phase 6 |
| §27 Tests | Phase 7 |
| §28 Acceptance | Phase 7 task 62 |
| §29 Phases | (this plan) |
| §30 Agent rules | (followed throughout) |
| §31 DoD | (followed throughout) |
| §32 MVP shape | All phases |
