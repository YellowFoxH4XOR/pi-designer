import Cocoa
import SwiftUI
import UserNotifications

final class ActivityPanel: NSPanel {
    override var canBecomeKey: Bool { true }
    override func cancelOperation(_ sender: Any?) { orderOut(nil) }
}

final class NativeActivityController: NSObject, UNUserNotificationCenterDelegate {
    let model = ActivityViewModel()
    let preferences = ActivityPreferences()
    var onOpen: ((ActivitySummary?) -> Void)?
    var onOpenDestination: ((String, String?) -> Void)?
    var onSettings: (() -> Void)?
    var onQuit: (() -> Void)?
    var screen: (() -> NSScreen?)?
    var workspaceVisible: (() -> Bool)?
    private var baseURL: URL?
    private var timer: Timer?
    private var request: URLSessionDataTask?
    private var statusItem: NSStatusItem?
    private var popover: NSPopover?
    private var panel: ActivityPanel?
    private var localMonitor: Any?
    private var globalMonitor: Any?
    private var initialized = false
    private var generation = 0
    private var outcomeCursor: Int64?
    private var notificationQueue: ActivityNotificationQueue

    override init() {
        notificationQueue = ActivityNotificationQueue(delivered: Set(preferences.defaults.stringArray(forKey: "activity.delivered") ?? []))
        super.init()
        UNUserNotificationCenter.current().delegate = self
        let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        let symbolURL = Bundle.main.resourceURL?.appendingPathComponent("BrandTemplate.svg")
        let symbol = symbolURL.flatMap { NSImage(contentsOf: $0) } ?? NSImage(systemSymbolName: "pencil.tip", accessibilityDescription: nil)
        symbol?.size = NSSize(width: 18, height: 18)
        symbol?.isTemplate = true
        item.button?.image = symbol
        item.button?.imagePosition = .imageLeading
        item.button?.title = ""
        item.button?.font = .systemFont(ofSize: 13, weight: .medium)
        item.button?.target = self
        item.button?.action = #selector(togglePopover)
        item.button?.setAccessibilityLabel("πDesigner activity")
        statusItem = item
        NotificationCenter.default.addObserver(self, selector: #selector(updateVisibility), name: NSApplication.didChangeScreenParametersNotification, object: nil)
        NotificationCenter.default.addObserver(self, selector: #selector(updateVisibility), name: NSApplication.didResignActiveNotification, object: nil)
        NotificationCenter.default.addObserver(self, selector: #selector(updateVisibility), name: NSApplication.didBecomeActiveNotification, object: nil)
        localMonitor = NSEvent.addLocalMonitorForEvents(matching: [.leftMouseDown, .keyDown]) { [weak self] event in
            if event.type == .keyDown && event.keyCode == 53 { self?.collapse() }
            else if event.type == .leftMouseDown { self?.collapseIfOutside() }
            return event
        }
        globalMonitor = NSEvent.addGlobalMonitorForEvents(matching: .leftMouseDown) { [weak self] _ in self?.collapseIfOutside() }
    }

    deinit {
        timer?.invalidate(); request?.cancel()
        if let monitor = localMonitor { NSEvent.removeMonitor(monitor) }
        if let monitor = globalMonitor { NSEvent.removeMonitor(monitor) }
        if let item = statusItem { NSStatusBar.system.removeStatusItem(item) }
        NotificationCenter.default.removeObserver(self)
    }
    func connect(_ url: URL) {
        generation += 1; request?.cancel(); request = nil
        baseURL = url; initialized = false; outcomeCursor = nil
        timer?.invalidate()
        timer = Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { [weak self] _ in self?.poll() }
        poll()
    }
    func stop() { timer?.invalidate(); request?.cancel(); panel?.orderOut(nil) }
    private func poll() {
        model.now = Date(); updateVisibility()
        guard request == nil, let base = baseURL else { return }
        let currentGeneration = generation
        var components = URLComponents(url: base.appendingPathComponent("api/activity"), resolvingAgainstBaseURL: false)!
        if let cursor = outcomeCursor { components.queryItems = [URLQueryItem(name: "after", value: String(cursor))] }
        var req = URLRequest(url: components.url!)
        req.timeoutInterval = 5; req.cachePolicy = .reloadIgnoringLocalCacheData
        request = URLSession.shared.dataTask(with: req) { [weak self] data, response, error in
            DispatchQueue.main.async {
                guard let self, currentGeneration == self.generation else { return }
                self.request = nil
                struct Envelope: Decodable { let activities: [ActivitySummary]; let outcomes: [ActivitySummary]?; let cursor: Int64? }
                guard error == nil, (response as? HTTPURLResponse)?.statusCode == 200,
                      let data, let result = try? JSONDecoder().decode(Envelope.self, from: data) else {
                    self.model.disconnected = true
                    self.statusItem?.button?.title = " !"
                    self.statusItem?.button?.toolTip = "Activity disconnected. Reconnecting…"
                    return
                }
                self.model.disconnected = false
                self.model.items = result.activities.sorted { a, b in a.active != b.active ? a.active : a.startedAt > b.startedAt }
                // Baseline snapshots establish a cursor without replaying history.
                // A reconnect must still preserve and retry any outcomes already
                // queued in memory.
                var outcomesCaptured = true
                if self.initialized {
                    for outcome in result.outcomes ?? [] {
                        if !self.notifyIfNeeded(outcome) {
                            outcomesCaptured = false
                            break
                        }
                    }
                }
                // On queue saturation, keep this journal page readable until
                // pending ownership frees capacity; do not lose later outcomes.
                if !self.initialized || outcomesCaptured { self.outcomeCursor = result.cursor }
                self.initialized = true
                self.schedulePendingNotification()
                let active = result.activities.filter { $0.active }
                self.statusItem?.button?.title = active.isEmpty ? "" : " \(active.count)"
                self.statusItem?.button?.toolTip = active.first.map { "\($0.projectName) — \($0.label)" } ?? "πDesigner — no active tasks"
                self.updateVisibility()
            }
        }
        request?.resume()
    }
    private func persistDeliveredIfChanged(from previous: Set<String>) {
        guard previous != notificationQueue.delivered else { return }
        preferences.defaults.set(Array(notificationQueue.delivered), forKey: "activity.delivered")
    }

    private func notificationCategory(for activity: ActivitySummary) -> String {
        activity.attention ? "attention" : activity.status == "failed" ? "failure" : "completed"
    }

    @discardableResult
    private func notifyIfNeeded(_ activity: ActivitySummary) -> Bool {
        guard activity.outcome != nil else { return true }
        let previous = notificationQueue.delivered
        let accepted = notificationQueue.enqueue(activity, enabled: preferences[notificationCategory(for: activity)])
        persistDeliveredIfChanged(from: previous)
        return accepted
    }

    private func schedulePendingNotification() {
        let previous = notificationQueue.delivered
        guard let activity = notificationQueue.takeNext(now: Date(), isEnabled: { self.preferences[self.notificationCategory(for: $0)] }) else {
            persistDeliveredIfChanged(from: previous)
            return
        }
        persistDeliveredIfChanged(from: previous)
        guard let key = ActivityNotificationQueue.key(for: activity) else { return }

        let content = UNMutableNotificationContent()
        content.title = activity.attention ? "omp needs your attention" : activity.status == "failed" ? "omp couldn’t finish" : "omp finished your task"
        content.body = preferences["previews"] ? "\(activity.projectName) — \(activity.label)" : "Open πDesigner to view the project."
        content.threadIdentifier = activity.projectId
        content.userInfo = ["projectId": activity.projectId, "conversationId": activity.conversationId ?? ""]
        if preferences["sound"] { content.sound = .default }
        let request = UNNotificationRequest(identifier: key, content: content, trigger: nil)
        UNUserNotificationCenter.current().add(request) { [weak self] error in
            DispatchQueue.main.async {
                guard let self else { return }
                let previous = self.notificationQueue.delivered
                _ = self.notificationQueue.complete(key: key, succeeded: error == nil, now: Date())
                self.persistDeliveredIfChanged(from: previous)
                if let error { NSLog("πDesigner notification unavailable: %@", error.localizedDescription) }
                self.schedulePendingNotification()
            }
        }
    }
    func requestNotificationPermission(completion: @escaping () -> Void) {
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge]) { _, _ in DispatchQueue.main.async { completion() } }
    }
    func settings(completion: @escaping ([String: Any]) -> Void) {
        UNUserNotificationCenter.current().getNotificationSettings { [weak self] settings in
            DispatchQueue.main.async {
                guard let self else { return }
                var result = self.preferences.dictionary
                result["authorization"] = settings.authorizationStatus == .authorized || settings.authorizationStatus == .provisional ? "authorized" : settings.authorizationStatus == .denied ? "denied" : "notDetermined"
                completion(result)
            }
        }
    }
    func updatePreferences(_ values: [String: Any]) { for key in preferences.keys { if let value = values[key] as? Bool { preferences[key] = value } }; updateVisibility() }
    func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse, withCompletionHandler completionHandler: @escaping () -> Void) {
        let info = response.notification.request.content.userInfo
        DispatchQueue.main.async { [weak self] in
            guard let self else { return }
            if let project = info["projectId"] as? String { self.onOpenDestination?(project, info["conversationId"] as? String) }
            else { self.onOpen?(nil) }
        }
        completionHandler()
    }
    func userNotificationCenter(_ center: UNUserNotificationCenter, willPresent notification: UNNotification, withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void) { completionHandler([.banner, .sound]) }
    private func rootView(notch: Bool) -> ActivitySurface {
        ActivitySurface(model: model, notch: notch, open: { [weak self] item in self?.popover?.close(); self?.collapse(); self?.onOpen?(item) }, stop: { [weak self] item in self?.cancel(item) }, toggle: { [weak self] in self?.model.expanded.toggle(); self?.positionPanel() }, settings: { [weak self] in self?.popover?.close(); self?.onSettings?() }, quit: { [weak self] in self?.onQuit?() })
    }
    @objc private func togglePopover() {
        guard let button = statusItem?.button else { return }
        if popover?.isShown == true { popover?.close(); return }
        let pop = NSPopover(); pop.behavior = .transient
        pop.contentViewController = NSHostingController(rootView: rootView(notch: false))
        pop.contentSize = NSSize(width: 370, height: 410)
        popover = pop; pop.show(relativeTo: button.bounds, of: button, preferredEdge: .minY)
    }
    @objc func updateVisibility() {
        let show = preferences["notch"] && model.items.contains(where: { $0.active }) && !(workspaceVisible?() ?? NSApp.isActive)
        if show {
            if panel == nil {
                let p = ActivityPanel(contentRect: .zero, styleMask: [.borderless, .nonactivatingPanel], backing: .buffered, defer: false)
                p.title = "πDesigner Activity"
                p.isOpaque = false; p.backgroundColor = .clear; p.hasShadow = true; p.level = .floating
                p.hidesOnDeactivate = false; p.isReleasedWhenClosed = false
                p.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .stationary]
                p.contentView = NSHostingView(rootView: rootView(notch: true)); panel = p
            }
            positionPanel(); panel?.orderFrontRegardless()
        } else { panel?.orderOut(nil); model.expanded = false }
    }
    private func positionPanel() {
        guard let panel, let display = screen?() ?? NSScreen.main else { return }
        let rows = min(model.items.count, 3)
        let height: CGFloat = model.expanded ? min(380, CGFloat(max(1, rows)) * 118 + 104) : 54
        let width: CGFloat = 370
        // visibleFrame keeps the panel below the menu bar and physical notch on
        // every display; it never occupies menu extras or a camera cutout.
        panel.setFrame(NSRect(x: display.visibleFrame.midX - width / 2, y: display.visibleFrame.maxY - height - 5, width: width, height: height), display: true)
    }
    private func collapse() { model.expanded = false; positionPanel() }
    private func collapseIfOutside() { if let panel, !panel.frame.contains(NSEvent.mouseLocation) { collapse() } }
    private func cancel(_ item: ActivitySummary) {
        guard let base = baseURL else { return }
        var req = URLRequest(url: base.appendingPathComponent("api/runs/\(item.runId)/cancel")); req.httpMethod = "POST"; req.timeoutInterval = 10
        URLSession.shared.dataTask(with: req) { [weak self] _, response, error in
            DispatchQueue.main.async {
                if error != nil || (response as? HTTPURLResponse)?.statusCode != 200 { self?.model.disconnected = true }
                self?.poll()
            }
        }.resume()
    }
}
