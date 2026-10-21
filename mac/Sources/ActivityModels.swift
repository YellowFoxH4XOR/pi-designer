import Cocoa
import SwiftUI

struct ActivitySummary: Decodable, Identifiable {
    var id: String { runId }
    let runId: String
    let projectId: String
    let projectName: String
    let conversationId: String?
    let status: String
    let phase: String
    let label: String
    let model: String?
    let startedAt: Double
    let endedAt: Double?
    let attention: Bool
    let attentionSeq: Int?
    var active: Bool { status == "running" || attention }
    var outcome: String? { attention ? "attention:\(attentionSeq ?? 0)" : status == "succeeded" ? "completed" : status == "failed" ? "failed" : nil }
    func elapsed(at date: Date) -> String {
        let seconds = max(0, Int(((endedAt ?? date.timeIntervalSince1970 * 1000) - startedAt) / 1000))
        return seconds < 60 ? "\(seconds)s" : "\(seconds / 60)m \(seconds % 60)s"
    }
}

final class ActivityViewModel: ObservableObject {
    @Published var items: [ActivitySummary] = []
    @Published var disconnected = false
    @Published var expanded = false
    @Published var now = Date()
}

// Only presentation preferences live in UserDefaults. Execution state always
// comes from the daemon's materialized event journal, even with no web window.
final class ActivityPreferences {
    let defaults: UserDefaults
    let keys = ["notch", "completed", "attention", "failure", "sound", "previews"]
    init(defaults: UserDefaults = ProcessInfo.processInfo.environment["PIDESIGNER_PREFERENCES_SUITE"].flatMap { UserDefaults(suiteName: $0) } ?? .standard) { self.defaults = defaults; defaults.register(defaults: ["activity.notch": true, "activity.completed": true, "activity.attention": true, "activity.failure": true, "activity.sound": false, "activity.previews": false]) }
    subscript(_ key: String) -> Bool { get { defaults.bool(forKey: "activity.\(key)") } set { if keys.contains(key) { defaults.set(newValue, forKey: "activity.\(key)") } } }
    var dictionary: [String: Any] { Dictionary(uniqueKeysWithValues: keys.map { ($0, self[$0] as Any) }) }
}
