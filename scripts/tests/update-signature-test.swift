// Runs on macOS CI. Exercise the real verification tool with ephemeral keys.
import Foundation
import CryptoKit

let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
defer { try? FileManager.default.removeItem(at: directory) }
let archive = directory.appendingPathComponent("test.dmg")
let feed = directory.appendingPathComponent("appcast.xml")
let original = Data("test update archive".utf8)
try original.write(to: archive)
let key = Curve25519.Signing.PrivateKey()
let signature = try key.signature(for: original).base64EncodedString()
let xml = """
<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:sparkle="http://www.andymatuschak.org/xml-namespaces/sparkle"><channel><item><enclosure url="https://example.com/test.dmg" sparkle:edSignature="\(signature)" length="\(original.count)"/></item></channel></rss>
"""
try xml.write(to: feed, atomically: true, encoding: .utf8)
func verify(_ publicKey: Data, succeeds: Bool) throws {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/swift")
    process.arguments = ["scripts/verify-update.swift", publicKey.base64EncodedString(), archive.path, feed.path]
    process.standardOutput = FileHandle.nullDevice
    process.standardError = FileHandle.nullDevice
    try process.run(); process.waitUntilExit()
    guard (process.terminationStatus == 0) == succeeds else { fatalError("Unexpected signature verification result") }
}
try verify(key.publicKey.rawRepresentation, succeeds: true)
try verify(Curve25519.Signing.PrivateKey().publicKey.rawRepresentation, succeeds: false)
try Data("tampered archive!".utf8).write(to: archive)
try verify(key.publicKey.rawRepresentation, succeeds: false)
print("PASS: signed archives accepted; wrong keys and tampered archives rejected")
