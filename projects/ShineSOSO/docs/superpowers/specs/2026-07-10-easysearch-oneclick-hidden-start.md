# EasySearch 一键隐藏启动（Windows）

> 状态：草案（v0）
> 作者：brainstorming 阶段产出，待用户审阅后转入 writing-plans
> 日期：2026-07-10
> 关联：`scripts/build.ps1`、`backend/cmd/easysearch/main.go`、`spec-o1.md`

## 1. 背景与目标

EasySearch 通过 `scripts\build.ps1` 编译为单文件 `dist\easysearch.exe`，该 exe 使用
`-H windowsgui` 子系统，本应不弹控制台。但项目当前缺少一个“普通用户双击即用”的
启动入口：

- 用户不熟悉命令行时，需要一个明确的**桌面/资源管理器入口**。
- 启动过程要**完全隐藏**终端窗口（PowerShell、VBS 的解释器窗口都不能闪一下）。
- `dist\easysearch.exe` 不存在时必须给出**明确提示**（不静默失败）。
- 不能破坏现有的 `scripts\dev.ps1`（开发模式）和 `scripts\smoke.ps1`（冒烟）。

**目标**：提供一个 `scripts\start-hidden.vbs`，双击即在后台以默认参数启动
`dist\easysearch.exe`；缺产物时弹 `MsgBox` 提示。

## 2. 范围

### 在范围内
- 新增 `scripts\start-hidden.vbs`。
- 在 `scripts\smoke.ps1` 末尾追加 `Test-HiddenStart` 段。
- 在 `docs\USER_GUIDE.md` 增补“隐藏终端启动”小节。
- 在 `README.md` “快速开始” 段加一行指向该入口。

### 不在范围内（YAGNI）
- 系统托盘 / 关闭按钮。
- 开机自启 / 计划任务。
- 自动调用 `scripts\build.ps1`。
- 修改 Go 代码或 `build.ps1`。
- 单实例检测 / 文件锁。

## 3. 设计

### 3.1 入口文件 `scripts\start-hidden.vbs`

```vbs
' start-hidden.vbs — silently start dist\easysearch.exe without flashing a console.
' Usage: double-click in Explorer, or run via `cscript //nologo scripts\start-hidden.vbs`.

Option Explicit

Const WIN_HIDDEN = 0      ' WshShell.Run WindowStyle: hidden
Const NON_BLOCK  = False ' do not block the script waiting for the child

Dim fso, shell, repoRoot, exePath, logPath
Set fso   = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("Wscript.Shell")

repoRoot = fso.GetParentFolderName(fso.GetParentFolderName(WScript.ScriptFullName))
exePath  = fso.BuildPath(repoRoot, "dist\easysearch.exe")
logPath  = fso.BuildPath( _
    shell.ExpandEnvironmentStrings("%APPDATA%"), _
    "EasySearch\data\start.log")

If Not fso.FileExists(exePath) Then
    Call Log("missing exe: " & exePath)
    MsgBox _
        "Missing " & vbCrLf & exePath & vbCrLf & vbCrLf & _
        "Please run scripts\build.ps1 first to build.", _
        vbCritical, "EasySearch start failed"
    WScript.Quit 1
End If

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
    Dim ts, tsFile
    ts     = Now
    tsFile = fso.BuildPath(fso.GetParentFolderName(logPath), "start.log")
    If Not fso.FolderExists(fso.GetParentFolderName(logPath)) Then
        fso.CreateFolder fso.GetParentFolderName(logPath)
    End If
    Dim stream
    Set stream = fso.OpenTextFile(logPath, 8, True) ' ForAppending
    stream.WriteLine FormatDateTime(ts, vbGeneralDate) & "  " & line
    stream.Close
End Sub
```

设计要点：
- `Run ... , 0, False` 是隐藏启动的关键；`0` 表示 `WindowStyle = Hidden`，
  `False` 让 VBS 不等待子进程（避免用户看到“已启动”窗口停留）。
- 缺 exe 时用 `MsgBox` 而不是 `Wscript.Echo`，因为 VBS 走 GUI 子系统时
  `Wscript.Echo` 走 WScript 弹窗，CScript 模式才走控制台——双击场景下用
  `MsgBox` 是最稳的 GUI 提示。
- `logPath` 始终指向 `%APPDATA%\EasySearch\data\start.log`（与 Go 端数据
  目录一致），便于用户/脚本查询最近启动记录。

### 3.2 Go 端行为保持默认

不修改 `main.go`：默认 `cfg.OpenBrowser = true`，双击 VBS 之后 Go 端会自己
调 `launcher.OpenURL(url)` 打开浏览器。**VBS 只负责隐藏终端窗口**。

### 3.3 冒烟脚本扩展

在 `scripts\smoke.ps1` 末尾追加 `Test-HiddenStart`：

1. 验证 `scripts\start-hidden.vbs` 文件存在。
2. 验证 VBS 语法：用 `cscript //nologo //B` 跑一次，期望返回非零且提示缺 exe
   （因为 smoke 用临时 dataDir 而不是默认 `%APPDATA%`，仍依赖 `dist\easysearch.exe`）。
3. 用 `Start-Process` 启动 VBS，`-WindowStyle Hidden`。
4. 等待 `Get-Content %APPDATA%\EasySearch\data\start.log` 出现新行（含日期）。
5. 通过 `.port` 文件 + `GET /api/v1/system/status` 确认进程健康。
6. `Stop-Process` 清理。

**前提**：`scripts\smoke.ps1` 现有的 `dist\easysearch.exe` 检查已在最前面跑过；
若尚未构建，smoke 会提前退出，本段不执行。

### 3.4 文档改动

- `docs\USER_GUIDE.md` 新增 “隐藏终端启动” 小节，1 段说明 + 1 句双击
  `scripts\start-hidden.vbs` 即可。
- `README.md` “快速开始” 第 2 步“启动” 段增加：
  > 也可双击 `scripts\start-hidden.vbs` 静默启动（不弹终端窗口）。
  > 双击后浏览器会自动打开 WebUI。

## 4. 错误处理

| 场景 | VBS 行为 |
|---|---|
| `dist\easysearch.exe` 缺失 | `MsgBox` 提示构建，退出码 1 |
| `Run` 失败（路径权限/特殊字符） | `MsgBox` 提示错误，退出码 2 |
| 日志文件无法写入（权限/盘满） | 静默忽略日志，不影响启动 |
| 已经有一个实例在跑 | 不检测；Go 端会因端口冲突快速失败，错误进 `easysearch.log` |

不做单实例锁的原因：双击 VBS 重启一次也是合理的（更新/卡顿场景），让 Go
端日志自己说清楚即可。

## 5. 测试与验收

### 5.1 自动验收（冒烟）
- `.\scripts\smoke.ps1` 全程通过 = 隐含 `Test-HiddenStart` 段通过。
- 手工 spot-check：`cscript //nologo scripts\start-hidden.vbs` 在缺产物时
  弹 `MsgBox`；构建后无窗口启动并写日志。

### 5.2 手动验收清单
- [ ] 双击 VBS：终端 0 闪动，浏览器由 Go 端默认自动打开 WebUI。
- [ ] 任务管理器出现 `easysearch.exe`，无可见窗口。
- [ ] 关闭浏览器后再次双击 VBS，进程不重复。
- [ ] 关闭 `easysearch.exe` 后 `start.log` 仍保留历史行。
- [ ] 删除 `dist\easysearch.exe` 后双击 VBS：`MsgBox` 弹出。

## 6. 风险与回滚

- **风险**：某些杀软对 `.vbs` 走 WScript 的拦截会触发 SmartScreen 提示。
  - **缓解**：纯文本、无网络操作、无注册表写入，路径仅在 `dist\easysearch.exe`
    上；提示是“该应用想使用脚本宿主”的标准 Windows 行为，不影响功能。
- **风险**：`%APPDATA%\EasySearch\data\start.log` 无限增长。
  - **缓解**：Go 端 `internal/logging` 已有 lumberjack 轮转；但 VBS 写的是
    自己的文件，每次启动写 1 行，体量极小，可忽略。VBS 不引入轮转逻辑（YAGNI）。
- **规格偏差**：§3.1 的 VBS `MsgBox` 文本使用英文 ASCII，而不是最初草案中的中文；
  这样可保持 UTF-8 无 BOM 文件在所有 Windows 代码页下都能解析。
- **回滚**：删除 `scripts\start-hidden.vbs`、撤销 `smoke.ps1` / `README.md` /
  `docs\USER_GUIDE.md` 的对应小段即可。

## 7. 文件改动清单

| 文件 | 类型 | 大致行数 |
|---|---|---|
| `scripts\start-hidden.vbs` | 新增 | ~50 |
| `scripts\smoke.ps1` | 追加 | +30 |
| `docs\USER_GUIDE.md` | 追加 | +10 |
| `README.md` | 追加 | +2 |
