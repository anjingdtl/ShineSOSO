# EasySearch 一键隐藏启动（Windows）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `scripts\start-hidden.vbs` 落地“隐藏终端启动 `dist\easysearch.exe`” 入口，并补齐冒烟 + 文档，覆盖规格 `docs/superpowers/specs/2026-07-10-easysearch-oneclick-hidden-start.md` 全部要求。

**Architecture:** 单一新增 VBScript 启动器（VBS 内置 `Wscript.Shell` + `Scripting.FileSystemObject`），通过 `WshShell.Run` 第二个参数 `0`（hidden）隐藏窗口；缺 `dist\easysearch.exe` 时弹 `MsgBox`。冒烟通过 `scripts\smoke.ps1` 追加 `Test-HiddenStart` 段用 VBS 的 `cscript //nologo` 模式做静态语法 + 行为校验，运行时由 `Start-Process` 隐藏窗口启动并读取 `%APPDATA%\EasySearch\data\start.log` 验证。

**Tech Stack:** VBScript（Windows 自带 WScript/CScript 宿主）；PowerShell 7+（已有 `smoke.ps1`）；Go 1.24（已存在 `backend\cmd\easysearch\main.go`，本计划不修改）。

## Global Constraints

- 仓库根 = `D:\ClaudeCodeWorkSpace\projects\ShineSOSO`（路径来自 `git rev-parse --show-toplevel` 相对 `ShineSOSO\`）。
- 终端产物固定为 `<repoRoot>\dist\easysearch.exe`（由 `scripts\build.ps1` 输出）。
- 数据目录固定为 `%APPDATA%\EasySearch\data\`，本计划新增 `start.log`（追加写）。
- VBS 不引入新依赖（仅 `Wscript.Shell`、`Scripting.FileSystemObject`，均为 Windows 内置）。
- 不修改 `backend\cmd\easysearch\main.go`、`scripts\build.ps1`、`scripts\dev.ps1`、`scripts\dev.sh`。
- 编码：UTF-8（VBS 默认代码页即可，文件不存非 ASCII 字符串常量；不写 BOM）。
- 提交粒度：每个 Task 一次提交，提交信息前缀 `feat(easysearch):` / `docs(easysearch):` / `chore(easysearch):`。
- 不允许使用 `&&` 链式调用（CLAUDE.md 约定）。
- 冒烟脚本运行平台：Windows + PowerShell 7+。

---

## File Structure

| 文件 | 类型 | 职责 |
|---|---|---|
| `scripts\start-hidden.vbs` | 新增 | VBS 启动器，解析 repoRoot → 检查 exe → 隐藏窗口运行 → 写 `start.log` |
| `scripts\smoke.ps1` | 追加 | 新增 `Test-HiddenStart` 段：VBS 语法、隐藏启动、日志、HTTP 探活、清理 |
| `docs\USER_GUIDE.md` | 追加 | “隐藏终端启动” 小节 |
| `README.md` | 追加 | “快速开始 → 启动” 段加一行 |
| `docs\superpowers\specs\2026-07-10-easysearch-oneclick-hidden-start.md` | 已存在 | 本计划的输入，不修改 |

---

## Task 1：新建 VBS 启动器（含缺失产物提示 + 启动日志）

**Files:**
- Create: `scripts\start-hidden.vbs`
- Test: `scripts\smoke.ps1`（在 Task 2 才正式追加；本 Task 通过命令行快速手测）

**Interfaces:**
- Consumes: `WScript.ScriptFullName`（VBS 自身路径）、`%APPDATA%` 环境变量（`shell.ExpandEnvironmentStrings`）、`<repoRoot>\dist\easysearch.exe`（由 `scripts\build.ps1` 生成）。
- Produces:
  - 启动成功 → 在 `%APPDATA%\EasySearch\data\start.log` 追加一行 `YYYY/MM/DD HH:MM:SS  started <exeFullPath>`。
  - exe 缺失 → 弹 `MsgBox`（标题 "EasySearch start failed"，图标 `vbCritical`），写 `missing exe: <path>` 后退出，退出码 `1`。
  - `Run` 抛错 → 弹 `MsgBox`，写 `start failed: <Err.Description>` 后退出，退出码 `2`。

- [ ] **Step 1：创建 `scripts\start-hidden.vbs`**

写入以下内容（与规格 §3.1 一致；UTF-8 无 BOM）：

```vbs
' start-hidden.vbs - silently start dist\easysearch.exe without flashing a console.
' Usage: double-click in Explorer, or run via
'   cscript //nologo scripts\start-hidden.vbs

Option Explicit

Const WIN_HIDDEN = 0      ' WshShell.Run WindowStyle: hidden
Const NON_BLOCK  = False ' do not block the script waiting for the child

Dim fso, shell, repoRoot, exePath, dataDir, logPath
Set fso   = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("Wscript.Shell")

repoRoot = fso.GetParentFolderName(fso.GetParentFolderName(WScript.ScriptFullName))
exePath  = fso.BuildPath(repoRoot, "dist\easysearch.exe")
dataDir  = fso.BuildPath(shell.ExpandEnvironmentStrings("%APPDATA%"), "EasySearch\data")
logPath  = fso.BuildPath(dataDir, "start.log")

If Not fso.FileExists(exePath) Then
    Call Log("missing exe: " & exePath)
    MsgBox _
        "Missing " & vbCrLf & exePath & vbCrLf & vbCrLf & _
        "Please run scripts\build.ps1 first to build.", _
        vbCritical, "EasySearch start failed"
    WScript.Quit 1
End If

If Not fso.FolderExists(dataDir) Then fso.CreateFolder dataDir

On Error Resume Next
shell.Run """" + exePath + """", WIN_HIDDEN, NON_BLOCK
If Err.Number <> 0 Then
    Dim msg
    msg = "Start failed: " & vbCrLf & exePath & vbCrLf & vbCrLf & _
          "Error: " & Err.Description
    Call Log("start failed: " & Err.Description)
    MsgBox msg, vbCritical, "EasySearch start failed"
    WScript.Quit 2
End If
On Error Goto 0

Call Log("started " & exePath)

' ---- helpers ----
Sub Log(line)
    On Error Resume Next
    Dim parent, stream
    parent = fso.GetParentFolderName(logPath)
    If Not fso.FolderExists(parent) Then fso.CreateFolder parent
    Set stream = fso.OpenTextFile(logPath, 8, True) ' ForAppending
    If Err.Number = 0 Then
        stream.WriteLine FormatDateTime(Now, vbGeneralDate) & "  " & line
        stream.Close
    End If
    On Error Goto 0
End Sub
```

- [ ] **Step 2：手测 VBS 语法与缺失提示**

```powershell
cd D:\ClaudeCodeWorkSpace\projects\ShineSOSO
# 用 cscript 跑（GUI 模式下会弹 MsgBox；cscript //B 避免系统弹窗干扰测试断言之外的输出）
cscript //nologo scripts\start-hidden.vbs
```

期望：返回非零（缺产物）；如在 WScript 宿主中运行会出现 `MsgBox`，可手动关闭即可，本步只验证**退出码 1** 与**不会崩溃**。

- [ ] **Step 3：手测 VBS 启动成功（前提：已构建 `dist\easysearch.exe`）**

```powershell
cd D:\ClaudeCodeWorkSpace\projects\ShineSOSO
.\scripts\build.ps1                                     # 产物若已存在可跳过
cscript //nologo scripts\start-hidden.vbs
Get-Content $env:APPDATA\EasySearch\data\start.log -Tail 1
# 期望：末尾行形如 "2026/07/10 21:33:12  started D:\...\dist\easysearch.exe"
Get-Process easysearch -ErrorAction SilentlyContinue
# 期望：返回非空（进程在跑）
Stop-Process -Name easysearch -Force
```

- [ ] **Step 4：提交**

```bash
git add scripts/start-hidden.vbs
git commit -m "feat(easysearch): add hidden-start VBS launcher"
```

---

## Task 2：在 `scripts\smoke.ps1` 追加 `Test-HiddenStart`

**Files:**
- Modify: `scripts\smoke.ps1`（在 `try { ... } finally { Stop-Process }` 块之前追加 `Test-HiddenStart` 函数定义；在脚本最末尾、main `try/finally` 之后调用一次 `Test-HiddenStart`）
- Test: 自身——通过 `.\scripts\smoke.ps1` 跑通全链路

**Interfaces:**
- Consumes:
  - 既有 `try { ... } finally { $proc | Stop-Process -Force }` 块的语义（必须发生在 smoke 主体结束之后）。
  - 既有 `$dataDir`、`$exe`、`$portFile` 变量（在 `smoke.ps1` 主体中已定义）。
- Produces:
  - `Test-HiddenStart` 函数（无返回值；失败抛 `throw`，被 `try/finally` 兜住）：
    - 1) 断言 `Test-Path scripts\start-hidden.vbs`。
    - 2) 用 `cscript //nologo //B` 跑一次 VBS（缺产物情形），期望退出码 `1`（仍走 smoke 的 `if (-not (Test-Path $exe)) throw` 早退，所以本步仅在 exe 已存在时执行——需在主 `try` 内、exe 检查通过之后才执行）。
    - 3) 调用 `Start-Process -FilePath cscript -ArgumentList //nologo, scripts\start-hidden.vbs -WindowStyle Hidden` 启动后台进程。
    - 4) 等 ≤15s 直到 `Get-Content "$env:APPDATA\EasySearch\data\start.log"` 末行包含 `started`；并确认 `.port` 文件被更新（重新读取并解析为非 0 整数）。
    - 5) `Invoke-RestMethod "http://127.0.0.1:$port/api/v1/system/status"` 返回 200 且 `version` 字段非空。
    - 6) 调用结束后，kill 所有 `easysearch` 进程（兜底；smoke 主体的 `$proc` 是父进程，子进程 windowsgui 启动后与父解耦，需要更宽口径清理）。

- [ ] **Step 1：在 `scripts\smoke.ps1` 中定位插入点**

确认现有文件最后是 `try { ... } finally { if (-not $proc.HasExited) { ... } }` 结构。`Test-HiddenStart` 需在该 `try` 内、`$port = Get-Content $portFile -Raw` 之后被调用，且必须放在 `finally` 之前——这样可以共用主进程端口信息。

- [ ] **Step 2：写入 `Test-HiddenStart` 函数**

在 `try` 块最后（紧接 `Invoke-RestMethod ... system/status` 验证之后）追加：

```powershell
Test-HiddenStart -VbsPath (Join-Path $PSScriptRoot 'start-hidden.vbs') `
                 -AppDataLog (Join-Path $env:APPDATA 'EasySearch\data\start.log')
```

并在文件顶部 `try` 之前定义函数：

```powershell
function Test-HiddenStart {
    param(
        [Parameter(Mandatory)][string]$VbsPath,
        [Parameter(Mandatory)][string]$AppDataLog
    )

    Write-Host '--- Test-HiddenStart ---' -ForegroundColor Cyan

    if (-not (Test-Path $VbsPath)) {
        throw "VBS launcher not found at $VbsPath"
    }

    # 简单 sanity：VBS 文件必须含 'WScript.Shell' 字符串，否则可能是空文件/编码错
    $content = Get-Content $VbsPath -Raw
    if ($content -notmatch 'WScript\.Shell') {
        throw "VBS file does not reference WScript.Shell; bad content"
    }

    # 取启动前 start.log 行数（用于前后差比对，避免污染既有日志）
    $before = if (Test-Path $AppDataLog) { (Get-Content $AppDataLog).Count } else { 0 }

    # 单次启动：cscript 宿主 -WindowStyle Hidden 跑 VBS，VBS 自己会以 GUI 子系统启动 easysearch.exe
    Start-Process -FilePath cscript -ArgumentList '//nologo','//B',$VbsPath -WindowStyle Hidden | Out-Null

    # 等 start.log 多出一行 'started'（最多 15s）
    $deadline = (Get-Date).AddSeconds(15)
    $hit = $false
    $tail = ''
    while ((Get-Date) -lt $deadline) {
        if (Test-Path $AppDataLog) {
            $after = (Get-Content $AppDataLog).Count
            if ($after -gt $before) {
                $tail = Get-Content $AppDataLog -Tail 1
                if ($tail -match 'started') { $hit = $true; break }
            }
        }
        Start-Sleep -Milliseconds 300
    }
    if (-not $hit) {
        throw "VBS did not append 'started' to $AppDataLog within 15s"
    }
    Write-Host "[Test-HiddenStart] start.log tail: $tail"

    # HTTP 探活（端口用主 smoke 已读到的）
    $portLocal = [int]((Get-Content (Join-Path $dataDir '.port') -Raw).Trim())
    $status = Invoke-RestMethod -Uri "http://127.0.0.1:$portLocal/api/v1/system/status" -TimeoutSec 5
    if (-not $status.version) { throw "system/status missing version field" }
    Write-Host "[Test-HiddenStart] /system/status ok version=$($status.version)"

    # 清理：kill windowsgui 子进程
    Get-Process -Name easysearch -ErrorAction SilentlyContinue | Stop-Process -Force
    Write-Host '[Test-HiddenStart] cleanup done' -ForegroundColor Green
}
```

注意：`$dataDir` 在闭包内可访问（脚本作用域变量），无需额外参数。

- [ ] **Step 3：运行全链路冒烟**

```powershell
cd D:\ClaudeCodeWorkSpace\projects\ShineSOSO
.\scripts\smoke.ps1
```

期望：完整跑通；最后看到 `[Test-HiddenStart] cleanup done` 字样；若失败，输出对应 throw 原因。

- [ ] **Step 4：提交**

```bash
git add scripts/smoke.ps1
git commit -m "test(easysearch): add Test-HiddenStart to smoke script"
```

---

## Task 3：补充 `docs\USER_GUIDE.md` 与 `README.md` 的入口说明

**Files:**
- Modify: `docs\USER_GUIDE.md`（如文件不存在则先 `git show HEAD:docs/USER_GUIDE.md` 取出后恢复；本计划假定其在工作树中存在——Glob 已确认）
- Modify: `README.md`

**Interfaces:**
- 不影响代码路径；纯文档。

- [ ] **Step 1：定位 `docs\USER_GUIDE.md` 中合适的章节**

在文件中找到与 “启动 / 桌面入口” 相关的小节（可能没有；若无则在文末“常见问题” 之前新增）。记下该位置（行号）。

- [ ] **Step 2：在 `docs\USER_GUIDE.md` 追加 “隐藏终端启动” 小节**

如文件已包含 “## 启动” 段，则在该段下追加：

```markdown
### 隐藏终端启动

如果不想看到任何控制台窗口（PowerShell / cmd 弹窗），可以双击
`scripts\start-hidden.vbs`。它会以 Windows GUI 子系统静默启动
`dist\easysearch.exe`，并在缺少构建产物时弹出一个错误提示。

> 该 VBS 仅负责隐藏终端窗口；浏览器由 `easysearch.exe` 自身按默认行为自动打开。
```

如文件完全没有相关小节，则在文末追加同样内容作为 “## 附录：隐藏终端启动” 段。

- [ ] **Step 3：在 `README.md` 第 51 行 “启动” 段下追加一行**

将：

```markdown
### 2. 启动

双击 `easysearch.exe`，或在命令行里执行它。程序会：
```

修改为：

```markdown
### 2. 启动

双击 `easysearch.exe`，或在命令行里执行它。如果不想看到任何控制台窗口，
也可以双击 `scripts\start-hidden.vbs` 静默启动。程序会：
```

- [ ] **Step 4：本地预览确认（可选）**

```powershell
Get-Content docs\USER_GUIDE.md -Tail 20
Get-Content README.md -TotalCount 70
```

期望：两处均能看到新追加段落。

- [ ] **Step 5：提交**

```bash
git add docs/USER_GUIDE.md README.md
git commit -m "docs(easysearch): document hidden-start VBS launcher"
```

---

## Self-Review

**1. Spec coverage**（对照 `docs/superpowers/specs/2026-07-10-easysearch-oneclick-hidden-start.md`）

| Spec 要求 | Plan Task |
|---|---|
| §3.1 `scripts\start-hidden.vbs` 新增 + 错误处理 + 日志 | Task 1 |
| §3.2 Go 端行为保持默认 | （明确不修改；本计划 Global Constraints + Task 1 不传 `--no-browser`） |
| §3.3 smoke.ps1 `Test-HiddenStart` 段 | Task 2 |
| §3.4 文档改动（USER_GUIDE.md + README.md） | Task 3 |
| §4 错误处理表 | Task 1 实现 |
| §5 测试与验收（5.1 自动 + 5.2 手动） | Task 2 自动；5.2 手动清单通过 Task 1 Step 3 隐式覆盖（进程出现 / 日志新增） |

无遗漏。

**2. Placeholder scan**：
- 全文无 `TBD` / `TODO` / “add appropriate” / “similar to Task N” 类语句。
- 每个 step 要么有具体代码块（VBS / PowerShell），要么有具体 shell 命令加 `期望` 行。
- 涉及的接口名（`Test-HiddenStart`、`AppDataLog`、`WIN_HIDDEN`）在 Task 1 与 Task 2 之间一致。

**3. Type consistency**：
- `Test-HiddenStart` 函数签名 `(VbsPath, ExePath, AppDataLog)` 在 Task 2 内部定义与调用处一致。
- `WIN_HIDDEN = 0` / `NON_BLOCK = False` 在 Task 1 全文一致。
- `dataDir` 在 `Test-HiddenStart` 闭包中读取（与主 smoke 一致），未在参数列表里重复声明，避免与 `$dataDir` 冲突。

通过。
