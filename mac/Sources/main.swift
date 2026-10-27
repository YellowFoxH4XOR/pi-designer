// PiDesigner: AppKit window + WKWebView, with the Go daemon as a sidecar.
// No Electron runtime, model provider, or credentials are embedded here.
import Cocoa
import WebKit

final class AppDelegate: NSObject, NSApplicationDelegate, NSWindowDelegate, WKNavigationDelegate, WKUIDelegate, WKScriptMessageHandler {
    private var window: NSWindow!
    private var webView: WKWebView!
    private var status: NSTextField!
    private var daemon = Daemon()
    private var baseURL: URL?
    private var captureStarted = false
    private var discardConfirmed = false
    private var activity: NativeActivityController?
    private var updates: AppUpdateController?

    private var terminationCheckInProgress = false
    func applicationDidFinishLaunching(_ notification: Notification) {
        // Explicitly refresh the Dock mark after a locally rebuilt bundle;
        // Launch Services may otherwise keep the prior icon cached.
        if let resources = Bundle.main.resourceURL,
           let icon = NSImage(contentsOf: resources.appendingPathComponent("AppIcon.icns")) {
            NSApp.applicationIconImage = icon
        }
        updates = AppUpdateController(enabled: Self.argument("--capture") == nil && ProcessInfo.processInfo.environment["PIDESIGNER_DISABLE_UPDATES"] != "1")
        updates?.onChange = { [weak self] in self?.publishUpdateSettings() }
        buildMenu()
        if Self.argument("--capture") == nil {
            let controller = NativeActivityController()
            controller.onOpen = { [weak self] item in self?.openActivity(item) }
            controller.onOpenDestination = { [weak self] project, conversation in self?.openDestination(project, conversation) }
            controller.onSettings = { [weak self] in self?.showWorkspace(); self?.settings() }
            controller.onQuit = { NSApp.terminate(nil) }
            controller.screen = { [weak self] in self?.window?.screen }
            controller.workspaceVisible = { [weak self] in NSApp.isActive && self?.window?.isVisible == true && self?.window?.isMiniaturized == false }
            activity = controller
        }
        let capturing = Self.argument("--capture") != nil
        let width = capturing ? Double(Self.argument("--capture-width") ?? "") ?? 1440 : 1440
        let height = capturing ? Double(Self.argument("--capture-height") ?? "") ?? 900 : 900
        window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: width, height: height),
            styleMask: [.titled, .closable, .miniaturizable, .resizable], backing: .buffered, defer: false)
        window.delegate = self
        window.title = Config.name
        window.titleVisibility = .hidden
        window.titlebarAppearsTransparent = true
        window.appearance = NSAppearance(named: .aqua)
        window.backgroundColor = Config.background
        window.contentMinSize = NSSize(width: 900, height: 620)
        window.collectionBehavior = [.fullScreenPrimary]
        let configuration = WKWebViewConfiguration()
        configuration.websiteDataStore = .default()
        configuration.userContentController.add(self, name: "pidesigner")
        configuration.userContentController.addUserScript(WKUserScript(source: "window.__pidesignerNative = true", injectionTime: .atDocumentStart, forMainFrameOnly: true))
        webView = WKWebView(frame: .zero, configuration: configuration)
        webView.navigationDelegate = self
        webView.uiDelegate = self
        webView.translatesAutoresizingMaskIntoConstraints = false
        if #available(macOS 13.3, *) { webView.isInspectable = true }
        let container = NSView()
        container.addSubview(webView)
        NSLayoutConstraint.activate([
            webView.topAnchor.constraint(equalTo: container.topAnchor),
            webView.bottomAnchor.constraint(equalTo: container.bottomAnchor),
            webView.leadingAnchor.constraint(equalTo: container.leadingAnchor),
            webView.trailingAnchor.constraint(equalTo: container.trailingAnchor)])
        status = NSTextField(labelWithString: "Opening your workspace…")
        status.font = .systemFont(ofSize: 14)
        status.textColor = .secondaryLabelColor
        status.alignment = .center
        status.maximumNumberOfLines = 0
        status.preferredMaxLayoutWidth = 560
        status.translatesAutoresizingMaskIntoConstraints = false
        container.addSubview(status)
        NSLayoutConstraint.activate([
            status.centerXAnchor.constraint(equalTo: container.centerXAnchor),
            status.centerYAnchor.constraint(equalTo: container.centerYAnchor),
            status.widthAnchor.constraint(lessThanOrEqualToConstant: 600)])
        window.contentView = container
        if capturing { window.center() }
        else {
            if !window.setFrameUsingName("PiDesignerWorkspace") { window.center() }
            window.setFrameAutosaveName("PiDesignerWorkspace")
        }
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
        launch()
        updates?.start()
    }
    private func launch() {
        daemon.onReady = { [weak self] url in
            guard let self else { return }
            self.baseURL = url
            self.activity?.connect(url)
            self.status.isHidden = true
            self.webView.isHidden = false
            var target = url
            if let route = Self.argument("--capture-route"), var components = URLComponents(url: url, resolvingAgainstBaseURL: false) {
                components.fragment = route; target = components.url ?? url
            }
            self.webView.load(URLRequest(url: target))
        }
        daemon.onFailure = { [weak self] message in self?.showError(message) }
        daemon.start()
    }
    private func showError(_ text: String) {
        status.isHidden = false
        status.stringValue = text
        status.textColor = .systemRed
        webView.isHidden = true
    }
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { false }
    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        guard webView != nil && !discardConfirmed else { return .terminateNow }
        guard !terminationCheckInProgress else { return .terminateLater }
        terminationCheckInProgress = true
        webView.evaluateJavaScript("document.documentElement.dataset.unsavedChanges === 'true'") { [weak self] value, error in
            DispatchQueue.main.async {
                guard let self else { sender.reply(toApplicationShouldTerminate: false); return }
                guard error == nil, let dirty = value as? Bool else {
                    self.showTerminationCheckFailure(error ?? TerminationGuardError.invalidResponse, replyTo: sender)
                    return
                }
                self.checkTermination(dirty: dirty, acknowledgedRunning: nil, replyTo: sender)
            }
        }
        return .terminateLater
    }
    private func checkTermination(dirty: Bool, acknowledgedRunning: Set<String>?, replyTo sender: NSApplication) {
        fetchRunningRunIDs { [weak self] result in
            guard let self else { sender.reply(toApplicationShouldTerminate: false); return }
            switch result {
            case .failure(let error):
                self.showTerminationCheckFailure(error, replyTo: sender)
            case .success(let running):
                guard let acknowledgedRunning else {
                    guard dirty || !running.isEmpty else {
                        self.finishTermination(replyTo: sender, approve: true)
                        return
                    }
                    self.confirmTermination(dirty: dirty, running: running, replyTo: sender)
                    return
                }
                if running.subtracting(acknowledgedRunning).isEmpty {
                    self.finishTermination(replyTo: sender, approve: true)
                } else {
                    self.confirmTermination(dirty: dirty, running: running, replyTo: sender)
                }
            }
        }
    }
    private func confirmTermination(dirty: Bool, running: Set<String>, replyTo sender: NSApplication) {
        showWorkspace()
        let alert = NSAlert()
        alert.messageText = !running.isEmpty ? "Quit and stop \(running.count) active task(s)?" : "Discard unsaved source changes?"
        if dirty && !running.isEmpty {
            alert.informativeText = "Saved project files remain on this Mac. Unsaved source edits will be lost, and active tasks will stop."
        } else if dirty {
            alert.informativeText = "Saved project files remain on this Mac. Unsaved source edits will be lost."
        } else {
            alert.informativeText = "Closing the window keeps tasks running. Quitting stops them."
        }
        alert.addButton(withTitle: "Keep Working")
        alert.addButton(withTitle: dirty ? "Discard and Quit" : "Stop Tasks and Quit")
        alert.beginSheetModal(for: window) { [weak self] response in
            guard let self else { sender.reply(toApplicationShouldTerminate: false); return }
            guard response == .alertSecondButtonReturn else {
                self.finishTermination(replyTo: sender, approve: false)
                return
            }
            // The user may have started more work while the confirmation was
            // visible. Recheck before replying yes; newly active runs need an
            // explicit, fresh confirmation of their cancellation.
            self.checkTermination(dirty: dirty, acknowledgedRunning: running, replyTo: sender)
        }
    }
    private func showTerminationCheckFailure(_ error: Error, replyTo sender: NSApplication) {
        showWorkspace()
        let alert = NSAlert()
        alert.messageText = "Quit without verifying the workspace?"
        alert.informativeText = "πDesigner couldn’t verify the current workspace state. Quitting may stop active tasks and discard unsaved source edits. Keep working unless you want to accept that risk.\n\n\(error.localizedDescription)"
        alert.addButton(withTitle: "Keep Working")
        alert.addButton(withTitle: "Quit Anyway")
        alert.beginSheetModal(for: window) { [weak self] response in
            guard let self else { sender.reply(toApplicationShouldTerminate: false); return }
            self.finishTermination(replyTo: sender, approve: response == .alertSecondButtonReturn)
        }
    }
    private func finishTermination(replyTo sender: NSApplication, approve: Bool) {
        if approve { discardConfirmed = true }
        terminationCheckInProgress = false
        sender.reply(toApplicationShouldTerminate: approve)
    }
    private func fetchRunningRunIDs(completion: @escaping (Result<Set<String>, Error>) -> Void) {
        guard let baseURL else {
            completion(.failure(TerminationGuardError.daemonUnavailable))
            return
        }
        var request = URLRequest(url: baseURL.appendingPathComponent("api/activity"))
        request.timeoutInterval = 5
        request.cachePolicy = .reloadIgnoringLocalCacheData
        URLSession.shared.dataTask(with: request) { data, response, error in
            let result: Result<Set<String>, Error>
            if let error {
                result = .failure(error)
            } else if let response = response as? HTTPURLResponse {
                if response.statusCode != 200 {
                    result = .failure(TerminationGuardError.httpStatus(response.statusCode))
                } else if let data, let activity = try? JSONDecoder().decode(ActivityEnvelope.self, from: data) {
                    result = .success(Set(activity.activities.filter { $0.status == "running" }.map(\.runId)))
                } else {
                    result = .failure(TerminationGuardError.invalidResponse)
                }
            } else {
                result = .failure(TerminationGuardError.invalidResponse)
            }
            DispatchQueue.main.async { completion(result) }
        }.resume()
    }
    private struct ActivityEnvelope: Decodable { let activities: [ActivitySummary] }
    private enum TerminationGuardError: LocalizedError {
        case daemonUnavailable
        case invalidResponse
        case httpStatus(Int)
        var errorDescription: String? {
            switch self {
            case .daemonUnavailable: return "The workspace daemon is not connected."
            case .invalidResponse: return "The workspace daemon returned an invalid activity response."
            case .httpStatus(let status): return "The workspace daemon returned HTTP \(status)."
            }
        }
    }
    func windowShouldClose(_ sender: NSWindow) -> Bool {
        // Close hides the workspace; it does not discard source, unload WebKit,
        // close the daemon pipe, or terminate active pi tasks.
        sender.orderOut(nil)
        activity?.updateVisibility()
        return false
    }
    func windowDidMiniaturize(_ notification: Notification) { activity?.updateVisibility() }
    func windowDidDeminiaturize(_ notification: Notification) { activity?.updateVisibility() }
    func windowDidMove(_ notification: Notification) { activity?.updateVisibility() }
    func applicationWillTerminate(_ notification: Notification) { activity?.stop(); daemon.stop() }
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        showWorkspace(); return true
    }
    @objc private func showWorkspace() {
        if window.isMiniaturized { window.deminiaturize(nil) }
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
        activity?.updateVisibility()
    }
    private func openActivity(_ item: ActivitySummary?) {
        showWorkspace()
        guard let item else { return }
        openDestination(item.projectId, item.conversationId)
    }
    private func openDestination(_ project: String, _ conversation: String?) {
        showWorkspace()
        var query = URLComponents()
        query.queryItems = [URLQueryItem(name: "conversation", value: conversation ?? ""), URLQueryItem(name: "activity", value: "1")]
        let route = "project/" + project + "?" + (query.percentEncodedQuery ?? "")
        guard let json = try? JSONSerialization.data(withJSONObject: route, options: .fragmentsAllowed), let literal = String(data: json, encoding: .utf8) else { return }
        webView.evaluateJavaScript("location.hash = " + literal + "; requestAnimationFrame(() => window.dispatchEvent(new Event('pidesigner:show-activity')))")
    }
    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        // Generated prototypes must never request permissions or manipulate the
        // host. Only our exact top-level loopback document can use this bridge.
        guard message.frameInfo.isMainFrame, message.webView === webView,
              let base = baseURL, message.frameInfo.securityOrigin.host == base.host,
              message.frameInfo.securityOrigin.protocol == base.scheme,
              message.frameInfo.securityOrigin.port == base.port,
              let document = message.frameInfo.request.url, ["/", "/index.html"].contains(document.path),
              let body = message.body as? [String: Any], let action = body["action"] as? String else { return }
        switch action {
        case "getUpdateSettings": publishUpdateSettings(); return
        case "checkForUpdates": updates?.checkForUpdates(nil); publishUpdateSettings(); return
        case "updateUpdateSettings": updates?.setPreferences(body["preferences"] as? [String: Any] ?? [:]); publishUpdateSettings(); return
        default: break
        }
        guard let activity else { return }
        let reply = { [weak self] in
            activity.settings { settings in
                guard let data = try? JSONSerialization.data(withJSONObject: settings), let json = String(data: data, encoding: .utf8) else { return }
                self?.webView.evaluateJavaScript("window.dispatchEvent(new CustomEvent('pidesigner:native-settings', {detail: " + json + "}))")
            }
        }
        switch action {
        case "getSettings": reply()
        case "updateSettings": activity.updatePreferences(body["preferences"] as? [String: Any] ?? [:]); reply()
        case "requestNotifications": activity.requestNotificationPermission(completion: reply)
        default: break
        }
    }
    private func publishUpdateSettings() {
        guard webView != nil, let state = updates?.state,
              let data = try? JSONSerialization.data(withJSONObject: state),
              let json = String(data: data, encoding: .utf8) else { return }
        webView.evaluateJavaScript("window.dispatchEvent(new CustomEvent('pidesigner:update-settings', {detail: " + json + "}))")
    }
    private func buildMenu() {
        let bar = NSMenu()
        func menu(_ title: String) -> NSMenu {
            let item = NSMenuItem(); let m = NSMenu(title: title)
            item.submenu = m; bar.addItem(item); return m
        }
        func action(_ menu: NSMenu, _ title: String, _ selector: Selector, _ key: String, target: AnyObject? = nil) {
            let item = NSMenuItem(title: title, action: selector, keyEquivalent: key)
            item.target = target; menu.addItem(item)
        }
        let app = menu(Config.name)
        action(app, "About πDesigner", #selector(NSApplication.orderFrontStandardAboutPanel(_:)), "")
        action(app, "Check for Updates…", #selector(AppUpdateController.checkForUpdates(_:)), "", target: updates)
        app.addItem(.separator())
        action(app, "Settings…", #selector(settings), ",", target: self)
        action(app, "Open Project Folder", #selector(openDataFolder), "", target: self)
        app.addItem(.separator())
        action(app, "Hide πDesigner", #selector(NSApplication.hide(_:)), "h")
        let hideOthers = NSMenuItem(title: "Hide Others", action: #selector(NSApplication.hideOtherApplications(_:)), keyEquivalent: "h")
        hideOthers.keyEquivalentModifierMask = [.command, .option]; app.addItem(hideOthers)
        action(app, "Show All", #selector(NSApplication.unhideAllApplications(_:)), "")
        app.addItem(.separator())
        action(app, "Quit πDesigner", #selector(NSApplication.terminate(_:)), "q")
        let file = menu("File")
        action(file, "New Prototype", #selector(newPrototype), "n", target: self)
        action(file, "Close Window", #selector(NSWindow.performClose(_:)), "w")
        let edit = menu("Edit")
        action(edit, "Undo", Selector(("undo:")), "z")
        action(edit, "Redo", Selector(("redo:")), "Z")
        edit.addItem(.separator())
        action(edit, "Cut", #selector(NSText.cut(_:)), "x")
        action(edit, "Copy", #selector(NSText.copy(_:)), "c")
        action(edit, "Paste", #selector(NSText.paste(_:)), "v")
        action(edit, "Select All", #selector(NSText.selectAll(_:)), "a")
        let view = menu("View")
        action(view, "Reload", #selector(reload), "r", target: self)
        action(view, "Restart Workspace", #selector(restart), "R", target: self)
        view.addItem(.separator())
        action(view, "Actual Size", #selector(actualSize), "0", target: self)
        action(view, "Zoom In", #selector(zoomIn), "+", target: self)
        action(view, "Zoom Out", #selector(zoomOut), "-", target: self)
        view.addItem(.separator())
        let fullScreen = NSMenuItem(title: "Enter Full Screen", action: #selector(NSWindow.toggleFullScreen(_:)), keyEquivalent: "f")
        fullScreen.keyEquivalentModifierMask = [.command, .control]; view.addItem(fullScreen)
        let windows = menu("Window")
        action(windows, "Show Workspace", #selector(showWorkspace), "", target: self)
        action(windows, "Minimize", #selector(NSWindow.performMiniaturize(_:)), "m")
        action(windows, "Zoom", #selector(NSWindow.performZoom(_:)), "")
        NSApp.windowsMenu = windows
        NSApp.mainMenu = bar
    }
    @objc private func settings() { webView.evaluateJavaScript("location.hash = 'settings'") }
    @objc private func newPrototype() { webView.evaluateJavaScript("location.hash = 'home'; requestAnimationFrame(() => document.getElementById('home-prompt')?.focus())") }
    @objc private func openDataFolder() {
        let dir = Config.dataDir.appendingPathComponent("projects", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        NSWorkspace.shared.open(dir)
    }
    @objc private func reload() { if daemon.isRunning { webView.reload() } else { restart() } }
    @objc private func restart() {
        daemon.stop(); daemon = Daemon()
        status.isHidden = false; status.textColor = .secondaryLabelColor
        status.stringValue = "Restarting your workspace…"
        launch()
    }
    @objc private func actualSize() { webView.pageZoom = 1 }
    @objc private func zoomIn() { webView.pageZoom = min(2, webView.pageZoom + 0.1) }
    @objc private func zoomOut() { webView.pageZoom = max(0.7, webView.pageZoom - 0.1) }

    // Keep the workspace in its own origin. User-clicked external links open in
    // the system browser; prototype frames remain sandboxed by the web UI.
    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction,
        decisionHandler: @escaping (WKNavigationActionPolicy) -> Void) {
        guard navigationAction.targetFrame?.isMainFrame == true,
              let url = navigationAction.request.url, let base = baseURL else {
            decisionHandler(.allow); return
        }
        if url.scheme == base.scheme && url.host == base.host && url.port == base.port {
            decisionHandler(.allow)
        } else {
            if navigationAction.navigationType == .linkActivated { NSWorkspace.shared.open(url) }
            decisionHandler(.cancel)
        }
    }
    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
        for navigationAction: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? {
        if navigationAction.navigationType == .linkActivated, let url = navigationAction.request.url,
           ["http", "https"].contains(url.scheme ?? "") { NSWorkspace.shared.open(url) }
        return nil
    }
    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        if (error as NSError).code != NSURLErrorCancelled { showError("The workspace could not load. Use View → Restart Workspace.\n\n\(error.localizedDescription)") }
    }
    func webView(_ webView: WKWebView, runJavaScriptConfirmPanelWithMessage message: String,
        initiatedByFrame frame: WKFrameInfo, completionHandler: @escaping (Bool) -> Void) {
        let alert = NSAlert(); alert.messageText = message
        alert.addButton(withTitle: "Discard changes"); alert.addButton(withTitle: "Keep editing")
        alert.beginSheetModal(for: window) { completionHandler($0 == .alertFirstButtonReturn) }
    }
    // Development-only capture flag: capture this app's own view, never the
    // desktop or other applications. Useful for verifying the real WebKit build.
    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        guard let path = Self.argument("--capture"), !captureStarted else { return }
        captureStarted = true
        DispatchQueue.main.asyncAfter(deadline: .now() + 2.5) { [weak self] in
            guard let self else { return }
            self.webView.takeSnapshot(with: nil) { image, error in
                if let tiff = image?.tiffRepresentation,
                   let bitmap = NSBitmapImageRep(data: tiff),
                   let png = bitmap.representation(using: .png, properties: [:]) {
                    try? png.write(to: URL(fileURLWithPath: path))
                } else { fputs("Capture failed: \(error?.localizedDescription ?? "no image")\n", stderr) }
                NSApp.terminate(nil)
            }
        }
    }
    private static func argument(_ name: String) -> String? {
        let args = CommandLine.arguments
        guard let index = args.firstIndex(of: name), index + 1 < args.count else { return nil }
        return args[index + 1]
    }
}
let application = NSApplication.shared
application.setActivationPolicy(.regular)
let delegate = AppDelegate()
application.delegate = delegate
application.run()
