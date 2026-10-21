import Cocoa
import ApplicationServices

// Own-app integration driver. Reads/drives only the explicitly supplied PID;
// never enumerates other apps or requests permission on the user's behalf.
guard CommandLine.arguments.count >= 3, let pid = Int32(CommandLine.arguments[1]), AXIsProcessTrusted() else {
    fputs("Usage: native-accessibility PID dump|windows|close|minimize|restore|click [label]. Accessibility permission required.\n", stderr); exit(2)
}
let app = AXUIElementCreateApplication(pid)
func value(_ element: AXUIElement, _ attribute: String) -> AnyObject? {
    var result: CFTypeRef?; guard AXUIElementCopyAttributeValue(element, attribute as CFString, &result) == .success else { return nil }; return result
}
func children(_ element: AXUIElement) -> [AXUIElement] { value(element, kAXChildrenAttribute) as? [AXUIElement] ?? [] }
func label(_ element: AXUIElement) -> String {
    [kAXTitleAttribute, kAXDescriptionAttribute, kAXValueAttribute].compactMap { value(element, $0) as? String }.joined(separator: " | ")
}
func walk(_ element: AXUIElement, _ depth: Int = 0) -> [AXUIElement] {
    if depth > 15 { return [] }
    return [element] + children(element).filter { (value($0, kAXTitleAttribute) as? String) != "Apple" }.prefix(150).flatMap { walk($0, depth + 1) }
}
let action = CommandLine.arguments[2]
let windows = value(app, kAXWindowsAttribute) as? [AXUIElement] ?? []
if action == "dump" {
    var nodes = walk(app)
    if let extras = value(app, "AXExtrasMenuBar"), CFGetTypeID(extras) == AXUIElementGetTypeID() { nodes += walk(unsafeBitCast(extras, to: AXUIElement.self)) }
    for item in nodes { print((value(item, kAXRoleAttribute) as? String ?? "?") + " " + label(item)) }
} else if action == "windows" {
    print(windows.map { label($0) }.joined(separator: "\n"))
} else if action == "click" {
    let target = CommandLine.arguments.dropFirst(3).joined(separator: " ")
    var nodes = walk(app)
    if let extras = value(app, "AXExtrasMenuBar"), CFGetTypeID(extras) == AXUIElementGetTypeID() { nodes += walk(unsafeBitCast(extras, to: AXUIElement.self)) }
    guard let button = nodes.first(where: { item in
        let role = value(item, kAXRoleAttribute) as? String
        return (role == kAXButtonRole || role == kAXMenuItemRole || role == kAXMenuBarItemRole) && label(item).localizedCaseInsensitiveContains(target)
    }) else { fputs("No action named \(target)\n", stderr); exit(3) }
    guard AXUIElementPerformAction(button, kAXPressAction as CFString) == .success else { exit(4) }
} else {
    guard let window = windows.first(where: { (value($0, kAXTitleAttribute) as? String) == "πDesigner" }) else { fputs("No πDesigner workspace window\n", stderr); exit(3) }
    if action == "close" {
        guard let close = value(window, kAXCloseButtonAttribute), CFGetTypeID(close) == AXUIElementGetTypeID() else { exit(4) }
        guard AXUIElementPerformAction(unsafeBitCast(close, to: AXUIElement.self), kAXPressAction as CFString) == .success else { exit(5) }
    } else {
        guard AXUIElementSetAttributeValue(window, kAXMinimizedAttribute as CFString, (action == "minimize" ? kCFBooleanTrue : kCFBooleanFalse)) == .success else { exit(6) }
    }
}
