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

从 [Releases](https://github.com/Han1enG/agent-bell/releases) 下载 macOS 可执行文件或 `.app.zip`。将 `agentbell` 放到 `~/bin`（或 PATH 中的其他目录），将 `AgentBell.app` 放到 `~/Applications`，然后运行：

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
| Claude Code | `PermissionRequest` | 等待授权 |
| Claude Code | `Stop` | 任务完成 |
| Claude Code | `StopFailure` | 执行失败 |
| Codex | `PermissionRequest` | 等待授权 |
| Codex | `Stop` | 任务完成；优先使用 `last_assistant_message` 作为摘要 |

通知标题使用项目名，正文使用 agent 提供的摘要，并由 macOS Notification Center 控制展示样式。

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

## 当前限制

- 目前只支持 macOS；Claude Code 和 Codex 的 hook 配置需要由当前用户可读写。
- Codex 仅使用官方稳定的 `PermissionRequest` 和 `Stop` 事件；不依赖 `Elicitation` 或 `StopFailure`。
- native helper 使用 macOS 的传统 `NSUserNotification` API，以兼容当前系统和应用图标展示；未来可迁移到更新的通知 API。
- 当前没有菜单栏、Dashboard、远程通知、历史查询或多机器同步功能。

## 本地开发

```bash
go test ./...
go run . doctor
go run . test
```

Homebrew tap 源码位于 [Han1enG/homebrew-agentbell](https://github.com/Han1enG/homebrew-agentbell)。

构建 macOS 发布包：

```bash
./scripts/build-macos.sh
```
