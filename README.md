# AgentBell

轻量级 Coding Agent Notification Layer：通过 Claude Code / Codex hooks，将任务完成、等待授权、等待输入和执行失败等事件发送为 macOS 原生通知。

## 当前进度

已完成第一批基础能力：

- Go 单二进制 CLI 骨架
- 统一 `AgentEvent` 事件模型
- JSON stdin 事件解析
- 使用 Claude Code / Codex `last_assistant_message` 作为完成摘要
- macOS 原生通知 helper 和 AgentBell.app
- `test`、`notify`、`doctor`、`version` 命令
- 幂等的 `install` / `uninstall` hooks 配置
- 事件解析和通知脚本单元测试

Claude Code 接入 `Notification`、`PermissionRequest`、`Stop` 和 `StopFailure`；Codex 当前只接入官方稳定的 `PermissionRequest` 和 `Stop`。

安装器只写入 Claude Code 的 `~/.claude/settings.json` 和 Codex 的 `~/.codex/hooks.json`，卸载时只删除 AgentBell 自己添加的命令项，并清理 AgentBell.app 和 launcher。

## 开发

```bash
go test ./...
go run . doctor
go run . test
```

Hook 调用入口示例：

```bash
echo '{"event":"completed","cwd":"/tmp/demo","message":"finished"}' \
  | go run . notify --source claude
```

产品设计与 v0.1 范围以 AgentBell 产品说明为准。
