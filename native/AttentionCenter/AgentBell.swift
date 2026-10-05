import AppKit
import SwiftUI
import UserNotifications

struct Session: Decodable, Identifiable {
    let id, agent, project, status, attention, summary: String
    let started_at, updated_at: String
    let finished_at, attention_at: String?
    let return_target: JSONValue?
    var label: String {
        switch attention {
        case "input": return "Waiting for input"
        case "approval": return "Approval required"
        case "error": return "Failed"
        default: return status == "done" ? "Done" : "Working"
        }
    }
    var symbol: String { attention == "none" ? (status == "done" ? "checkmark.circle" : "circle.fill") : "exclamationmark.circle" }
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
    func command(_ args: [String], completion: (() -> Void)? = nil) {
        DispatchQueue.global(qos: .userInitiated).async {
            defer { if let completion = completion { DispatchQueue.main.async(execute: completion) } }
            let p = Process(); p.executableURL = self.binary; p.arguments = args
            let output = Pipe(); p.standardOutput = FileHandle.nullDevice; p.standardError = output
            do {
                try p.run()
                let data = output.fileHandleForReading.readDataToEndOfFile(); p.waitUntilExit()
                if p.terminationStatus != 0 {
                    DispatchQueue.main.async { self.failure = String(data: data, encoding: .utf8) ?? "Action failed." }
                }
            } catch { DispatchQueue.main.async { self.failure = error.localizedDescription } }
        }
    }
    func returnTo(_ target: JSONValue) {
        guard let data = try? JSONEncoder().encode(target), let text = String(data: data, encoding: .utf8) else { return }
        command(["return", text])
    }
}
struct AttentionView: View {
    @ObservedObject var model: Model
    @State private var showMore = false
    var close: () -> Void
    func age(_ value: String) -> String {
        let f = ISO8601DateFormatter(); f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        let date = f.date(from: value) ?? ISO8601DateFormatter().date(from: value) ?? Date()
        let seconds = max(0, Int(Date().timeIntervalSince(date)))
        if seconds < 60 { return "\(seconds)s" }
        if seconds < 3600 { return "\(seconds / 60)m" }
        if seconds < 86400 { return "\(seconds / 3600)h" }
        return "\(seconds / 86400)d"
    }
    @ViewBuilder func section(_ name: String, _ sessions: [Session]) -> some View {
        HStack {
            Text(name).tracking(0.8)
            Spacer()
            Text("\(sessions.count)").monospacedDigit()
        }.font(.system(size: 10, weight: .semibold)).foregroundColor(.secondary).padding(.top, 7).accessibilityAddTraits(.isHeader)
        ForEach(sessions) { session in
            HStack(alignment: .top, spacing: 9) {
                Image(systemName: session.symbol).foregroundColor(session.attention == "none" ? .secondary : .orange).frame(width: 14).padding(.top, 3).accessibilityHidden(true)
                VStack(alignment: .leading, spacing: 3) {
                    Text(session.project).font(.system(size: 13, weight: .medium)).lineLimit(1)
                    Text("\(session.agent == "claude" ? "Claude" : "Codex") · \(session.label) · \(age(session.attention_at ?? session.finished_at ?? session.started_at))")
                        .font(.caption).foregroundColor(.secondary)
                    if !session.summary.isEmpty { Text(session.summary).font(.caption).foregroundColor(.secondary).lineLimit(2) }
                }.frame(maxWidth: .infinity, alignment: .leading)
                if let target = session.return_target {
                    Button { model.returnTo(target); close() } label: {
                        HStack(spacing: 3) { Text("Return"); Image(systemName: "arrow.up.forward") }
                            .font(.system(size: 11, weight: .medium)).padding(.horizontal, 7).padding(.vertical, 5)
                            .background(Color.primary.opacity(0.055), in: RoundedRectangle(cornerRadius: 6))
                    }.buttonStyle(.plain)
                        .help(target.capability == "exact_context" ? "Return to this session" : "Return capability: \(target.capability)")
                        .accessibilityLabel("Return to \(session.project), \(session.agent)")
                }
            }.padding(.vertical, 5)
        }
    }
    var body: some View {
        TimelineView(.periodic(from: .now, by: 15)) { _ in
            VStack(alignment: .leading, spacing: 6) {
                HStack {
                    Text("AgentBell").font(.headline)
                    Spacer()
                    if !model.state.needs_you.isEmpty { Text("\(model.state.needs_you.count) need you").font(.caption).foregroundColor(.secondary) }
                }
                Divider()
                ScrollView {
                    VStack(alignment: .leading, spacing: 5) {
                        if !model.state.needs_you.isEmpty { section("NEEDS YOU", model.state.needs_you) }
                        if !model.state.working.isEmpty { section("WORKING", model.state.working) }
                        if !model.state.recent.isEmpty { section("RECENT", Array(model.state.recent.prefix(showMore ? model.state.recent.count : (model.state.recent_limit ?? 5)))) }
                        if model.state.needs_you.isEmpty && model.state.working.isEmpty && model.state.recent.isEmpty {
                            VStack(spacing: 7) {
                                Image(systemName: "checkmark.circle").font(.system(size: 23))
                                Text("All caught up").font(.system(size: 13, weight: .medium))
                                Text("Your agents will appear here as they work.").font(.caption)
                            }.foregroundColor(.secondary).frame(maxWidth: .infinity).padding(.vertical, 25)
                        }
                        if model.state.recent.count > (model.state.recent_limit ?? 5) { Button(showMore ? "Show Less" : "Show More") { showMore.toggle() }.buttonStyle(.link) }
                    }.frame(maxWidth: .infinity, alignment: .leading)
                }.frame(height: min(390, max(105, CGFloat(model.state.needs_you.count + model.state.working.count + min(model.state.recent.count, showMore ? model.state.recent.count : (model.state.recent_limit ?? 5))) * 91 + CGFloat([model.state.needs_you, model.state.working, model.state.recent].filter { !$0.isEmpty }.count) * 23)))
                Divider()
                if let error = model.failure { Text(error).font(.caption).foregroundColor(.secondary).lineLimit(3) }
                if let error = model.notificationFailure { Text(error).font(.caption).foregroundColor(.secondary).lineLimit(3) }
                if let error = model.state.storage_error, !error.isEmpty { Text("Session history unavailable").font(.caption).help(error) }
                HStack {
                    Image(systemName: model.state.paused ? "bell.slash" : "bell")
                    Text(model.state.paused ? "Notifications paused" : "Notifications on")
                    Spacer()
                    Menu {
                        Button(model.state.paused ? "Resume Notifications" : "Pause Notifications") { model.command(["attention-control", model.state.paused ? "resume" : "pause"]) }
                        Button("Clear Recent") { model.command(["attention-control", "clear_recent"]) }.disabled(model.state.recent.isEmpty)
                        Divider()
                        Button("Open Config…") { model.command(["open-config"]) }
                        Button("Quit AgentBell") { NSApp.terminate(nil) }.keyboardShortcut("q")
                    } label: { Image(systemName: "ellipsis.circle").font(.system(size: 16)) }
                    .menuStyle(.borderlessButton).fixedSize().help("AgentBell actions")
                }.font(.system(size: 11)).foregroundColor(.secondary).padding(.top, 3)
            }.padding(14).frame(width: 360).onExitCommand { close() }
        }
    }
}
final class AppDelegate: NSObject, NSApplicationDelegate, UNUserNotificationCenterDelegate {
    let model = Model()
    var statusItem: NSStatusItem!
    let popover = NSPopover()
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
        popover.behavior = .transient
        popover.contentViewController = NSHostingController(rootView: AttentionView(model: model, close: { [weak self] in self?.popover.performClose(nil) }))
        model.onChange = { [weak self] in self?.refresh() }
        refresh(); startCore()
    }
    func refresh() {
        let count = model.state.needs_you.count
        if popover.isShown { markRecentSeen() }
        statusItem.button?.image = BellIcon.menu(paused: model.state.paused)
        statusItem.button?.imagePosition = .imageLeading
        statusItem.button?.font = .monospacedDigitSystemFont(ofSize: 12, weight: .semibold)
        statusItem.button?.title = count > 0 ? " \(count)" : (hasUnseenCompletion ? " •" : "")
        statusItem.button?.toolTip = count > 0 ? "\(count) sessions need you" : (hasUnseenCompletion ? "New completed sessions" : "AgentBell")
        statusItem.button?.setAccessibilityLabel("AgentBell, \(count) sessions need you\(model.state.paused ? ", notifications paused" : "")")
    }
    @objc func toggle() {
        if popover.isShown { popover.performClose(nil) }
        else if let button = statusItem.button { markRecentSeen(); refresh(); popover.show(relativeTo: button.bounds, of: button, preferredEdge: .minY); popover.contentViewController?.view.window?.makeKey() }
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
                        self.model.state = state; self.model.failure = nil; self.model.onChange?()
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
