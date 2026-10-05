# AgentBell

**AgentBell tells you when your coding agent needs you — and takes you back to the right session.**

The Attention Center gives you one quiet place to see which agents need you, which are still working, and what just finished.

Works across terminals, editors, and multiplexers with graceful fallback when exact return isn't available.

v0.4 is an Attention Center release candidate. Real multi-agent Tabby/GoLand UI and notification-click acceptance remain required before calling it stable.

## Attention Center

`AgentBell.app` is a native SwiftUI menu bar app, with no Dock icon. It owns an embedded Go core and a private local Unix socket; there is no separately installed daemon. Sessions appear only in **NEEDS YOU**, **WORKING**, and **RECENT**. The badge counts distinct sessions with input, confirmed approval, or error attention. Working and completed sessions never count. Clicking Return uses the existing Universal Return providers and never clears attention; only reliable resumed work or completion events clear a wait.

`agentbell install` copies the bundled app to `~/Applications`, preserves config, updates owned hooks, and launches the app. Login launch defaults to enabled via an AgentBell-owned LaunchAgent that opens the app, with no keep-alive daemon. Existing running AgentBell is asked to quit gracefully during upgrade so state can be flushed. Other apps and agents are untouched.

```bash
agentbell status           # NEEDS YOU / WORKING / RECENT
agentbell status --json    # pure JSON, schema_version=1
agentbell version         # CLI / installed or running App / Protocol
agentbell version --short # CLI version only
agentbell logs            # latest 100 lines
agentbell logs --follow
```

The menu provides Pause/Resume Notifications, Clear Recent, Open Config, and Quit. Pause persists across app restarts and affects only system notifications. Clear Recent removes completed sessions; unresolved errors and active sessions remain. Recent initially shows five sessions and offers Show More.

Session state uses SQLite schema v1 at `~/Library/Application Support/AgentBell/agentbell.db`, with seven-day completed/stale retention and a transition journal capped at 200 entries. Metadata is coalesced into snapshots; tool calls are never saved as history. Directory permissions are 0700, database and socket permissions are 0600. Events contain no raw payload, prompt, environment, tool arguments, or output. Short summaries use the same 180-character filter as notifications.

Hooks update state before notification configuration, debounce or pause. IPC has a 40ms total deadline; a missing, incompatible or wedged app falls back to the existing native notification path. `status` reads the app first, then opens SQLite read-only without launching a GUI. Storage failure leaves live in-memory tracking and notifications available.

Working and waiting sessions are reconciled at startup and every minute using pinned agent process generations or confirmed expired local surface contexts. Inconclusive probe failures preserve state. Sessions without reliable process identity expire after 24 hours without activity. Stale sessions are archived as unknown and never fabricated as done. Process discovery inspects executable ancestry, never terminal content.

Codex `PermissionRequest` is observational even when notification opt-in is enabled. Claude's pre-routing `PermissionRequest` notification also does not create attention; `Notification/permission_prompt` confirms an actual wait. Codex currently exposes no official needs-input hook, so its input state requires a reliable explicitly supplied event; request hooks cannot substitute for that signal. Hook contracts are documented by [Claude](https://code.claude.com/docs/en/hooks) and [Codex](https://developers.openai.com/codex/hooks).

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
| Claude Code | `Notification` (`agent_needs_input`, `idle_prompt`, `elicitation_dialog`) | 等待输入 |
| Claude Code | `Notification` (`permission_prompt`) | 明确等待人工授权 |
| Claude Code / Codex | `SessionStart`, `UserPromptSubmit` | 更新为 WORKING；不弹通知 |
| Claude Code / Codex | `PreToolUse`, `PostToolUse` | 刷新活跃时间；不保存工具历史 |
| Claude Code | `PermissionRequest` | 收到授权请求；打开 Claude Code 确认是否仍需处理 |
| Claude Code | `Stop` | 任务完成 |
| Claude Code | `StopFailure` | 执行失败 |
| Codex | `PermissionRequest` → `permission_request` | Experimental，默认关闭；开启后仅显示 “Permission requested”，可返回会话 |
| Codex | `Stop` | 任务完成；优先使用 `last_assistant_message` 作为摘要 |

通知标题使用项目名，正文使用 agent 提供的摘要，并由 macOS Notification Center 控制展示样式。自动任务的 `<heartbeat>` 结构只显示其中的 `message` 正文，隐藏 automation ID 和控制字段；无有效正文时使用事件默认提示。Claude 授权请求提醒表示 Hook 收到了请求，客户端可能已经处理。Codex 权限请求不是等待人工审批的证据；实验通知使用固定中性文案，不展示工具输入中的审批摘要。

## 卸载

```bash
agentbell uninstall
```

卸载删除 AgentBell 添加的 hook、自己安装的 launcher/native helper，以及带管理标记的 Tabby 集成文件。其他插件、额外文件和用户修改过的插件文件会保留；Homebrew 管理的 app 仍由 Homebrew 卸载。Tabby 下次启动后停止加载已移除的集成。

## 隐私

- AgentBell 在本机运行；菜单栏 App 托管状态核心，不安装独立 daemon，也不上传数据。
- AgentBell does not upload session state, project paths, or notification history.
- Hook JSON 从 stdin 读取，解析后用于本地会话状态和通知；原始 JSON 不保存。
- 通知内容可能包含 agent 摘要，并会按 macOS 行为出现在本机 Notification Center。
- AgentBell 不需要云端账号、API key 或网络连接才能工作。

## 配置

配置可选，Codex 权限请求实验通知默认关闭，其他事件保持默认开启。点击优先返回来源 App；只有来源不可用时才打开项目。文件不存在时无需初始化：

```toml
[attention_center]
enabled = true # false 保留 v0.3 notification-only 路径
launch_at_login = true
retention_days = 7
recent_limit = 5
done_notifications = false # While App runs, Done updates RECENT/dot without a banner.

[notifications]
done = true
needs_input = true
needs_approval = true # Claude 审批事件
permission_request = false # Experimental Codex 请求提示；显式 true 才开启
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

普通通知及实验权限请求按来源、session、事件类型在 3 秒内去重；Claude 审批事件在同一 session 内一分钟去重一次。关闭 Codex 实验通知通过事件分类和配置实现，不依赖 debounce。

### Codex capability matrix

| 能力 | v0.2.2 状态 |
| --- | --- |
| Stop / 完成摘要 | 支持，默认通知 |
| PermissionRequest 捕获 | 支持，独立 `EventPermissionRequest`，不映射 `NeedsApproval` |
| 权限请求提示 | Experimental，默认关闭，`notifications.permission_request = true` 显式开启 |
| 可靠人工审批等待检测 | 不支持：上游 hook 在审批路由前触发，缺少最终人工等待信号 |
| 实验提示 CTA | 支持现有 Return-to-Context；目标失效时安全降级 |

`doctor` 明确输出此 upstream limitation。开启实验提示不能保证请求尚待处理，也不能保证真正需要人工操作时必有提醒。

## 当前限制

限制见 [KNOWN_ISSUES.md](docs/KNOWN_ISSUES.md)，验收证据见 [Stabilization 报告](docs/STABILIZATION_V022.md)。v0.2.2 将 Codex 请求提示与真实审批语义分离；可靠人工等待检测作为上游限制保留。Classic 和尚未完成的真实 GUI 矩阵属于 documented limitations，不作为本次 release blocker。

- GoLand Classic、新插件完整 Reworked 矩阵、多 project window/IDE restart，以及 Tabby/Terminal 完整真实 GUI 矩阵尚未全部验收；已有编译、IPC、回归和局部真实检查不能替代完整 GUI 验收。
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

### Return Support

Basic return works broadly. Exact return depends on the capabilities of each
terminal, editor, or multiplexer.

- **Basic**: originating app, then validated project fallback.
- **Enhanced**: existing window/tab when that provider can verify it.
- **Exact**: the original terminal/pane/session with a live ID and safe focus.

The table describes implemented provider contracts, not completed GUI acceptance.
All exact capabilities are conditional on live identity, permission and attachment.
See [v0.3 validation and remaining GUI checks](docs/UNIVERSAL_RETURN_V03.md).

<!-- agentbell-capabilities:start -->
| Provider | Maturity | App | Window | Exact | Project | Validation |
|---|---|---|---|---|---|---|
| tmux | stable | — | — | ✓ | — | real tmux CLI; composite GUI pending |
| jetbrains | stable | ✓ | — | ✓ | — | GoLand 2025.3 / build 253; bridge regression |
| tabby | stable | ✓ | — | ✓ | — | regression tested; v0.3 GUI closure pending |
| iterm | experimental | ✓ | — | ✓ | — | fixture only; real GUI pending |
| wezterm | experimental | ✓ | — | ✓ | — | fixture only; real GUI pending |
| terminal | stable | ✓ | — | ✓ | — | regression tested; v0.3 GUI closure pending |
| generic | stable | ✓ | — | — | ✓ | regression tested; v0.3 GUI closure pending |
<!-- agentbell-capabilities:end -->

This block is generated by `go run ./scripts/capability-matrix` and checked in Go
tests against `Capabilities()`. Ghostty currently has basic ancestry return only.
No standalone window capability is advertised without a safe window identity.

### v0.3 — Universal Return

```bash
agentbell surfaces
agentbell surfaces --json
agentbell doctor
agentbell surface probe <provider> <context-id>
agentbell return --current  # development diagnostic; activates the current target
```

`surfaces` distinguishes installed providers, providers detected in this shell,
and live read-only context availability. `--json` emits only schema-versioned JSON
on stdout (`schema_version`, `current`, `providers`). Errors stay on stderr.
Doctor adds Return Stack and Surface Integrations alongside installation health.

Composite return activates the outer app, verifies the original outer terminal
against the attached tmux client TTY, restores the outer context and selects the
saved tmux window/pane. Stable IDs, socket generation, server PID/start time and
client identity are rechecked on click. Closed panes, restart ID reuse, detached
clients and ambiguous attachments fall back without guessing by cwd or names.
Terminal, the updated Tabby bridge and iTerm can provide the required TTY binding.
WezTerm + tmux uses the official CLI `tty_name` field for binding; versions
without this field and basic-only outer applications fall back to app. tmux itself must be installed by the user.

The Context Protocol is public; providers are trusted and registered at compile
time. There is no dynamic plugin loader. See [Context Protocol v1](docs/context-protocol.md)
and [the provider example](docs/provider-example/README.md).

Upgrade with `brew upgrade agentbell` then `agentbell install`; bundled managed
integration upgrades retain modified-plugin protection. iTerm and WezTerm use
native APIs and do not install user shell scripts.


Tabby PoC 源码和验证步骤见 [agentbell-tabby](integrations/agentbell-tabby/README.md)。它只负责临时 ContextID、focus 和新本地 Tab 的协议环境变量，不管理 agent、hook 或通知。没有插件时仍可返回 Tabby App。本机已安装插件，并实测新本地 Tab 的自动环境注入和通知点击返回原 Tab。恢复的旧 PTY/旧 agent 不会自动获得变量，应新建本地 Tab 后再启动 agent；SSH、分屏 pane 和重启后的 Context 恢复尚未验收。

`agentbell surface detect` 输出检测目标。`doctor` 分别输出 Current Session 和 Integration Health：插件已安装、内容版本匹配、桥接可达，均不代表当前 shell 已接入。Exact 必须通过只读 Provider Probe；缺少 ID、旧会话或失效桥接不会导致健康安装的 doctor 失败。Tabby/GoLand 旧 Tab 应在重启 App 后新建本地 Tab。

`agentbell surface probe <provider> <context-id>` 只验证目标，不切换 Tab。Terminal 的 doctor 检查先使用 native helper 查询自动化权限（不请求授权），已授权后读取窗口/Tab；未授权或无法确认时报告不可用。点击通知仍可申请正常的自动化授权并在失败后返回 App。返回日志记录 `surface`、`provider`、`capability`、`result` 和标准 `reason`，不记录 Prompt。

### macOS Terminal.app 返回原 Tab

Terminal.app 无需插件。AgentBell 从 hook 的进程祖先记录原 TTY 和持有该 TTY 的会话进程身份；点击通知时校验进程 PID、启动时间和 TTY，再通过 Terminal 的官方脚本字典选中匹配的 Tab 并激活窗口。不会按 cwd 猜 Tab、执行终端命令或创建会话。检测阶段不发送 Apple event。

首次点击可能出现 macOS 自动化授权，请允许 AgentBell 控制 Terminal 以启用选中 Tab。拒绝授权、会话已关闭或 TTY/进程被复用时，降级为打开来源 App。v0.3 在唯一 attached client 和 TTY 绑定可验证时支持 Terminal + tmux exact pane；否则保持 App fallback。新版本只对更新后产生的通知携带完整目标；旧通知仍保留旧能力。

### GoLand 内置终端返回会话

GoLand 2025.3（build 253）可通过随 AgentBell 提供的 JetBrains 插件定位原终端
Tab。`agentbell install` 检测到受支持版本后自动安装，不下载依赖、不重启 IDE。
安装后重启 GoLand，并新建本地终端 Tab；旧会话未注入标识，只能激活应用。
Classic 与 Reworked 引擎均已接入启动信息和 Tab 选择接口；本机用户已确认
GoLand 通知返回可用，两种引擎的完整场景仍需分别验证。

`agentbell install --skip-goland` 记住跳过偏好，`--goland` 重新启用。插件文件受
所有权和 hash 保护，卸载保留其他插件及用户修改。SSH、远程 IDE、tmux pane
及其他 IDE build 暂无经过验证的精确定位能力。

### Universal layered return (v0.3)

Universal Return understands terminals, editors, and multiplexers as layered work surfaces.

Exact-return support varies by environment. AgentBell falls back safely instead of guessing.

AgentBell combines the originating GUI surface with a tmux pane. On a notification click it validates the stored outer context, attached client and pane before selecting them. If an exact target expires or cannot be validated, it safely falls back to the originating app or project; it never guesses a replacement from its title or directory.

iTerm2 and WezTerm providers are **Experimental**, validated with fixtures only. Their declared exact capability describes the implementation contract and does not imply real GUI validation. Real Tabby+tmux and Terminal+tmux notification-click GUI E2E and stale-notification GUI fallback remain unvalidated. v0.3 is released with these explicitly documented limitations; CLI tests do not replace GUI acceptance.

Installation configures hooks; it does not prove that running agents loaded them.
Restart running Codex after installation or a hook-path change, and use a fresh
Claude Code session if it predates installation. Newly installed/upgraded Tabby
or GoLand integrations require restarting the host. The Agent Connections menu
reports real events received during the current App run; restored history does
not count as a fresh connection. Installed hooks use the stable Applications
bundle path so ordinary App replacements do not require a new hook path.

RECENT rows can be removed individually with ×. This preserves other completed
and active sessions; Clear Recent still removes all completions. While the App
runs, completion banners are off by default; set
`attention_center.done_notifications = true` to opt in. When the App is absent,
the existing `notifications.done` fallback setting still applies. App-level
return is labeled Open App; exact-session return remains Return.
