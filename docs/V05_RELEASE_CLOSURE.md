# v0.5.0-rc.2 Release Closure

候选版本，不发布 Release/tag，不更新公开 Homebrew Formula。Tabby 一键恢复延期至 v0.5.1。

进度按用户要求的真实验收口径计算。合成 Hook、测试进程和 CLI help 不替代真实历史恢复或交互式 GUI。当前完整通过 **2/14**；其余项有部分自动化证据，但尚未完成真实验收。

| 验收项 | 状态与证据 |
| --- | --- |
| Claude NEEDS YOU → 真实 SessionEnd → badge 0 | 待真实 Claude；合成 SessionEnd 原生 bundle 回归已有覆盖 |
| 关闭真实 Tab，确认进程退出并更新 | 待真实 Tab；测试专用进程退出在五秒轮询内更新 |
| Unknown 可 Dismiss，重启不复活 | IPC/SQLite 自动化通过；待 GUI 点击验收 |
| Claude --resume 恢复原聊天 | **部分证据，待交互式恢复**：原 UUID 被 CLI 接受并触发 SessionStart(source=resume)，但 --print 验收结束即退出，没有打开可继续使用的终端会话；已知 TUN 下的 403 不作为本项 blocker |
| Codex resume 恢复原聊天 | 待真实保存历史；帮助参数检查不计通过 |
| 复制命令、选择 CWD、错误反馈 | 安全条件及错误路径回归通过；待 GUI |
| 恢复后的 Runtime 更新 ReturnTarget | 模型回归通过；待真实恢复链路 |
| 最新提交 arm64/Intel CI | RC.1 的 `4979a4e` 两架构通过；RC.2 排序修复待最新远端 CI |
| 最终签名 App CLOSED/Resume GUI | 本地双架构 RC 构建；待人工交互验收 |
| 真实 Homebrew v0.4 → v0.5 升级 | **通过（本地 RC 包）**：brew upgrade 0.4.0→0.5.0-rc.1，CLI/App 一致、6 条记录保留、配置及其他 Hook 保留；公开 tap 恢复 v0.4，未发布 |
| 原生通知和旧配置不回归 | 真实升级后配置/非 AgentBell Hook 保留；新旧 helper 均 notDetermined，实际测试通知返回 UNErrorDomain 1，通知验收未通过 |
| tombstone 长期存储与容量 | **通过**：6,000 记录约 3.9 MB，精简后无项目路径/标题/摘要；10,000 身份上限，水位不淘汰 |
| Unknown 界面语义 | 已显示 Runtime unconfirmed 与此前等待状态；待 GUI 观感确认 |
| Tabby 受控启动独立 PoC | 只有 API 可行性证据；尚未实际运行，v0.5 产品不接入 |

容量策略：Dismiss 保留身份水位；七天后仅保留身份、时间与进程代际证据。精简 JSON 回归预算 16 KiB，不声称 SQLite 文件存在硬字节上限。10,000 身份上限拒绝新增身份，保留已有身份的处理能力，提供诊断提示。未变快照跳过数据库重写；持久化序列化与磁盘写入不持有 Hook 引擎锁。隐藏记录不继续周期性探测进程。

真实恢复使用已退出的验收会话和可靠原生 UUID/CWD。用户后续明确：重新打开原历史会话即可，已知 TUN 下的 API 403 不阻塞恢复项，无需模型回答来确认。用户随后指出已关闭会话没有恢复：此前 --print 调用已退出，尚未打开可持续交互的原历史会话，应撤回完整通过判断。模型记忆复述可作为额外证据，不是本次必需条件；交互式恢复、新 Runtime/ReturnTarget 和菜单行为仍须实际验证。

用户随后提供了一条 Claude 历史 UUID，已在记录的 CWD 运行真实 `claude --resume`。历史含一条用户消息与一条 API 错误，没有此前成功的助手回复。CLI 接受 UUID 并发出 `SessionStart(source=resume)`；无工具的上下文复述请求返回 `403 Your IP address is not allowed`。用户说明这是已知 TUN 问题，403 不作为 blocker；但 --print 验收结束即退出，未打开可继续使用的交互式会话，因此完整恢复项仍待验收。真实 `SessionEnd(reason=other)` 已使隔离 v0.5 从 Running 转为 CLOSED，Attention=0，但未经过真实 NEEDS YOU，不能勾选完整生命周期项。ReturnTarget 来自验收 Python 启动器，仅为 App 能力，不能证明真实终端 Return。没有修改用户 Hook 配置或项目文件；原历史追加了本次验收请求及 API 错误。Codex UUID/目录仍待提供。GUI 与原生通知仍待验收；真实本地 Homebrew RC 升级已完成。

本地覆盖已完成：从当前源码重建双架构 RC 并通过签名/版本/checksum smoke；使用仅本地 file URL 的临时 Formula 执行真实 brew upgrade，随后恢复公开 tap 原文件。运行 agentbell install 后，用户 Applications 中的 App 已重启，IPC 与 CLI 均报告 0.5.0-rc.1；原有六条身份均保留，SQLite integrity_check=ok。旧 0.4.0 keg 保留供回退，App/配置/数据库备份在 `/tmp/agentbell-before-rc-20261006-150258`。未重启 agent、Tabby 或 GoLand。

## RC.2 CLOSED 列表稳定性

用户反馈短 ID 持续换位，实际四条会话的身份和关闭时间均未变化。升级后的首次进程检测将多条记录在同一时刻标记 CLOSED，原排序只比较 UpdatedAt，没有相同时间的次级排序，导致 map 遍历顺序进入 UI。修复为时间倒序、相同时间按完整会话 ID 升序；前五条成员也固定。回归测试在原实现失败，修复后连续 200 次快照稳定；attention 与主包 race、vet 通过。RC.2 已重新编译覆盖本地安装，CLI 与运行 App 均为 0.5.0-rc.2，六条记录与配置保留。
