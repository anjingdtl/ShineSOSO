# EasySearch 项目进度

> **项目代号**：EasySearch（目录名 `ShineSOSO`，是工作空间级代号沿用）
> **目标**：Windows 单可执行文件的本地资源搜索工具 — Go 1.24 后端 + React 18 前端，嵌入式 SQLite，并发多索引器搜索。
> **规格**：`spec-o1.md`（v0.x 草案）
> **实施计划**：`docs/superpowers/plans/2026-07-09-easysearch-mvp.md`

---

## 总览

| 维度 | 数值 |
|---|---|
| 总计划任务 | 62 |
| 已完成 | 55 (89%) |
| 当前阶段 | Phase 6 ✅ 完成 |
| 下一阶段 | Phase 7 — 测试与发布 |
| commit 数（含 review gate） | 47 |
| 最终可执行文件大小 | ~14.6 MB（`dist/easysearch.exe`，单文件，无外部依赖） |
| 后端 Go 包 | 14（含 2 个 sub-package） |
| 单元测试总数 | ~95+（含 16 个 Torznab + 8 个 Updater + 14 个 catalog builtin） |

---

## 各 Phase 完成情况

### ✅ Phase 1 — 项目骨架（9/9）
**目标**：`easysearch.exe` 启动后绑定 `127.0.0.1:0` 随机端口，自动打开浏览器，返回 `/api/v1/system/status`。

- `go.mod`、Vite 6 + React 18 + TS strict 脚手架
- `chi` 路由 + `slog` 旋转日志（10 MB × 5 文件，30 天）
- `//go:embed all:web` 前端嵌入 + dev proxy
- PowerShell `build.ps1` 产出单 exe

### ✅ Phase 2 — 单索引器搜索（10/10）
**目标**：对一个 HTML mock 索引器能搜索、看 SSE 进度、点击"复制"/"打开"。

- `model.IndexerDefinition / SearchResult / SearchQuery / SearchSession`
- `indexer.Client` 含 SSRF 防护、redirect cap 5、post-DNS IP re-check
- `IndexerAdapter` 接口 + `AdapterFactory` registry
- HTML 解析（`PuerkitoBio/goquery`）、normalizer（title/size/date/infohash）
- `SearchOrchestrator` 单 indexer、SSE handler、`ResultCard.tsx`

### ✅ Phase 3 — 多索引器聚合（8/8）
**目标**：N 个 enabled indexer 并发聚合，含 per-indexer timeout、总 timeout、cancel、dedup、score 排序。

- `errgroup.WithContext` + semaphore（默认 6 并发）
- 强 dedup（InfoHash / normalized URL）+ 弱 dedup（title 0.92，size ±2%）
- `ranker.Compute(result, query, sourceCount)`
- `filters.Apply(query, results)` 支持 minSize / maxSize / minSeeders / publishedAfter / indexerIDs

### ✅ Phase 4 — 索引器管理（10/10）
**目标**：完整 indexer CRUD UI：内置目录浏览、添加、启用/停用、删除、健康状态可见、状态持久化。

- SQLite（`modernc.org/sqlite` 纯 Go）+ 4 张表 migration
- `indexer_repo`、`health_repo`、`settings_repo`
- `GET/POST/PATCH/DELETE /api/v1/indexers` + `POST /indexers/{id}/test`
- 后台 health loop（每 12 h，跳过 < 30 min 已查）
- IndexerPage UI 三段式（Installed / Catalog / Import）

### ✅ Phase 5 — YAML 引擎（10/10）
**目标**：写一个 YAML 文件就能定义一个新公共索引器。

- `catalog.DefinitionFile` / Loader（512 KB 上限，strict mode）
- `catalog.Validator` §13.8 全规则（schema、id 正则、HTTPS、type=public、私有 IP 屏蔽、模板白名单）
- 受限 `text/template`（仅 `.Query.*` + `.Indexer.BaseURL`）
- 11 个过滤器（trim/lower/upper/replace/regex_extract/parse_int/parse_float/parse_size/parse_date/resolve_url/extract_info_hash）
- `declarative.go` HTML adapter（Phase 5 只支持 HTML）
- `POST /api/v1/indexer-catalog/import` + `GET /indexer-catalog/imported`
- `ImportedDefinitionRepo`（SHA-256 校验和持久化）
- `ImportDialog` UI（modal，文件选择/粘贴 + 内联错误 + 三段动作）
- 两个示例 YAML（`example-public-html.yml` + `example-torznab.yml`）
- `scripts/phase5-smoke.ps1`：12/12 check 全过

### ✅ Phase 6 — Torznab 与目录更新（7/7）  ← **当前位置**
**目标**：通用 Torznab 适配器 + 远程目录更新机制（manifest + SHA-256 + 版本回退）。

#### 关键交付
- `internal/indexer/torznab.go` — 通用 Torznab 适配器
  - `?t=search&q=…&cat=…` URL 构建（覆盖默认值）
  - `encoding/xml` 解析 RSS + `torznab:attr` 字段映射
  - title/link/enclosure/pubDate/size/seeders/peers/grabs/magneturl/infohash
  - 自动从 infohash 构造 magnet
  - 多 pubDate 格式兼容（RFC1123Z / RFC1123 / RFC850 / ANSIC / 自定义）
- `internal/catalog/builtin/` — 嵌入式目录
  - `manifest.json`（schema=1, version, generatedAt, definitions）
  - `definitions/*.yml` + `signatures/`（预留）
  - `cmd/catalog-manifest/` 重新生成 manifest 的工具
- `internal/catalog/updater.go` — 远程更新机制
  - 拉取 manifest → SHA-256 校验 → §13.8 校验 → 原子切换
  - source 标签（embedded / cache / remote）
  - rollback-on-failure
  - `OnDefinitionActivated` 回调 → `BumpDefinitionVersion` 同步 installed_indexers
- `internal/store/indexer_repo.go` 新增 `BumpDefinitionVersion`
- `internal/api/catalog_update_handler.go` — `POST /api/v1/indexer-catalog/update` + `GET /indexer-catalog/status`
- `internal/security/url_validator.go` 加测试用 `AllowLoopback` 开关
- `internal/config/config.go` 加 `CatalogManifestURL`（env `EASYSEARCH_CATALOG_URL`）
- `scripts/phase6-smoke.ps1` — 4 步 / 6 check 全过

#### 单元测试覆盖（新增 13 项）
- `internal/catalog/builtin` — 5 项（manifest parse、checksum verify、YAML parse、all YAMLs、stable IDs）
- `internal/catalog` — 8 项 updater（embedded activate、checksum reject、invalid YAML reject、diff、Fetch OK、bad SHA、validate fail、version bump hook）
- `internal/indexer/torznab` — 7 项（factory、buildURL、defaults、overrides、search 解析、test、date parser）

### ⏳ Phase 7 — 测试与发布（0/8）
**目标**：后端 ≥ 80% 覆盖核心包；前端组件测试；E2E happy path；Windows 单 exe；用户文档。

待办：
- [ ] 后端覆盖率（`go test -coverprofile` 对 `normalize`、`security`、`search`、`indexer`）
- [ ] 后端集成测试（`tests/integration/` 走 §27.2 场景）
- [ ] 前端单测（`useSearchStream`、`format.ts`、`ResultCard`、`SearchStatus`）
- [ ] 前端 E2E（Playwright：启动 → 添加 mock → 搜索 → 复制 magnet）
- [ ] `scripts/smoke.ps1` 启动二进制 + 完整流程
- [ ] `GET /api/v1/system/diagnostics` 导出 ZIP（无 magnet / 无 keywords）
- [ ] README + 用户手册
- [ ] 验收清单 §28 共 28 项 + CHANGELOG

---

## 代码结构速览

```
ShineSOSO/
├── spec-o1.md                          ← 规格说明
├── progress.md                         ← 本文档
├── README.md
├── go.mod
├── docs/superpowers/
│   └── plans/2026-07-09-easysearch-mvp.md
├── backend/
│   ├── cmd/
│   │   ├── easysearch/main.go          ← 二进制入口
│   │   └── catalog-manifest/main.go    ← manifest 重新生成工具
│   └── internal/
│       ├── config/                     ← Config + env 覆盖
│       ├── logging/                    ← 旋转 slog
│       ├── launcher/                   ← .port 文件 + 浏览器
│       ├── model/                      ← indexer / result / search / catalog
│       ├── security/                   ← URL validator + SSRF
│       ├── normalize/                  ← title / size / date / infohash
│       ├── indexer/                    ← adapter / factory / client
│       │                                declarative / torznab /
│       │                                template / filters / demo / mock
│       ├── search/                     ← orchestrator / event / deduper / ranker
│       ├── catalog/                    ← builtin (embed) / loader / validator
│       │                                updater / examples
│       ├── store/                      ← sqlite + repos
│       ├── api/                        ← router + handlers
│       └── webembed/                   ← go:embed frontend dist
├── frontend/
│   ├── src/
│   │   ├── pages/                      ← SearchPage / IndexerPage
│   │   ├── features/                   ← search/* + indexers/* + ImportDialog
│   │   ├── services/api.ts
│   │   ├── stores/                     ← Zustand
│   │   └── types/
│   └── tests/unit/                     ← Vitest
├── scripts/
│   ├── build.ps1
│   ├── dev.ps1
│   ├── dev.sh
│   ├── phase4-smoke.ps1
│   ├── phase5-smoke.ps1
│   └── phase6-smoke.ps1                ← 新增
└── dist/
    └── easysearch.exe                  ← 单文件可执行（~14.6 MB）
```

---

## 验收清单进度（spec §28）

> Phase 7 完成后才会逐项检查。当前状态：未开始。

| 类别 | 总数 | 通过 |
|---|---|---|
| 安装与启动 | 5 | 0 |
| 索引器管理 | 8 | 0 |
| 搜索体验 | 7 | 0 |
| 性能与稳定性 | 4 | 0 |
| 安全与隐私 | 4 | 0 |

---

## 已知遗留 / 风险

1. **JSON/XML declarative adapter** — Phase 5 留了 `ErrFormatUnsupported`，目前只支持 HTML；JSON/XML 解码器已写入计划但未实现（YAML 引擎已为 declarative 路径铺好路）。
2. **签名槽空着** — `signatures/` 目录已留，但 `cmd/catalog-manifest` 不生成 manifest.sig，spec §26.3 "建议对 manifest 进行数字签名" 仍未启用。
3. **`path` 包错误过滤** — `validator.go` 仍使用相对宽松的 `quickHostSafetyCheck`，运行时安全由 `security.DefaultValidator` 二次把关。
4. **Phase 7 测试覆盖目标 ≥ 80%** — 当前已有大量单测，但未量化覆盖率；正式发布前需跑 `go test -coverprofile` 并补足缺口。

---

## 变更日志摘要

- **a54bdc5** chore(easysearch): phase 6 torznab-and-catalog review gate passed
- **34601bd** feat(easysearch): phase6 source labeling + phase6 smoke test
- **234e435** feat(easysearch): phase6 definition version bump on catalog update
- **378b21d** feat(easysearch): phase6 catalog update api endpoint + status
- **5d5e1cc** feat(easysearch): phase6 catalog updater with checksum verify + atomic swap
- **fe7e8a7** feat(easysearch): phase6 builtin catalog with embedded manifest + sha256
- **7ce3cde** feat(easysearch): phase6 torznab adapter with rss parsing + ssrf test bypass
- **80fed48** chore(easysearch): phase 5 yaml-engine review gate passed
- …（Phase 1-5 共 ~40 commits，按 Conventional Commits `feat(easysearch):` / `chore(easysearch):` / `release(easysearch):` 组织）

---

## 下一步

进入 **Phase 7 — 测试与发布**，共 8 个子任务：
1. 后端覆盖率审计与补测
2. 集成测试套件
3. 前端组件单测
4. Playwright E2E
5. `scripts/smoke.ps1` 全链路
6. `GET /system/diagnostics` 诊断导出
7. README + 用户手册
8. §28 验收清单 + CHANGELOG + `v0.1.0` tag

Phase 7 完成后即可发布 `easysearch v0.1.0`。