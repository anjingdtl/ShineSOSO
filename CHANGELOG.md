# CHANGELOG

All notable changes to EasySearch are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added (Phase 7)

- `GET /api/v1/system/diagnostics` — sanitized diagnostic ZIP bundle
  (version, OS, schema version, indexer summary, redacted logs)
- New `internal/diagnostics` package with magnet + keyword redaction
  (`sanitize.go`) and ZIP builder (`diagnostics.go`)
- `Store.SchemaVersion()` — highest applied migration version
- `IndexerRepo.DiagnosticsSummary()` — flat per-indexer status for
  diagnostics
- `scripts/smoke.ps1` — full end-to-end smoke test (boot → add indexer
  → search → diagnostics → shutdown)
- `docs/USER_GUIDE.md` — user-facing manual covering install, search,
  YAML import, diagnostics, FAQ
- **JSON / XML 声明式 adapter** — `internal/indexer/declarative{_json,_xml}.go`。工厂现在接受 `Format: "" | html | json | xml`；继续拒绝 `torznab` 由 Torznab 路径处理。详见 progress.md。
- **目录 manifest Ed25519 签名** — `internal/catalog/sign.go` + `updater.go` 验证闸。可选用 `EASYSEARCH_CATALOG_PUBKEY` 公钥启用；空值保持 SHA-256-only 行为。

### Removed

- Historical `projects/ShineSOSO/` directory removed (pre-superproject era leftover; v0.1.0 release lives at repo root). Pre-cleanup state preserved as git tag `backup-pre-reset` (commit `2339f5e`).

## [0.4.0] - 2026-07-09 (Phase 6)

### Added

- **Torznab adapter** (`internal/indexer/torznab.go`) — generic
  RSS+Torznab-attr parser with multi-format pubDate handling
- **Builtin catalog** (`internal/catalog/builtin/`) — embedded
  manifest.json + YAML definitions + SHA-256 checksums
- **Catalog updater** (`internal/catalog/updater.go`) — manifest
  fetch → SHA-256 verify → atomic swap with rollback on failure
- **`POST /api/v1/indexer-catalog/update`** + **`GET .../status`**
- `internal/cmd/catalog-manifest` — manifest regeneration tool
- `EASYSEARCH_CATALOG_URL` env var for remote catalog source
- `IndexerRepo.BumpDefinitionVersion()` — keeps user enable state
  across catalog updates
- `scripts/phase6-smoke.ps1` — Torznab + catalog update verification
- 13 new unit tests (5 builtin + 8 updater, plus existing torznab
  tests)

### Changed

- `cmd/easysearch/main.go` now activates the embedded catalog on boot
  and wires the updater into the router

## [0.3.0] - 2026-07-09 (Phase 5 — YAML engine)

### Added

- `internal/catalog/loader.go` — strict YAML loader (512 KB cap)
- `internal/catalog/validator.go` — spec §13.8 rule enforcement
  (id regex, HTTPS, schema=1, no private IPs, template whitelist)
- Restricted `text/template` engine (only `.Query.*` + `.Indexer.BaseURL`)
- 11 filters (`trim`, `lower`, `upper`, `replace`, `regex_extract`,
  `parse_int`, `parse_float`, `parse_size`, `parse_date`,
  `resolve_url`, `extract_info_hash`)
- `internal/indexer/declarative.go` — HTML adapter driven by YAML
- `POST /api/v1/indexer-catalog/import` + `GET .../imported`
- `ImportDialog` UI (modal, file/paste, inline errors, 3 actions)
- 2 example YAMLs (`example-public-html.yml`, `example-torznab.yml`)
- `scripts/phase5-smoke.ps1` — 12/12 checks pass
- `ImportedDefinitionRepo` with SHA-256 checksum persistence

## [0.2.0] - 2026-07-08 (Phase 4 — indexer management)

### Added

- SQLite (pure Go via `modernc.org/sqlite`) + 4-table migration
- `IndexerRepo`, `HealthRepo`, `SettingsRepo`
- `GET/POST/PATCH/DELETE /api/v1/indexers`
- `POST /indexers/{id}/test` — manual health probe
- `GET /api/v1/indexer-catalog`
- Background health loop (12h cadence, skip-if-checked-within-30m)
- 3-section IndexerPage UI (Installed / Catalog / Import)
- `scripts/phase4-smoke.ps1`

## [0.1.0] - 2026-07-08 (Phase 3 — multi-indexer aggregation)

### Added

- `errgroup.WithContext` + semaphore (default 6 concurrent)
- Strong dedup (InfoHash / normalized URL) + weak dedup
  (title 0.92 + size ±2%)
- `internal/search/ranker.go` — `Compute(result, query, sourceCount)`
- `internal/search/filters.go` — minSize / maxSize / minSeeders /
  publishedAfter / indexerIDs
- Per-indexer timeout + total timeout + cancel propagation

## [0.0.1] - 2026-07-07 (Phase 2 — single indexer search)

### Added

- Data models: `IndexerDefinition`, `SearchResult`, `SearchQuery`,
  `SearchSession`
- `indexer.Client` with SSRF guard + redirect cap 5 + post-DNS IP
  re-check
- `IndexerAdapter` interface + `AdapterFactory` registry
- HTML parser (`goquery`), normalizers (title/size/date/infohash)
- `SearchOrchestrator` (single indexer) + SSE handler
- `ResultCard` UI

## [0.0.0] - 2026-07-07 (Phase 1 — skeleton)

### Added

- `go.mod` (Go 1.24)
- Vite 6 + React 18 + TypeScript (strict) scaffold
- `chi` router + rotating `slog` (10 MB × 5 files, 30 day retention)
- `//go:embed all:web` frontend embedding
- `scripts/build.ps1` — single-file `easysearch.exe` with
  `-ldflags "-H windowsgui"`
- Auto-port + browser launch + `.port` file
- Two-page UI shell (Search / Indexers)

---

## Notes

- Internal build version is `0.4.0` (Phase 6 gate).
- External release tag for first public MVP will be `v0.1.0` per
  `docs/superpowers/plans/2026-07-09-easysearch-mvp.md` §"Final
  commit".
- All phases 1–6 are committed to the main branch. Phase 7 deliverables
  are landing in `Unreleased` above.
