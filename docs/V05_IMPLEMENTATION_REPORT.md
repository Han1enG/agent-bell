# AgentBell v0.5 实现与验收报告

日期：2026-10-06。未发布。已执行本地 Homebrew v0.4.0→RC.1→RC.2→RC.3，当前 CLI 与运行 App 均为 0.5.0-rc.4。

## NEEDS YOU 卡住的根因与复现

v0.4 的 RemoveRecent 明确拒绝 NEEDS YOU/WORKING/ERROR；SwiftUI 只给 READY 显示移除按钮。Claude 安装列表缺少 SessionEnd，所以退出后没有直接清除等待状态的事件。Reconcile 每分钟运行，仅检查 WORKING/NEEDS YOU/ERROR，READY 没有进程退出检测；缺少进程身份时依赖 24 小时归档。它还把 Tabby/JetBrains/tmux 的 ContextNotFound 当作 agent 退出，将 Surface 与 Runtime 混在一起。

旧测试中的移除限制、24 小时归档覆盖了原行为。新测试在本地复现等待状态，用测试专用 sleep 进程模拟强制终止，验证一次五秒轮询内转为 CLOSED、角标计数归零。不关闭用户终端，不杀用户 agent。

## SessionEnd 与状态规则

Claude SessionEnd 映射为独立 session_ended，保留 clear/resume/logout/prompt_input_exit/other 原因，清除 Attention，结束运行实例，不删除对话。Stop 仍是当前轮次完成，进入 READY，运行实例继续存在。安装、升级、卸载沿用只管理 AgentBell Hook 的实现，幂等保留其他配置；Claude Hook 数量从 9 增至 10。Codex 不安装、也不接受未经验证的 SessionEnd Hook。

进程检测返回 Alive/Exited/Unknown。已记录 PID 不存在或启动时间不匹配（PID 复用）才推断退出；缺少身份、超时、权限或其他 ps 错误为 Unknown，不超时伪造退出。检测命令固定为本地 ps，只读身份，保留原有 lstart 精度。Tab 不存在只更新 surface_state 与 Return 能力；bridge 不可达保持不确定。Agent 活着但终端断开仍不关闭 Runtime。

## Dismiss、Clear All 与持久化

所有显示状态均可 Dismiss，立即从快照与 badge 消失。clear_all 同样隐藏全部记录；菜单在存在等待或运行状态时确认。这些操作立即 SQLite 提交后才返回成功；失败报告持久化错误，并保留本次运行中的隐藏结果。不终止进程，不删除对话或目录。

持久化 dismissed_at 水位，保留原记录身份。迟到的普通 Hook（包括 UserPromptSubmit/Working）不能重新显示隐藏记录；需要新的明确 SessionStart。已识别的重复 SessionStart 不恢复隐藏 Attention；独立进程实例可重新显示。七天后精简隐藏记录的标题、摘要、目录和 ReturnTarget，保留身份与水位，避免旧事件复活。隐藏墓碑的身份水位不按历史保留期限删除。精简后不保留标题、摘要、项目路径、ReturnTarget 或退出原因；输入字段长度限制下，墓碑 JSON 的回归预算为 16 KiB。跟踪身份上限为 10,000，达到上限拒绝新增身份并显示诊断提示，不淘汰防复活水位；已有记录与 Dismiss 仍可用。此预算不等同 SQLite 文件大小硬上限，旧库不会强制裁剪。

## Conversation / Runtime / Recovery 模型

Session 保留原来的内部 id、status、attention、ReturnTarget，并新增 native_session_id、agent_flavor、runtime_state、runtime_instance_id、runtime_started_at、exited_at、exit_reason/last_exit_reason、previous_runtime_instance_id、surface_state、recovery_capability 与 dismissed_at。

Agent 原生 UUID 从事件的原生 session/thread 标识获取，不从内部 temporary key、项目名、标题或 --last 推导。只在可靠识别 Claude CLI/Codex CLI 且有 UUID 时标记命令恢复能力；桌面来源与未知来源不假定能用 CLI 恢复。v0.4 旧记录的缺失字段初始化 Unknown。RC.3 仅在本地 Claude 历史的 sessionId 与 cwd 精确匹配时补充 CLI 身份；内部 key 仅作查询候选，扫描有容量边界，不保留对话内容。

退出后的同原生 ID SessionStart 更新实例、进程身份和 ReturnTarget，清除旧 Attention，保留标题/项目元数据与上次退出信息。已识别的并行进程分配独立记录；旧进程的迟到事件不能覆盖已恢复实例。无可靠身份时无法证明两个事件来自不同并行进程，保持保守判断。

## 恢复实现与 CLI 验证

独立定义 ResumeProvider、ResumeTarget、ResumeResult；RC.3 注册 TabbyResumeProvider。IPC 提供 recovery 查询与 resume_session 请求；启动与等待不持有 Hook 引擎锁，成功前立即提交新 Runtime/ReturnTarget。

本机验证：Claude Code 2.1.289 的帮助支持 --resume；Codex CLI 0.159.0 的 resume --help 支持 SESSION_ID。官方命令分别为 claude --resume <UUID> 和 codex resume <UUID>。复制动作要求 Runtime 已退出、原进程不再存在或没有身份冲突、无其他可能运行的同对话实例，验证目录、固定受信任安装位置的可执行程序及当前帮助命令契约。使用固定 argv 模板，复制内容正确 shell quoting，不执行 shell、不传 Prompt、不绕过审批。

真实保存历史恢复 E2E：Claude **部分证据，交互式恢复未完成**；Codex CLI **未验证**。用户提供 Claude UUID 后，从对应文件的元数据确认 CWD，以 --resume 和禁用工具的单次请求验收。真实 SessionStart 标明 source=resume，但 API 返回 `403 Your IP address is not allowed`；未验证模型继续回答。用户确认这是已知 TUN 问题，403 不作为恢复 blocker。随后指出已关闭会话未恢复：此前 --print 调用结束即退出，未打开可继续使用的交互式会话，因此撤回本项完整通过判断。历史追加了验收请求与 API 错误。未修改用户 Hook 配置或项目文件，未构造对话历史，也没有把 CLI 帮助检查当作恢复成功。真实 SessionEnd(reason=other) 已将隔离 v0.5 Runtime 关闭并使 Attention=0；没有真实等待前置状态，因此完整 NEEDS YOU→SessionEnd 项仍待验收。CLI 不存在、UUID 不可靠、目录删除/迁移、活跃实例、Desktop 来源与并发实例的拒绝路径有自动化覆盖。恢复失败保留旧记录。目录迁移可在关闭行右键选择 Choose Project Folder，并重新验证。

## Tabby 与 Codex Desktop

本机 Tabby 包版本 1.0.235。官方 SessionOptions 定义 command/args/cwd，TerminalService.openTab 可用 LocalProfile 创建新终端；技术上具备受控启动的 API 基础。RC.3 使用 Tabby 原生 run 的结构化 argv 和内置 Run 确认，通过固定已安装 AgentBell 启动器设置 CWD，再 exec 原 CLI。现有定位桥接提供新标签页环境与精确上下文，无需重启 Tabby。真实新 Runtime、活跃 PID 和实时 exact context 都成立才确认恢复。没有键盘模拟、命令注入或强制打开 Terminal.app；Open Project 使用本地 open 打开项目。

Codex Desktop 只提供 Open App，不把其 ID 用于 Codex CLI resume。没有声明 Desktop 会话 deep-link 或精确恢复支持。

## 动态动作与去重观感

快照提供 action/action_reason；菜单分别呈现 Return/Open App/Resume in Tabby/Copy Resume Command/Open Project/Unavailable。已知同对话还有活跃实例、CWD 不存在或没有安全 CLI 文件时，不显示 Copy Resume Command；CLI 命令契约在点击时重新核实。点击重新校验；来源失效反馈错误并更新动作。菜单 Return 使用严格定位，检查每层上下文及 tmux 连接绑定，检查后发生关闭也返回错误，不能把仅打开 App 当作 Return 成功。既有通知 Universal Return 的回退策略保持原样。

用户截图中的 WORKING 与 READY 实际有不同原生 ID：01a10f81… 是当前聊天；01a10fc1… 是另一条完成事件，本地聊天目录查不到其对应标题/记录。后者使用项目名 Toy，造成重复观感。其来源尚不能确认，不按 CWD/标题盲目合并。缺少标题的行增加短 ID，以便区分。截图来自已安装 v0.4.0，并非本次隔离 v0.5 App。

## SQLite / IPC / 旧配置

沿用 SQLite schema v1 的 session_json 加字段，无需变更表结构；测试旧 JSON 恢复保留标题、摘要、CWD 和 READY 语义。IPC v1 加可选字段/命令，未知版本仍明确拒绝，旧通知路径不因新字段失效。recent 继续代表 READY；closed 独立、最多五条，不产生注意力或完成未读角标。关闭历史保留七天。

## 验证结果

- 当前完整 Go race（`-p 1`）：通过；Go vet：通过。并发全包运行两次触发临时 CLI fixture 的两秒检查超时，逐包复核通过；CI 同样逐包运行，不放宽产品超时。
- Node：6 个桥接/上下文测试通过（需沙箱外 Unix socket）；Python：3 个回归测试通过。
- 6,000 条精简墓碑：SQLite 约 3.9 MB，加载约 177 ms，快照平均约 0.39 ms；未变记录不再重写。250 次 Hook 跨持久化周期最长 ACK 约 6.7 ms。race 环境下测量，不承诺跨机器固定耗时。持久化失败回滚后缓存正确性通过。
- Java↔Go 真实桥接：通过，含权限、身份、检测、定位、失效与请求边界。
- macOS arm64/amd64 本地签名构建：通过；checksums 与 codesign smoke 通过。
- 实际原生 bundle 自动化：旧通知降级/暂停/退出/恢复测试通过；新 SessionEnd、Dismiss、Clear All、SQLite 重启、新 SessionStart 测试通过。Hook 是合成输入，不冒充真实 Claude 退出事件。
- 交互式 GUI：**未完成**。GUI 工具长时间等待后，隔离测试已结束；返回界面不作为验收证据。自动启动的临时测试实例已单独清理。
- tmux 完整 E2E：本机未安装 tmux；Go tmux 模块回归通过，远端 CI 会安装并运行真实 E2E。
- Homebrew：既有升级/用户配置保留/Hook 幂等测试通过；真实本地 RC brew upgrade 已执行；运行 App 与 CLI 均为 0.5.0-rc.1，6 条记录、配置和其他 Hook 保留。临时 Formula 仅使用本地包，公开 tap 已恢复原文件，旧 keg 与完整备份保留。

## CI 与发布条件

CI 保留 macos-15（arm64）与 macos-15-intel 矩阵，加入新原生生命周期 E2E 与 v0.5 非空 release notes。用户明确授权后，已将首个提交 ed998e2 推送到独立分支 codex/v0.5-session-lifecycle。[首个提交 CI](https://github.com/Han1enG/agent-bell/actions/runs/37420964295)：arm64 与 Intel 全部通过，包含 race/vet、Node/Python/Java、真实 tmux E2E、签名构建、smoke 与原生生命周期。Release 作业跳过，没有发布。此轮实现提交 `4979a4e` 已推送；[最新 CI](https://github.com/Han1enG/agent-bell/actions/runs/37423508421) 的 arm64/Intel 全部通过，Release skipped。最终本地 RC 签名、校验和、版本 smoke 与合成 Hook 原生生命周期测试通过。没有创建 Release/tag。初次 gh auth status 在沙箱内误报无效，沙箱外复核登录有效。未创建 tag/GitHub Release，未更新已发布 Homebrew Formula。

发布前仍需：真实 Claude/Codex 交互式历史恢复、真实 SessionEnd/Tab 关闭验收、GUI 点击与复制/目录选择验收、原生通知权限与 GUI 验收。Tabby 启动已实现，真实恢复仍需完成验收。本报告中的自动化成功不替代这些未完成项。

## 参考

- [Claude Hook lifecycle 与 SessionEnd](https://code.claude.com/docs/en/hooks)
- [Claude CLI reference](https://code.claude.com/docs/en/cli-reference)
- [Codex CLI developer commands](https://learn.chatgpt.com/docs/developer-commands?surface=cli)
- [Tabby SessionOptions](https://docs.tabby.sh/local/interfaces/SessionOptions.html)
- [Tabby TerminalService 源码](https://github.com/Eugeny/tabby/blob/master/tabby-local/src/services/terminal.service.ts)

## 本地覆盖后的健康检查

用户要求功能改动后自动编译并覆盖本地安装，已记入 AGENTS.md。当前 App/CLI 为 0.5.0-rc.4，签名、IPC、SQLite 及已配置 Hook 正常，Tabby/GoLand bridge 均可达。通知检查尚未通过：新旧 helper 均报告 notDetermined，实际测试通知返回 UNErrorDomain error 1。没有将 doctor 整体记为通过，也没有修改系统通知权限。公开 Release/tag/tap 未发布。

## RC.2 CLOSED 排序修复

四条历史会话在同次退出检测中具有相同 UpdatedAt。CLOSED 原排序未提供时间相等时的规则，map 遍历随机性导致刷新换位，并可能改变最多五条的显示成员。增加完整 ID 作为次级顺序，保留原来的时间倒序规则。200 次刷新回归在旧实现失败、修复后通过，相关 attention/主包 race 与 vet 通过。已按用户要求重新编译并覆盖本地安装，CLI 与正在运行的 App 均为 0.5.0-rc.2。签名/checksum/version smoke 通过，六条记录和配置保留；远端 RC.2 `36d3f64` 双架构 CI 全部通过，Release skipped。真实用户 App 连续 12 次 IPC 快照顺序保持一致。

## RC.3 展开与恢复交互

Recently Closed 改为整行按钮，包含文字和空白区域；保持同一个 ScrollView，通过测量内容高度向下调整固定顶部的面板，合并布局更新，不播放重新定位动画。Resume 显示 Restoring 并禁用重复点击，服务端同原 UUID 锁防止重复启动。缺少历史、UUID/CWD 不匹配、活跃/并发实例、错误 argv、新 Tab 无 Hook、失效 Context 与死亡进程均有拒绝路径回归。

RC.3 完整 race、vet、双架构签名构建、smoke 和合成 Hook 原生生命周期测试通过；本地 Homebrew RC.2→RC.3 已完成，CLI/App 一致，六条记录、配置和其他 Hook 保留。备份 `/tmp/agentbell-before-rc-20261006-215127`。首次真实恢复请求已启动原生 Run 确认，但 50 秒内未收到新 Runtime，明确失败。Computer Use 禁止控制 org.tabby，osascript 没有辅助访问权限，需用户完成原生 Run。真实恢复和展开观感仍待验收。

## RC.4 原终端恢复

针对用户对 Run 确认及新开窗口的反馈，恢复改走受管 Tabby 插件 API，固定启动器/参数不变。原窗口优先；其他窗口只选相同终端；已有窗口不调用 open，不调用 run。新插件通过 capabilities/resume 操作提供受控新标签页。恢复动作显示 Resume；非 Tabby 原来源目前仅复制命令，不强制 Tabby。更新插件后必须由 Tabby 重新加载，AgentBell 不自动重启用户终端。真实免确认恢复待加载新版插件后验收。

RC.4 真实免确认恢复已通过：用户加载新版插件后，正式 Resume 在请求前已有 Tabby 窗口新增 Tab，原 f54d5717 UUID 对应新 PID 14004；进程代际、exact Context 和 SQLite 新绑定均已核实。正式 Return 成功，bridge activeContextID 匹配恢复 Tab。当前只有一个 live terminal window，交互进程保持运行。此证据替代早期 --print 的不完整恢复验收；Codex CLI 与完整菜单 GUI 矩阵仍未验证。

RC.4 `95163ec` 双架构 CI 已全部通过（race/vet、真实 tmux/Java IPC、签名构建、smoke、原生生命周期），Release skipped。RC.5 只补齐无窗口但 App 仍运行的原生激活边界；插件代码不变，无需再次重启用户终端。
