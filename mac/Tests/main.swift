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

// Source assets, not a cached Dock image: check the actual AppKit SVG renderer.
func raster(_ image: NSImage, size: Int) -> NSBitmapImageRep {
    let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
    NSColor.clear.setFill(); NSRect(x: 0, y: 0, width: size, height: size).fill(using: .copy)
    image.draw(in: NSRect(x: 0, y: 0, width: size, height: size))
    NSGraphicsContext.restoreGraphicsState()
    return bitmap
}
guard let mark = NSImage(contentsOfFile: "ui/public/brand.svg"), let template = NSImage(contentsOfFile: "ui/public/brand-symbol.svg") else { fatalError("Brand SVG assets missing") }
for size in [16, 32, 128, 512] {
    let bitmap = raster(mark, size: size)
    var coral = 0, paper = 0, ink = 0
    for y in 0..<size { for x in 0..<size {
        guard let color = bitmap.colorAt(x: x, y: y)?.usingColorSpace(.deviceRGB), color.alphaComponent > 0.5 else { continue }
        if color.redComponent > 0.65 && color.greenComponent < 0.55 && color.blueComponent < 0.4 { coral += 1 }
        if color.redComponent > 0.9 && color.greenComponent > 0.8 && color.blueComponent > 0.7 { paper += 1 }
        if color.redComponent < 0.4 && color.greenComponent < 0.45 && color.blueComponent < 0.45 { ink += 1 }
    } }
    check(coral > size * size / 6 && paper > size * size / 12 && ink > size * size / 80, "Designer mark retains its coral/artboard/nib silhouette at \(size)px")
    check((bitmap.colorAt(x: 0, y: 0)?.alphaComponent ?? 1) < 0.1, "App mark has a transparent outer corner at \(size)px")
}
for size in [18, 32] {
    let bitmap = raster(template, size: size)
    var opaque = 0, colored = 0
    for y in 0..<size { for x in 0..<size {
        guard let color = bitmap.colorAt(x: x, y: y)?.usingColorSpace(.deviceRGB), color.alphaComponent > 0.5 else { continue }
        opaque += 1
        if max(color.redComponent, max(color.greenComponent, color.blueComponent)) > 0.05 { colored += 1 }
    } }
    check(opaque > size * size / 12 && colored == 0, "Menu-bar nib is a true alpha template, not a white-painted cutout, at \(size)px")
}
