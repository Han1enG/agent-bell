import AppKit
import SwiftUI
import UserNotifications

struct Session: Decodable, Identifiable {
    let id, agent, project, status, attention, summary: String
    let title: String?
    var displayTitle: String { (title?.isEmpty == false ? title : nil) ?? project }
    let started_at, updated_at: String
    let finished_at, attention_at: String?
    let return_target: JSONValue?
    var label: String {
        switch attention {
        case "input": return "Waiting for input"
        case "approval": return "Approval required"
        case "error": return "Failed"
        default: return status == "done" ? "Ready to continue" : "Working"
        }
    }
    var symbol: String { attention == "none" ? (status == "done" ? "bubble.left" : "circle.fill") : "exclamationmark.circle" }
}
indirect enum JSONValue: Codable {
    case object([String: JSONValue]), array([JSONValue]), string(String), number(Double), bool(Bool), null
    init(from decoder: Decoder) throws {
        let c = try decoder.singleValueContainer()
        if c.decodeNil() { self = .null }
        else if let v = try? c.decode(String.self) { self = .string(v) }
        else if let v = try? c.decode(Bool.self) { self = .bool(v) }
        else if let v = try? c.decode(Double.self) { self = .number(v) }
        else if let v = try? c.decode([String:JSONValue].self) { self = .object(v) }
        else { self = .array(try c.decode([JSONValue].self)) }
    }
    func encode(to encoder: Encoder) throws {
        var c = encoder.singleValueContainer()
        switch self {
        case .object(let v): try c.encode(v)
        case .array(let v): try c.encode(v)
        case .string(let v): try c.encode(v)
        case .number(let v): try c.encode(v)
        case .bool(let v): try c.encode(v)
        case .null: try c.encodeNil()
        }
    }
    var capability: String {
        if case .object(let fields) = self, case .string(let value) = fields["Capability"] { return value }
        return "project"
    }
}
struct SessionSnapshot: Decodable {
    var schema_version = 1
    var needs_you: [Session] = []
    var working: [Session] = []
    var recent: [Session] = []
    var paused = false
    var storage_error: String?
    var recent_limit: Int?
    var observed_agents: [String]?
}
struct NativeNotification: Decodable {
    let kind, title, subtitle, body, return_target, action, source, type, session_id: String
}
final class Model: ObservableObject {
    @Published var state = SessionSnapshot()
    @Published var failure: String?
    @Published var notificationFailure: String?
    var onChange: (() -> Void)?
    let binary = Bundle.main.bundleURL.appendingPathComponent("Contents/MacOS/agentbell")
    static var disabledURL: URL {
        FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Library/Application Support/AgentBell/user-disabled")
    }
    func quit() {
        do {
            try FileManager.default.createDirectory(at: Self.disabledURL.deletingLastPathComponent(), withIntermediateDirectories: true)
            try Data("User quit AgentBell\n".utf8).write(to: Self.disabledURL, options: .atomic)
            NSApp.terminate(nil)
        } catch { failure = "Unable to disable AgentBell: \(error.localizedDescription)" }
    }
    func command(_ args: [String], completion: (() -> Void)? = nil, success: (() -> Void)? = nil) {
        DispatchQueue.global(qos: .userInitiated).async {
            defer { if let completion = completion { DispatchQueue.main.async(execute: completion) } }
            let p = Process(); p.executableURL = self.binary; p.arguments = args
            let output = Pipe(); p.standardOutput = FileHandle.nullDevice; p.standardError = output
            do {
                try p.run()
                let data = output.fileHandleForReading.readDataToEndOfFile(); p.waitUntilExit()
                if p.terminationStatus == 0, let success = success { DispatchQueue.main.async(execute: success) }
                if p.terminationStatus != 0 {
                    DispatchQueue.main.async { self.failure = String(data: data, encoding: .utf8) ?? "Action failed." }
                }
            } catch { DispatchQueue.main.async { self.failure = error.localizedDescription } }
        }
    }
    func returnTo(_ target: JSONValue, success: @escaping () -> Void) {
        guard let data = try? JSONEncoder().encode(target), let text = String(data: data, encoding: .utf8) else { return }
        command(["return", text], success: success)
    }
}
private struct RemoveButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        RemoveButtonFace(configuration: configuration)
    }
    private struct RemoveButtonFace: View {
        let configuration: ButtonStyle.Configuration
        @State private var hovering = false
        var body: some View {
            configuration.label
                .foregroundColor(hovering ? Color.primary : Color.secondary)
                .background(Color.primary.opacity(configuration.isPressed ? 0.16 : (hovering ? 0.08 : 0)), in: RoundedRectangle(cornerRadius: 5))
                .scaleEffect(configuration.isPressed ? 0.94 : 1)
                .animation(.easeOut(duration: 0.12), value: hovering)
                .animation(.easeOut(duration: 0.08), value: configuration.isPressed)
                .onHover { hovering = $0 }
        }
    }
}
private let attentionPanelWidth: CGFloat = 460

struct AttentionView: View {
    @ObservedObject var model: Model
    @State private var showMore = false
    var close: () -> Void
    func age(_ value: String, now: Date) -> String {
        let f = ISO8601DateFormatter(); f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let date = f.date(from: value) ?? ISO8601DateFormatter().date(from: value) ?? now
        let seconds = max(0, Int(now.timeIntervalSince(date)))
        if seconds < 60 { return "\(seconds)s" }
        if seconds < 3600 { return "\(seconds / 60)m" }
        if seconds < 86400 { return "\(seconds / 3600)h" }
        return "\(seconds / 86400)d"
    }
    func visibleSummary(_ session: Session) -> String {
        let summary = session.summary.trimmingCharacters(in: .whitespacesAndNewlines)
        if summary == "Claude is waiting for your input" || summary == "Codex is waiting for your input" || summary == session.label { return "" }
        return summary
    }
    @ViewBuilder func section(_ name: String, _ sessions: [Session]) -> some View {
        HStack {
            Text(name).tracking(0.6)
            Spacer()
            Text("\(sessions.count)").monospacedDigit()
        }.font(.system(size: 11, weight: .semibold)).foregroundColor(.primary.opacity(0.65)).padding(.top, 9).padding(.bottom, 3).accessibilityAddTraits(.isHeader)
        ForEach(sessions) { session in
            VStack(alignment: .leading, spacing: 6) {
                HStack(spacing: 8) {
                    Image(systemName: session.symbol).font(.system(size: 14)).foregroundColor(session.attention == "none" ? .secondary : .orange).accessibilityHidden(true)
                    Text(session.displayTitle).font(.system(size: 14, weight: .semibold)).foregroundColor(.primary).lineLimit(1).help(session.displayTitle)
                    Spacer(minLength: 8)
                    if let target = session.return_target {
                        Button { model.returnTo(target, success: close) } label: {
                            HStack(spacing: 4) { Text(target.capability == "app" ? "Open App" : (target.capability == "project" ? "Open Project" : "Return")); Image(systemName: "arrow.up.forward") }
                                .font(.system(size: 12, weight: .medium)).padding(.horizontal, 9).padding(.vertical, 5)
                                .background(Color.primary.opacity(0.07), in: RoundedRectangle(cornerRadius: 6))
                        }.buttonStyle(.plain)
                            .help(target.capability == "exact_context" ? "Return to this session" : "Return capability: \(target.capability)")
                            .accessibilityLabel("Return to \(session.project), \(session.agent)")
                    }
                    if session.status == "done" {
                        Button { model.command(["attention-control", "remove_recent", session.id]) } label: {
                            Image(systemName: "xmark").font(.system(size: 10, weight: .semibold)).frame(width: 22, height: 22)
                        }.buttonStyle(RemoveButtonStyle()).help("Remove from this list; the agent session stays open")
                            .accessibilityLabel("Remove \(session.project) from Ready")
                    }
                }
                TimelineView(.periodic(from: .now, by: 1)) { context in
                    Text("\(session.agent == "claude" ? "Claude" : "Codex") · \(session.label) · \(age(session.attention_at ?? session.finished_at ?? session.started_at, now: context.date))")
                        .font(.system(size: 12)).monospacedDigit().foregroundColor(.primary.opacity(0.65))
                }
                if !visibleSummary(session).isEmpty {
                    Text(visibleSummary(session)).font(.system(size: 13)).foregroundColor(.primary.opacity(0.88)).lineSpacing(3).fixedSize(horizontal: false, vertical: true)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
            }.padding(10).frame(maxWidth: .infinity, alignment: .leading)
                .background(Color.primary.opacity(0.035), in: RoundedRectangle(cornerRadius: 9))
        }
    }
    var header: some View {
        HStack {
            Text("AgentBell").font(.system(size: 15, weight: .semibold))
            Spacer()
        }
    }
    var sessionContent: some View {
            VStack(alignment: .leading, spacing: 5) {
                if !model.state.needs_you.isEmpty { section("NEEDS YOU", model.state.needs_you) }
                if !model.state.working.isEmpty { section("WORKING", model.state.working) }
                if !model.state.recent.isEmpty { section("READY", Array(model.state.recent.prefix(showMore ? model.state.recent.count : (model.state.recent_limit ?? 5)))) }
                if model.state.needs_you.isEmpty && model.state.working.isEmpty && model.state.recent.isEmpty {
                    VStack(spacing: 7) {
                        Image(systemName: "checkmark.circle").font(.system(size: 23))
                        Text("All caught up").font(.system(size: 13, weight: .medium))
                        Text("Restart agents that were running before installation. New events will appear here.").font(.caption).multilineTextAlignment(.center)
                    }.foregroundColor(.secondary).frame(maxWidth: .infinity).padding(.vertical, 25)
                }
                if model.state.recent.count > (model.state.recent_limit ?? 5) { Button(showMore ? "Show Less" : "Show More") { showMore.toggle() }.buttonStyle(.link) }
            }.frame(maxWidth: .infinity, alignment: .leading)
                .fixedSize(horizontal: false, vertical: true)
    }
    var displayedCount: Int {
        model.state.needs_you.count + model.state.working.count + min(model.state.recent.count, showMore ? model.state.recent.count : (model.state.recent_limit ?? 5))
    }
    @ViewBuilder var sessionList: some View {
        // Small lists use their natural height: no measured-scroll feedback loop.
        if displayedCount <= 2 {
            sessionContent
        } else {
            ScrollView { sessionContent }.frame(height: 390)
        }
    }
    @ViewBuilder var diagnostics: some View {
        if let error = model.failure { Text(error).font(.caption).foregroundColor(.secondary).lineLimit(3) }
        if let error = model.notificationFailure { Text(error).font(.caption).foregroundColor(.secondary).lineLimit(3) }
        if let error = model.state.storage_error, !error.isEmpty { Text("Session history unavailable").font(.caption).help(error) }
    }
    var footer: some View {
        HStack {
            Button { model.command(["attention-control", model.state.paused ? "resume" : "pause"]) } label: {
                Image(systemName: model.state.paused ? "bell.slash" : "bell").font(.system(size: 14)).frame(width: 24, height: 24)
            }.buttonStyle(.plain)
                .help(model.state.paused ? "Resume notifications" : "Pause notifications")
                .accessibilityLabel(model.state.paused ? "Resume notifications" : "Pause notifications")
            Spacer()
            Menu {
                Menu("Agent Connections") {
                    Text((model.state.observed_agents ?? []).contains("claude") ? "Claude: event received this App run" : "Claude: awaiting a real event")
                    Text((model.state.observed_agents ?? []).contains("codex") ? "Codex: event received this App run" : "Codex: awaiting a real event")
                    Divider()
                    Text("Restart agents that were running before installation.")
                }
                Divider()
                Button("Clear Ready") { model.command(["attention-control", "clear_recent"]) }.disabled(model.state.recent.isEmpty)
                Divider()
                Button("Open Config…") { model.command(["open-config"]) }
                Button("Quit AgentBell") { model.quit() }.keyboardShortcut("q")
            } label: { Image(systemName: "gearshape").font(.system(size: 14)).frame(width: 24, height: 24) }
            .menuStyle(.borderlessButton).menuIndicator(.hidden).fixedSize().help("AgentBell actions").accessibilityLabel("AgentBell actions")
        }.font(.system(size: 11)).foregroundColor(.secondary).padding(.top, 3)
    }
    var body: some View {
            VStack(alignment: .leading, spacing: 6) {
                header
                Divider()
                sessionList
                Divider()
                diagnostics
                footer
            }.padding(14).frame(width: attentionPanelWidth).fixedSize(horizontal: false, vertical: true).onExitCommand { close() }
    }

}
// A borderless panel avoids NSPopover's system-drawn arrow using public APIs.
private final class AttentionPanel: NSPanel {
    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { false }
}
final class AppDelegate: NSObject, NSApplicationDelegate, UNUserNotificationCenterDelegate {
    let model = Model()
    var statusItem: NSStatusItem!
    var panel: NSPanel!
    var panelBackground: NSVisualEffectView!
    var panelController: NSHostingController<AttentionView>!
    var outsideClickMonitor: Any?
    var localClickMonitor: Any?
    var resizeObservation: NSObjectProtocol?
    var core: Process?
    var input: Pipe?
    var output: Pipe?
    var buffer = Data()
    var centerEnabled = true
    var terminating = false
    var restartCount = 0
    var visibilityObservation: NSKeyValueObservation?
    let seenKey = "AgentBell.lastViewedCompletion"
    func completionTime(_ session: Session) -> TimeInterval {
        guard let value = session.finished_at else { return 0 }
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return (formatter.date(from: value) ?? ISO8601DateFormatter().date(from: value))?.timeIntervalSince1970 ?? 0
    }
    func markRecentSeen() {
        let latest = model.state.recent.map(completionTime).max() ?? 0
        if latest > UserDefaults.standard.double(forKey: seenKey) {
            UserDefaults.standard.set(latest, forKey: seenKey)
        }
    }
    var hasUnseenCompletion: Bool {
        let seen = UserDefaults.standard.double(forKey: seenKey)
        return model.state.recent.contains { completionTime($0) > seen }
    }
    func applicationDidFinishLaunching(_ notification: Notification) {
        UNUserNotificationCenter.current().delegate = self
        let configTask = Process(); configTask.executableURL = model.binary; configTask.arguments = ["attention-config"]
        let configOutput = Pipe(); configTask.standardOutput = configOutput; configTask.standardError = FileHandle.nullDevice
        if (try? configTask.run()) != nil {
            let data = configOutput.fileHandleForReading.readDataToEndOfFile(); configTask.waitUntilExit()
            if let object = try? JSONSerialization.jsonObject(with: data) as? [String: Any], let enabled = object["Enabled"] as? Bool { centerEnabled = enabled }
        }
        if !centerEnabled {
            // Retain the delegate for cold notification clicks in CLI-only mode.
            DispatchQueue.main.asyncAfter(deadline: .now() + 30) { NSApp.terminate(nil) }
            return
        }
        do {
            if FileManager.default.fileExists(atPath: Model.disabledURL.path) { try FileManager.default.removeItem(at: Model.disabledURL) }
        } catch {
            model.failure = "Unable to re-enable AgentBell: \(error.localizedDescription)"
        }
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        statusItem.autosaveName = "AgentBell.AttentionCenter"
        statusItem.behavior = []
        statusItem.isVisible = true
        visibilityObservation = statusItem.observe(\.isVisible, options: [.new]) { [weak self] item, _ in
            DispatchQueue.main.async {
                guard let self = self, !self.terminating, !item.isVisible else { return }
                item.isVisible = true
            }
        }
        statusItem.button?.target = self; statusItem.button?.action = #selector(toggle)
        panel = AttentionPanel(contentRect: NSRect(x: 0, y: 0, width: attentionPanelWidth, height: 180), styleMask: [.borderless, .nonactivatingPanel], backing: .buffered, defer: false)
        panel.level = .popUpMenu
        panel.isOpaque = false; panel.backgroundColor = .clear; panel.hasShadow = true
        panel.isReleasedWhenClosed = false; panel.hidesOnDeactivate = false
        panel.collectionBehavior = [.moveToActiveSpace, .fullScreenAuxiliary]
        let background = NSVisualEffectView()
        panelBackground = background
        background.material = .popover; background.blendingMode = .behindWindow; background.state = .active
        background.wantsLayer = true; background.layer?.cornerRadius = 13; background.layer?.masksToBounds = true
        panelController = NSHostingController(rootView: AttentionView(model: model, close: { [weak self] in self?.closePanel() }))
        panel.contentView = background
        let content = panelController.view
        content.translatesAutoresizingMaskIntoConstraints = false
        background.addSubview(content)
        NSLayoutConstraint.activate([
            content.leadingAnchor.constraint(equalTo: background.leadingAnchor),
            content.trailingAnchor.constraint(equalTo: background.trailingAnchor),
            content.topAnchor.constraint(equalTo: background.topAnchor),
            content.bottomAnchor.constraint(equalTo: background.bottomAnchor)
        ])
        content.postsFrameChangedNotifications = true
        resizeObservation = NotificationCenter.default.addObserver(forName: NSView.frameDidChangeNotification, object: content, queue: .main) { [weak self] _ in
            self?.resizePanel()
        }
        model.onChange = { [weak self] in self?.refresh() }
        refresh(); startCore()
    }
    func refresh() {
        let count = model.state.needs_you.count
        if panel.isVisible { markRecentSeen(); DispatchQueue.main.async { [weak self] in self?.resizePanel() } }
        statusItem.button?.image = BellIcon.menu(paused: model.state.paused, count: count, unread: hasUnseenCompletion)
        statusItem.button?.imagePosition = .imageLeading
        statusItem.button?.font = .monospacedDigitSystemFont(ofSize: 12, weight: .semibold)
        statusItem.button?.title = ""
        statusItem.button?.toolTip = count > 0 ? "\(count) sessions need you" : (hasUnseenCompletion ? "New completed sessions" : "AgentBell")
        statusItem.button?.setAccessibilityLabel("AgentBell, \(count) sessions need you\(model.state.paused ? ", notifications paused" : "")")
    }
    func closePanel() {
        panel.orderOut(nil)
        statusItem.button?.highlight(false)
        if let monitor = outsideClickMonitor { NSEvent.removeMonitor(monitor); outsideClickMonitor = nil }
        if let monitor = localClickMonitor { NSEvent.removeMonitor(monitor); localClickMonitor = nil }
    }
    func resizePanel() {
        guard panel.isVisible, let button = statusItem.button, let window = button.window else { return }
        let desired = panelController.sizeThatFits(in: NSSize(width: attentionPanelWidth, height: 700))
        let anchor = window.convertToScreen(button.convert(button.bounds, to: nil))
        let screen = window.screen ?? NSScreen.main
        let bounds = screen?.visibleFrame ?? anchor
        let width = attentionPanelWidth
        let height = min(max(desired.height, 100), bounds.height - 12)
        let x = min(max(anchor.midX - width / 2, bounds.minX + 6), bounds.maxX - width - 6)
        let y = max(bounds.minY + 6, anchor.minY - height - 6)
        let frame = NSRect(x: x, y: y, width: width, height: height)
        if !NSEqualRects(panel.frame, frame) { panel.setFrame(frame, display: true) }
        updatePanelMask()
    }
    func updatePanelMask() {
        let size = panelBackground.bounds.size
        guard size.width > 0 && size.height > 0 else { return }
        // CornerRadius only clips a CALayer; maskImage also clips the native backdrop.
        panelBackground.maskImage = NSImage(size: size, flipped: false) { bounds in
            NSColor.white.setFill()
            NSBezierPath(roundedRect: bounds, xRadius: 13, yRadius: 13).fill()
            return true
        }
        panelController.view.wantsLayer = true
        panelController.view.layer?.cornerRadius = 13
        panelController.view.layer?.masksToBounds = true
        panel.invalidateShadow()
    }
    @objc func toggle() {
        if panel.isVisible { closePanel(); return }
        markRecentSeen(); refresh()
        panel.orderFront(nil); resizePanel(); panel.makeKey()
        statusItem.button?.highlight(true)
        outsideClickMonitor = NSEvent.addGlobalMonitorForEvents(matching: [.leftMouseDown, .rightMouseDown]) { [weak self] _ in
            self?.closePanel()
        }
        localClickMonitor = NSEvent.addLocalMonitorForEvents(matching: [.leftMouseDown, .rightMouseDown, .keyDown]) { [weak self] event in
            guard let self = self else { return event }
            if event.type == .keyDown && event.keyCode == 53 { self.closePanel(); return nil }
            if event.type != .keyDown && event.window != self.panel && event.window != self.statusItem.button?.window { self.closePanel() }
            return event
        }
    }
    func startCore() {
        let p = Process(), incoming = Pipe(), outgoing = Pipe()
        p.executableURL = model.binary; p.arguments = ["attention-host"]
        p.standardInput = incoming; p.standardOutput = outgoing; p.standardError = FileHandle.nullDevice
        input = incoming; output = outgoing; core = p; buffer = Data()
        outgoing.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            DispatchQueue.main.async {
                guard let self = self else { return }
                if data.isEmpty { handle.readabilityHandler = nil; return }
                self.buffer.append(data)
                while let range = self.buffer.range(of: Data([10])) {
                    let line = self.buffer.subdata(in: 0..<range.lowerBound); self.buffer.removeSubrange(0..<range.upperBound)
                    if let message = try? JSONDecoder().decode(NativeNotification.self, from: line), message.kind == "notification" {
                        self.postNotification(message)
                        continue
                    }
                    if let state = try? JSONDecoder().decode(SessionSnapshot.self, from: line), state.schema_version == 1 {
                        self.model.state = state; self.model.onChange?()
                    }
                }
            }
        }
        p.terminationHandler = { [weak self] process in
            DispatchQueue.main.async {
                guard let self = self, !self.terminating else { return }
                self.model.failure = "Attention Center unavailable. Notifications use the fallback."
                if self.restartCount < 3 { self.restartCount += 1; DispatchQueue.main.asyncAfter(deadline: .now() + 2) { self.startCore() } }
            }
        }
        do { try p.run() } catch { model.failure = error.localizedDescription }
    }
    func postNotification(_ message: NativeNotification) {
        let center = UNUserNotificationCenter.current()
        center.requestAuthorization(options: [.alert, .sound]) { granted, error in
            guard granted else {
                DispatchQueue.main.async { self.model.notificationFailure = "Notifications unavailable. Enable AgentBell in System Settings → Notifications." }
                return
            }
            let content = UNMutableNotificationContent()
            content.title = message.title; content.subtitle = message.subtitle; content.body = message.body
            content.sound = .default
            content.userInfo = ["return_target": message.return_target, "source": message.source, "type": message.type, "session_id": message.session_id]
            let submit = {
                center.add(UNNotificationRequest(identifier: UUID().uuidString, content: content, trigger: nil)) { error in
                    DispatchQueue.main.async { self.model.notificationFailure = error?.localizedDescription }
                }
            }
            if !message.action.isEmpty && message.return_target != "null" {
                let categoryID = "RETURN_TO_CONTEXT_" + message.action
                let action = UNNotificationAction(identifier: "RETURN_TO_CONTEXT", title: message.action, options: .foreground)
                let category = UNNotificationCategory(identifier: categoryID, actions: [action], intentIdentifiers: [], options: [])
                center.getNotificationCategories { categories in
                    var updated = categories; updated.update(with: category)
                    center.setNotificationCategories(updated)
                    content.categoryIdentifier = categoryID
                    submit()
                }
            } else { submit() }
        }
    }
    func applicationWillTerminate(_ notification: Notification) {
        terminating = true
        output?.fileHandleForReading.readabilityHandler = nil
        try? input?.fileHandleForWriting.close()
        // Core gets EOF, flushes its database and exits. No kill that skips persistence.
        if let core = core, core.isRunning { core.waitUntilExit() }
    }
    func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse, withCompletionHandler completionHandler: @escaping () -> Void) {
        if response.actionIdentifier != UNNotificationDismissActionIdentifier,
           let target = response.notification.request.content.userInfo["return_target"] as? String,
           target != "null", target.utf8.count <= 65536 {
            model.command(["return", target], completion: { if !self.centerEnabled { NSApp.terminate(nil) } })
        } else if !centerEnabled { DispatchQueue.main.async { NSApp.terminate(nil) } }
        completionHandler()
    }
    func userNotificationCenter(_ center: UNUserNotificationCenter, willPresent notification: UNNotification, withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) { completionHandler([.banner, .list]) }
}
if CommandLine.arguments.contains("--quit-existing") {
    let existing = NSRunningApplication.runningApplications(withBundleIdentifier: "com.agentbell.AgentBell").filter { $0.processIdentifier != ProcessInfo.processInfo.processIdentifier }
    for application in existing { _ = application.terminate() }
    let deadline = Date().addingTimeInterval(5)
    while existing.contains(where: { !$0.isTerminated }) && Date() < deadline { Thread.sleep(forTimeInterval: 0.05) }
    exit(existing.contains(where: { !$0.isTerminated }) ? 1 : 0)
}
let app = NSApplication.shared
let delegate = AppDelegate()
app.setActivationPolicy(.accessory)
app.delegate = delegate
UNUserNotificationCenter.current().delegate = delegate
app.run()
