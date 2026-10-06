# AgentBell v0.5 实现与验收报告

日期：2026-10-06。未发布，未升级用户当前运行的 Homebrew v0.4.0。代码与临时签名 App 为 v0.5.0。

## NEEDS YOU 卡住的根因与复现

v0.4 的 RemoveRecent 明确拒绝 NEEDS YOU/WORKING/ERROR；SwiftUI 只给 READY 显示移除按钮。Claude 安装列表缺少 SessionEnd，所以退出后没有直接清除等待状态的事件。Reconcile 每分钟运行，仅检查 WORKING/NEEDS YOU/ERROR，READY 没有进程退出检测；缺少进程身份时依赖 24 小时归档。它还把 Tabby/JetBrains/tmux 的 ContextNotFound 当作 agent 退出，将 Surface 与 Runtime 混在一起。

旧测试中的移除限制、24 小时归档覆盖了原行为。新测试在本地复现等待状态，用测试专用 sleep 进程模拟强制终止，验证一次五秒轮询内转为 CLOSED、角标计数归零。不关闭用户终端，不杀用户 agent。

## SessionEnd 与状态规则

Claude SessionEnd 映射为独立 session_ended，保留 clear/resume/logout/prompt_input_exit/other 原因，清除 Attention，结束运行实例，不删除对话。Stop 仍是当前轮次完成，进入 READY，运行实例继续存在。安装、升级、卸载沿用只管理 AgentBell Hook 的实现，幂等保留其他配置；Claude Hook 数量从 9 增至 10。Codex 不安装、也不接受未经验证的 SessionEnd Hook。

进程检测返回 Alive/Exited/Unknown。已记录 PID 不存在或启动时间不匹配（PID 复用）才推断退出；缺少身份、超时、权限或其他 ps 错误为 Unknown，不超时伪造退出。检测命令固定为本地 ps，只读身份，保留原有 lstart 精度。Tab 不存在只更新 surface_state 与 Return 能力；bridge 不可达保持不确定。Agent 活着但终端断开仍不关闭 Runtime。

## Dismiss、Clear All 与持久化

所有显示状态均可 Dismiss，立即从快照与 badge 消失。clear_all 同样隐藏全部记录；菜单在存在等待或运行状态时确认。这些操作立即 SQLite 提交后才返回成功；失败报告持久化错误，并保留本次运行中的隐藏结果。不终止进程，不删除对话或目录。

持久化 dismissed_at 水位，保留原记录身份。迟到的普通 Hook（包括 UserPromptSubmit/Working）不能重新显示隐藏记录；需要新的明确 SessionStart。已识别的重复 SessionStart 不恢复隐藏 Attention；独立进程实例可重新显示。七天后精简隐藏记录的标题、摘要、目录和 ReturnTarget，保留身份与水位，避免旧事件复活。隐藏墓碑的身份水位不按历史保留期限删除。

## Conversation / Runtime / Recovery 模型

Session 保留原来的内部 id、status、attention、ReturnTarget，并新增 native_session_id、agent_flavor、runtime_state、runtime_instance_id、runtime_started_at、exited_at、exit_reason/last_exit_reason、previous_runtime_instance_id、surface_state、recovery_capability 与 dismissed_at。

Agent 原生 UUID 从事件的原生 session/thread 标识获取，不从内部 temporary key、项目名、标题或 --last 推导。只在可靠识别 Claude CLI/Codex CLI 且有 UUID 时标记命令恢复能力；桌面来源与未知来源不假定能用 CLI 恢复。v0.4 旧记录的缺失字段初始化 Unknown，不推导可恢复 ID。

退出后的同原生 ID SessionStart 更新实例、进程身份和 ReturnTarget，清除旧 Attention，保留标题/项目元数据与上次退出信息。已识别的并行进程分配独立记录；旧进程的迟到事件不能覆盖已恢复实例。无可靠身份时无法证明两个事件来自不同并行进程，保持保守判断。

## 恢复实现与 CLI 验证

独立定义 ResumeProvider、ResumeTarget、ResumeResult；本版本不注册自动启动 Provider。IPC 提供 recovery 查询与明确的 resume_session 请求；没有安全自动 Provider 时，resume_session 返回具体原因，不启动进程。

本机验证：Claude Code 2.1.289 的帮助支持 --resume；Codex CLI 0.159.0 的 resume --help 支持 SESSION_ID。官方命令分别为 claude --resume <UUID> 和 codex resume <UUID>。复制动作要求 Runtime 已退出、原进程不再存在或没有身份冲突、无其他可能运行的同对话实例，验证目录、固定受信任安装位置的可执行程序及当前帮助命令契约。使用固定 argv 模板，复制内容正确 shell quoting，不执行 shell、不传 Prompt、不绕过审批。

真实保存历史恢复 E2E：Claude **未验证**；Codex CLI **未验证**。已请求用于验收的已退出原生 UUID/CWD，目前未获得。没有扫描、修改或构造用户对话历史，也没有把 CLI 帮助检查当作恢复成功。CLI 不存在、UUID 不可靠、目录删除/迁移、活跃实例、Desktop 来源与并发实例的拒绝路径有自动化覆盖。恢复失败保留旧记录。目录迁移可在关闭行右键选择 Choose Project Folder，并重新验证。

## Tabby 与 Codex Desktop

本机 Tabby 包版本 1.0.235。官方 SessionOptions 定义 command/args/cwd，TerminalService.openTab 可用 LocalProfile 创建新终端；技术上具备受控启动的 API 基础。AgentBell 当前桥接仍只有定位功能，未扩展启动协议、未验收真实 argv/CWD 启动，因此本版本只提供 Copy Resume Command。没有键盘模拟、命令注入或强制打开 Terminal.app；Open Project 使用本地 open 打开项目。

Codex Desktop 只提供 Open App，不把其 ID 用于 Codex CLI resume。没有声明 Desktop 会话 deep-link 或精确恢复支持。

## 动态动作与去重观感

快照提供 action/action_reason；菜单分别呈现 Return/Open App/Copy Resume Command/Open Project/Unavailable。点击重新校验；来源失效反馈错误并更新动作。菜单 Return 使用严格定位，检查每层上下文及 tmux 连接绑定，检查后发生关闭也返回错误，不能把仅打开 App 当作 Return 成功。既有通知 Universal Return 的回退策略保持原样。

用户截图中的 WORKING 与 READY 实际有不同原生 ID：01a10f81… 是当前聊天；01a10fc1… 是另一条完成事件，本地聊天目录查不到其对应标题/记录。后者使用项目名 Toy，造成重复观感。其来源尚不能确认，不按 CWD/标题盲目合并。缺少标题的行增加短 ID，以便区分。截图来自已安装 v0.4.0，并非本次隔离 v0.5 App。

## SQLite / IPC / 旧配置

沿用 SQLite schema v1 的 session_json 加字段，无需变更表结构；测试旧 JSON 恢复保留标题、摘要、CWD 和 READY 语义。IPC v1 加可选字段/命令，未知版本仍明确拒绝，旧通知路径不因新字段失效。recent 继续代表 READY；closed 独立、最多五条，不产生注意力或完成未读角标。关闭历史保留七天。

## 验证结果

- 完整 Go race：通过；最终受影响模块 race 与 Go vet：通过。
- Node：6 个桥接/上下文测试通过；Python：3 个回归测试通过。
- Java↔Go 真实桥接：通过，含权限、身份、检测、定位、失效与请求边界。
- macOS arm64/amd64 本地签名构建：通过；checksums 与 codesign smoke 通过。
- 实际原生 bundle 自动化：旧通知降级/暂停/退出/恢复测试通过；新 SessionEnd、Dismiss、Clear All、SQLite 重启、新 SessionStart 测试通过。Hook 是合成输入，不冒充真实 Claude 退出事件。
- 交互式 GUI：**未完成**。GUI 工具长时间等待后，隔离测试已结束；返回界面不作为验收证据。自动启动的临时测试实例已单独清理。
- tmux 完整 E2E：本机未安装 tmux；Go tmux 模块回归通过，远端 CI 会安装并运行真实 E2E。
- Homebrew：既有升级/用户配置保留/Hook 幂等测试通过；真实 brew upgrade 未执行，tap 未更新未发布的 v0.5 URL/hash。

## CI 与发布条件

CI 保留 macos-15（arm64）与 macos-15-intel 矩阵，加入新原生生命周期 E2E 与 v0.5 非空 release notes。远端 CI **未运行**：自动审批拒绝提交/推送，理由为未获明确授权向远端导出可能私有的源码与仓库历史；已请求用户授权。拒绝发生在命令执行前，当前仍为原分支上的未提交改动。初次 gh auth status 在沙箱内误报无效，沙箱外复核登录有效。未创建 tag/GitHub Release，未更新已发布 Homebrew Formula。

发布前仍需：真实 Claude/Codex 历史恢复、真实 SessionEnd/Tab 关闭验收、GUI 点击与复制/目录选择验收、远端双架构 CI、实际 Homebrew 升级。自动 Resume 未实现；不可宣传一键恢复。本报告中的自动化成功不替代这些未完成项。

## 参考

- [Claude Hook lifecycle 与 SessionEnd](https://code.claude.com/docs/en/hooks)
- [Claude CLI reference](https://code.claude.com/docs/en/cli-reference)
- [Codex CLI developer commands](https://learn.chatgpt.com/docs/developer-commands?surface=cli)
- [Tabby SessionOptions](https://docs.tabby.sh/local/interfaces/SessionOptions.html)
- [Tabby TerminalService 源码](https://github.com/Eugeny/tabby/blob/master/tabby-local/src/services/terminal.service.ts)
