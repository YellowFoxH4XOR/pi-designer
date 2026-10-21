import Cocoa

func check(_ condition: @autoclosure () -> Bool, _ message: String) {
    guard condition() else { fputs("FAIL: \(message)\n", stderr); exit(1) }
    print("PASS", message)
}
let json = """
{"runId":"r","projectId":"p","projectName":"Test","conversationId":"c","status":"succeeded","phase":"attention","label":"Needs your answer","startedAt":1000,"endedAt":5000,"attention":true,"attentionSeq":12}
"""
let a = try JSONDecoder().decode(ActivitySummary.self, from: Data(json.utf8))
check(a.active, "Completed turn needing an answer remains active")
check(a.outcome == "attention:12", "Attention has a stable semantic notification key")
check(a.elapsed(at: Date(timeIntervalSince1970: 100)) == "4s", "Finished elapsed time does not keep increasing")
let completed = try JSONDecoder().decode(ActivitySummary.self, from: Data(json.replacingOccurrences(of: "\"attention\":true", with: "\"attention\":false").utf8))
check(completed.outcome == "completed" && !completed.active, "Completion is distinct from attention")
let failed = try JSONDecoder().decode(ActivitySummary.self, from: Data(json.replacingOccurrences(of: "\"attention\":true", with: "\"attention\":false").replacingOccurrences(of: "succeeded", with: "failed").utf8))
check(failed.outcome == "failed", "Failure has its own notification outcome")
let canceled = try JSONDecoder().decode(ActivitySummary.self, from: Data(json.replacingOccurrences(of: "\"attention\":true", with: "\"attention\":false").replacingOccurrences(of: "succeeded", with: "canceled").utf8))
check(canceled.outcome == nil, "Cancellation never sends a completion notification")
let suite = "dev.pidesigner.native-tests.\(UUID().uuidString)"
let defaults = UserDefaults(suiteName: suite)!
defer { defaults.removePersistentDomain(forName: suite) }
let preferences = ActivityPreferences(defaults: defaults)
check(preferences["notch"] && preferences["attention"], "Notch and attention preferences default on")
check(!preferences["previews"] && !preferences["sound"], "Notification previews and sounds default private/quiet")
preferences["notch"] = false
check(!ActivityPreferences(defaults: defaults)["notch"], "Native preferences persist independently of the web view")
preferences["unknown"] = true
check(defaults.object(forKey: "activity.unknown") == nil, "Unknown native preference keys are ignored")
