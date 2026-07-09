# EasySearch 资源搜索工具开发规格说明书

> 文档类型：产品规格 + 技术实现规格  
> 目标读者：开发 Agent、前端工程师、后端工程师、测试工程师  
> 版本：v1.0  
> 状态：可进入开发  

---

## 1. 项目概述

### 1.1 项目名称

暂定名称：`EasySearch`

### 1.2 项目定位

EasySearch 是一款运行在个人电脑上的轻量级资源聚合搜索工具。

产品只解决两个问题：

1. 用户输入关键词后，从已启用的公开索引器中搜索资源，并展示可点击或可复制的下载入口。
2. 用户可以从内置索引器目录中一键添加公开索引器，也可以导入符合本项目规范的索引器定义。

产品不以复刻 Prowlarr 为目标，也不实现 Prowlarr 的下载器管理、应用同步、私有站点认证、自动订阅、RSS 监控、通知、代理配置等能力。

### 1.3 核心产品原则

- 首页即搜索，不要求用户先理解协议或索引器概念。
- 添加索引器应尽量做到“一次点击完成”。
- 默认配置可直接使用，高级参数隐藏。
- 搜索结果字段统一，不暴露不同网站的原始结构差异。
- 某个索引器失败时，不影响其他索引器返回结果。
- 工具仅用于搜索用户有权访问、下载和使用的公开资源。
- 不绕过验证码、登录、付费墙、Cloudflare、访问控制或其他安全机制。

---

## 2. 产品目标

### 2.1 必须实现

#### 功能 A：资源搜索

用户能够：

- 输入资源关键词。
- 选择可选分类。
- 同时搜索多个已启用的公开索引器。
- 实时查看各索引器搜索状态。
- 查看统一格式的搜索结果。
- 复制或打开磁力链接、种子链接或公开直链。
- 根据相关度、发布时间、文件大小、做种数排序。
- 查看资源来自哪些索引器。
- 对重复结果进行自动合并。

#### 功能 B：公开索引器加载

用户能够：

- 查看内置公开索引器目录。
- 一键添加公开索引器。
- 在添加前或添加后自动测试可用性。
- 启用、停用和删除已添加索引器。
- 查看索引器健康状态、最近响应时间和最近错误。
- 导入本地 YAML 索引器定义文件。
- 在不升级主程序的情况下更新索引器定义目录。

### 2.2 成功标准

满足以下条件可视为 MVP 成功：

- 首次启动后，普通用户在三次点击以内完成索引器添加。
- 用户从输入关键词到看到第一条结果，正常网络条件下不超过 5 秒。
- 单个索引器失败不会导致整次搜索失败。
- 搜索结果中存在可用下载入口时，用户可以一次点击复制或打开。
- 新增一个简单公开 HTML 索引器时，只需增加 YAML 文件，不修改主程序代码。
- Windows 用户无需安装额外运行环境即可启动。

---

## 3. 功能边界

### 3.1 本期范围

#### 搜索

- 关键词搜索。
- 分类筛选。
- 多索引器并发搜索。
- HTML、JSON、XML、Torznab 响应解析。
- 搜索进度展示。
- 搜索结果标准化。
- 下载链接提取。
- InfoHash 提取。
- 去重与来源合并。
- 排序与基础筛选。
- 复制链接。
- 调用系统默认程序打开链接。

#### 索引器

- 内置索引器目录。
- 公开索引器一键添加。
- 自定义 URL 参数覆盖。
- YAML 定义导入。
- 健康检测。
- 启用和停用。
- 删除。
- 定义在线更新。
- 索引器失败自动降级。

### 3.2 明确不做

以下功能不得进入 v1.0：

- 私有站点账号登录。
- Cookie、用户名、密码、API Key 登录。
- 验证码识别。
- Cloudflare 或其他反爬机制绕过。
- FlareSolverr 集成。
- 浏览器自动化抓取。
- DHT 网络爬取或建立磁力数据库。
- 自动下载资源。
- 内置 BT 下载器。
- qBittorrent、Transmission 等下载器管理。
- Sonarr、Radarr、Lidarr 等应用同步。
- RSS 自动监控。
- 自动订阅电视剧、电影或关键词。
- Usenet。
- 用户系统和多租户。
- 公网服务器部署管理。
- 远程访问。
- 资源内容审核系统。
- 搜索历史云同步。

### 3.3 后续可扩展但本期不实现

- qBittorrent 一键推送。
- 搜索建议与历史记录。
- 资源详情页。
- 媒体元数据匹配。
- 本地收藏夹。
- 多语言界面。
- 插件市场。

---

## 4. 用户角色与核心场景

### 4.1 用户角色

本项目只有一种角色：本地用户。

用户不需要注册和登录。

### 4.2 核心场景一：首次使用

1. 用户启动程序。
2. 程序自动打开本地 WebUI。
3. 首页提示“尚未添加索引器”。
4. 用户点击“添加公开索引器”。
5. 系统展示推荐索引器列表。
6. 用户点击“添加”。
7. 系统自动测试索引器。
8. 测试成功后索引器自动启用。
9. 用户返回搜索页并开始搜索。

### 4.3 核心场景二：搜索资源

1. 用户输入关键词。
2. 用户点击“搜索”或按 Enter。
3. 系统对所有已启用索引器发起并发请求。
4. 页面即时显示搜索状态。
5. 任一索引器返回结果后，页面立即展示结果。
6. 系统继续接收其他索引器结果。
7. 系统自动去重并合并来源。
8. 用户点击“复制链接”或“打开链接”。

### 4.4 核心场景三：索引器失效

1. 用户发起搜索。
2. 某索引器超时或解析失败。
3. 页面显示该索引器失败，但继续展示其他结果。
4. 系统记录错误信息。
5. 连续失败达到阈值后，索引器标记为“异常”。
6. 用户可在索引器页重新测试或停用。

### 4.5 核心场景四：导入索引器定义

1. 用户进入索引器页面。
2. 点击“导入 YAML”。
3. 选择本地文件。
4. 系统校验 YAML 结构和安全规则。
5. 系统执行连接测试。
6. 测试成功后保存并启用。
7. 测试失败时允许保存为停用状态。

---

## 5. 页面与交互规格

项目只包含两个主页面：

- 搜索页。
- 索引器页。

设置、日志和关于信息使用抽屉或弹窗，不增加主导航页面。

---

## 6. 搜索页规格

### 6.1 页面结构

```text
┌──────────────────────────────────────────────────────────┐
│ EasySearch                               [索引器 6/8]     │
├──────────────────────────────────────────────────────────┤
│ [ 输入关键词                                      ][搜索] │
│ 分类：[全部 ▼]   排序：[综合排序 ▼]   [更多筛选]          │
├──────────────────────────────────────────────────────────┤
│ 搜索状态：5 成功 · 1 搜索中 · 1 超时 · 1 失败            │
├──────────────────────────────────────────────────────────┤
│ 结果卡片                                                 │
│ 结果卡片                                                 │
│ 结果卡片                                                 │
└──────────────────────────────────────────────────────────┘
```

### 6.2 搜索框

要求：

- 默认获得焦点。
- 支持 Enter 搜索。
- 空关键词不得发起请求。
- 自动去除首尾空格。
- 最长 200 个字符。
- 搜索过程中允许修改关键词，但新搜索会取消上一轮未完成请求。

### 6.3 分类筛选

首版统一分类：

| 分类值 | 显示名称 |
|---|---|
| all | 全部 |
| movie | 电影 |
| tv | 剧集 |
| music | 音乐 |
| game | 游戏 |
| software | 软件 |
| book | 图书 |
| anime | 动漫 |
| other | 其他 |

索引器原始分类必须映射为上述统一分类。

### 6.4 排序方式

支持：

- 综合排序。
- 做种数优先。
- 最新发布。
- 文件从大到小。
- 文件从小到大。

### 6.5 更多筛选

首版仅支持：

- 最小文件大小。
- 最大文件大小。
- 最少做种数。
- 发布时间范围。
- 指定索引器。

默认收起。

### 6.6 搜索状态区域

每次搜索展示：

- 总索引器数量。
- 搜索中数量。
- 成功数量。
- 超时数量。
- 失败数量。
- 原始结果数量。
- 合并后结果数量。
- 搜索总耗时。

点击状态区域可展开查看每个索引器的状态。

状态枚举：

```text
pending
running
success
empty
timeout
error
cancelled
```

### 6.7 结果卡片

每条结果至少展示：

- 标题。
- 统一分类。
- 文件大小。
- 做种数。
- 下载数或吸血数，数据存在时展示。
- 发布时间。
- 来源索引器。
- 下载入口类型。
- 复制链接按钮。
- 打开链接按钮。
- 详情页按钮，存在详情链接时展示。

推荐卡片结构：

```text
资源标题
8.4 GB · 326 做种 · 18 下载 · 2 小时前
来源：Indexer A +2
[复制磁力链接] [打开] [详情]
```

### 6.8 下载入口优先级

同一结果存在多个下载入口时，按以下优先级选择主按钮：

1. Magnet。
2. Torrent 文件 URL。
3. 公开直链。
4. 详情页 URL。

页面必须明确标记链接类型，禁止把详情页链接伪装为下载链接。

### 6.9 打开链接行为

- Magnet：调用操作系统默认磁力链接处理程序。
- Torrent URL：调用默认浏览器下载。
- HTTP/HTTPS 直链：调用默认浏览器。
- 详情页：新标签页打开。

调用失败时显示可复制文本，不得导致页面崩溃。

### 6.10 空状态

#### 未添加索引器

```text
尚未添加索引器
添加至少一个公开索引器后即可开始搜索。
[添加公开索引器]
```

#### 无结果

```text
未找到相关资源
可以尝试缩短关键词、切换分类或添加更多索引器。
```

#### 全部失败

```text
本次搜索未能连接任何索引器
[查看索引器状态] [重新搜索]
```

---

## 7. 索引器页规格

### 7.1 页面结构

```text
┌──────────────────────────────────────────────────────────┐
│ 索引器                                      [导入 YAML]  │
├──────────────────────────────────────────────────────────┤
│ 已添加索引器                                             │
│ [索引器卡片]                                             │
│ [索引器卡片]                                             │
├──────────────────────────────────────────────────────────┤
│ 推荐公开索引器                                           │
│ [推荐卡片] [推荐卡片] [推荐卡片]                         │
└──────────────────────────────────────────────────────────┘
```

### 7.2 已添加索引器卡片

展示字段：

- 名称。
- 图标，可选。
- 描述。
- 支持分类。
- 语言。
- 协议类型。
- 当前状态。
- 最近响应时间。
- 最近成功时间。
- 最近错误摘要。
- 启用开关。
- 测试按钮。
- 删除按钮。

### 7.3 推荐索引器卡片

展示字段：

- 名称。
- 描述。
- 支持分类。
- 语言。
- 官方或公开主页域名。
- 当前目录版本。
- 添加按钮。

添加流程：

1. 点击添加。
2. 创建临时索引器实例。
3. 执行健康测试。
4. 成功则保存并启用。
5. 失败则提示错误，并提供：
   - 重试。
   - 仍然添加但保持停用。
   - 取消。

### 7.4 删除索引器

删除前确认：

```text
确认删除“索引器名称”？
删除后不会影响其他索引器。
```

删除只移除本地实例，不删除内置目录定义。

### 7.5 启用与停用

- 停用后不参与搜索。
- 停用不删除配置。
- 恢复启用前可选执行一次测试。
- 连续失败自动标记异常，但默认不自动永久停用。

### 7.6 索引器健康状态

状态枚举：

| 状态 | 含义 |
|---|---|
| healthy | 最近测试成功 |
| degraded | 最近搜索部分字段解析失败或响应过慢 |
| unhealthy | 最近测试失败 |
| unknown | 从未测试 |
| disabled | 用户主动停用 |

### 7.7 健康检测规则

一次健康检测至少验证：

- URL 格式有效。
- 域名可解析。
- HTTP 请求可完成。
- 响应状态码符合定义。
- 能执行测试查询。
- 能解析至少一个结果，或确认合法空结果。
- 必填字段解析器无语法错误。

### 7.8 自动健康检测

- 程序启动后异步检测一次已启用索引器。
- 默认每 12 小时检测一次。
- 搜索失败会更新状态。
- 同一索引器 30 分钟内不重复后台检测。
- 后台检测不得阻塞主界面启动。

---

## 8. 技术架构

### 8.1 推荐技术栈

#### 前端

- React。
- TypeScript。
- Vite。
- React Router。
- TanStack Query。
- Zustand 或 React Context。
- CSS Modules、Tailwind CSS 或轻量组件库。

#### 后端

- Go 1.24 或当前稳定版本。
- 标准库 `net/http` 或 Gin/Fiber。
- SQLite。
- `goquery` 解析 HTML。
- `gopkg.in/yaml.v3` 解析 YAML。
- 标准库解析 JSON 和 XML。
- SSE 返回实时搜索进度。

#### 桌面运行方式

首选：

- Go 后端编译为单个可执行文件。
- 前端构建产物通过 `embed.FS` 嵌入可执行文件。
- 启动后监听 `127.0.0.1` 随机可用端口。
- 自动打开默认浏览器。
- SQLite 和用户配置写入用户数据目录。

Windows 数据目录建议：

```text
%APPDATA%/EasySearch/
```

目录结构：

```text
EasySearch/
├── easysearch.exe
└── data/
    ├── easysearch.db
    ├── indexers/
    ├── logs/
    └── cache/
```

### 8.2 系统架构

```text
┌───────────────────────────────┐
│          React WebUI          │
│    Search Page / Indexers     │
└───────────────┬───────────────┘
                │ REST + SSE
┌───────────────▼───────────────┐
│           API Layer           │
│ Search / Indexer / Catalog    │
└───────────────┬───────────────┘
                │
┌───────────────▼───────────────┐
│       Search Orchestrator     │
│ 并发、取消、超时、聚合、排序   │
└───────────────┬───────────────┘
                │
┌───────────────▼───────────────┐
│        Indexer Engine         │
│ Torznab / HTML / JSON / XML   │
└───────────────┬───────────────┘
                │
┌───────────────▼───────────────┐
│     Normalizer / Deduper      │
│ 字段标准化、InfoHash、去重     │
└───────────────┬───────────────┘
                │
┌───────────────▼───────────────┐
│            SQLite             │
│ Indexers / Health / Settings  │
└───────────────────────────────┘
```

---

## 9. 后端模块划分

```text
/backend
├── cmd/
│   └── easysearch/
│       └── main.go
├── internal/
│   ├── api/
│   │   ├── search_handler.go
│   │   ├── indexer_handler.go
│   │   ├── catalog_handler.go
│   │   └── system_handler.go
│   ├── search/
│   │   ├── orchestrator.go
│   │   ├── event.go
│   │   ├── normalizer.go
│   │   ├── deduplicator.go
│   │   ├── ranker.go
│   │   └── filters.go
│   ├── indexer/
│   │   ├── adapter.go
│   │   ├── factory.go
│   │   ├── torznab.go
│   │   ├── declarative.go
│   │   ├── request_builder.go
│   │   ├── html_parser.go
│   │   ├── json_parser.go
│   │   ├── xml_parser.go
│   │   └── health_checker.go
│   ├── catalog/
│   │   ├── loader.go
│   │   ├── updater.go
│   │   └── validator.go
│   ├── storage/
│   │   ├── sqlite.go
│   │   ├── migrations/
│   │   ├── indexer_repository.go
│   │   ├── health_repository.go
│   │   └── settings_repository.go
│   ├── security/
│   │   ├── url_validator.go
│   │   ├── network_policy.go
│   │   └── sanitize.go
│   └── model/
│       ├── indexer.go
│       ├── result.go
│       ├── search.go
│       └── catalog.go
├── indexers/
└── web/
```

---

## 10. 前端模块划分

```text
/frontend/src
├── app/
│   ├── App.tsx
│   ├── router.tsx
│   └── providers.tsx
├── pages/
│   ├── SearchPage.tsx
│   └── IndexerPage.tsx
├── features/
│   ├── search/
│   │   ├── SearchBar.tsx
│   │   ├── SearchFilters.tsx
│   │   ├── SearchStatus.tsx
│   │   ├── ResultList.tsx
│   │   ├── ResultCard.tsx
│   │   └── useSearchStream.ts
│   └── indexers/
│       ├── InstalledIndexerList.tsx
│       ├── CatalogList.tsx
│       ├── IndexerCard.tsx
│       ├── ImportDialog.tsx
│       └── TestResultDialog.tsx
├── services/
│   ├── api.ts
│   ├── search.ts
│   └── indexers.ts
├── types/
└── utils/
```

---

## 11. 核心数据模型

### 11.1 IndexerDefinition

表示索引器目录中的静态定义。

```go
type IndexerDefinition struct {
    ID          string
    Name        string
    Description string
    Version     string
    Language    string
    Type        string
    Protocol    string
    Categories  []string
    Links       []string
    Search      SearchDefinition
    Result      ResultDefinition
    RateLimit   *RateLimitDefinition
}
```

### 11.2 InstalledIndexer

表示用户已添加的索引器实例。

```go
type InstalledIndexer struct {
    ID                string
    DefinitionID      string
    Name              string
    Enabled           bool
    BaseURL           string
    DefinitionVersion string
    Status            string
    LastCheckedAt     *time.Time
    LastSuccessAt     *time.Time
    LastError         string
    ResponseTimeMs    int64
    ConsecutiveFails  int
    CreatedAt         time.Time
    UpdatedAt         time.Time
}
```

### 11.3 SearchQuery

```go
type SearchQuery struct {
    Keyword        string
    Category       string
    MinSizeBytes   *int64
    MaxSizeBytes   *int64
    MinSeeders     *int
    PublishedAfter *time.Time
    IndexerIDs     []string
    Sort           string
}
```

### 11.4 SearchResult

```go
type SearchResult struct {
    ID            string
    Title         string
    NormalizedTitle string
    Category      string
    SizeBytes     *int64
    Seeders       *int
    Leechers      *int
    Downloads     *int
    PublishedAt   *time.Time
    MagnetURL     string
    TorrentURL    string
    DirectURL     string
    DetailURL     string
    InfoHash      string
    IndexerID     string
    IndexerName   string
    Score         float64
    Sources       []ResultSource
    Raw           map[string]any
}
```

### 11.5 ResultSource

```go
type ResultSource struct {
    IndexerID    string
    IndexerName  string
    MagnetURL    string
    TorrentURL   string
    DirectURL    string
    DetailURL    string
    Seeders      *int
    PublishedAt  *time.Time
}
```

### 11.6 SearchSession

```go
type SearchSession struct {
    ID             string
    Query          SearchQuery
    Status         string
    StartedAt      time.Time
    FinishedAt     *time.Time
    TotalIndexers  int
    CompletedCount int
    RawResultCount int
    MergedCount    int
}
```

---

## 12. 索引器适配器接口

所有索引器必须实现统一接口：

```go
type IndexerAdapter interface {
    ID() string
    Test(ctx context.Context) TestResult
    Search(ctx context.Context, query SearchQuery) ([]SearchResult, error)
}
```

工厂接口：

```go
type AdapterFactory interface {
    Create(def IndexerDefinition, installed InstalledIndexer) (IndexerAdapter, error)
}
```

首版适配器类型：

- `TorznabAdapter`。
- `DeclarativeAdapter`。

`DeclarativeAdapter` 根据 YAML 定义处理 HTML、JSON 或 XML。

---

## 13. YAML 索引器定义规范

### 13.1 设计目标

- 简单公开站点无需写 Go 代码。
- 定义文件可独立更新。
- 禁止在 YAML 中执行任意脚本。
- 不支持登录和验证绕过。
- 所有表达式必须是受限模板或声明式选择器。

### 13.2 基础示例

```yaml
schema: 1
id: example-public
name: Example Public
version: 1.0.0
description: Example public indexer
language: zh-CN
type: public
protocol: declarative

links:
  - https://example.com

categories:
  movie:
    - "1"
  tv:
    - "2"
  anime:
    - "3"

search:
  method: GET
  path: /search
  query:
    keyword: "{{ query.keyword }}"
    category: "{{ query.category_id }}"
  headers:
    Accept: text/html
  timeout_seconds: 12

response:
  format: html
  rows:
    selector: ".result-item"

fields:
  title:
    selector: ".title"
    value: text
    required: true

  detail_url:
    selector: ".title a"
    value: attr
    attribute: href
    resolve_url: true

  size:
    selector: ".size"
    value: text
    filters:
      - trim
      - parse_size

  seeders:
    selector: ".seeders"
    value: text
    filters:
      - trim
      - parse_int

  published_at:
    selector: ".date"
    value: text
    filters:
      - trim
      - parse_date
    date_layouts:
      - "2006-01-02 15:04"

  magnet_url:
    selector: "a[href^='magnet:']"
    value: attr
    attribute: href

  torrent_url:
    selector: "a.torrent"
    value: attr
    attribute: href
    resolve_url: true
```

### 13.3 支持的响应格式

```yaml
response:
  format: html | json | xml | torznab
```

### 13.4 支持的字段

```text
title
category
size
seeders
leechers
downloads
published_at
magnet_url
torrent_url
direct_url
detail_url
info_hash
```

`title` 为必填字段。

下载入口至少存在以下之一时，结果才可作为可下载结果展示：

- `magnet_url`
- `torrent_url`
- `direct_url`

只有 `detail_url` 的结果可展示，但必须标记为“详情入口”，不得显示为“下载”。

### 13.5 支持的字段取值方式

HTML：

```text
text
html
attr
```

JSON：

```text
json_path
```

XML：

```text
xpath
attribute
```

首版 JSONPath 和 XPath 只支持受限子集，禁止执行函数或脚本。

### 13.6 支持的过滤器

```text
trim
lower
upper
replace
regex_extract
parse_int
parse_float
parse_size
parse_date
resolve_url
extract_info_hash
```

### 13.7 模板变量

允许：

```text
{{ query.keyword }}
{{ query.category }}
{{ query.category_id }}
{{ query.page }}
{{ indexer.base_url }}
```

禁止：

- 任意代码执行。
- 文件读取。
- 环境变量读取。
- Shell 命令。
- 动态网络请求链。

### 13.8 YAML 校验

导入时必须校验：

- `schema` 是否支持。
- `id` 是否符合 `[a-z0-9-]+`。
- `name` 是否存在。
- `links` 是否至少一个合法 HTTPS URL。
- `type` 必须为 `public`。
- `protocol` 是否支持。
- 搜索路径和参数是否合法。
- 选择器语法是否合法。
- 是否存在危险 URL。
- 是否包含禁止字段。
- 是否会访问本地网络地址。

---

## 14. Torznab 支持规格

### 14.1 支持请求

```text
GET {base_url}/api?t=search&q={keyword}&cat={category_ids}
```

可根据索引器定义覆盖：

- API 路径。
- 查询参数名。
- 分类映射。
- 超时时间。

### 14.2 支持字段

从 RSS/XML 与 Torznab 属性中读取：

- title。
- guid。
- link。
- pubDate。
- enclosure。
- category。
- size。
- seeders。
- peers。
- grabs。
- magneturl。
- infohash。

### 14.3 不支持

- 需要 API Key 的私有 Torznab 服务。
- 用户凭据配置。
- Newznab/Usenet 专用能力。

---

## 15. 搜索执行流程

### 15.1 总流程

```text
接收搜索请求
    ↓
校验关键词和筛选条件
    ↓
获取已启用索引器
    ↓
创建 SearchSession
    ↓
为每个索引器创建独立 context 与超时
    ↓
限制并发数后开始搜索
    ↓
实时推送索引器状态事件
    ↓
标准化每批结果
    ↓
提取 InfoHash 和下载链接
    ↓
过滤无效结果
    ↓
去重和合并来源
    ↓
计算综合分
    ↓
推送增量结果
    ↓
所有索引器完成后推送 done 事件
```

### 15.2 并发控制

默认：

- 最大并发索引器数：6。
- 单索引器搜索超时：15 秒。
- 整次搜索最长：30 秒。
- 单索引器最大结果数：100。
- 整次搜索最大原始结果数：1000。

参数必须可在内部设置中修改，但不在普通界面中展示。

### 15.3 请求取消

以下场景取消当前搜索：

- 用户开始新搜索。
- 用户主动点击停止。
- 页面关闭 SSE 连接且无其他订阅者。
- 达到整次搜索超时。

取消时：

- 立即取消未完成 HTTP 请求。
- 已得到结果继续保留在当前页面。
- 状态标记为 `cancelled`。

### 15.4 搜索事件

SSE 事件类型：

```text
session_started
indexer_started
indexer_result
indexer_completed
indexer_failed
results_merged
session_completed
session_cancelled
```

示例：

```json
{
  "event": "indexer_completed",
  "data": {
    "sessionId": "abc",
    "indexerId": "example",
    "status": "success",
    "resultCount": 28,
    "durationMs": 843
  }
}
```

---

## 16. 字段标准化

### 16.1 标题标准化

用于去重的标准化规则：

- 转为小写。
- Unicode 归一化。
- 将连续空白转为单空格。
- 将 `.`, `_`, `-` 统一为空格。
- 移除首尾标点。
- 保留年份、分辨率、编码、季度、集数等关键信息。

不得修改向用户展示的原始标题。

### 16.2 文件大小

统一存储为字节 `int64`。

支持解析：

```text
B
KB
KiB
MB
MiB
GB
GiB
TB
TiB
```

解析失败时存储为 `null`。

### 16.3 日期

统一存储为 UTC。

索引器定义可声明多个日期格式。

无法解析时存储为 `null`，不得使用当前时间代替。

### 16.4 InfoHash

提取顺序：

1. 原始 `info_hash` 字段。
2. Magnet 中的 `xt=urn:btih:`。
3. Torrent 元数据，首版不主动下载 Torrent 文件计算哈希。

统一转为大写 40 位十六进制字符串；Base32 BTIH 需转换为十六进制。

---

## 17. 去重规则

按以下顺序判定：

### 17.1 强去重

满足任一条件直接合并：

- InfoHash 完全一致。
- Magnet BTIH 一致。
- 规范化后的 Torrent URL 完全一致。
- 规范化后的 Direct URL 完全一致。

### 17.2 弱去重

同时满足时可合并：

- 标准化标题相似度不低于 0.92。
- 文件大小均存在且误差不超过 2%。
- 关键特征不冲突。

关键特征包括：

- 年份。
- 季度和集数。
- 分辨率。
- 编码。
- 音轨或语言。

### 17.3 合并规则

合并后：

- 保留所有来源。
- 做种数取最大值。
- 发布时间取最新值，同时保留来源原始值。
- 下载入口按优先级选择主入口。
- 标题选择信息最完整的标题。
- 文件大小优先选择出现频次最高的值。

---

## 18. 综合排序

### 18.1 默认综合分

建议：

```text
score =
  text_match_score * 0.45
+ seed_score      * 0.20
+ freshness_score * 0.15
+ source_score    * 0.10
+ completeness    * 0.10
```

### 18.2 各项说明

- `text_match_score`：标题和关键词匹配度。
- `seed_score`：对做种数进行对数归一化。
- `freshness_score`：发布时间越近分数越高。
- `source_score`：来源越多分数越高。
- `completeness`：字段越完整分数越高。

搜索排序必须稳定；相同分数时按发布时间、做种数、标题排序。

---

## 19. API 规格

统一前缀：

```text
/api/v1
```

### 19.1 创建搜索会话

```http
POST /api/v1/search/sessions
Content-Type: application/json
```

请求：

```json
{
  "keyword": "example",
  "category": "movie",
  "sort": "relevance",
  "filters": {
    "minSizeBytes": null,
    "maxSizeBytes": null,
    "minSeeders": 0,
    "publishedAfter": null,
    "indexerIds": []
  }
}
```

响应：

```json
{
  "sessionId": "01J...",
  "streamUrl": "/api/v1/search/sessions/01J.../events"
}
```

### 19.2 订阅搜索事件

```http
GET /api/v1/search/sessions/{sessionId}/events
Accept: text/event-stream
```

### 19.3 取消搜索

```http
POST /api/v1/search/sessions/{sessionId}/cancel
```

### 19.4 获取已安装索引器

```http
GET /api/v1/indexers
```

### 19.5 添加目录索引器

```http
POST /api/v1/indexers
Content-Type: application/json
```

```json
{
  "definitionId": "example-public",
  "baseUrl": "https://example.com",
  "testBeforeEnable": true
}
```

### 19.6 更新索引器

```http
PATCH /api/v1/indexers/{id}
```

```json
{
  "enabled": false,
  "baseUrl": "https://example.com"
}
```

### 19.7 删除索引器

```http
DELETE /api/v1/indexers/{id}
```

### 19.8 测试索引器

```http
POST /api/v1/indexers/{id}/test
```

### 19.9 获取索引器目录

```http
GET /api/v1/indexer-catalog
```

查询参数：

```text
language
category
installed
keyword
```

### 19.10 导入 YAML

```http
POST /api/v1/indexer-catalog/import
Content-Type: multipart/form-data
```

响应包含：

- 校验结果。
- 风险提示。
- 测试结果。
- 是否已保存。

### 19.11 更新目录

```http
POST /api/v1/indexer-catalog/update
```

### 19.12 系统状态

```http
GET /api/v1/system/status
```

返回：

- 版本。
- 数据库状态。
- 索引器定义版本。
- 已安装索引器数量。
- 服务启动时间。

---

## 20. SQLite 数据库设计

### 20.1 installed_indexers

```sql
CREATE TABLE installed_indexers (
    id TEXT PRIMARY KEY,
    definition_id TEXT NOT NULL,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    definition_version TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'unknown',
    last_checked_at TEXT,
    last_success_at TEXT,
    last_error TEXT,
    response_time_ms INTEGER,
    consecutive_fails INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

### 20.2 indexer_health_events

```sql
CREATE TABLE indexer_health_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    indexer_id TEXT NOT NULL,
    status TEXT NOT NULL,
    duration_ms INTEGER,
    error_code TEXT,
    error_message TEXT,
    created_at TEXT NOT NULL
);
```

仅保留最近 5000 条健康事件或最近 30 天数据。

### 20.3 settings

```sql
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

### 20.4 imported_definitions

```sql
CREATE TABLE imported_definitions (
    id TEXT PRIMARY KEY,
    version TEXT NOT NULL,
    content TEXT NOT NULL,
    checksum TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
```

搜索结果首版不持久化保存。

---

## 21. 网络与安全约束

### 21.1 服务监听

- 默认仅监听 `127.0.0.1`。
- 禁止默认监听 `0.0.0.0`。
- 不提供公网访问开关。
- WebUI 和 API 同源。

### 21.2 SSRF 防护

索引器 URL 必须阻止访问：

- `localhost`。
- `127.0.0.0/8`。
- `0.0.0.0/8`。
- `10.0.0.0/8`。
- `172.16.0.0/12`。
- `192.168.0.0/16`。
- `169.254.0.0/16`。
- IPv6 回环和私有地址。
- `file://`、`ftp://`、`gopher://` 等非 HTTP 协议。
- 云元数据服务地址。

默认只允许 HTTPS；内置定义可显式声明允许 HTTP，但界面必须显示风险警告。

必须在 DNS 解析后再次验证目标 IP，防止 DNS Rebinding。

### 21.3 重定向限制

- 最多 5 次重定向。
- 每次重定向重新执行 URL 和 IP 安全校验。
- 禁止重定向到本地网络。

### 21.4 响应限制

- 单次响应最大 10 MB。
- 超过限制立即中止。
- 禁止自动解压无限体积响应。
- HTML、JSON、XML 解析必须具备资源限制。

### 21.5 YAML 安全

- 禁止 YAML 自定义类型实例化。
- 禁止脚本。
- 禁止任意模板函数。
- 禁止读取本地文件。
- 禁止写文件。
- 导入文件最大 512 KB。

### 21.6 日志隐私

- 默认不记录完整下载链接。
- Magnet 只记录前缀和脱敏 InfoHash。
- 不记录用户搜索关键词到长期日志。
- 调试日志由用户主动开启。
- 日志不得包含用户本地路径和敏感请求头。

---

## 22. 错误处理

### 22.1 统一错误结构

```json
{
  "error": {
    "code": "INDEXER_TIMEOUT",
    "message": "索引器请求超时",
    "details": {
      "indexerId": "example-public"
    }
  }
}
```

### 22.2 错误码

```text
INVALID_REQUEST
EMPTY_KEYWORD
NO_ENABLED_INDEXERS
INDEXER_NOT_FOUND
INDEXER_DISABLED
INDEXER_TIMEOUT
INDEXER_NETWORK_ERROR
INDEXER_HTTP_ERROR
INDEXER_PARSE_ERROR
INDEXER_RATE_LIMITED
INVALID_INDEXER_DEFINITION
UNSAFE_INDEXER_URL
CATALOG_UPDATE_FAILED
SEARCH_CANCELLED
INTERNAL_ERROR
```

### 22.3 用户提示原则

- 页面提示应可理解，不直接展示 Go 堆栈。
- 详细错误写入本地日志。
- 用户可复制诊断摘要。
- 单索引器错误不得弹出阻塞式全局弹窗。

---

## 23. 性能要求

### 23.1 启动

- 冷启动到 WebUI 可访问：不超过 3 秒。
- WebUI 首屏可交互：不超过 2 秒。

### 23.2 搜索

在 8 个索引器、正常网络条件下：

- 第一条结果目标时间：5 秒以内。
- 搜索状态必须在 500 毫秒内出现。
- 页面处理 1000 条原始结果时不得明显卡顿。
- 合并后列表滚动保持流畅。

### 23.3 内存

- 空闲内存目标：低于 150 MB。
- 搜索时峰值目标：低于 300 MB。

### 23.4 数据库

- 启动时自动执行迁移。
- 数据库损坏时保留原文件并给出恢复提示。

---

## 24. 可用性要求

- 所有主要操作可通过键盘完成。
- 搜索框支持 Enter。
- 按钮具备明确文字，不仅使用图标。
- 状态不只依赖颜色表达。
- 复制成功后显示短暂提示。
- 长标题支持换行和展开。
- 下载入口必须明确标记类型。
- 默认界面不展示协议术语和解析器参数。

---

## 25. 日志与诊断

### 25.1 日志级别

```text
error
warn
info
debug
```

默认 `info`。

### 25.2 日志轮转

- 单文件最大 10 MB。
- 最多保留 5 个文件。
- 自动清理 30 天前日志。

### 25.3 诊断包

可通过“导出诊断信息”生成 ZIP，包含：

- 程序版本。
- 操作系统版本。
- 索引器状态摘要。
- 脱敏日志。
- 数据库 schema 版本。
- 索引器定义版本。

不得包含：

- 完整搜索关键词历史。
- 完整 Magnet 链接。
- 下载内容。
- 用户其他文件。

---

## 26. 索引器目录更新机制

### 26.1 目录结构

```text
indexer-catalog/
├── manifest.json
├── definitions/
│   ├── example-public.yml
│   └── example-torznab.yml
└── signatures/
```

### 26.2 manifest.json

```json
{
  "schema": 1,
  "version": "2026.07.1",
  "generatedAt": "2026-07-09T00:00:00Z",
  "definitions": [
    {
      "id": "example-public",
      "version": "1.0.0",
      "file": "definitions/example-public.yml",
      "sha256": "..."
    }
  ]
}
```

### 26.3 更新规则

- 程序内置一份基础目录。
- 用户可手动检查更新。
- 默认每 24 小时检查一次目录版本。
- 更新文件必须校验 SHA-256。
- 建议对 manifest 进行数字签名验证。
- 更新失败继续使用当前本地版本。
- 索引器定义升级不得覆盖用户启用状态和自定义 Base URL。

### 26.4 兼容策略

- YAML 中包含 `schema` 版本。
- 不支持的 schema 不得加载。
- 定义升级失败时自动回退旧版本。
- 数据库记录索引器当前定义版本。

---

## 27. 测试规格

### 27.1 单元测试

必须覆盖：

- 关键词校验。
- 分类映射。
- URL 安全校验。
- HTML 选择器解析。
- JSONPath 受限解析。
- XPath 受限解析。
- 文件大小解析。
- 日期解析。
- Magnet InfoHash 提取。
- Base32 BTIH 转换。
- 标题标准化。
- 强去重。
- 弱去重。
- 排序评分。
- YAML schema 校验。
- HTTP 超时和取消。

后端核心模块测试覆盖率目标不低于 80%。

### 27.2 集成测试

使用本地 Mock Server 模拟：

- HTML 正常结果。
- JSON 正常结果。
- XML 正常结果。
- Torznab 正常结果。
- 合法空结果。
- HTTP 404。
- HTTP 429。
- HTTP 500。
- 连接超时。
- 响应过大。
- 重定向到私有地址。
- HTML 结构变化。
- 字段缺失。
- 无效 Magnet。

### 27.3 前端测试

必须覆盖：

- 未添加索引器空状态。
- 搜索输入和提交。
- SSE 增量结果。
- 单索引器失败。
- 搜索取消。
- 结果去重后来源展示。
- 复制链接。
- 打开链接失败提示。
- 索引器添加。
- 索引器测试。
- 索引器停用和删除。
- YAML 导入错误。

### 27.4 端到端测试

核心 E2E：

1. 启动应用。
2. 添加 Mock 公开索引器。
3. 输入关键词。
4. 等待第一条结果。
5. 验证结果字段。
6. 复制 Magnet。
7. 停用索引器。
8. 再次搜索并验证不再调用该索引器。

---

## 28. 验收标准

### 28.1 搜索功能验收

- [ ] 没有已启用索引器时，页面明确引导添加索引器。
- [ ] 关键词为空时不能搜索。
- [ ] 搜索会并发调用所有已启用索引器。
- [ ] 页面能实时显示每个索引器的状态。
- [ ] 任一索引器完成后结果可立即出现。
- [ ] 单索引器失败不影响其他结果。
- [ ] 结果字段统一。
- [ ] Magnet、Torrent、直链和详情页被正确区分。
- [ ] 用户能够复制下载入口。
- [ ] 用户能够调用系统默认程序打开入口。
- [ ] InfoHash 相同的结果被合并。
- [ ] 合并结果能够查看所有来源。
- [ ] 支持综合、做种数、发布时间和大小排序。
- [ ] 支持分类和基础筛选。
- [ ] 新搜索能够取消旧搜索。

### 28.2 索引器功能验收

- [ ] 能查看内置公开索引器目录。
- [ ] 能一键添加目录索引器。
- [ ] 添加时自动测试。
- [ ] 测试失败可重试或以停用状态保存。
- [ ] 能启用和停用索引器。
- [ ] 能删除已添加索引器。
- [ ] 能查看健康状态、响应时间和最近错误。
- [ ] 能导入合法 YAML。
- [ ] 非法或危险 YAML 被拒绝。
- [ ] 索引器目录可独立更新。
- [ ] 更新失败时不会破坏已有索引器。

### 28.3 安全验收

- [ ] 服务只监听本机回环地址。
- [ ] 导入索引器不能访问本地网络地址。
- [ ] 重定向后仍执行 SSRF 校验。
- [ ] 响应大小受到限制。
- [ ] YAML 不能执行代码。
- [ ] 日志不记录完整 Magnet 和长期搜索历史。
- [ ] 不支持登录、验证码绕过和反爬绕过。

### 28.4 安装与运行验收

- [ ] Windows 可执行文件可独立启动。
- [ ] 不需要用户安装 Node.js、Go、Python 或数据库。
- [ ] 首次启动自动创建数据目录和数据库。
- [ ] 启动后自动打开 WebUI。
- [ ] 程序退出后端口释放。

---

## 29. 开发阶段划分

### 阶段 1：项目骨架

交付：

- Go 服务。
- React WebUI。
- SQLite 初始化。
- 两个主页面。
- 前后端嵌入构建。
- Windows 单文件启动。

### 阶段 2：单索引器搜索

交付：

- `IndexerAdapter` 接口。
- 一个 HTML Mock 索引器。
- 搜索 API。
- SSE。
- 结果卡片。
- 复制和打开链接。

### 阶段 3：多索引器聚合

交付：

- 并发调度。
- 超时和取消。
- 搜索状态。
- 标准化。
- 去重。
- 排序。

### 阶段 4：索引器管理

交付：

- 内置目录。
- 添加、启用、停用、测试、删除。
- 健康状态。
- SQLite 持久化。

### 阶段 5：YAML 引擎

交付：

- YAML schema。
- HTML、JSON、XML 声明式解析。
- 导入和校验。
- 安全限制。

### 阶段 6：Torznab 与目录更新

交付：

- 通用 Torznab 适配器。
- manifest 更新。
- 校验和。
- 版本回退。

### 阶段 7：测试与发布

交付：

- 单元测试。
- 集成测试。
- E2E。
- Windows 构建。
- 使用说明。
- 诊断导出。

---

## 30. 开发 Agent 执行要求

开发 Agent 必须遵守：

1. 严格控制范围，不主动增加下载器、账号登录、RSS 或应用同步。
2. 优先完成可运行的垂直切片，不先搭建过度复杂的框架。
3. 每个阶段结束后保证主分支可编译、可启动、可测试。
4. 所有网络请求必须支持 context 取消和超时。
5. 所有外部响应均视为不可信输入。
6. 禁止在索引器 YAML 中加入任意脚本执行能力。
7. 不通过无头浏览器绕过站点访问限制。
8. 新增索引器优先使用定义文件，不在业务代码中硬编码站点规则。
9. 不将索引器特有字段直接暴露给前端。
10. 错误必须隔离到单个索引器。
11. 数据库变更必须使用迁移。
12. 核心功能必须配套测试。
13. 所有用户可见文本使用统一文案文件管理。
14. 不保存搜索结果和下载链接，除非后续规格明确要求。
15. UI 始终保持两项主功能：搜索、索引器。

---

## 31. Definition of Done

一个功能只有同时满足以下条件才算完成：

- 代码已实现。
- 代码格式化和静态检查通过。
- 单元测试通过。
- 相关集成测试通过。
- 错误路径已处理。
- 不破坏其他索引器搜索。
- UI 有加载、成功、空和错误状态。
- 接口文档已更新。
- 数据库迁移已提供。
- Windows 构建成功。
- 未新增本规格范围之外的主功能。

---

## 32. MVP 最终形态

### 32.1 用户看到的主导航

```text
搜索
索引器
```

### 32.2 用户首次启动

```text
尚未添加索引器
添加公开索引器后即可开始搜索。
[添加公开索引器]
```

### 32.3 用户完成搜索

```text
关键词：Example
已搜索 8 个索引器
5 个成功 · 1 个空结果 · 1 个超时 · 1 个失败
原始结果 96 条 · 合并后 48 条
```

### 32.4 结果操作

每条结果只提供必要动作：

```text
[复制下载链接] [打开] [详情]
```

### 32.5 索引器操作

每个索引器只提供必要动作：

```text
[启用/停用] [测试] [删除]
```

本项目的最终判断标准不是“支持多少复杂功能”，而是普通用户能否在不了解 Torznab、HTML 选择器、API 参数和站点协议的情况下，完成添加索引器、输入关键词、获得资源结果并使用下载入口的完整流程。
