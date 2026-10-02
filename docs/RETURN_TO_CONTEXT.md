# AgentBell v0.2.1 implementation report

## 实现了什么

移除 native host 的 Terminal/iTerm 固定点击逻辑。新增 Surface 模型、来源 App 检测、返回降级、原生按钮、配置、诊断和 Tabby 桥接。保留原有 README/docs 改动。已安装到本机用户插件目录，保留原 hook 入口。

## Return-to-Context 架构

Hook adapter → AgentEvent → Surface detection → ReturnTarget → native notification payload → 点击/按钮 → 独立 `agentbell return` 进程 → provider fallback。返回失败与原 Hook 分离。没有新增 AgentBell daemon、数据库或 registry。

## ReturnTarget 数据结构

`Surface`, `AppName`, `AppBundleID`, `ContextID`, `WindowID`, `AgentSessionID`, `CWD`, `Capability`。字段可空。`SurfaceType` 与 `ReturnCapability` 已定义。AgentEvent 通过可选指针持有目标。ContextID 不由 Core 解释。

## SurfaceProvider 设计

`Name`, `Detect`, `CanHandle`, `Return`。Manager 逐级遍历 provider。GenericProvider 实现 App/Project；独立 `internal/surface/tabby` 实现实验 Exact。窗口能力留有接口，不虚报支持。

## Generic Provider 如何工作

优先 `/usr/bin/open -b <bundle-id>` 返回来源 App。无法激活时验证绝对 cwd 仍存在且为目录，再使用 configured fallback。auto 优先检测 /Applications 下的已安装终端，最后使用 Terminal。不会为失效 cwd 打开用户 home。所有子进程使用参数数组和超时，不拼 shell/AppleScript。

## 来源 App 如何识别

显式 AgentBell 协议优先。Tabby 的 `TERM_PROGRAM=Tabby` 已从官方源码验证，bundle ID 从本机 Info.plist 读取。其他来源沿当前 hook PID 的父进程查找 `.app/Contents` 并读取 CFBundleIdentifier。不会使用无关前台 App。识别失败保留 cwd。尚未逐一验证其他终端环境变量，因此未猜测支持。

## Capability fallback 顺序

Exact Context → Window → Origin App → Project。空字段跳过，provider 返回错误后继续。Generic 并不因收到未知 ContextID 而宣布 Exact；Tabby 需通过 list 请求确认 ContextID 存活。关闭 Tab 后 Exact 返回失败并降级。

## 通知 CTA 如何工作

UNNotificationAction 标识为 `RETURN_TO_CONTEXT`。标题按能力显示「返回会话 / 返回窗口 / 打开 App / 打开项目」。返回 JSON 存在 userInfo，默认点击和按钮调用同一入口。UI 显示来源与项目名，cwd 不作标题。helper 从自身 bundle 找 packaged agentbell，不执行 payload 中的路径。无 helper 的开发用 osascript 路径缺少 CTA/click，发布包包含 helper。

## Tabby PoC 结果

[官方 AppService](https://docs.tabby.sh/classes/AppService.html) 提供 selectTab；[HostWindowService](https://docs.tabby.sh/classes/HostWindowService.html) 提供 bringToFront；本机安装包亦包含这些方法。已建立小型 Angular 插件：terminal leaf / 普通 Tab 对象各获随机 ContextID；每窗口临时 Unix socket 接收 list/focus/status；调用公开 selectTab/bringToFront 和 split focus 接口。监听 TabOpened/TabAdded 为新本地 shell 注入协议变量；克隆每 Tab 的 profile options，不改共享 profile。桥接层测试和本机新 Tab E2E 通过，未采用 URL/Accessibility/PID hack。

## Tabby Exact Return 是否成功

**本机新本地 Tab 的完整 Exact Return 已通过用户实测。** 插件正常加载，外部 focus 切换三个真实 Tab，status 确认 activeTab。用户在新 Tab 中执行 surface detect 得到非空 ContextID 与 exact_context，然后确认通知点击返回原 Tab。新 Tab 自动环境注入与正常点击链路已经验证；关闭 Context、App 缺失和分屏 pane 的完整 UI 场景仍未全部验收。

## tmux 支持情况

未实现 tmux provider，不强制依赖 tmux。运行在 tmux 中时，若仍能识别来源 App，支持 App 返回；不宣称 pane/session 精确返回。

## doctor 新增检查

Surface Integration 输出来源 Surface、App、bundle、能力、Exact 和 Window 状态、Generic fallback 与 tmux limitation。App 激活标记为未实际操作；不会将缺少 Exact 当作失败。`surface detect` 输出 JSON，检测细节进入现有 debug 日志。

## 测试结果

- 全部 Go tests 通过，包括目标选择、未知 App、fallback 四层顺序、过期 Context、参数安全、无效 cwd、CTA、notification payload、配置和相同 cwd 不同 Context 的去重。
- Tabby Go provider 通过真实临时 Unix socket 的 list/focus/expired 通信测试。
- Node bridge 三 Tab 测试通过，A/C 同名，B focus 与关闭后 failure 正确。
- go vet 与 git diff --check 通过。
- arm64 / amd64 原生应用构建及 codesign 校验通过，使用已安装 MacOSX15.4 SDK。系统默认 MacOSX27.0 SDK 与当前 linker 不兼容；未改动系统工具链。
- 用户已确认无插件点击激活已有 Tabby，以及安装插件后新本地 Tab exact_context 与通知点击返回原 Tab。未知 App、关闭 Context/App、分屏 pane UI E2E 尚未全部执行。

## 当前支持矩阵

| Surface | App | Window | Exact |
|---|---|---|---|
| Tabby | 检测 + bundle activation | — | 新本地 Tab 已实测 |
| Terminal.app | bundle activation | — | 原 TTY + 进程身份；用户点击已验证 |
| GoLand 2025.3 | bundle activation | — | 插件加载后新本地 Tab；用户确认可用 |
| iTerm2 | best effort ancestry | planned | planned |
| 其他 terminal/editor | best effort ancestry | — | — |
| tmux | 配合来源 App | — | planned |
| Generic cwd | — | — | 项目 fallback |

## 当前 limitation

Tabby 插件已安装，支持新本地 Tab 自动环境注入。恢复的旧 PTY、已有 shell/agent 和 SSH 不注入；split focus 接口已接入但 pane E2E 未验收。ContextID 在重启后失效，按设计降级。macOS 可在悬停/展开时显示通知 action。来源检测在进程访问受限、SSH 或祖先已退出时可能只能得到 Project。本机常规 Exact 点击已验收，关闭 Context/App 的负向 UI 场景仍待补齐，不能据此宣称所有兼容场景均已通过。

## 下一步建议

本机安装、新 Tab 环境注入、真实 focus 和通知 CTA 返回已验证。接下来按 integrations/agentbell-tabby/README.md 补齐同 cwd、关闭 Context、未知 App、App 缺失 fallback 和 pane UI 验收；完善可分发的插件包，随后按需要添加 tmux。不会继续开发 Menu Bar / Dashboard。

## 2026-10-02 本地集成验收进展

用户确认不带插件的通知点击可激活已有 Tabby。随后明确授权安装 PoC 和重启 Tabby。插件已安装到用户插件目录；启动日志证明 Angular 模块加载成功。AgentBell 外部 focus 已切换三个真实 Tab，status API 验证对应 activeTab，不再仅有测试替身证据。

新增本地 Tab 的协议环境注入：监听 TabOpened/TabAdded，克隆每 Tab 的 local profile env，保留其他变量。跳过现有 session、恢复的 PTY 和 SSH。相关自动测试通过。真实 shell 继承与通知点击返回原 Tab 正等待用户在新 Tab 中验证；计算机控制工具禁止操作 Tabby UI，因此不以另一种 UI 自动化绕过限制。

Tabby 官方 run CLI 在本机启动测试命令时遇到 cliui/stripAnsi 错误。未修补 Tabby、未添加终端输入 workaround。测试命令启动失败，自动环境注入的 shell 继承不能据此视为通过。

用户随后实测新本地 Tab：surface detect 输出有效 ContextID 与 exact_context，并确认「返回了」。至此自动环境注入与 native 通知点击回原 Tab 的正常链路在本机通过。之前“等待用户确认”的条目由此结果取代。

## 自动安装与生命周期

按用户最新选择，`agentbell install` 检测到 Tabby 后默认自动安装，不询问、不重启。插件以 go:embed 随 AgentBell 二进制携带，不需 npm/网络/源码目录。`--skip-tabby` 记录跳过偏好，保留已安装插件；`--tabby` 可重新启用。

受管插件保存所有权和文件 hash；重复安装更新旧版本或修复缺失文件，拒绝覆盖非受管/本地修改/符号链接路径。完全匹配的旧手工安装可纳入管理。卸载只删除受管且未修改的文件，保留其他插件及额外文件。doctor 显示安装、管理、版本与当前 bundle 是否一致；doctor --fix 只修复此前启用的集成。

已添加并通过自动测试：默认检测安装、无 Tabby 跳过、记住 opt-out、显式重新启用、重复安装、旧版本升级、缺失文件修复、手工副本接管、保护其他插件/用户修改和符号链接；CLI skip/enable 与 dry-run 也覆盖。完整 arm64/amd64 release 重新构建并验证签名。

## Terminal.app provider

已新增独立 internal/surface/terminal provider。Terminal 自带脚本字典确认 tab.tty、window.selected tab、miniaturized、index、frontmost 的官方属性。固定 AppleScript 通过 osacompile；TTY 作为 argv 参数，不插入脚本源码。

来源通过 bundle ancestry 或原生 Apple_Terminal 标识识别。记录同 TTY 最上层祖先会话的 PID、lstart 与 TTY，点击再次校验以防过期或复用；不按 cwd 匹配会话，不执行 do script。不在 hook 检测时请求 Apple events 授权。tmux 和缺少稳定 TTY 的来源保持 App 能力。

测试包括会话祖先选择、无 TTY、tmux、明确协议优先、现有会话安全 focus、PID 启动时间/TTY 变化、恶意输入和自动化权限拒绝后的 App fallback。全套 Go tests 和 vet 通过。Terminal 用户真实通知点击验收仍在进行，未宣称已通过。

用户报告 exact_context 通知仍无法返回。其 ContextID 中 lstart 为中文；用同一真实 PID 分别运行 LC_ALL=C 与 zh_CN.UTF-8 的 ps，确认输出不同，导致后台校验误判过期。现统一 ps 的 LC_ALL=C，并添加真实进程跨 locale 回归测试。点击各次 provider 尝试及失败原因写入 agentbell.log，成功 fallback 也保留原定位错误。需要重新发出通知；旧通知仍携带旧语言格式的身份。用户真实点击复验待确认。

再次复验日志确认 Terminal 脚本在 `every tab of item 2 of every window` 返回 -1728，然后 App fallback 成功。只读查询窗口 ID / Tab TTY，确认窗口列表包含 missing value 的辅助窗口，真实 /dev/ttys005 可通过有效窗口 ID 找到。脚本改为快照固定窗口 ID、排除非整数 ID、跳过已关闭窗口的 -1728，保留其他权限/脚本错误；按 tab.selected 选中会话并通过固定 ID 前置窗口。未读取终端内容或向会话发送命令，真实通知点击复验待确认。

用户随后确认 Terminal.app「可以了」，正常通知点击已通过本机验收。

## GoLand / JetBrains 集成

本机 GoLand 2025.3.5.1（253.33813.70）内置 API 编译的插件包已加入。LocalTerminalCustomizer 为新本地会话注入 UUID；Classic ShellStartupOptions 与 Reworked TerminalView.startupOptionsDeferred 提供同一 ID。通过 ContentManager 选中对应 Tab、激活 Terminal 工具窗口及 IDE frame。私有 0700 目录 / 0600 Unix socket 只接受 list/focus，无终端输入或输出读取操作。过期/未注入 ID 保持 App fallback。

自动安装使用 product-info.json 的受校验 dataDirectoryName，兼容范围限制 253 build；包随 Go binary embed，无网络依赖。不覆盖非受管、修改或符号链接文件。dry-run/doctor/卸载及 --goland / --skip-goland 已接入。插件编译、Go 安装保护与真实 socket 回归测试通过；Java↔Go 实际 IPC 验证了唯一 ID、list/focus、精确检测、过期、文件权限、请求边界与清理。真实 IDE 新 Tab 环境继承、通知点击还待插件加载后验证，未提前宣称成功。

用户随后确认「可以了，发布吧」，据此记录 GoLand 正常返回路径本机可用。未取得两种引擎、分屏和远程场景分别实测的结果，不扩展此确认的验收范围。
