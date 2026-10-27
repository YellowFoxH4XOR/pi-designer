// Verify against the public key embedded in the app, not just the private key
// used by the signing tool. No private signing material is needed here.
import Foundation
import CryptoKit

func fail(_ message: String) -> Never {
    fputs("Update verification failed: \(message)\n", stderr)
    exit(1)
}
let args = CommandLine.arguments
guard args.count == 4 else { fail("Usage: verify-update public-key archive appcast.xml") }
do {
    guard let key = Data(base64Encoded: args[1]), key.count == 32 else { fail("Invalid public key") }
    let archive = try Data(contentsOf: URL(fileURLWithPath: args[2]), options: .mappedIfSafe)
    let feed = try XMLDocument(contentsOf: URL(fileURLWithPath: args[3]), options: .nodeLoadExternalEntitiesNever)
    let enclosures = try feed.nodes(forXPath: "/rss/channel/item/enclosure")
    guard enclosures.count == 1, let enclosure = enclosures[0] as? XMLElement,
          let value = enclosure.attribute(forLocalName: "edSignature", uri: "http://www.andymatuschak.org/xml-namespaces/sparkle")?.stringValue,
          let signature = Data(base64Encoded: value), signature.count == 64,
          enclosure.attribute(forName: "length")?.stringValue == String(archive.count) else { fail("Invalid signature or size metadata") }
    let publicKey = try Curve25519.Signing.PublicKey(rawRepresentation: key)
    guard publicKey.isValidSignature(signature, for: archive) else { fail("Archive signature does not match the embedded public key") }
    print("Verified update archive signature and size")
} catch { fail(error.localizedDescription) }
