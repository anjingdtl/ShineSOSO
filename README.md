# EasySearch

> **目录名 `ShineSOSO` 是历史遗留**：仓库在历史工作空间中以这个代号存放过别的项目，后被 EasySearch 复用。规格文档、计划、进度都使用 `EasySearch` 这个正式名称。

一款运行在本地 Windows 上的轻量级**资源搜索**工具：聚合多个公开索引器，统一展示去重后的搜索结果，一键复制磁力链接。**不抓取、不下载、不做账号登录**。

![status](https://img.shields.io/badge/status-v0.1.0-success)
![version](https://img.shields.io/badge/version-v0.1.0%20(public)%20%2F%200.4.0%20(internal)-blue)
![license](https://img.shields.io/badge/license-MIT-lightgrey)

## ✨ 功能亮点

- 🔍 **多源并发搜索**：N 个已启用公开索引器并发请求，带 SSE 实时进度
- 🧹 **自动去重**：强去重（InfoHash / URL）+ 弱去重（标题 + 大小 ±2%）
- 📊 **统一结果字段**：标题、大小、发布、做种数、来源数统统对齐
- 🗂️ **内置目录 + 一键添加**：内置三个示例公开索引器，三次点击即可开始搜索
- 🛠️ **YAML 自定义**：通过 YAML 定义文件扩展新的公开索引器，**无需修改主程序**
- 🔐 **默认安全**：仅监听回环地址、SSRF 防护、响应大小限制、日志脱敏
- 🖥️ **单文件运行**：编译后是单个 `easysearch.exe`，不需要 Node / Go / Python
- 🌐 **Torznab 支持**：可通过 YAML 接入任意 Torznab 索引器

## 📦 项目状态

| 阶段 | 状态 |
|---|---|
| Phase 1 — 项目骨架 | ✅ |
| Phase 2 — 单索引器搜索 | ✅ |
| Phase 3 — 多索引器聚合 | ✅ |
| Phase 4 — 索引器管理 | ✅ |
| Phase 5 — YAML 引擎 | ✅ |
| Phase 6 — Torznab 与目录更新 | ✅ |
| Phase 7 — 测试与发布 | ✅ |

详细进度见 [`progress.md`](./progress.md)；规格见 [`spec-o1.md`](./spec-o1.md)。

## 🚀 快速开始（Windows 用户）

### 1. 获取可执行文件

从 `dist/easysearch.exe` 拿到单文件二进制（约 14.6 MB）。如果仓库里没有，最简单的方式是在 Windows 上自行构建：

```powershell
git clone https://github.com/anjingdtl/ShineSOSO.git
cd ShineSOSO
.\scripts\build.ps1
```

构建产物：`dist\easysearch.exe`

### 2. 启动

双击 `easysearch.exe`，或在命令行里执行它。程序会：

1. 绑定 `127.0.0.1` 上的随机端口（写入 `data\.port`）
2. 自动打开默认浏览器到 WebUI
3. 创建并初始化 SQLite 数据库（首次运行）

数据目录默认位置：`%APPDATA%\EasySearch\data\`

### 3. 添加第一个索引器

进入 **索引器** 页 → **已添加** 标签 → 点 **+ 添加公开索引器** → 选 `demo-alpha` → 点 **添加**。

完成后到 **搜索** 页输入任意关键词即可。

> 想跳过内置示例？参见 [`docs/USER_GUIDE.md`](./docs/USER_GUIDE.md) 了解 YAML 导入、Torznab 配置和诊断导出。

## 🛠️ 开发

### 仓库布局

```text
ShineSOSO/
├── spec-o1.md           规格说明书（产品 + 技术规格）
├── progress.md          项目进度
├── README.md            本文件
├── docs/
│   ├── USER_GUIDE.md    用户手册
│   └── superpowers/
│       └── plans/
│           └── 2026-07-09-easysearch-mvp.md   实施计划
├── backend/             Go 后端
│   ├── cmd/
│   │   ├── easysearch/           主入口
│   │   └── catalog-manifest/     manifest 重新生成工具
│   └── internal/
│       ├── api/          HTTP 路由 + handlers
│       ├── catalog/      YAML 引擎 + 内置目录
│       ├── config/       配置加载
│       ├── diagnostics/  诊断 ZIP 打包 + 脱敏
│       ├── indexer/      适配器（HTML/Torznab/mock）
│       ├── launcher/     端口文件 + 浏览器
│       ├── logging/      旋转日志
│       ├── model/        数据模型
│       ├── normalize/    字段标准化（标题/大小/日期/infohash）
│       ├── search/       编排器 + 去重 + 排序
│       ├── security/     URL 校验 + SSRF 防护
│       ├── store/        SQLite + repos
│       └── webembed/     前端嵌入
├── frontend/            React + Vite WebUI
└── scripts/             构建 / 冒烟脚本
```

### 后端（Go 1.24+）

```bash
go test ./backend/internal/...                 # 跑单元测试
go test -coverprofile=cov.out ./backend/...    # 覆盖率
go build -o backend/easysearch.exe ./backend/cmd/easysearch
./backend/easysearch.exe --version             # 0.4.0
（对外版本号 = `v0.1.0`；`0.4.0` 是 `go build -ldflags -X main.version` 的内部编译标识。）
```

### 前端（Node 18+）

```bash
cd frontend
npm install
npm run dev          # 开发服务器（端口 3848，代理到 Go）
npm run build        # 生产构建（产物被 Go 嵌入）
npm test             # 单元测试（Vitest）
npm run e2e          # 端到端（Playwright）
```

### 构建脚本

| 脚本 | 用途 |
|---|---|
| `scripts/build.ps1` | Windows 一键构建（前端 + Go → `dist\easysearch.exe`） |
| `scripts/dev.ps1` | 开发模式启动后端（让 `npm run dev` 自动代理） |
| `scripts/dev.sh` | 同上，Bash 版本 |
| `scripts/phase4-smoke.ps1` | Phase 4 冒烟（已纳入 `smoke.ps1` 全链路；保留作历史单步用例） |
| `scripts/phase5-smoke.ps1` | Phase 5 冒烟 |
| `scripts/phase6-smoke.ps1` | Phase 6 冒烟（Torznab + 目录更新） |
| `scripts/smoke.ps1` | **全链路冒烟**（启动 → 添加 → 搜索 → 诊断 → 退出） |

### 环境变量

| 变量 | 默认 | 作用 |
|---|---|---|
| `EASYSEARCH_DATA_DIR` | `%APPDATA%\EasySearch\data` | 数据目录 |
| `EASYSEARCH_CATALOG_URL` | （空） | 远程目录 manifest URL（留空则用内置目录） |
| `EASYSEARCH_CATALOG_PUBKEY` | （空） | 远程目录 manifest Ed25519 公钥（base64, 32 bytes）；留空则跳过签名验证（仅校验 SHA-256） |
| `EASYSEARCH_CATALOG_PRIVKEY` | （空） | `cmd/catalog-manifest --sign` 的私钥（base64, 64 bytes）；仅签名时使用，不在运行时读取 |
| `EASYSEARCH_LOG_LEVEL` | `info` | 日志级别（`debug`/`info`/`warn`/`error`） |

## 🤝 贡献

见 [`spec-o1.md`](./spec-o1.md) §30 了解开发约束；实施细节见 `docs/superpowers/plans/2026-07-09-easysearch-mvp.md`。

## 📝 许可

MIT License。详见 [`LICENSE`](./LICENSE)。

## 🙏 致谢

本项目仅作为公开索引器之上的搜索聚合层，所有数据均来自用户主动配置的第三方公开网站。EasySearch 不收录、不缓存、不分发任何资源内容，也不绕过任何访问控制。
