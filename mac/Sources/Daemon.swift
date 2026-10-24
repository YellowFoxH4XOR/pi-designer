import Cocoa
import WebKit

enum Config {
    static let name = "πDesigner"
    static let readyPrefix = "PIDESIGNER_READY "
    static var dataDir: URL {
        if let override = ProcessInfo.processInfo.environment["PIDESIGNER_DATA_DIR"], !override.isEmpty { return URL(fileURLWithPath: override, isDirectory: true) }
        return FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("PiDesigner", isDirectory: true)
    }
    static let background = NSColor(srgbRed: 244/255, green: 245/255, blue: 247/255, alpha: 1)
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
            onFailure?("The application resources could not be located. Rebuild πDesigner.")
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
        if let pi = ProcessInfo.processInfo.environment["PIDESIGNER_PI_BIN"], !pi.isEmpty { proc.arguments?.append(contentsOf: ["--pi", pi]) }
        var environment = ProcessInfo.processInfo.environment
        // Finder launches don't inherit a login shell PATH. Look for pi in its
        // install locations (bun global, npm prefix, pi installer) before the
        // inherited PATH and the usual Homebrew/system directories.
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        environment["PATH"] = ["\(home)/.bun/bin", "\(home)/.local/bin",
            "\(home)/.pi/agent/bin", environment["PATH"] ?? "",
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
        // EOF normally reaps the daemon and its pi process tree immediately.
        let deadline = Date().addingTimeInterval(4)
        while process.isRunning && Date() < deadline { usleep(30_000) }
        if process.isRunning { process.terminate() }
    }
    deinit { input?.fileHandleForWriting.closeFile() }
}
