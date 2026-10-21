// Render the canonical SVG into the standard macOS iconset. The interface and
// Dock icon share one source so their geometry cannot silently diverge.
import Cocoa

let output = CommandLine.arguments.dropFirst().first ?? "build/AppIcon.iconset"
let source = CommandLine.arguments.count > 2 ? CommandLine.arguments[2] : "ui/public/brand.svg"
guard let vector = NSImage(contentsOfFile: source) else {
    fputs("Could not read the canonical app icon: \(source)\n", stderr)
    exit(1)
}
try FileManager.default.createDirectory(atPath: output, withIntermediateDirectories: true)
let sizes: [(String, Int)] = [
    ("icon_16x16.png", 16), ("icon_16x16@2x.png", 32),
    ("icon_32x32.png", 32), ("icon_32x32@2x.png", 64),
    ("icon_128x128.png", 128), ("icon_128x128@2x.png", 256),
    ("icon_256x256.png", 256), ("icon_256x256@2x.png", 512),
    ("icon_512x512.png", 512), ("icon_512x512@2x.png", 1024),
]
for (name, size) in sizes {
    let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: size, pixelsHigh: size,
        bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
        colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
    NSGraphicsContext.current?.imageInterpolation = .high
    let margin = CGFloat(size) / 16
    vector.draw(in: NSRect(x: margin, y: margin, width: CGFloat(size) - 2*margin, height: CGFloat(size) - 2*margin))
    NSGraphicsContext.restoreGraphicsState()
    try bitmap.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: output).appendingPathComponent(name))
}
print("Rendered \(sizes.count) icon sizes from \(source)")
