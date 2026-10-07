# AgentBell v0.5.0-rc.6 当前版本报告

日期：2026-10-07。当前是已安装的候选版本，尚不批准正式发布。

## 当前交付状态

- CLI 与运行 App 均为 **0.5.0-rc.6**；已编译并覆盖本地安装。
- 实时快照：WORKING 1、NEEDS YOU 0、READY 1、CLOSED 4；未报告存储错误。
- RC.6 arm64/Intel 本地签名构建、校验和与版本 smoke 通过。
- 本次升级保留六条身份、用户配置及其他 Hook，公开 Homebrew tap 已恢复原文件。
- 备份：`/tmp/agentbell-before-rc-20261007-094659`。
- **远端双架构 CI 已通过的是 RC.5 功能与验收提交 `24f27bc`。RC.6 的界面改动目前未提交/推送，尚无 RC.6 远端 CI 通过证据。**

## 已实现能力

| 模块 | 当前行为 | 验证程度 |
| --- | --- | --- |
| 生命周期建模 | Conversation、Runtime、Surface 分离；Tab 消失不等于进程退出 | 模型与 IPC 回归通过 |
| 退出判断 | 明确 SessionEnd 或可靠 PID/启动时间失效才关闭；无法确认则 Unknown | 自动化通过，完整真实生命周期待验收 |
| Attention 清理 | Claude SessionEnd 清除等待/错误；Stop 仍表示轮次完成 | 合成 Hook 原生 E2E 通过 |
| Dismiss / Clear All | 持久化隐藏，不杀进程；迟到普通 Hook 不复活记录 | IPC、SQLite 重启回归通过 |
| CLOSED 历史 | 七天保留、最多显示五条；相同时间按完整 ID 排序 | 排序回归及真实连续快照通过 |
| Claude 恢复 | 新版 Tabby 插件在原窗口新 Tab 恢复原 UUID，不弹 Tabby Run 确认 | **真实交互式恢复通过** |
| Return 更新 | 新 Runtime 更新精确 Context 并持久化；Return 定位恢复 Tab | **真实精确返回通过** |
| 原终端选择 | 优先原窗口，失效时复用同终端其他窗口；无窗口时激活原 App 创建窗口 | 原窗口实测通过；无窗口分支回归通过 |
| 其他终端 | 尚无自动恢复适配时提供 Copy Resume Command，不强制切换到 Tabby；Codex Desktop 保留 Open App | 能力边界明确 |
| 展开 UX | 整行可点击、顶部固定向下展开；RC.6 去掉重复 CLOSED 标题，数量放在外层右侧 | 本地签名编译通过；完整 GUI 矩阵待验收 |
| tombstone 容量 | 精简身份水位；不长期保留项目路径、标题和摘要；身份上限 10,000 | 6,000 记录约 3.9 MB 的容量回归通过 |

真实恢复使用用户提供的 Claude UUID `f54d5717-0719-464a-98d9-2eab04110be1`。验收时核实新 PID、原窗口复用、实时 exact Context、Return 后的 activeContextID 和 SQLite 新绑定。早期 `--print` 证据已被交互式恢复替代。已知 TUN 下的 API 403 不作为本次恢复 blocker；没有宣称模型回答或网络问题已经修复。

## 原 14 项发布验收清单

| 验收项 | RC.6 当前结论 |
| --- | --- |
| Claude NEEDS YOU → 真实 SessionEnd → badge 0 | 待完整真实验收；合成 E2E 已通过 |
| 关闭真实 Tab，确认退出后更新 | 待真实 Tab 验收；测试进程回归已通过 |
| Unknown 可 Dismiss，重启不复活 | 自动化通过，待 GUI 点击 |
| Claude CLI 恢复原历史 | **通过** |
| Codex CLI 恢复原历史 | 待真实恢复 |
| 复制命令、CWD 选择、错误反馈 GUI | 安全与错误路径回归通过，待 GUI |
| 新 Runtime 更新 ReturnTarget | **通过** |
| 最新代码 arm64/Intel CI | RC.5 **通过**；RC.6 **待验证** |
| 最终签名 App CLOSED/Resume GUI | 已有局部交互证据，完整矩阵待验收 |
| Homebrew v0.4 → v0.5 升级 | **通过本地 RC 包升级**；公开发布包未验收 |
| 原生通知及旧配置不回归 | 配置保留通过；通知尚无通过证据，先前测试为 notDetermined / UNErrorDomain 1 |
| tombstone 长期存储与容量 | **通过** |
| Unknown 界面语义 | 运行状态移入鼠标提示；保留“此前等待”措辞，待 GUI 确认 |
| Tabby 受控启动 PoC | **通过，并已接入实际恢复** |

原 RC.5 报告为 6/14。严格按 RC.6 最新源码计算，当前完整通过 5/14；第六项 CI 已在 RC.5 通过，RC.6 尚需补齐。未完成项不能由编译、CLI help 或合成 Hook 替代。

## 发布判断

保持 RC，不发布正式 v0.5。剩余重点是：真实完整生命周期、Codex CLI 历史恢复、完整菜单 GUI 与 Unknown 语义、原生通知验收，以及 RC.6 最新提交的双架构 CI。

这版已经跑通 Claude 的 **CLOSED → Resume → 原历史新 Runtime → 精确 Return**。自动恢复当前只适配 Tabby；其他终端仍需手动执行复制的命令。

## 证据

- [RC.5 双架构 CI](https://github.com/Han1enG/agent-bell/actions/runs/37484651803)：arm64、Intel success，Release skipped。
- RC.6 安装记录：`/tmp/agentbell-v05-rc6-install/local-upgrade-evidence.json`。
- RC.6 本地 smoke：`/tmp/ab-rc6-smoke.log`。
- 真实 Claude 恢复记录：`/tmp/agentbell-v05-rc4-install/claude-live-recovery-evidence.json`。

未创建正式 Release/tag，未更新公开 Homebrew 发布版本。

## Release Closure 本轮验收（2026-10-07）

保持 RC.6，不新增恢复提供方或修改生命周期架构。主界面副标题不再显示 Runtime running / exited / unconfirmed，运行状态移入鼠标提示；Unknown 的此前等待措辞保留，避免表示确定仍在等待。

- 最新源码 `go test -race -p 1 -count=1 ./...` 在真实本机进程环境通过；沙箱内 /bin/ps 禁止访问不作为生命周期失败证据。
- 重新编译 arm64 / Intel 签名包，校验和与版本 smoke 通过；已重新覆盖本机 CLI/App，仅重启 AgentBell，配置、Hook 与六条会话身份保留。
- 真实 doctor 返回通知授权 notDetermined。真实 test 在 requestAuthorization 阶段返回 UNErrorDomain 1，未调用 addNotificationRequest。这是授权请求失败证据，尚无可见通知或退出 App 后点击返回的通过证据。
- 普通与归档 Codex 历史均标记为 Codex Desktop，未找到 CLI 历史。专用 CLI 验收停在目录首次信任提示，自动审批拒绝接受持久信任设置，已请求用户选择。尚未建立验收聊天或验证 CLI 恢复。
- Claude 真实 NEEDS YOU 验收需要终端真实权限等待，已请求用户在专用 Tab 触发。普通 idle_prompt、合成 Hook、403 错误不替代真实等待。正常退出及单独关闭 Tab 待配合验收。

本轮证据：`/tmp/ab-rc6-closure-real-tests.log`、`/tmp/ab-rc6-closure-build.log`、`/tmp/ab-rc6-closure-smoke.log`、`/tmp/agentbell-v05-rc6-install/local-closure-reinstall-evidence.json`、`/tmp/agentbell-rc6-closure-evidence/notification-authorization.json`。
