import Cocoa
import Sparkle

// Sparkle owns download verification, replacement, authorization and relaunch.
// No GitHub token, private signing key or custom installer runs in the app.
final class AppUpdateController: NSObject, SPUUpdaterDelegate, NSMenuItemValidation {
    private var controller: SPUStandardUpdaterController?
    private var observations: [NSKeyValueObservation] = []
    private var message = "Check GitHub Releases for a newer version."
    var onChange: (() -> Void)?

    init(enabled: Bool) {
        super.init()
        let info = Bundle.main.infoDictionary ?? [:]
        guard enabled else { message = "Update checks are disabled for this test session."; return }
        guard let key = info["SUPublicEDKey"] as? String, Data(base64Encoded: key)?.count == 32,
              let feed = info["SUFeedURL"] as? String, let url = URL(string: feed),
              url.scheme == "https", url.host != nil, url.user == nil, url.password == nil else {
            message = "This local build has no signed update feed. Install a release build to enable updates."
            return
        }
        controller = SPUStandardUpdaterController(startingUpdater: false, updaterDelegate: self, userDriverDelegate: nil)
    }

    func start() {
        guard let controller else { return }
        let updater = controller.updater
        observations = [
            updater.observe(\.canCheckForUpdates, options: [.new]) { [weak self] _, _ in self?.onChange?() },
            updater.observe(\.sessionInProgress, options: [.new]) { [weak self] _, _ in self?.onChange?() },
            updater.observe(\.automaticallyChecksForUpdates, options: [.new]) { [weak self] _, _ in self?.onChange?() },
            updater.observe(\.automaticallyDownloadsUpdates, options: [.new]) { [weak self] _, _ in self?.onChange?() }
        ]
        controller.startUpdater()
        onChange?()
    }

    var state: [String: Any] {
        let updater = controller?.updater
        return [
            "version": Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "Unknown",
            "configured": updater != nil,
            "canCheck": updater?.canCheckForUpdates ?? false,
            "busy": updater?.sessionInProgress ?? false,
            "automaticChecks": updater?.automaticallyChecksForUpdates ?? false,
            "automaticDownloads": updater?.automaticallyDownloadsUpdates ?? false,
            "message": message
        ]
    }

    @objc func checkForUpdates(_ sender: Any? = nil) {
        guard let controller, controller.updater.canCheckForUpdates else { onChange?(); return }
        message = "Checking for updates…"
        onChange?()
        controller.checkForUpdates(sender)
    }

    func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {
        controller?.updater.canCheckForUpdates ?? false
    }

    func setPreferences(_ preferences: [String: Any]) {
        guard let updater = controller?.updater else { return }
        // Sparkle persists these settings. Set them only in response to user input.
        if let automatic = preferences["automaticChecks"] as? Bool { updater.automaticallyChecksForUpdates = automatic }
        if let automatic = preferences["automaticDownloads"] as? Bool { updater.automaticallyDownloadsUpdates = automatic }
        onChange?()
    }

    func updater(_ updater: SPUUpdater, didFindValidUpdate item: SUAppcastItem) {
        message = "Version \(item.displayVersionString) is available. Use the update window to install it."
        onChange?()
    }

    func updater(_ updater: SPUUpdater, didFinishUpdateCycleFor updateCheck: SPUUpdateCheck, error: Error?) {
        if let error = error as NSError? {
            if error.domain == SUSparkleErrorDomain && error.code == Int(SUError.noUpdateError.rawValue) {
                message = "You’re up to date."
            } else {
                message = "Couldn’t update: \(error.localizedDescription)"
            }
        } else {
            message = "Update check finished. Available updates appear in the update window."
        }
        onChange?()
    }
}
