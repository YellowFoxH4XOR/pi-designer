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

struct ActivityNotificationQueue {
    private struct Pending {
        let activity: ActivitySummary
        var failures: Int
        var nextAttempt: Date
    }

    private let maximumPending: Int
    private(set) var delivered: Set<String>
    private var pending: [String: Pending] = [:]
    private var order: [String] = []
    private(set) var inFlightKey: String?

    var pendingCount: Int { pending.count }

    init(delivered: Set<String> = [], maximumPending: Int = 512) {
        self.delivered = delivered
        self.maximumPending = max(0, maximumPending)
    }

    static func key(for activity: ActivitySummary) -> String? {
        guard let outcome = activity.outcome else { return nil }
        return activity.runId + ":" + outcome
    }

    mutating func enqueue(_ activity: ActivitySummary, enabled: Bool) -> Bool {
        guard let key = Self.key(for: activity) else { return true }
        guard !delivered.contains(key), inFlightKey != key else { return true }
        if !enabled {
            removePending(key)
            recordDelivered(key)
            return true
        }
        guard pending[key] == nil else { return true }
        guard pending.count < maximumPending else { return false }
        pending[key] = Pending(activity: activity, failures: 0, nextAttempt: .distantPast)
        order.append(key)
        return true
    }

    mutating func takeNext(now: Date, isEnabled: (ActivitySummary) -> Bool) -> ActivitySummary? {
        guard inFlightKey == nil else { return nil }
        var suppressed: [String] = []
        for key in order {
            guard let item = pending[key], !isEnabled(item.activity) else { continue }
            suppressed.append(key)
        }
        if !suppressed.isEmpty {
            let suppressedKeys = Set(suppressed)
            for key in suppressed {
                pending.removeValue(forKey: key)
                recordDelivered(key)
            }
            order.removeAll { suppressedKeys.contains($0) }
        }
        for key in order {
            guard let item = pending[key], item.nextAttempt <= now else { continue }
            inFlightKey = key
            return item.activity
        }
        return nil
    }

    @discardableResult
    mutating func complete(key: String, succeeded: Bool, now: Date) -> Bool {
        guard inFlightKey == key, var item = pending[key] else { return false }
        inFlightKey = nil
        if succeeded {
            removePending(key)
            return recordDelivered(key)
        }
        // Retry only on later daemon polls, with a 30s-to-5m exponential backoff.
        let delay = min(30 * (1 << min(item.failures, 4)), 300)
        item.failures += 1
        item.nextAttempt = now.addingTimeInterval(TimeInterval(delay))
        pending[key] = item
        return false
    }

    private mutating func removePending(_ key: String) {
        pending.removeValue(forKey: key)
        order.removeAll { $0 == key }
    }

    @discardableResult
    private mutating func recordDelivered(_ key: String) -> Bool {
        guard delivered.insert(key).inserted else { return false }
        if delivered.count > 1024 { delivered = Set(delivered.sorted().suffix(512)) }
        return true
    }
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
