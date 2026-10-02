# AgentBell

AgentBell is a small, local-only macOS notification bridge for Claude Code and Codex CLI.

它把 coding agent 的完成、授权请求、等待输入和失败事件转成清晰的 macOS 原生通知，帮助你在多个终端会话之间快速定位需要处理的任务。

## 安装

### Homebrew（推荐）

使用独立的 Homebrew tap，安装方式是：

```bash
brew tap Han1enG/agentbell
brew trust --formula Han1enG/agentbell/agentbell
brew install agentbell
agentbell install
agentbell doctor
```

Homebrew 7 默认要求用户显式信任第三方 tap 的 Formula；上面的 `brew trust` 只信任 AgentBell 这一项，不会信任整个 tap。

### GitHub Release

从 [Releases](https://github.com/Han1enG/agent-bell/releases) 下载与你的 Mac 架构匹配的 `.tar.gz`，解压后将 `AgentBell.app` 放到 `~/Applications`，再将其中 `Contents/MacOS/agentbell` 放到 PATH 中，然后运行：

```bash
agentbell install
agentbell doctor
```

`install` 会添加 AgentBell 自己的 Claude/Codex hook；检测到 Tabby 时，默认自动安装随二进制打包的返回原 Tab 集成，无需下载插件或手动 export。安装不会重启 Tabby，下次启动后请在新建的本地 Tab 中运行 agent。重复执行会更新 AgentBell 管理的集成，保留其他插件和用户修改。

```bash
agentbell install --dry-run    # 预览，不修改文件
agentbell install --skip-tabby # 跳过 Tabby 集成安装/更新，记住偏好
agentbell install --tabby      # 重新启用或显式安装 Tabby 集成
```

`--skip-tabby` 不移除已安装的插件。安装偏好保存在 `~/.config/agentbell/integrations.json`。

## 支持的事件

| 客户端 | 事件 | 通知含义 |
| --- | --- | --- |
| Claude Code | `Notification` (`agent_completed`) | 任务完成 |
| Claude Code | `Notification` (`agent_needs_input`) | 等待输入 |
| Claude Code | `PermissionRequest` | 收到授权请求；打开 Claude Code 确认是否仍需处理 |
| Claude Code | `Stop` | 任务完成 |
| Claude Code | `StopFailure` | 执行失败 |
| Codex | `PermissionRequest` | 收到授权请求；打开 Codex 确认是否仍需处理 |
| Codex | `Stop` | 任务完成；优先使用 `last_assistant_message` 作为摘要 |

通知标题使用项目名，正文使用 agent 提供的摘要，并由 macOS Notification Center 控制展示样式。自动任务的 `<heartbeat>` 结构只显示其中的 `message` 正文，隐藏 automation ID 和控制字段；无有效正文时使用事件默认提示。授权请求提醒表示 Hook 收到了请求；Codex 或 Claude Code 可能已自动处理，因此请检查客户端确认是否仍需操作。授权请求在同一 session 内一分钟最多提醒一次。

## 卸载

```bash
agentbell uninstall
```

卸载删除 AgentBell 添加的 hook、自己安装的 launcher/native helper，以及带管理标记的 Tabby 集成文件。其他插件、额外文件和用户修改过的插件文件会保留；Homebrew 管理的 app 仍由 Homebrew 卸载。Tabby 下次启动后停止加载已移除的集成。

## 隐私

- AgentBell 在本机运行，不启动后台服务，也不上传数据。
- Hook JSON 从 stdin 读取，解析后只用于生成本机通知。
- 通知内容可能包含 agent 摘要，并会按 macOS 行为出现在本机 Notification Center。
- AgentBell 不需要云端账号、API key 或网络连接才能工作。

## 配置

配置可选，默认通知全部事件。点击优先返回来源 App；只有来源不可用时才打开项目。文件不存在时无需初始化：

```toml
[notifications]
done = true
needs_input = true
needs_approval = true
error = true

[return]
enabled = true
fallback_app = "auto" # 本机偏好可设置为 "tabby"
```

保存到 `~/.config/agentbell/config.toml`。格式无效时 hook 会记入本地日志并使用默认值，`agentbell doctor` 会报告配置错误。

## 安装检查

```bash
agentbell install --dry-run
agentbell doctor
agentbell doctor --fix
```

`doctor --fix` 会重新向 macOS 注册原生通知应用，并修复已经存在 AgentBell hook 的客户端配置；首次安装仍使用 `agentbell install`。Tabby 集成状态包含 detected、installed、managed 和 bundled current；`doctor --fix` 可更新/修复已启用的受管集成，不会首次安装未启用的集成。

普通通知会按来源、session、事件类型在 3 秒内去重；授权请求通知在同一 session 内一分钟去重一次。

## 当前限制

遗漏与待解决事项见 [KNOWN_ISSUES.md](docs/KNOWN_ISSUES.md)，包括 Codex 自动审查导致的审批通知误报。

- 目前只支持 macOS；Claude Code 和 Codex 的 hook 配置需要由当前用户可读写。
- Codex 仅使用官方稳定的 `PermissionRequest` 和 `Stop` 事件；不依赖 `Elicitation` 或 `StopFailure`。
- 点击通知和显式按钮执行相同返回动作：Exact → Window → 来源 App → 项目。Exact/Window 需要对应 provider，缺少字段或上下文失效会降级。
- native helper 使用 macOS `UserNotifications.framework`。首次发通知时 macOS 会请求通知权限。
- 本地更新需要替换完整的 `AgentBell.app`（包括签名和 `Info.plist`），然后运行 `agentbell doctor --fix`。只替换 Go 主程序无法更新通知点击功能。发布包对完整应用做本地签名，安装时向 LaunchServices 注册点击入口。
- 当前没有菜单栏、Dashboard、远程通知、历史查询或多机器同步功能。

## 本地开发

```bash
go test ./...
go run . doctor
go run . test
```

Homebrew tap 源码位于 [Han1enG/homebrew-agentbell](https://github.com/Han1enG/homebrew-agentbell)。

构建 macOS arm64 和 amd64 `.tar.gz` 发布包及 SHA256（需要 macOS SDK）：

```bash
./scripts/build-macos.sh
```

## v0.2.1 — Return to Context

AgentBell takes you back to where your agent needs you.

Exact session return depends on the capabilities of your terminal or editor. AgentBell always falls back gracefully to the originating app or project.

Works with any terminal at a basic level, with enhanced return-to-session support for selected surfaces.

来源通过 hook 进程祖先的 `.app` 和 `CFBundleIdentifier` 检测；已验证的 Tabby `TERM_PROGRAM` 可提前识别来源。不会用当前前台 App 代替来源。未知来源只保留 cwd。配置中的 `fallback_app` 仅用于项目降级，旧 `[terminal] app` 仍作为兼容配置读取。

`AGENTBELL_SURFACE` / `AGENTBELL_CONTEXT_ID` 是通用协议。ContextID 原样交给对应 provider；单有 ID 不代表 Exact 能力，必须确认桥接中仍有该上下文。完整 cwd 仅在内部 payload 中携带。

通知使用 `UNNotificationAction`，显示「返回会话」「返回窗口」「打开 App」或「打开项目」。macOS 可能在展开通知或悬停后显示 action。开发环境缺少 native helper 的 osascript 路径只支持基础通知，不支持点击和 CTA；发布包包含 helper。

| Surface | Open App | Window | Exact Session |
|---|---|---|---|
| Tabby | ✓ | — | ✓ 新本地 Tab 已实测 |
| Terminal.app | ✓ | — | 原 TTY + 会话进程校验；点击需自动化权限 |
| GoLand 2025.3 | ✓ | — | ✓ 插件加载后的新本地 Tab |
| iTerm2 | ✓ 祖先 bundle 检测 | planned | planned |
| 其他 terminal/editor | best effort bundle 检测 | — | — |
| tmux | 依赖可识别的来源 App | — | planned |
| Generic cwd | — | — | 打开项目 fallback |

Tabby PoC 源码和验证步骤见 [agentbell-tabby](integrations/agentbell-tabby/README.md)。它只负责临时 ContextID、focus 和新本地 Tab 的协议环境变量，不管理 agent、hook 或通知。没有插件时仍可返回 Tabby App。本机已安装插件，并实测新本地 Tab 的自动环境注入和通知点击返回原 Tab。恢复的旧 PTY/旧 agent 不会自动获得变量，应新建本地 Tab 后再启动 agent；SSH、分屏 pane 和重启后的 Context 恢复尚未验收。

`agentbell surface detect` 输出检测目标，`doctor` 输出 Surface Integration 与能力。缺少 Exact 不导致 doctor 失败。检测结果同时写入现有 debug 日志。

### macOS Terminal.app 返回原 Tab

Terminal.app 无需插件。AgentBell 从 hook 的进程祖先记录原 TTY 和持有该 TTY 的会话进程身份；点击通知时校验进程 PID、启动时间和 TTY，再通过 Terminal 的官方脚本字典选中匹配的 Tab 并激活窗口。不会按 cwd 猜 Tab、执行终端命令或创建会话。检测阶段不发送 Apple event。

首次点击可能出现 macOS 自动化授权，请允许 AgentBell 控制 Terminal 以启用选中 Tab。拒绝授权、会话已关闭或 TTY/进程被复用时，降级为打开来源 App。此 provider 不精确定位 tmux pane；tmux 内保持 App fallback。新版本只对更新后产生的通知携带完整目标；旧通知仍保留旧能力。

### GoLand 内置终端返回会话

GoLand 2025.3（build 253）可通过随 AgentBell 提供的 JetBrains 插件定位原终端
Tab。`agentbell install` 检测到受支持版本后自动安装，不下载依赖、不重启 IDE。
安装后重启 GoLand，并新建本地终端 Tab；旧会话未注入标识，只能激活应用。
Classic 与 Reworked 引擎均已接入启动信息和 Tab 选择接口；本机用户已确认
GoLand 通知返回可用，两种引擎的完整场景仍需分别验证。

`agentbell install --skip-goland` 记住跳过偏好，`--goland` 重新启用。插件文件受
所有权和 hash 保护，卸载保留其他插件及用户修改。SSH、远程 IDE、tmux pane
及其他 IDE build 暂无经过验证的精确定位能力。
