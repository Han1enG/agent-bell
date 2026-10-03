## v0.3 发布状态

维护者在收到 RC 报告后明确授权发布 v0.3.0，接受真实 GUI notification-click 与 stale GUI fallback 尚未完成的限制。候选代码与报告的远端双架构 CI 均 success。发布和 Homebrew 升级按本次授权继续执行；最终状态见 GitHub Release 与后续验收记录。

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

代码提交 143252e1d5363e7e5e24430ecfc7d8963d1f9c6a 的 [正式 CI](https://github.com/Han1enG/agent-bell/actions/runs/37120836877) success：macos-15 与 macos-15-intel 均通过 race/vet、Node、Python、真实 Java bridge、真实 tmux、native 双架构、codesign、release smoke。两个 runner 的验证包均上传为 Actions artifacts。首轮 fixture PTY 退出超时已通过持续排空测试 PTY 和显式 detach 修复；未放宽身份验证。后续 Intel 复验发现 click-time tmux 命令 75ms 超时过紧，以及 bridge probe 被祖先查询耗尽预算。tmux click-time 命令改为 250ms；hook detection 仍共享 90ms。Tabby/JetBrains 只读 bridge probe 前置，身份仍需原 bridge 匹配。修正后完整双架构 CI 已重新通过（上述代码提交）。

## Homebrew Upgrade

未验证 v0.3 正式升级。当前安装是 0.2.0（另有 0.1.1 keg），tap formula 仍指向 0.2.1。维护者已明确授权在保留 GUI 限制的前提下发布；正式发布后再更新 tap 并执行真实升级，不把旧版本 brew upgrade 当作 v0.3 升级证据。旧配置、hook 保留、受管升级和用户修改保护的回归测试通过；真实 v0.3 upgrade/install/doctor 仍待正式发布。

## Release Assets

本地 arm64/amd64 签名候选包及 checksums 位于 /private/tmp/agentbell-v03-closure。build、deep strict codesign verification、双包 checksum、当前架构执行、surfaces JSON smoke 通过。包内版本为 0.3.0，但这些是未发布 RC 产物，不代表存在正式 tag/release。docs/RELEASE_NOTES_V03.md 为准备好的 release body，workflow 已设置 body_path。

## 已知限制

GUI 控制策略拒绝两个必测终端；不完整的通知点击链路作为明确接受的发布限制保留。冷路径系统归因及 cold native dispatch 尚不完整。iTerm2/WezTerm 无真实 GUI Matrix。GoLand 仅 2025.3 / build 253。未 merge main、未创建正式 tag、未更新 tap。

## 发布链接

[候选 draft PR #4](https://github.com/Han1enG/agent-bell/pull/4) · [双架构成功 CI / RC artifacts](https://github.com/Han1enG/agent-bell/actions/runs/37120836877)。没有 v0.3.0 正式发布链接。
