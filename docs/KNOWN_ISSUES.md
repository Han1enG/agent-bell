# 遗漏与待解决事项

本文件记录尚未解决的问题、已验证的限制和候选方案；列入本文件不表示已经修复或确定排期。

## AB-001：Codex 自动审查触发不需要人工处理的审批通知

- 状态：待解决，尚无可靠修复。
- 记录日期：2026-10-02。
- 影响版本：v0.2.0。

### 现象与影响

Codex 没有展示需要用户处理的审批提示，但 AgentBell 仍弹出权限请求通知。
这会造成误报；反过来，全部关闭 Codex 审批通知会漏掉真实的人工审批，因此不能作为修复方案。

### 原因与已验证的限制

AgentBell 当前把 Codex 的 `PermissionRequest` hook 映射为审批通知。
该 hook 在最终审批路由之前触发，自动审查也会触发；现有输入不足以可靠区分自动审查与正在等待人工审批。

App Server 提供明确的审批请求及 `serverRequest/resolved` 事件，但需要接入对应会话的协议连接。
2026-10-02 本机只读检查发现：

- 当前桌面 App Server 启动参数没有指定监听端点，符合默认 stdio 通信方式。
- 默认控制 socket 不存在，`codex app-server proxy` 连接失败。
- 未发现已检查桌面进程的 TCP 监听端口。

因此，当前环境下不能直接通过公开端点旁路监听已有桌面会话。
这是本机验证结果，不代表所有 Codex 版本和部署方式都不支持接入。
另起 App Server 也不能据此获取原桌面进程中会话的实时状态。

### 已撤回的方案

默认屏蔽全部 Codex `PermissionRequest` 通知的补丁已撤回，未发布为 v0.2.1。
当前保留审批提醒；本问题不能标记为已修复。

### 候选方向（尚未实现）

1. 优先等待或验证上游提供审批路由后的人工注意事件，或有效的人工审批标记。
2. 若能接入对应 App Server 会话，按明确审批请求、处理完成事件管理通知生命周期；不得代替用户作出审批决定。
3. 兼容方案可实验性使用延迟与后续状态取消通知，但必须先做旁路验证。慢速自动审查、执行中的命令和日志格式变化仍可能导致误判，不能宣称完全准确。

接管或改变会话启动方式需要另行确认；不得为减少通知而更改用户的安全审批策略。

### 验收要求

- 自动审查允许或拒绝且无需人工处理时，不弹出“需要人工审批”通知。
- 真实人工审批仍能提醒，包括命令、文件修改及权限请求等受支持类型。
- 已处理或取消的请求不继续产生过期提醒，多会话状态不能互相干扰。
- 验证慢速自动审查与长时间命令执行，不能把单纯未完成视为等待审批。
- 明确区分桌面端与 CLI 的支持范围，以及准确事件方案与启发式兼容方案。

### 参考

- [Codex 上游 #28833：被动通知缺少人工审批信号](https://github.com/openai/codex/issues/28833)
- [Codex 上游 #23465：hook 缺少有效审批 reviewer 信息](https://github.com/openai/codex/issues/23465)
- [官方 App Server 审批协议](https://learn.chatgpt.com/docs/app-server#approvals)
- [AgentNotifier：延迟与调用结果取消的兼容方案源码](https://github.com/almuqrin/agentnotifier/blob/main/agentnotifier/codex.py)
- [tele-codex：App Server 事件流监听及接入限制](https://github.com/Kentaczi/tele-codex#optional-exact-app-server-state-watcher)
