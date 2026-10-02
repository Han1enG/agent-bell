# AgentBell v0.2.2 Stabilization — development report

日期：2026-10-02。当前版本：`0.2.2-dev`。

**未达到 Definition of Done，不能标记 v0.2.2 完成或发布。** Codex AB-001、真实 payload corpus、完整 Surface GUI 矩阵和远端 CI 均未完成。下面严格区分代码回归、真实桥接检查和 GUI 验收。

## 修复了什么

- 添加可选 `ProbeableProvider`；Tabby、GoLand、Terminal 均实现只读验证。点击时 Manager 重新 Probe，Return 仍重新解析 live target，失效后降级。
- doctor 将 Current Session 与 Integration Health 分开，独立显示安装/内容版本匹配、桥接可达和当前 shell 的有效身份。
- 标准化返回失败 reason；失败日志使用 surface/provider/capability/result/reason，不输出原始工具输入或 Prompt。
- Tabby 原 8 KiB 响应读取上限无法容纳 100 个 Context。现使用有界 64 KiB 响应，输入仍限制 8 KiB，新增 100 Context 的 JS 与 Go 回归。
- Tabby split root 缺少 `focus(child)` 时，报告不具备精确 pane 能力，focus 在任何选择动作前失败；Core 返回 App。
- GoLand focus 在改变选择前检查内容有效性、所属 project frame 和请求 deadline；无法定位 frame 时不声称成功。
- 补充正常受管旧版本升级、用户修改保护、通知与 Automation 拒绝独立性的回归。

## Codex 误审批根因

现有代码将所有 `PermissionRequest` 直接映射为 `NEEDS_APPROVAL`。用户确认误报发生在 Auto-review 下；该 hook 在最终审批路由之前触发，其他 hook 或自动审查可能允许/拒绝请求，而没有人工等待。

官方 [Hooks 文档](https://learn.chatgpt.com/docs/hooks) 定义了这一审批前置语义。当前公开输入未提供最终 reviewer、审批决策或确认人工等待的字段；`permission_mode` 不能替代它。**AB-001 未修复。** 仓库已记录默认关闭全部提醒的方案因漏掉真实人工审批而撤回，本次没有重新采用该方案，也没有增加 debounce 来伪装修复。

## Codex 新事件判断逻辑

映射集中到 `classifyCodexEvent`，仍保留现有 Stop/PermissionRequest 行为以避免静默丢失人工审批。没有根据 command、description、耗时或未完成状态猜测 reviewer；没有编造自动审查状态字段。

已通过现有通知入口的短时透传采集取得 `approval-auto-review.json`：Codex desktop runtime 0.159.2，Auto-review 自动允许一次本地 Go 测试，没有人工审批。真实输入为 `PermissionRequest` + `permission_mode: default`，没有最终 reviewer/decision/human-wait 字段。launcher 已恢复原 symlink；hook 配置和审批策略保持原样。脱敏和采集方式详见 `testdata/codex/README.md`。

会话 transcript 的 `turn_context` 有 `approvals_reviewer: auto_review`，但这是回合配置，并不确认某个请求正在等人，不能据此将所有 human 模式请求判为需要操作。公开 app-server 协议有 `waitingOnApproval` 与 approval server requests，但本机没有现成可接入原桌面会话的 daemon control socket；启动另一个 server 不能观察本会话。

现有两个示例仍明确标注不是实时采集。真实 user-required/auto-accepted/Stop/failure corpus 待取得；稳定过滤仍需可靠的路由后信号。

## doctor 新能力

Current Session 输出来源、App return、缺少身份的原因和 live Probe 结果；Integration Health 输出安装、受管内容、bundled current 和可达桥接数。失效 socket 不会因为文件存在而被当作健康桥接。

缺少当前 Context 或未接入的旧 shell 不影响健康安装的退出码；新增测试确认没有 Exact Context 时仍可退出 0。`doctor --fix` 延用受保护安装逻辑，无法覆盖被用户修改的插件。

Terminal native helper 增加不请求授权的 Automation 状态查询。doctor 只有在查询已授权后才进行读取验证；拒绝、无法确认或 App 未运行时不会以 Exact 健康报告。当前机器 Terminal 未运行，最终 helper 返回 `app_not_running`，没有弹出授权框。

## Current Session 如何检测

Tabby/GoLand 必须有匹配的 `AGENTBELL_SURFACE` 和 `AGENTBELL_CONTEXT_ID`，且对应 UUID 在所属 window/instance 的实时 `list` 中存在。仅有环境变量不足以成立。Tabby 新桥接还检查 pane 是否具备 focus API。

Terminal 使用现有 TTY、PID 和固定 locale 的 process start time 身份链，并只读查找匹配的实际 Tab。普通未知 shell 显示 Exact 不可用。新增 `surface probe tabby|jetbrains|terminal <context-id>` 诊断入口，不切换会话。

## Tabby 验收结果

| 场景 | 证据 | 状态 |
| --- | --- | --- |
| 普通多 Tab、同 cwd/title、改名、关闭 | Node 桥接回归 | 通过 |
| 100 Context 唯一性和大列表 | Node + Go socket 回归 | 通过 |
| window/instance 重启后旧 ID 不命中新 Tab | 两个独立桥接实例的模拟回归 | 通过；真实 App 重启待验收 |
| 旧 PTY/已运行 shell、SSH 不改环境 | context 回归 | 通过 |
| 新 local Tab 独立注入，不污染共享 profile | context 回归 | 通过 |
| 支持/不支持 split pane API | Node 回归 | 通过；真实 pane GUI 待验收 |
| socket 缺失、失效、非法 ID | Go fallback 回归 | 通过 |
| 已安装的真实 bridge | 1 个可达 bridge、3 个 live Context；另有 4 个失效 socket | Probe 通过，active Context 未变化 |
| 通知点击、真实关闭/重启、退出竞态、权限错误完整矩阵 | 尚无完整 GUI 证据 | 待验收 |

电脑使用工具明确禁止控制 `org.tabby`，因此没有通过其他 UI 自动化路径绕过这一限制。真实 Probe 检查的是之前已安装的 bridge，本次更新插件没有部署到用户 App。

## GoLand Classic 验收结果

新插件已使用本机 GoLand 2025.3 SDK 编译；Java↔Go socket 检查通过。运行中的旧插件有 3 个 live Context，逐一 Probe 前后 list/Selected 完全相同。

这些检查未辨认或操作 Classic GUI，不能替代单 Tab、多 Tab、同 cwd、关闭、多个 project window 和 IDE restart 的完整验收。**Classic 矩阵待验收。**

## GoLand Reworked 验收结果

Reworked API 引用通过 2025.3 SDK 编译；共享 IPC 回归通过。锁屏已解除，实际 GoLand 2025.3.5.1 中创建两个一次性 terminal tab，同 cwd，分别获得不同 Context ID。第一个会话运行开发版 doctor，显示 live read-only Exact available；临时 helper 的通知授权不可用导致 doctor overall exit 1，未把它冒称为完整健康验收。

关闭第二个测试会话后，开发版 Probe 返回 `context_not_found: JetBrains context expired`。两个测试会话均已退出，bridge Context 数量恢复原有 3 个；逐一 Probe 不改变 list/Selected。运行中的插件仍为旧安装版本，terminal engine 尚未确认，因此上述事实不标为新 Reworked 或 Classic 插件完整验收。一次诊断输入误落入 go.work 编辑器，已两次 Undo 恢复原内容，并从磁盘核对；未留下该输入。

未更改用户 terminal engine 设置，也未重启正在使用的 IDE。完整 GUI 矩阵仍待验收。

兼容范围保持 GoLand 2025.3 / build 253，没有扩展其他 IDE 或 build line。

## Terminal.app 验收结果

现有 process identity、locale、PID/TTY reuse、关闭/失效、权限拒绝降级回归全部通过；新增 Probe 只读分支及通知与权限独立性测试通过。native helper 在两种架构完成编译和签名校验。

真实多窗口、多 Tab、同 cwd、App 重启和实际权限拒绝 GUI 矩阵待验收。Terminal 当前未运行，电脑使用工具禁止控制 `com.apple.Terminal`，没有创建测试窗口或修改 TCC 授权。

## Return fallback 验收结果

Exact → Window → App → Project 的顺序保持不变。新增点击重新 Probe 回归；expired Context 不调用精确 focus。Tabby missing/non-socket bridge 和非法 ID 均记录原因后返回原 App。Terminal 拒绝 Automation 时仍调用 `open -b com.apple.Terminal`。Generic 项目路径仍要求绝对、存在且为目录。

缺少对应 provider 也记录 `provider_unavailable`。标准 reason 包括 context_not_found、provider_unavailable、bridge_unreachable、app_not_running、permission_denied、invalid_target、invalid_cwd、unsupported_surface 和 unknown。

## stale context 处理

通知保存的是线索。点击重新验证所属 bridge 和当前 Context；Tabby/GoLand 每个运行实例使用新的随机 UUID，不按 cwd、标题或 project name 查找替代会话。Terminal 重新核对 PID/启动时间/TTY，身份不一致时不发送 Apple event。

以上包含自动化回归与真实只读检查，但不能等同于所有 App restart GUI 场景已经验收。

## 权限拒绝行为

返回阶段失败不会阻止通知发送。新增测试模拟 native 通知成功，然后 Terminal Probe 返回 permission_denied；原 fallback 测试验证仍激活 Terminal。doctor 的权限查询不请求新授权。不主动修改用户的审批策略或系统授权。

实际拒绝授权后的通知点击 E2E 仍待验证。

## 安装/升级保护

两个插件新增受管旧内容升级回归。原有 modified/foreign/symlink/extra-file 安装和卸载保护仍通过；不增加强制覆盖路径。Tabby 修改文件的卸载提示明确为 `Plugin has local modifications ... Skipping removal; files preserved.`

修改过的插件会阻止卸载继续执行，原有保护行为保持；报告跳过而不是宣称已清理。新插件与开发包尚未安装到用户应用。

## 测试结果

- 基线：完整 Go suite、3 个原 Node tests、Java↔Go bridge tests 均通过（socket tests 需要允许本地 Unix socket）。
- 最终 `go test -race -count=1 ./...`：通过。
- 最终 `go vet ./...`：通过。
- 最终 Node：5 个 tests 通过，0 失败。
- Python 捕获/脱敏工具：3 个 tests 通过，0 失败。
- 新 GoLand jar 的 Java↔Go bridge test：通过；SDK 编译通过。
- arm64/amd64 完整 native + Go 开发包构建、codesign 校验：通过。默认 SDK 27 与本机 linker 不兼容，本地使用已安装的 SDK 26.5。
- `git diff --check`：通过。
- CI 配置保持 Apple Silicon/Intel 两个平台，新增 race、vet 和 Python tests。**未推送/触发远端 CI，不能声明远端 CI 已通过。**

开发构建位于 `/private/tmp/agentbell-v022-stabilization`，包含两种架构的 tar.gz 和 checksums.txt。未创建 release/tag、未发布、未安装到用户应用。

## 尚未解决的问题

1. AB-001：缺少可靠的路由后人工等待信号，现有误审批通知仍可能出现。
2. 已取得真实 Auto-review fixture；user-required/auto-accepted/Stop/failure corpus 尚未齐全，审批分类要求未通过。
3. Tabby、GoLand Classic/Reworked、Terminal 的完整真实 GUI 矩阵尚未通过；Tabby/Terminal 受电脑使用工具限制。GoLand 已补做真实新会话、同 cwd 唯一身份和关闭失效检查，但新插件矩阵仍待完成。
4. 远端 arm64/amd64 CI 尚未运行。

上述任何一项都不能用 mock、示例 fixtures、编译成功或本地双架构构建代替。

## tmux 预研结果（如果有）

未开展。P0/P1 未全部完成，不进入 tmux 研究或开发。

## 下一版本建议

先解除上述 v0.2.2 发布阻塞：取得实际路由后审批信号及真实捕获，完成真实 Surface GUI 验收，再执行远端 CI。此前不发布 v0.2.2，不开发 v0.3 或新的 Surface。
