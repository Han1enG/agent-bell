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

`install` 只会在 Claude Code 的 `~/.claude/settings.json` 和 Codex 的 `~/.codex/hooks.json` 中添加 AgentBell 自己的 hook 命令，并且可以重复执行。

## 支持的事件

| 客户端 | 事件 | 通知含义 |
| --- | --- | --- |
| Claude Code | `Notification` (`agent_completed`) | 任务完成 |
| Claude Code | `Notification` (`agent_needs_input`) | 等待输入 |
| Claude Code | `PermissionRequest` | 收到授权请求；打开 Claude Code 确认是否仍需处理 |
| Claude Code | `Stop` | 任务完成 |
| Claude Code | `StopFailure` | 执行失败 |
| Codex | `PermissionRequest` | 默认关闭；可选的信息提醒，不代表需要人工审批 |
| Codex | `Stop` | 任务完成；优先使用 `last_assistant_message` 作为摘要 |

通知标题使用项目名，正文使用 agent 提供的摘要，并由 macOS Notification Center 控制展示样式。自动任务的 `<heartbeat>` 结构只显示其中的 `message` 正文，隐藏 automation ID 和控制字段；无有效正文时使用事件默认提示。授权请求提醒表示 Hook 收到了请求；Codex 或 Claude Code 可能已自动处理，因此请检查客户端确认是否仍需操作。授权请求在同一 session 内一分钟最多提醒一次。

## 卸载

```bash
agentbell uninstall
```

卸载只删除 AgentBell 添加的 hook 项，并清理 AgentBell 自己安装的 launcher 和 native helper；不会删除或重写其他 hook 配置。

## 隐私

- AgentBell 在本机运行，不启动后台服务，也不上传数据。
- Hook JSON 从 stdin 读取，解析后只用于生成本机通知。
- 通知内容可能包含 agent 摘要，并会按 macOS 行为出现在本机 Notification Center。
- AgentBell 不需要云端账号、API key 或网络连接才能工作。

## 配置

配置可选，默认通知全部事件，默认使用 Terminal.app。文件不存在时无需初始化：

```toml
[notifications]
done = true
needs_input = true
needs_approval = true
codex_permission_requests = false # 默认关闭 Codex 原始授权请求提醒
error = true

[terminal]
app = "terminal" # 或 "iterm2"
```

保存到 `~/.config/agentbell/config.toml`。格式无效时 hook 会记入本地日志并使用默认值，`agentbell doctor` 会报告配置错误。

Codex 的 `PermissionRequest` 在审核前触发，无法判断请求是否已由自动审查处理，因此默认不发通知。确需接收所有 Codex 授权请求时，将 `codex_permission_requests` 和 `needs_approval` 都设为 `true`；这仍不等于“正在等待人工审批”。此设置不影响 Codex 自身的权限检查或审批界面。

## 安装检查

```bash
agentbell install --dry-run
agentbell doctor
agentbell doctor --fix
```

`doctor --fix` 会重新向 macOS 注册原生通知应用，并修复已经存在 AgentBell hook 的客户端配置；首次安装仍使用 `agentbell install`。普通通知会按来源、session、事件类型在 3 秒内去重；授权请求通知在同一 session 内一分钟去重一次。

## 当前限制

- 目前只支持 macOS；Claude Code 和 Codex 的 hook 配置需要由当前用户可读写。
- Codex 仅使用官方稳定的 `PermissionRequest` 和 `Stop` 事件；不依赖 `Elicitation` 或 `StopFailure`。
- 点击通知会启动 Terminal.app 或 iTerm2 并切换到 hook 提供的 `cwd`；不会恢复原 Terminal tab 或 agent session。
- native helper 使用 macOS `UserNotifications.framework`。首次发通知时 macOS 会请求通知权限；点击项目目录需要允许 AgentBell 自动化所选 Terminal 应用。
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
