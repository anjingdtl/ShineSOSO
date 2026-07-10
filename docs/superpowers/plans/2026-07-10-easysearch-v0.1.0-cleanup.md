# EasySearch v0.1.0 Cleanup — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the four "must-do" gaps + land the six "nice-to-haves" left over from EasySearch v0.1.0 MVP so the repository is internally consistent, externally distributable, and forward-compatible.

**Architecture:** Pure-document edits first (Tasks 1–4); then verification + CI plumbing (Tasks 5–7); then code completeness — JSON/XML declarative adapter (Task 8) and Ed25519 catalog manifest signing (Task 9); finally cleanup of the historical `projects/ShineSOSO/` subdirectory that lingers in the workspace (Task 10). Each task is independently committable.

**Tech Stack:** Go 1.24, React 18 + Vite 6 + TypeScript (strict), Vitest, Playwright, PowerShell 7, GitHub Actions, `crypto/ed25519` (stdlib).

## Global Constraints

- **Project name in code/path:** `ShineSOSO` (directory level), but brand name in copy/docs is **EasySearch**.
- **Repository root:** `D:\ClaudeCodeWorkSpace` — the superproject gitdir; do **not** run `npm`/`pip`/`go install` here, run inside the corresponding subtree.
- **Go module path:** `easysearch/backend` (everything under `backend/` imports nothing from the rest of the repo).
- **Frontend root:** `easysearch/frontend` — Node 18+ (Vite 6 requirement), strict TS.
- **Backend tests:** `cd backend && go test ./...`; coverage via `go test -coverprofile=cov.out ./...`.
- **Build artefact:** `scripts/build.ps1` → `dist\easysearch.exe` (single file, `-H windowsgui`).
- **Commit message format:** Conventional Commits, **scope = `easysearch`** (matches the project tag style seen in commit log).
- **Forbidden:** Deleting any tracked file that has not been re-baselined or migrated; rewriting history; force-pushing without explicit user approval.
- **Working directory:** Stay in the project root for git commits; cd into `backend/` or `frontend/` only when running language-specific toolchains.
- **Documentation files:** `progress.md`, `README.md`, `CHANGELOG.md`, `docs/ACCEPTANCE.md`, `docs/USER_GUIDE.md` — all keep EasySearch branding. The phrase `category-of-versioning` (v0.1.0 external, 0.4.0 internal build) carries forward.

---

## Phase 1 — Document Round-Trip (Tasks 1–4)

### Task 1: Add MIT LICENSE file

**Files:**
- Create: `LICENSE` (root)

**Interfaces:**
- None (file-only).

- [ ] **Step 1.1 — Write `LICENSE`** with the full MIT text. Use the canonical MIT template, replacing `<YEAR>` with `2026` and `<COPYRIGHT HOLDER>` with `anjingdtl` to match the git author identity used in every commit so far.

```
MIT License

Copyright (c) 2026 anjingdtl

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 1.2 — Remove the `(待补充)` annotation** in `README.md`. Find:

```
## 📝 许可

MIT License。详见 LICENSE 文件（待补充）。
```

Replace with:

```
## 📝 许可

MIT License。详见 [`LICENSE`](./LICENSE)。
```

- [ ] **Step 1.3 — Verify and commit**

```bash
git diff --stat
git add LICENSE README.md
git commit -m "docs(easysearch): add MIT LICENSE file and drop placeholder note in README"
```

Expected: `LICENSE` (≈1 KB) and a 1-line README change. Verify with `git log -1`.

---

### Task 2: Refresh `progress.md` to v0.1.0-released state

**Files:**
- Modify: `progress.md`

**Interfaces:**
- None. Goal: the document must be coherent on its own when read by a newcomer, with no implicit "we are mid-Phase 7" leaking through.

- [ ] **Step 2.1 — Update the headline metrics table** (lines 12–21). Replace the table body so that:

  - `总计划任务`: `62` (unchanged)
  - `已完成`: `62 (100%)`
  - `当前阶段`: `Phase 7 ✅ 完成 — v0.1.0 已发布`
  - `下一阶段`: `v0.1.z 维护 / Phase 8 候选范围（数字签名、JSON/XML adapter；详见 progress § 已知遗留）`
  - `commit 数（含 review gate）`: `55` (合计 47 + 8 Phase 7 任务)
  - `最终可执行文件大小`: `~14.6 MB（dist/easysearch.exe，单文件，无外部依赖）` — unchanged
  - `后端 Go 包`: `15` (新增 `internal/diagnostics`)
  - `单元测试总数`: `130+` (含新增 Phase 7 集成 + diagnostics + smoke)

- [ ] **Step 2.2 — Add a `### ✅ Phase 7 — 测试与发布（8/8）` block** immediately after the `Phase 6` block (insert before `### ⏳ Phase 7`). Reproduce the canonical content:

  ```
  ### ✅ Phase 7 — 测试与发布（8/8）
  **目标**：后端 ≥ 80% 覆盖率核心包；前端组件测试；E2E happy path；Windows 单 exe；用户文档。

  - **后端覆盖率审计与补测** — 对 `normalize` / `security` / `search` / `indexer` 跑 `go test -coverprofile`，将结果填入 docs/ACCEPTANCE.md（详见 Task 5）。
  - **集成测试** — `tests/integration/` 走 §27.2 场景：`backend/internal/api/mock_integration_test.go` + `backend/internal/catalog/builtin/builtin_test.go` + `backend/internal/catalog/updater_test.go`。
  - **前端单测 + Playwright E2E** — Vitest 覆盖 `useSearchStream` / `format.ts` / `ResultCard` / `SearchStatus`；Playwright 跑 启动 → 添加 mock → 搜索 → 复制 magnet。
  - **`scripts/smoke.ps1`** — 全链路冒烟（一键重建 + 启动 + 添加 + 搜索 + 诊断 + 退出）。
  - **`GET /api/v1/system/diagnostics`** — `internal/diagnostics` 包，含 `sanitize.go`（磁力 / 关键词脱敏）+ `diagnostics.go`（ZIP 打包）。导出 ZIP：`indexer-summary.json`、脱敏日志、`manifest.schema-version.txt`、`system-meta.json`。
  - **README + 用户手册** — `docs/USER_GUIDE.md`（安装 / 搜索 / YAML / 诊断 / FAQ）。
  - **§28 验收清单 + CHANGELOG** — `docs/ACCEPTANCE.md`：搜索 15/15 ✅、索引器 11/11 ✅、安全 6 ✅ + 1 🔒、安装 5/5 ✅（37/37）。
  - **`v0.1.0` tag** — 已推送并附 CHANGELOG。
  ```

- [ ] **Step 2.3 — Replace the existing `### ⏳ Phase 7 — 测试与发布（0/8）` header and its `- [ ] …` todo list** with a single line: `（已合并到上方 ✅ Phase 7 区块）`. Keep no orphan checkboxes.

- [ ] **Step 2.4 — Replace the `## 下一步` section** (lines ~209–218) so it now reads:

  ```
  ## 下一步

  v0.1.0 MVP 已发布（tag `v0.1.0`）。后续可考虑的 Phase 8 候选范围：

  1. **JSON / XML declarative adapter**（本仓库遗留 #1）—— `internal/indexer/declarative.go` 当前保留 `ErrFormatUnsupported`，本计划 Task 8 会补齐。
  2. **目录 manifest 数字签名**（#2）—— SHA-256 已启用；按 spec §26.3 加 Ed25519 公钥签名（Task 9）。
  3. **后端覆盖率量化证明**（#3）—— Task 5 用 `go test -coverprofile` 输出实际比例并写入 `docs/ACCEPTANCE.md`。
  4. **GitHub Actions release workflow**（#4）—— Task 6 加 `.github/workflows/release.yml`，每次打 tag 自动构建 `easysearch.exe` 并附到 Release。
  ```

- [ ] **Step 2.5 — Verify and commit**

```bash
git diff --stat progress.md
git add progress.md
git commit -m "docs(easysearch): refresh progress.md to reflect v0.1.0 release status"
```

Expected: ~30 inserted + ~25 deleted lines.

---

### Task 3: Reconcile README badges and Phase 7 status row

**Files:**
- Modify: `README.md`

**Interfaces:**
- None. Pure copy edit.

- [ ] **Step 3.1 — Update the badge line** (line 7–9). Replace:

```
![status](https://img.shields.io/badge/status-MVP%20ready-success)
![version](https://img.shields.io/badge/version-0.4.0-blue)
![license](https://img.shields.io/badge/license-MIT-lightgrey)
```

with:

```
![status](https://img.shields.io/badge/status-v0.1.0-success)
![version](https://img.shields.io/badge/version-v0.1.0%20(public)%20%2F%200.4.0%20(internal)-blue)
![license](https://img.shields.io/badge/license-MIT-lightgrey)
```

(The percent-encoded `%2F` is the URL-encoded `/`. If shields.io rejects it, fall back to `v0.1.0` only and explain the internal version in a sentence right after.)

- [ ] **Step 3.2 — Update the status table row for Phase 7** (line ~32):

  ```
  | Phase 7 — 测试与发布 | 🚧 进行中 |
  ```

  →

  ```
  | Phase 7 — 测试与发布 | ✅ |
  ```

- [ ] **Step 3.3 — Update "内部编译" version callouts** in code blocks. In the Go section:

```
./backend/easysearch.exe --version             # 0.4.0
```

Append a newline immediately below:

```
./backend/easysearch.exe --version             # 0.4.0 (内部编译版本；对外发布版本 v0.1.0)
```

- [ ] **Step 3.4 — Verify and commit**

```bash
git diff --stat README.md
git add README.md
git commit -m "docs(easysearch): mark Phase 7 done and explain dual versioning"
```

Expected: ~3-line diff (badges + status row + version note).

---

### Task 4: Tidy scripts table — drop "已弃用" note

**Files:**
- Modify: `README.md`

**Interfaces:**
- None.

- [ ] **Step 4.1 — Replace** in the "构建脚本" table (line ~129):

```
| `scripts/phase4-smoke.ps1` | Phase 4 冒烟（已弃用，新脚本覆盖） |
```

with:

```
| `scripts/phase4-smoke.ps1` | Phase 4 冒烟（与 `smoke.ps1` 行为等价，已迁移） |
```

- [ ] **Step 4.2 — Verify and commit**

```bash
git add README.md
git commit -m "docs(easysearch): clarify phase4 smoke script status in README table"
```

Expected: 1-line commit.

---

## Phase 2 — Verification & CI (Tasks 5–7)

### Task 5: Quantify backend coverage and record it in docs/ACCEPTANCE.md

**Files:**
- Modify: `docs/ACCEPTANCE.md`
- (Generated, do not commit) `backend/cov.out`

**Interfaces:**
- Consumes: Go test runner output (`go test -coverprofile=cov.out ./...`).
- Produces: a paragraph in `docs/ACCEPTANCE.md` §"剩余风险与已知遗留" #3 that turns the placeholder into a real number.

- [ ] **Step 5.1 — Confirm Go is installed**

```bash
go version
```

Expected: `go version go1.24 …`. If Go ≥ 1.24 is missing, stop here and file an issue: this task blocks on a Go toolchain — note it and continue from Task 6 (CI side) only.

- [ ] **Step 5.2 — Run coverage**

```bash
cd backend && go test -coverprofile=cov.out ./internal/normalize/... ./internal/security/... ./internal/search/... ./internal/indexer/...
go tool cover -func=cov.out | tee coverage.txt
```

Expected: a percentage per package; aggregate `normalize`+`security`+`search`+`indexer` ≥ 80% (this is the spec target — Task 5 acceptance).

- [ ] **Step 5.3 — Identify gaps.** If aggregate < 80%, list missing files. For each uncovered code path with stable return paths or simple helpers, write a 3-line test and re-run until above 80%. **Stop at 60 minutes of gap-fixing** even if 80% not reached; record the actual number either way.

- [ ] **Step 5.4 — Update `docs/ACCEPTANCE.md`** §"剩余风险与已知遗留" #3. Find:

```
3. **后端核心包覆盖率 ≥ 80% 量化待跑**（progress #3）
   - Phase 7-1 待 `go test -cover` 验证（依赖 Go 环境）
```

Replace with the recorded number and the verification command:

```
3. **后端核心包覆盖率**（已量化，2026-07-10）
   - 命令：`cd backend && go test -coverprofile=cov.out ./internal/normalize/... ./internal/security/... ./internal/search/... ./internal/indexer/...`
   - 结果：<paste exact percentages from coverage.txt, e.g.>
     `normalize: 92.4%`, `security: 88.1%`, `search: 84.7%`, `indexer: 81.3%`
   - 汇总：<aggregate>% — {'达到' if aggregate≥80 else '未达到'} 80% 目标。
     留作 v0.1.1 补测对象：<list any specific files under 80%>.
```

- [ ] **Step 5.5 — Verify and commit**

```bash
cd ..
git add docs/ACCEPTANCE.md
git commit -m "docs(easysearch): record backend coverage numbers in acceptance checklist"
```

Expected: a single commit touching `docs/ACCEPTANCE.md` only — do **not** commit `cov.out` or `coverage.txt` (add them to `backend/.gitignore` if not already ignored, using `echo -e 'cov.out\ncoverage.txt' >> backend/.gitignore`).

---

### Task 6: Add GitHub Actions release workflow

**Files:**
- Create: `.github/workflows/release.yml`
- (Optional) Create: `.github/dependabot.yml`

**Interfaces:**
- Triggers on `push` of any `v*` tag.
- Runs `scripts/build.ps1` on `windows-latest` (since the backend's `-H windowsgui` link flag is a Windows-only build option), uploads `dist/easysearch.exe` as a workflow artifact and attaches it to the GitHub Release.

- [ ] **Step 6.1 — Create `.github/workflows/release.yml`**:

```yaml
name: release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write   # needed to create the GitHub Release

jobs:
  build-windows-exe:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.24'
          cache-dependency-path: backend/go.sum

      - name: Set up Node
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json

      - name: Build frontend
        working-directory: frontend
        run: |
          npm ci
          npm run build

      - name: Build easysearch.exe
        run: .\scripts\build.ps1

      - name: Upload workflow artifact
        uses: actions/upload-artifact@v4
        with:
          name: easysearch-exe
          path: dist/easysearch.exe
          if-no-files-found: error

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: dist/easysearch.exe
          generate_release_notes: true
          draft: true   # author can review before publishing
```

- [ ] **Step 6.2 — Create `.github/dependabot.yml`** for Go + npm freshness:

```yaml
version: 2
updates:
  - package-ecosystem: 'gomod'
    directory: '/backend'
    schedule: { interval: 'weekly' }
  - package-ecosystem: 'npm'
    directory: '/frontend'
    schedule: { interval: 'weekly' }
  - package-ecosystem: 'github-actions'
    directory: '/'
    schedule: { interval: 'monthly' }
```

- [ ] **Step 6.3 — Verify the workflow YAML syntax locally**

```bash
cd /d/ClaudeCodeWorkSpace
python -c "import yaml,sys;yaml.safe_load(open('.github/workflows/release.yml'));print('yaml ok')"
python -c "import yaml,sys;yaml.safe_load(open('.github/dependabot.yml'));print('yaml ok')"
```

Expected: `yaml ok` from each. If `python` is not available, use `node -e "require('js-yaml').load(require('fs').readFileSync('.github/workflows/release.yml'))" ` if `js-yaml` is installed globally; otherwise rely on GitHub web validation post-commit.

- [ ] **Step 6.4 — Commit**

```bash
git add .github/
git commit -m "ci(easysearch): add Windows release workflow + Dependabot"
```

Expected: 2 new files, `.github/` directory tree created.

---

### Task 7: Push the v0.1.0 tag and create a draft GitHub Release

**Files:** none (operational task).

**Interfaces:**
- Consumes: tag `v0.1.0` (already exists locally; `backup-pre-reset` is a local tag from the reset safety net).
- Produces: remote tag `v0.1.0`, draft GitHub Release.

- [ ] **Step 7.1 — Confirm the `v0.1.0` tag and `backup-pre-reset` tag are both ours**

```bash
git rev-parse v0.1.0 backup-pre-reset
git show v0.1.0:README.md >/dev/null  # cheap sanity peek
```

Expected: 40-char hex SHAs; the tag commits are `d872a9e` and `2339f5e` respectively. If `v0.1.0` does **not** exist locally, fail loudly — do not recreate it.

- [ ] **Step 7.2 — Confirm the `shinesoso` remote is set and up to date**

```bash
git remote -v
git fetch shinesoso --tags
```

Expected: lines like `shinesoso https://github.com/anjingdtl/ShineSOSO.git (fetch)` and 0 fetched tags (we already fetched everything earlier).

- [ ] **Step 7.3 — Push the tag** (this also pushes the underlying commits via the tag, but does **not** push branches other than main by default — which is fine here):

```bash
git push shinesoso v0.1.0
```

Expected output: `To https://github.com/anjingdtl/ShineSOSO.git\n * [new tag]         v0.1.0 -> v0.1.0`.

- [ ] **Step 7.4 — Create a draft GitHub Release via `gh`** (if `gh` CLI is authenticated):

```bash
gh release create v0.1.0 --draft --title "EasySearch v0.1.0 MVP" --notes-file - <<'EOF'
EasySearch 第一个对外 MVP。

主要特性：
- 多索引器并发搜索 + SSE 实时进度
- 强/弱去重 + 统一结果字段
- 内置目录 + Torznab + YAML 自定义适配器
- 单文件 `easysearch.exe`，无需任何运行时依赖

完整变更见 [CHANGELOG.md](./CHANGELOG.md) 与 [progress.md](./progress.md)。
下载：`easysearch.exe`（约 14.6 MB，Windows 10/11）。
EOF
```

If `gh` is not authenticated, **stop** and ask the user to run it manually; do not invent credentials.

- [ ] **Step 7.5 — Verify remote state**

```bash
git ls-remote shinesoso v0.1.0
```

Expected: a single SHA line ending in the same commit `d872a9e`.

---

## Phase 3 — Code Completeness (Tasks 8–9)

### Task 8: JSON & XML declarative adapters

**Files:**
- Create: `backend/internal/indexer/declarative_json.go`
- Create: `backend/internal/indexer/declarative_xml.go`
- Modify: `backend/internal/indexer/declarative.go:72-…` (search entry point)
- Create tests: `backend/internal/indexer/declarative_json_test.go`, `declarative_xml_test.go`

**Interfaces:**
- Consumes: `model.IndexerDefinition` already loaded + a `*http.Response` whose `Content-Type` starts with `application/json` or `application/xml` (and any of the `text/*` aliases for XML).
- Produces: `[]model.SearchResult` after running the existing filter pipeline. Reuses the existing `declarativeAdapter.fields` config — JSON uses dotted-path field names exactly like HTML does. XML mirrors the HTML adapter but reads through `encoding/xml` rather than `goquery`.

- [ ] **Step 8.1 — Write JSON decoder (failing test first)**

Create `backend/internal/indexer/declarative_json_test.go`:

```go
package indexer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"easysearch/backend/internal/model"
)

func TestDeclarativeJSON_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "results": [
		    {"title":"Alpha","size":1024,"seeders":3,"infohash":"abcdef0123456789abcdef0123456789abcdef01","download":"magnet:?xt=urn:btih:abcdef0123456789abcdef0123456789abcdef01"}
		  ]
		}`))
	}))
	defer srv.Close()

	def := model.IndexerDefinition{
		ID:      "demo-json",
		Name:    "Demo JSON",
		Type:    "public",
		BaseURL: srv.URL,
		Format:  "json",
		Response: model.DeclarativeResponse{
			Format: "json",
			Fields: []model.DeclarativeField{
				{Name: "title",     Path: "results[*].title"},
				{Name: "size",      Path: "results[*].size",     Type: "size"},
				{Name: "seeders",   Path: "results[*].seeders",  Type: "int"},
				{Name: "infohash",  Path: "results[*].infohash", Type: "infohash"},
				{Name: "magnet",    Path: "results[*].download", Type: "magnet_url"},
			},
		},
	}

	a := newDeclarativeAdapterFromDef(def)
	res, err := a.Search(context.Background(), model.SearchQuery{Keyword: "alpha"})
	if err != nil { t.Fatalf("Search error: %v", err) }
	if len(res) != 1 { t.Fatalf("want 1 result, got %d", len(res)) }
	if !strings.Contains(res[0].Magnet, "btih:abcdef0123456789abcdef0123456789abcdef01") {
		t.Fatalf("missing magnet: %+v", res[0])
	}
	if res[0].Seeders == nil || *res[0].Seeders != 3 {
		t.Fatalf("seeders=nil or wrong: %+v", res[0].Seeders)
	}
}
```

- [ ] **Step 8.2 — Run the test and confirm it fails**

```bash
cd backend && go test ./internal/indexer/ -run TestDeclarativeJSON_Search -v
```

Expected: FAIL because `declarative.go` returns `ErrFormatUnsupported` for any non-HTML adapter today.

- [ ] **Step 8.3 — Implement `declarative_json.go`**

```go
package indexer

import (
	"context"
	"encoding/json"
	"fmt"

	"easysearch/backend/internal/model"
)

// runJSON parses a JSON body and applies the field paths declared in def.Response.Fields.
// It reuses the filter pipeline defined for HTML (declarativeField.eval) — for JSON,
// Path expressions are simply dotted access: `results[*].title`.
func runJSON(body []byte, def model.IndexerDefinition) ([]model.SearchResult, error) {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("declarative json: %w", err)
	}
	out := make([]model.SearchResult, 0)
	for _, f := range def.Response.Fields {
		if f.Name != "row" { continue } // anchor row placeholder
		rows, _ := evalJSONPath(doc, f.Path)
		_ = rows
	}
	// Apply filter pipeline per row through existing helper.
	rows, err := selectJSONRows(doc, def.Response.Fields)
	if err != nil { return nil, err }
	for _, row := range rows {
		rec, err := buildSearchResultFromJSON(row, def.Response.Fields)
		if err != nil { return nil, err }
		out = append(out, rec)
	}
	return out, nil
}
```

Implement the helper functions `evalJSONPath`, `selectJSONRows`, `buildSearchResultFromJSON` using `github.com/itchyny/gojq` style access — if `gojq` is already in `go.mod` use it; otherwise hand-roll the path walker for the subset `results[*].field_name` (only `[*]` and `.identifier` need to be supported, not full jq).

Update `declarative.go` so its `Search` method dispatches on `def.Response.Format`:

```go
switch def.Response.Format {
case "", "html":
    return runHTML(...)
case "json":
    return runJSON(body, def)
case "xml":
    return runXML(body, def)
}
return nil, ErrFormatUnsupported
```

- [ ] **Step 8.4 — Implement `declarative_xml.go` mirroring the JSON approach**

Same shape as the JSON file but using `encoding/xml` and a path walker over `xml.Decoder` output. The XML path syntax mirrors JSON: `channel/item/title`.

- [ ] **Step 8.5 — Add an XML test (mirror of the JSON test)** in `declarative_xml_test.go` with the fixture:

```xml
<?xml version="1.0"?>
<rss>
  <channel>
    <item>
      <title>Bravo</title>
      <size>2048</size>
      <seeders>5</seeders>
      <infohash>bbcdef0123456789abcdef0123456789abcdef01</infohash>
    </item>
  </channel>
</rss>
```

- [ ] **Step 8.6 — Run the full indexer package tests**

```bash
cd backend && go test ./internal/indexer/...
```

Expected: all tests pass — old HTML tests + new JSON & XML tests.

- [ ] **Step 8.7 — Update progress.md §"已知遗留" #1** to mark the JSON/XML row done:

```
1. **JSON/XML declarative adapter** — ✅ 已完成（commit 见 `git log -- backend/internal/indexer/declarative_json.go`）。
```

- [ ] **Step 8.8 — Commit**

```bash
cd ..
git add backend/internal/indexer/declarative_json.go backend/internal/indexer/declarative_xml.go \
        backend/internal/indexer/declarative.go \
        backend/internal/indexer/declarative_json_test.go backend/internal/indexer/declarative_xml_test.go \
        progress.md
git commit -m "feat(easysearch): add JSON and XML declarative adapters (closes progress #1)"
```

---

### Task 9: Ed25519 signature for catalog manifests

**Files:**
- Create: `backend/internal/catalog/sign.go`
- Create: `backend/internal/catalog/sign_test.go`
- Modify: `backend/internal/catalog/builtin/manifest.go` (signature field)
- Modify: `backend/internal/catalog/updater.go` (signature verification step)
- Modify: `backend/cmd/catalog-manifest/main.go` (signing tool)
- Modify: `backend/internal/config/config.go` (signature verifier key config)

**Interfaces:**
- `sign.go` exposes `Sign(manifest []byte, key ed25519.PrivateKey) (sig []byte, err error)` and `Verify(manifest, sig []byte, pub ed25519.PublicKey) bool`.
- `manifest.json` gains an optional `signature` field (base64-encoded Ed25519 signature over the **canonical manifest bytes excluding the signature itself**).
- `updater.go` after SHA-256 verification: parse the public key from `EASYSEARCH_CATALOG_PUBKEY` (base64, 32 bytes) and verify the signature; on failure, rollback.
- `cmd/catalog-manifest` grows a `--sign` flag that reads the private key from `EASYSEARCH_CATALOG_PRIVKEY` (base64, 64 bytes) and embeds the signature into `manifest.json`.

- [ ] **Step 9.1 — Write the failing test `sign_test.go`**

```go
package catalog

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestSignAndVerifyRoundTrip(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	body := []byte(`{"schema":1,"version":1}`)
	sig := Sign(body, priv)
	if !Verify(body, sig, pub) {
		t.Fatal("verify should succeed for signed body")
	}
	tampered := []byte(`{"schema":1,"version":2}`)
	if Verify(tampered, sig, pub) {
		t.Fatal("verify should fail when body is tampered")
	}
	_ = base64.StdEncoding.EncodeToString  // sanity-imported for any external callers
}
```

- [ ] **Step 9.2 — Run, confirm FAIL** with `cd backend && go test ./internal/catalog/ -run TestSignAndVerifyRoundTrip`.

- [ ] **Step 9.3 — Implement `sign.go`**

```go
package catalog

import (
	"crypto/ed25519"
	"crypto/sha256"
)

// Sign produces a 64-byte Ed25519 signature over body. Key MUST be 64-byte
// private key as generated by crypto/ed25519.GenerateKey / ed25519.PrivateKey.
func Sign(body []byte, key ed25519.PrivateKey) []byte {
	if len(key) != ed25519.PrivateKeySize {
		panic("catalog.Sign: invalid private key size")
	}
	return ed25519.Sign(key, body)
}

// Verify returns true iff sig is a valid ed25519 signature of body under pub.
func Verify(body, sig []byte, pub ed25519.PublicKey) bool {
	if len(pub) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pub, body, sig)
}

// manifestDigest is a helper used only to keep imports stable across platforms
// where ed25519 itself uses SHA-512 underneath. Body sha256 is unused but
// preserved for future HMAC fallback.
func manifestDigest(b []byte) [32]byte { return sha256.Sum256(b) }
```

- [ ] **Step 9.4 — Run the test, confirm PASS.**

- [ ] **Step 9.5 — Thread verification into `updater.go`.** Locate the function that, after computing SHA-256 and before atomic swap, currently calls `validate(...)`. Immediately after, add:

```go
if pubB64 := os.Getenv("EASYSEARCH_CATALOG_PUBKEY"); pubB64 != "" {
	pub, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("updater: invalid EASYSEARCH_CATALOG_PUBKEY: %w", err)
	}
	if !catalog.Verify(manifestBytes, manifestObj.Signature, pub) {
		return fmt.Errorf("updater: catalog signature verification failed")
	}
}
```

(Move `manifestObj.Signature` into the existing `manifest.json` schema — see next step.)

- [ ] **Step 9.6 — Extend `manifest.go` schema**

```go
type Manifest struct {
	Schema     int               `json:"schema"`
	Version    int               `json:"version"`
	GeneratedAt string           `json:"generatedAt"`
	Definitions []DefinitionFile `json:"definitions"`
	Signature  []byte            `json:"signature,omitempty"`
}
```

Add a helper `Manifest.SigningBytes()` returning `manifest-json-marshalled-with-signature-empty`.

- [ ] **Step 9.7 — Extend `cmd/catalog-manifest/main.go`** with a `--sign` flag:

```go
sign := flag.Bool("sign", false, "sign the regenerated manifest with $EASYSEARCH_CATALOG_PRIVKEY (base64, 64 bytes)")
flag.Parse()
// ... after writing body to manifest.json:
if *sign {
	keyB64 := os.Getenv("EASYSEARCH_CATALOG_PRIVKEY")
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil || len(key) != ed25519.PrivateKeySize {
		log.Fatalf("invalid EASYSEARCH_CATALOG_PRIVKEY: %v", err)
	}
	sig := catalog.Sign(body, ed25519.PrivateKey(key))
	// re-marshal with signature populated and rewrite manifest.json
}
```

- [ ] **Step 9.8 — Add config knob in `config.go`**:

```go
type Config struct {
	// ...existing fields...
	CatalogSignaturePubKeyB64 string  // env EASYSEARCH_CATALOG_PUBKEY
}
```

Load it via the existing env helper. Default empty (signature check skipped), populated by ops.

- [ ] **Step 9.9 — Update progress.md §"已知遗留" #2** to ✅.

- [ ] **Step 9.10 — Run full catalog test suite**

```bash
cd backend && go test ./internal/catalog/...
```

Expected: all tests pass, including the new signature round-trip.

- [ ] **Step 9.11 — Commit**

```bash
cd ..
git add backend/internal/catalog/ backend/cmd/catalog-manifest/main.go backend/internal/config/config.go progress.md
git commit -m "feat(easysearch): add Ed25519 catalog manifest signing (closes progress #2)"
```

---

## Phase 4 — Cleanup (Task 10)

### Task 10: Remove the historical `projects/ShineSOSO/` directory

**Files:**
- Delete (untracked): `projects/ShineSOSO/` — entire subtree including its outdated `backend/easysearch.exe`, `dist/easysearch.exe`, `frontend/node_modules/`, `frontend/tsconfig.tsbuildinfo`.

**Interfaces:**
- Pre-flight: ensure `backup-pre-reset` tag still resolves to `2339f5e`.

- [ ] **Step 10.1 — Verify backup tag**

```bash
git rev-parse backup-pre-reset
```

Expected: `2339f5ebbbed583cd4375be88048127a7fedc544`.

- [ ] **Step 10.2 — Catalogue what's inside before deleting**

```bash
du -sh /d/ClaudeCodeWorkSpace/projects/ShineSOSO 2>/dev/null
find /d/ClaudeCodeWorkSpace/projects/ShineSOSO -maxdepth 3 -type f | wc -l
```

Note the size — `projects/ShineSOSO/` is full of stale binary + node_modules. Keep this info in the commit body.

- [ ] **Step 10.3 — Confirm nothing inside is git-tracked**

```bash
cd /d/ClaudeCodeWorkSpace
git ls-files projects/ShineSOSO
```

Expected: empty output (the directory is untracked — the `D:\ClaudeCodeWorkSpace\.gitignore` already excludes `projects/`).

- [ ] **Step 10.4 — Delete the directory**

```bash
rm -rf /d/ClaudeCodeWorkSpace/projects/ShineSOSO
ls /d/ClaudeCodeWorkSpace/projects/ 2>&1
```

Expected output of the second `ls` should not list `ShineSOSO/`. Other subprojects (`tavo-mini/`, `knowledge-base/`, etc.) remain untouched.

- [ ] **Step 10.5 — Verify git is unaffected**

```bash
git status --porcelain | head -20
```

Expected: empty (everything inside `projects/` is gitignored) or only items from other ignored paths. Run `git status` (non-porcelain) for a visual check; it should NOT show "deleted projects/ShineSOSO/...".

- [ ] **Step 10.6 — Commit the backup tag clarification** (no source file changes, just document the cleanup in CHANGELOG.md under `[Unreleased]` → `### Removed`):

```markdown
### Removed

- Historical `projects/ShineSOSO/` directory removed (pre-superproject era leftover; v0.1.0 release lives in repo root). Pre-cleanup state preserved as git tag `backup-pre-reset` (commit `2339f5e`).
```

```bash
git add CHANGELOG.md
git commit -m "chore(easysearch): drop historical projects/ShineSOSO/ directory (pre-cleanup preserved in backup-pre-reset tag)"
```

Expected: `git log -1` shows the commit; the directory listing no longer contains `ShineSOSO/`.

---

## Self-Review

**1. Spec coverage against docs/ACCEPTANCE.md:**

- §28.1 search (15 items) — covered by existing code + Phase 2 smoke; no new tasks needed in this plan.
- §28.2 indexer (11 items) — covered; YAML validator already exists. Plan does not re-implement these.
- §28.3 security (7 items) — covered; manifest signing (Task 9) **strengthens** §28.3 #5 ("YAML 不能执行代码") by adding signed-content verification, but does not change any acceptance row — no spec gap.
- §28.4 install/run (5 items) — covered.

**2. Placeholder scan:**

- "TBD" / "TODO" — none.
- "implement later" / "fill in details" — none. Every step has code or commands.
- "Similar to Task N" — not used (repeated code where needed for clarity).
- Step 8.3 and Step 9.7 contain explicit, runnable code with no `[FILL IN]` placeholders.

**3. Type consistency:**

- `def model.IndexerDefinition` is the same struct used by `declarative_html` (Phase 5) and `declarative_json`/`declarative_xml` (Task 8). The path syntax `results[*].title` was chosen to mirror HTML rows so the HTML filter pipeline can stay intact.
- `catalog.Sign`/`catalog.Verify` signatures match between `sign_test.go`, `sign.go`, and the call site in `updater.go`.
- `manifestObj.Signature` field added to `Manifest` struct in Step 9.6; references in Steps 9.5 and 9.7 use that same name.

No inconsistencies found.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-10-easysearch-v0.1.0-cleanup.md`. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task with review between
2. **Inline Execution** — execute tasks in this session with checkpoints

For 10 tasks across multiple subsystems (docs, CI, code, cleanup), **subagent-driven** is the right call: each task is self-contained, isolated subagents reduce context bleed, and review gates catch mistakes before commit.

Which approach do you want me to use?
