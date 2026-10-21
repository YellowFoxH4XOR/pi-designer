// PiDesigner: AppKit window + WKWebView, with the Go daemon as a sidecar.
// No Electron runtime, model provider, or credentials are embedded here.
import Cocoa
import WebKit

private enum Config {
    static let name = "PiDesigner"
    static let readyPrefix = "PIDESIGNER_READY "
    static var dataDir: URL {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("PiDesigner", isDirectory: true)
    }
    static let background = NSColor(srgbRed: 236/255, green: 238/255, blue: 243/255, alpha: 1)
}

final class Daemon {
    private var process: Process?
    private var input: Pipe?
    private var output: Pipe?
    private var errors: Pipe?
    private var buffer = Data()
    private var diagnostics = ""
    private var stopping = false
    private(set) var isRunning = false
    var onReady: ((URL) -> Void)?
    var onFailure: ((String) -> Void)?

    func start() {
        guard let executable = Bundle.main.executableURL,
              let resources = Bundle.main.resourceURL else {
            onFailure?("The application resources could not be located. Rebuild PiDesigner.")
            return
        }
        let binary = executable.deletingLastPathComponent().appendingPathComponent("pidesigner")
        guard FileManager.default.isExecutableFile(atPath: binary.path) else {
            onFailure?("The bundled daemon is missing. Run scripts/build-mac.sh to rebuild the app.")
            return
        }
        let proc = Process()
        proc.executableURL = binary
        proc.arguments = ["--port", "0", "--print-url", "--exit-on-stdin-close",
            "--data", Config.dataDir.path,
            "--content", resources.appendingPathComponent("content").path,
            "--ui", resources.appendingPathComponent("ui").path]
        var environment = ProcessInfo.processInfo.environment
        // Finder launches don't inherit a login shell PATH. Include both Pi's
        // own installer location and common Node installations explicitly.
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        environment["PATH"] = ["\(home)/.pi/agent/bin", environment["PATH"] ?? "",
            "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin"].joined(separator: ":")
        environment["PIDESIGNER_APP"] = "1"
        proc.environment = environment
        let stdin = Pipe(), stdout = Pipe(), stderr = Pipe()
        input = stdin; output = stdout; errors = stderr
        proc.standardInput = stdin; proc.standardOutput = stdout; proc.standardError = stderr
        stdout.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty else { handle.readabilityHandler = nil; return }
            DispatchQueue.main.async { self?.consume(data) }
        }
        stderr.fileHandleForReading.readabilityHandler = { [weak self] handle in
            let data = handle.availableData
            guard !data.isEmpty else { handle.readabilityHandler = nil; return }
            DispatchQueue.main.async {
                self?.diagnostics += String(decoding: data, as: UTF8.self)
                if let text = self?.diagnostics { self?.diagnostics = String(text.suffix(8000)) }
            }
        }
        proc.terminationHandler = { [weak self] p in
            DispatchQueue.main.async {
                guard let self else { return }
                self.isRunning = false
                if !self.stopping {
                    self.onFailure?("The workspace daemon stopped (status \(p.terminationStatus)). Use View → Restart Workspace.\n\n\(self.diagnostics.suffix(1000))")
                }
            }
        }
        do { try proc.run(); process = proc; isRunning = true }
        catch { onFailure?("The workspace could not start: \(error.localizedDescription)") }
    }
    private func consume(_ data: Data) {
        buffer.append(data)
        while let newline = buffer.firstIndex(of: 0x0A) {
            let line = String(decoding: buffer[..<newline], as: UTF8.self)
            buffer.removeSubrange(...newline)
            guard line.hasPrefix(Config.readyPrefix),
                  let url = URL(string: String(line.dropFirst(Config.readyPrefix.count))) else { continue }
            onReady?(url)
        }
    }
    func stop() {
        stopping = true
        input?.fileHandleForWriting.closeFile()
        guard let process, process.isRunning else { return }
        // EOF normally reaps the daemon and its Pi process tree immediately.
        let deadline = Date().addingTimeInterval(4)
        while process.isRunning && Date() < deadline { usleep(30_000) }
        if process.isRunning { process.terminate() }
    }
    deinit { input?.fileHandleForWriting.closeFile() }
}

final class AppDelegate: NSObject, NSApplicationDelegate, NSWindowDelegate, WKNavigationDelegate, WKUIDelegate {
    private var window: NSWindow!
    private var webView: WKWebView!
    private var status: NSTextField!
    private var daemon = Daemon()
    private var baseURL: URL?
    private var captureStarted = false
    private var discardConfirmed = false

    func applicationDidFinishLaunching(_ notification: Notification) {
        // Explicitly refresh the Dock mark after a locally rebuilt bundle;
        // Launch Services may otherwise keep the prior icon cached.
        if let resources = Bundle.main.resourceURL,
           let icon = NSImage(contentsOf: resources.appendingPathComponent("AppIcon.icns")) {
            NSApp.applicationIconImage = icon
        }
        buildMenu()
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
    }
    private func launch() {
        daemon.onReady = { [weak self] url in
            guard let self else { return }
            self.baseURL = url
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
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool { true }
    func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
        guard webView != nil && !discardConfirmed else { return .terminateNow }
        webView.evaluateJavaScript("document.documentElement.dataset.unsavedChanges === 'true'") { [weak self] value, _ in
            guard let self else { sender.reply(toApplicationShouldTerminate: true); return }
            guard value as? Bool == true else { sender.reply(toApplicationShouldTerminate: true); return }
            let alert = NSAlert()
            alert.messageText = "Discard unsaved source changes?"
            alert.informativeText = "Your saved project files will remain on this Mac. Unsaved edits in the source editor will be lost."
            alert.addButton(withTitle: "Keep Editing")
            alert.addButton(withTitle: "Discard and Quit")
            alert.beginSheetModal(for: self.window) { sender.reply(toApplicationShouldTerminate: $0 == .alertSecondButtonReturn) }
        }
        return .terminateLater
    }
    func windowShouldClose(_ sender: NSWindow) -> Bool {
        if discardConfirmed { return true }
        webView.evaluateJavaScript("document.documentElement.dataset.unsavedChanges === 'true'") { [weak self] value, _ in
            guard let self else { return }
            if value as? Bool != true {
                self.discardConfirmed = true; sender.performClose(nil); return
            }
            let alert = NSAlert()
            alert.messageText = "Discard unsaved source changes?"
            alert.addButton(withTitle: "Keep Editing")
            alert.addButton(withTitle: "Discard and Close")
            alert.beginSheetModal(for: sender) { response in
                if response == .alertSecondButtonReturn { self.discardConfirmed = true; sender.performClose(nil) }
            }
        }
        return false
    }
    func applicationWillTerminate(_ notification: Notification) { daemon.stop() }
    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
        if window.isMiniaturized { window.deminiaturize(nil) }
        window.makeKeyAndOrderFront(nil)
        return true
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
        action(app, "About PiDesigner", #selector(NSApplication.orderFrontStandardAboutPanel(_:)), "")
        app.addItem(.separator())
        action(app, "Settings…", #selector(settings), ",", target: self)
        action(app, "Open Project Folder", #selector(openDataFolder), "", target: self)
        app.addItem(.separator())
        action(app, "Hide PiDesigner", #selector(NSApplication.hide(_:)), "h")
        let hideOthers = NSMenuItem(title: "Hide Others", action: #selector(NSApplication.hideOtherApplications(_:)), keyEquivalent: "h")
        hideOthers.keyEquivalentModifierMask = [.command, .option]; app.addItem(hideOthers)
        action(app, "Show All", #selector(NSApplication.unhideAllApplications(_:)), "")
        app.addItem(.separator())
        action(app, "Quit PiDesigner", #selector(NSApplication.terminate(_:)), "q")
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
