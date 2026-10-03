## v0.3 发布状态

v0.3.0 已正式发布。维护者在收到 RC 报告后明确授权发布，接受真实 GUI notification-click 与 stale GUI fallback 尚未完成的限制；release notes 与 README 保留这一事实。PR #4 已合并，main/tag 双架构 CI、正式发布包验证以及真实 Homebrew 升级完成。

## tmux GUI E2E

Tabby + tmux、Terminal + tmux 均未完成。Computer Use 分别拒绝访问 org.tabby 和 com.apple.Terminal（“for safety reasons”）；没有使用其他 UI 控制路径绕过限制。通知权限只读检查为 authorized，但通知 dispatch 不证明点击返回。

真实 tmux CLI E2E 已通过：独立 socket、单 attached client、分屏、相同 cwd、多 window/session、zoom、关闭 pane/window/session、重启 server 后复用 pane ID、多个 client 安全降级。它不能替代上述 GUI 验收。

## stale-context GUI E2E

未完成。真实 CLI stale pane/restarted server 测试及 composite 安全降级回归通过；旧通知点击、不误选同 cwd/title 新 pane、原终端 app fallback 的 GUI 证据仍缺失。日志会记录 pane_not_found / server_identity_mismatch 等明确 reason；binding failure 也保留原因。

## Cold Start 调查

独立串行采样，无并发 build/test：新路径首次一次、20 次 fresh-path cold-ish、20 次 warm。另做 21 次真实 native notify（第一调用 + 20 次 warm，helper 授权检查已预热）。原始数据见 COLD_START_V03.json、NOTIFY_TIMING_V03.json；可用 scripts/cold-start.py 复现。

| 路径 | first-run ms | p50 ms | p95 ms | max ms |
|---|---:|---:|---:|---:|
| detection 新路径首次 | 843.8 | 843.8 | 843.8 | 843.8 |
| detection cold-ish（20） | 550.4 | 353.7 | 648.4 | 669.9 |
| detection warm（20） | 29.2 | 24.2 | 27.0 | 29.2 |
| notify warm（20） | 69.9 | 73.8 | 80.6 | 82.7 |

新路径首次 app total 32.0ms、tmux 15.1ms，其余 811.8ms 在 Go 用户代码计时入口外。cold-ish app total p95 38.1ms/max 38.4ms。notify warm app total p95 76.4ms/max 78.4ms，达到本次 <100ms 目标。

Timing 通过 AGENTBELL_DEBUG_TIMING=1 输出 stderr，默认静默。startup 表示 Go 初始化后的入口时间；total 覆盖用户代码同步工作，tmux_detect 是 surface_detect 的子阶段，不可相加。config_load/adapter_parse/notification_dispatch 在 notify 路径计量。外部差值包含 runtime 初始化、OS launch/wait、调度等，不能单凭差值认定 Gatekeeper/codesign/DNS。此路径无 DNS 访问；没有 OS 缓存清空、系统级 trace 或首次冷 native helper 的归因证据。完整 cold notification 仍需补测，不能宣称全部环境无秒级等待。

## Provider Maturity Matrix

| Provider | Maturity | Validation |
|---|---|---|
| Tabby | Stable | bridge regression；v0.3 tmux GUI pending |
| Terminal.app | Stable | identity regression；v0.3 tmux GUI pending |
| GoLand | Stable | 2025.3 / build 253；真实 Java↔Go bridge |
| tmux | Stable | 真实 CLI lifecycle；composite GUI pending |
| Generic | Stable | app/project fallback regression |
| iTerm2 | Experimental | fixture only；GUI pending |
| WezTerm | Experimental | fixture only；GUI pending |

Maturity 独立于 implemented capabilities，surfaces 与 JSON 均提供。Context Protocol v1 已冻结；v0.2 无 Layers JSON 的 live 与 expired/fallback 回归通过。

## Remote CI

[main CI](https://github.com/Han1enG/agent-bell/actions/runs/37134610569) 与 [v0.3.0 tag/release CI](https://github.com/Han1enG/agent-bell/actions/runs/37134773403) 均 success，发布提交 f56c27d392d0aedcf5e8a2a1f3fc9e5c5785f2a4。macos-15 与 macos-15-intel 均通过 race/vet、Node、Python、真实 Java bridge、真实 tmux、native 双架构、codesign、release smoke。

首次 fixture PTY 退出超时通过持续排空测试 PTY 和显式 detach 修复；Intel click-time tmux 命令的 75ms 超时改为 250ms，hook detection 仍共享 90ms。Tabby/JetBrains 只读 bridge probe 前置，避免祖先查询先耗尽预算。未放宽身份检查。修正后的候选和 main/tag 全部重新通过。

## Homebrew Upgrade

正式 tap commit dbf245c 已推送。真实 `brew upgrade agentbell` 完成 0.2.0 → 0.3.0；`agentbell install`、`agentbell doctor`（exit 0，Everything looks good）与 `brew test agentbell` 均通过。

升级前后核对：既有 Claude/Codex hooks 与其它配置内容保留；旧 config.toml 逐字节不变且 doctor 成功加载；受管 Tabby 插件从 0.2.1 → 0.3.0，GoLand 受管集成 current。对旧受管插件的隔离副本添加用户修改后，用已安装 v0.3 CLI 实测安装：输出 modified; preserved，修改文件 hash 不变。

运行中的 Tabby/GoLand 未被自动重启。新 bridge 需要用户重启应用后在新的本地终端加载；已有运行终端不注入新环境。

## Release Assets

正式 [v0.3.0 Release](https://github.com/Han1enG/agent-bell/releases/tag/v0.3.0) 已上传 arm64/amd64 tar.gz 和 checksums.txt，release body 非空且明确说明未完成的 GUI 验收。已下载实际发布包，双包 SHA256、deep strict codesign、当前架构 CLI 与 surfaces JSON smoke 全通过。

- arm64 SHA256: 2adc899ce71e2bd8abb30ddc0c3acd49c930cb1c43021a6e5e24996fd7a08089
- amd64 SHA256: 8c14de6730f8e820c6b0cfe5d82981721799900f134998fd5524f55aa1e9f99d

## 已知限制

GUI 控制策略拒绝两个必测终端；不完整的通知点击链路作为明确接受的发布限制保留。冷路径系统归因及 cold native dispatch 尚不完整。iTerm2/WezTerm 无真实 GUI Matrix。GoLand 仅 2025.3 / build 253。维护者明确接受这些已知限制后授权发布；没有将 CLI/fixture 结果宣称为真实 GUI 通过。

## 发布链接

[GitHub Release v0.3.0](https://github.com/Han1enG/agent-bell/releases/tag/v0.3.0) · [已合并 PR #4](https://github.com/Han1enG/agent-bell/pull/4) · [发布 CI](https://github.com/Han1enG/agent-bell/actions/runs/37134773403) · [Homebrew tap 更新](https://github.com/Han1enG/homebrew-agentbell/commit/dbf245c)
