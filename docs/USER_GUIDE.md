# EasySearch 用户手册

> 适用版本：v0.4.0 及以上
> 阅读对象：最终用户、贡献者

## 📚 目录

1. [安装与首次启动](#1-安装与首次启动)
2. [添加索引器](#2-添加索引器)
3. [搜索资源](#3-搜索资源)
4. [导入自定义索引器定义（YAML）](#4-导入自定义索引器定义yaml)
5. [更新内置索引器目录](#5-更新内置索引器目录)
6. [诊断与故障排查](#6-诊断与故障排查)
7. [常见问题 FAQ](#7-常见问题-faq)
8. [隐私与数据](#8-隐私与数据)

---

## 1. 安装与首次启动

### 1.1 系统要求

- Windows 10 / 11（64-bit）
- 至少 200 MB 可用磁盘空间（含日志和数据库）
- 可选：现代浏览器（Chrome / Edge / Firefox）—— 启动后会自动打开

### 1.2 启动步骤

1. 双击项目根目录的 `启动 EasySearch.vbs`（推荐；后台启动，不显示终端窗口）
2. 浏览器自动打开 `http://127.0.0.1:<随机端口>`
3. 看到搜索页 = 启动成功

如果只拿到了发布产物，也可以直接双击 `easysearch.exe`。

**首次启动会自动创建**：

- `%APPDATA%\EasySearch\data\easysearch.db` — SQLite 数据库
- `%APPDATA%\EasySearch\data\logs\` — 日志目录（最多 5 个 10MB 文件，30 天滚动）
- `%APPDATA%\EasySearch\data\catalog-cache\` — 远程目录缓存

### 1.3 不打开浏览器启动

```powershell
.\easysearch.exe --no-browser
```

启动后会输出 URL，手动复制到浏览器即可。常用于 SSH / 远程会话场景。

### 1.4 关闭程序

- 直接关闭浏览器窗口**不会**关闭 EasySearch
- 在原控制台窗口按 `Ctrl+C` 干净退出
- 或在任务管理器结束 `easysearch.exe` 进程（不推荐，会留下 `.port` 文件）

---

## 2. 添加索引器

### 2.1 发现中心一键添加（推荐）

1. 进入 **索引器** 页面。
2. 在 **发现中心** 输入名称、语言或协议搜索内置目录。
3. 点击 **一键添加**；软件会在本机测试，通过后自动启用。

发现中心完全离线可用：目录随软件一起安装，不读取 Prowlarr、不要求 API Key，也不依赖云端服务。目录更新是可选能力，不影响本机已有索引器继续搜索。

当前内置的可用公共源为 **Internet Archive（公开数字馆藏）**，可搜索公开书籍、音频、视频和软件资源；结果可直接打开其详情页。

### 2.2 联网发现候选索引器

在 **联网发现索引器** 中输入关键词后，程序会向公开搜索服务查询候选网页。它不会直接把网页加入搜索：只有候选 URL 通过 Torznab 能力探测和连接测试后，才会保存为索引器。

> 搜索关键词会发送给公开搜索服务；不希望联网发现时，可继续只使用本地发现中心。

### 2.2 手动添加内置索引器

1. 进入 **索引器** 页面
2. 在 **内置目录** 标签里找到目标索引器
3. 点击 **添加**，程序会自动测试可用性

测试通过 → 索引器变为 **已启用**；测试失败 → 变为 **已停用**，但仍可手动启用重试。

### 2.2 内置的三个示例

| ID | 名称 | 用途 |
|---|---|---|
| `demo-alpha` | 示例 A（内置） | 稳定返回 5 条结果的 mock 索引器 |
| `demo-beta` | 示例 B（内置） | 第二个 mock，与 alpha 共享部分标题用于演示去重 |
| `demo-gamma` | 示例 C（故意失败） | 永远失败的 mock，用于演示错误隔离 |

> 这些示例**不访问真实网站**，纯本地 mock。生产环境使用前请删除它们。

### 2.3 启用 / 停用 / 删除

| 操作 | 路径 |
|---|---|
| 启用 / 停用 | 索引器卡片右上角开关 |
| 删除 | 索引器卡片右上角 ⋯ → 删除 |
| 手动重新测试 | 索引器卡片 → 测试按钮 |

### 2.4 健康状态说明

| 状态 | 含义 |
|---|---|
| `healthy` | 最近一次测试成功 |
| `degraded` | 部分子请求失败但主请求成功 |
| `unhealthy` | 连续失败超过阈值 |
| `unknown` | 从未测试过 |
| `disabled` | 用户主动停用 |

后台健康检查每 12 小时跑一次；30 分钟内已查过的会跳过。

---

## 3. 搜索资源

### 3.1 基本用法

1. 在 **搜索** 页输入关键词
2. （可选）选分类：`全部 / 电影 / 电视 / 音乐 / 游戏 / 软件 / 书籍 / 动漫 / 其他`
3. （可选）选排序：`综合 / 做种数 / 发布时间 / 大小↑ / 大小↓`
4. 按 Enter 或点击 **搜索**

### 3.2 实时进度

搜索启动后页面顶部会显示每个索引器的状态：

| 状态 | 含义 |
|---|---|
| `等待中` | 排队中（受并发数限制） |
| `搜索中` | 已发出请求 |
| `完成 · N 条` | 返回 N 条结果 |
| `空结果` | 索引器正常返回但无匹配 |
| `超时` | 超过单索引器超时阈值（默认 15 秒） |
| `失败` | 网络错误或解析错误 |
| `已取消` | 用户手动取消 |

任一索引器返回结果后立即出现，不需要等所有完成。

### 3.3 结果卡片

每张卡片展示：

- **标题** — 标准化后的标题（NFKC + 去标点）
- **大小 / 发布时间 / 做种数** — 统一字段
- **来源** — 来自哪些索引器（去重后的合并来源数）
- **复制 / 打开** — 下载入口类型（磁力 / 种子 / 直链）

### 3.4 复制 vs 打开

| 按钮 | 行为 |
|---|---|
| **复制** | 把下载入口复制到剪贴板（磁力 / 种子 / 直链） |
| **打开** | 调用系统默认程序打开链接（浏览器 / 磁力客户端） |

> 磁力链接会被复制为 `magnet:?xt=urn:btih:...` 完整字符串。需要先安装一个磁力处理程序（如 qBittorrent、迅雷）才能真正打开。

### 3.5 取消搜索

搜索进行中点击 **取消** 按钮会立即中断所有未完成的请求，已返回的结果会保留展示。

---

## 4. 导入自定义索引器定义（YAML）

### 4.1 适用场景

- 想接入不在内置目录里的公开索引器
- 想接入任意 Torznab / Torznab 兼容接口
- 想调试或本地修改某个索引器定义

### 4.2 步骤

1. 准备一个 YAML 文件（参考 [`backend/internal/catalog/examples/`](./backend/internal/catalog/examples/) 里的示例）
2. 进入 **索引器** 页面 → **导入** 标签
3. 拖拽或选择 YAML 文件
4. 程序会自动校验 + 测试
5. 三选一：
   - **启用并保存**（测试通过）
   - **保存为停用**（测试失败但结构合法）
   - **取消**

### 4.3 YAML 关键字段

```yaml
schema: 1                  # 必须填 1
id: my-custom-indexer       # 只能小写字母数字 + 连字符
name: 我的自定义索引器
type: public                # 仅支持 public
protocol: torznab           # torznab | html | json | xml
links:
  base_url: https://example.com
search:
  paths:
    - method: GET
      url: '{{ .Indexer.BaseURL }}/api?t=search&q={{ .Query.Keyword }}'
  request:
    encoding: url
response:
  format: html              # 与 protocol 一致
fields:
  - name: title
    source: css
    selector: 'a.result-title'
    extract: text
```

完整规范见 [`spec-o1.md` §13](./spec-o1.md)。

### 4.4 会被拒绝的情况

- 缺少 `id` / `name` / `links.base_url`
- `id` 不符合正则（小写字母 + 数字 + 连字符）
- `base_url` 用了 HTTP（必须 HTTPS）
- `base_url` 指向私有 / 回环 / 链路本地 IP
- YAML 文件超过 512 KB
- `base_url` 包含 `user:password@` 段
- selector 试图执行任意代码（沙箱限制）

---

## 5. 更新内置索引器目录

### 5.1 自动更新

启动时会自动尝试从 `EASYSEARCH_CATALOG_URL` 拉取最新 manifest。配置该环境变量后：

- 每 24 小时检查一次
- 校验 SHA-256
- 校验失败 / 网络失败 → 保留本地版本
- 激活后用户的启用状态和自定义 BaseURL **不会** 被覆盖

### 5.2 手动更新

调用 API 触发：

```bash
curl -X POST http://127.0.0.1:<port>/api/v1/indexer-catalog/update
```

或在 UI 里点 **更新目录** 按钮。

### 5.3 查看当前状态

```bash
curl http://127.0.0.1:<port>/api/v1/indexer-catalog/status
```

返回：

```json
{
  "source": "remote",          // 或 "embedded" / "cache"
  "version": "2026.07.1",
  "definitions": 3,
  "lastCheckedAt": "2026-07-09T12:00:00Z"
}
```

---

## 6. 诊断与故障排查

### 6.1 导出诊断包

```bash
curl -O http://127.0.0.1:<port>/api/v1/system/diagnostics
```

或在 UI 设置页点 **导出诊断**。

### 6.2 包内容

下载的 ZIP 包含：

| 文件 | 内容 |
|---|---|
| `README.txt` | 包说明 |
| `version.txt` | 程序版本 + 构建目标 |
| `os.txt` | OS / Go 版本 |
| `uptime.txt` | 进程启动时间 + 运行时长 |
| `schema.txt` | 数据库 schema 版本 + 索引器定义版本 |
| `indexers.json` | 已安装索引器状态摘要 |
| `catalog.json` | 当前目录源 + 版本 |
| `logs/*.log` | 最近 5 MB 的脱敏日志 |

### 6.3 隐私保证

诊断包**绝不包含**：

- ❌ 完整磁力链接（`magnet:?xt=urn:btih:...`）
- ❌ 搜索关键词
- ❌ 下载内容或用户文件
- ❌ 凭证化的 URL

日志里所有磁力链接会替换成 `magnet:?xt=urn:btih:<redacted>`，裸 InfoHash 替换成 `<btih-redacted>`。

### 6.4 常见故障

| 现象 | 可能原因 | 排查方法 |
|---|---|---|
| 启动后浏览器没自动打开 | 系统默认浏览器未配置 / `--no-browser` 启动 | 检查命令行输出里的 URL，手动打开 |
| 端口被占用 | 上次未正常退出残留 `.port` | 删除 `%APPDATA%\EasySearch\data\.port` |
| 添加索引器测试一直失败 | 目标站点对客户端 UA / IP 做了限制 | 看 `logs\easysearch.log` 里的错误码 |
| 搜索一直返回空 | 所有已启用索引器都失败 | 检查 **搜索** 页顶部状态区 |
| 程序占用内存过大 | 已启用索引器过多 | 控制并发在 6 以内（默认） |

---

## 7. 常见问题 FAQ

### 7.1 为什么不内置更多索引器？

EasySearch 只内置三个示例。添加真实索引器需要：

1. 与各站点的 ToS 不冲突
2. 提供稳定可访问的公开 API
3. 维护 YAML 定义文件

这些都需要人工维护，所以我们提供 **导入** 接口，让用户自己决定接入哪些站点。

### 7.2 是否支持私有站点（PT）？

**不支持**，且不会支持。

私有站点需要 Cookie / 用户名 / 密码 / API Key 登录，违反我们的安全原则（见 `spec-o1.md` §3.2）。

### 7.3 是否会下载资源？

**不会**。EasySearch 只负责搜索和展示下载入口，下载由用户点击按钮后交给系统默认程序。

### 7.4 是否会上传我的搜索历史？

**不会**。所有搜索在内存里进行，**不持久化搜索关键词或结果**。

### 7.5 是否会"学习"我的搜索偏好？

**不会**。EasySearch 没有遥测、没有云同步、没有用户系统。

### 7.6 能加一个 i18n 支持吗？

未来计划（`spec-o1.md` §3.3），目前仅中文 UI。

### 7.7 编译后多大？需要 .NET / VC++ 运行时吗？

`easysearch.exe` 约 14.6 MB，**无任何外部依赖**。SQLite 是纯 Go 编译进去的。

### 7.8 启动报"数据目录权限不足"怎么办？

通常因为 `%APPDATA%\EasySearch` 被其它进程占用或权限错乱。删除该目录后重启即可。

### 7.9 多用户系统 / 远程访问？

**不支持**。EasySearch 故意只监听 `127.0.0.1`，不允许局域网或互联网访问。

---

## 8. 隐私与数据

### 8.1 数据存储

| 数据 | 位置 | 用途 |
|---|---|---|
| 已安装索引器配置 | SQLite `installed_indexers` 表 | 启用 / 停用持久化 |
| 健康检查历史 | SQLite `indexer_health_events` 表 | 最近 5000 条 / 30 天滚动 |
| 远程目录缓存 | `%APPDATA%\EasySearch\data\catalog-cache\` | 离线启动可用 |
| 日志 | `%APPDATA%\EasySearch\data\logs\` | 排障 |

### 8.2 不存储的数据

- ❌ 搜索关键词历史
- ❌ 搜索结果（不缓存）
- ❌ 完整磁力链接（仅在内存中出现）
- ❌ 下载链接历史
- ❌ 用户行为分析

### 8.3 网络请求

EasySearch 仅向**用户主动配置的**索引器发起请求：

- 每次搜索：N 个 GET 请求（N = 已启用索引器数）
- 健康检查：每 12 小时 1 次
- 目录更新：每 24 小时最多 1 次

**绝不**向 EasySearch 作者、第三方分析服务或任何未配置的域名发送请求。

### 8.4 日志脱敏

`logs\easysearch.log` 里所有磁力链接 / 搜索关键词会被运行时脱敏。但**用户主动**在 UI 输入的搜索词仍可能作为参数出现在日志里——这是为了排障故意保留的。诊断包导出时会**二次**脱敏。

---

## 附录 A：HTTP API 摘要

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/v1/system/status` | 服务状态 |
| `GET` | `/api/v1/system/diagnostics` | 下载诊断 ZIP |
| `GET` | `/api/v1/indexers` | 已安装索引器列表 |
| `POST` | `/api/v1/indexers` | 从目录添加索引器 |
| `GET` | `/api/v1/indexers/{id}` | 单个索引器详情 |
| `PATCH` | `/api/v1/indexers/{id}` | 更新启用 / BaseURL |
| `DELETE` | `/api/v1/indexers/{id}` | 删除 |
| `POST` | `/api/v1/indexers/{id}/test` | 手动测试 |
| `GET` | `/api/v1/indexer-catalog` | 内置目录 |
| `POST` | `/api/v1/indexer-catalog/import` | 导入 YAML |
| `GET` | `/api/v1/indexer-catalog/imported` | 已导入列表 |
| `POST` | `/api/v1/indexer-catalog/update` | 触发远程更新 |
| `GET` | `/api/v1/indexer-catalog/status` | 当前目录状态 |
| `POST` | `/api/v1/search/sessions` | 创建搜索会话 |
| `GET` | `/api/v1/search/sessions/{id}/events` | SSE 事件流 |
| `POST` | `/api/v1/search/sessions/{id}/cancel` | 取消搜索 |

## 附录 B：术语表

| 术语 | 含义 |
|---|---|
| **索引器 (Indexer)** | 提供资源搜索 API 的服务（公开网站） |
| **适配器 (Adapter)** | EasySearch 与具体索引器之间的协议翻译层 |
| **Torznab** | 新znab 派生协议，社区事实标准 |
| **InfoHash** | BitTorrent 信息哈希，BT 资源的唯一标识 |
| **去重 (Dedup)** | 多源返回相同资源时合并展示 |
| **强去重 / 弱去重** | 按 InfoHash / URL 完全相同 vs 按标题 + 大小相近 |
| **SSE** | Server-Sent Events，浏览器实时接收服务端推送的协议 |
| **SSRF** | Server-Side Request Forgery，EasySearch 通过 URL 校验拦截 |
| **manifest** | 内置索引器目录的元数据 + 校验和清单 |
