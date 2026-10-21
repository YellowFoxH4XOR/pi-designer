import SwiftUI
import AppKit

struct ActivitySurface: View {
    @ObservedObject var model: ActivityViewModel
    let notch: Bool
    let open: (ActivitySummary?) -> Void
    let stop: (ActivitySummary) -> Void
    let toggle: () -> Void
    let settings: () -> Void
    let quit: () -> Void
    var body: some View {
        VStack(spacing: 0) {
            Button(action: notch ? toggle : { open(nil) }) {
                HStack(spacing: 10) {
                    Image(nsImage: NSApp.applicationIconImage).resizable().interpolation(.high).frame(width: 32, height: 32).accessibilityHidden(true)
                    VStack(alignment: .leading, spacing: 3) {
                        Text(notch && model.expanded ? "Workspace activity" : model.items.first(where: { $0.active })?.projectName ?? "πDesigner").font(.system(size: 12, weight: .semibold)).lineLimit(1)
                        Text(model.disconnected ? "Activity disconnected · reconnecting" : model.items.first(where: { $0.active })?.label ?? "No active tasks").font(.system(size: 11)).foregroundColor(.secondary).lineLimit(1)
                    }
                    Spacer(minLength: 5)
                    Image(systemName: notch && model.expanded ? "chevron.up" : "chevron.down").font(.system(size: 10))
                }.padding(.horizontal, 15).frame(height: 54).contentShape(Rectangle())
            }.buttonStyle(.plain).accessibilityLabel("πDesigner activity. Click to expand or collapse")
            .accessibilityValue((model.items.first(where: { $0.active })?.projectName ?? "No active tasks") + " — " + (model.disconnected ? "Activity disconnected" : model.items.first(where: { $0.active })?.label ?? "Idle"))
            if !notch || model.expanded {
                Divider()
                ScrollView {
                    VStack(alignment: .leading, spacing: 0) {
                        if model.items.isEmpty { Text("Start a design in πDesigner. Its progress will appear here.").font(.system(size: 12)).foregroundColor(.secondary).padding(16) }
                        ForEach(model.items.prefix(12)) { item in
                            VStack(alignment: .leading, spacing: 7) {
                                Button { open(item) } label: {
                                    VStack(alignment: .leading, spacing: 5) {
                                        Text(item.projectName).font(.system(size: 12, weight: .semibold)).lineLimit(1)
                                        Label(item.label, systemImage: item.attention ? "bubble.left.and.exclamationmark.bubble.right" : item.status == "running" ? "waveform.path" : item.status == "failed" ? "exclamationmark.triangle" : item.status == "canceled" ? "stop.circle" : "checkmark.circle")
                                            .font(.system(size: 11)).foregroundColor(item.attention ? .orange : .secondary).lineLimit(2)
                                        Text("\(item.elapsed(at: model.now)) · \(item.model ?? "omp agent")").font(.system(size: 10)).foregroundColor(.secondary).lineLimit(1)
                                    }.frame(maxWidth: .infinity, alignment: .leading).contentShape(Rectangle())
                                }.buttonStyle(.plain).accessibilityLabel("Open \(item.projectName). \(item.label)")
                                if item.status == "running" { Button("Stop task", role: .destructive) { stop(item) }.font(.system(size: 11)).buttonStyle(.borderless) }
                            }.padding(.horizontal, 16).padding(.vertical, 12)
                            Divider().padding(.horizontal, 16)
                        }
                    }
                }
                HStack { Button("Open workspace") { open(nil) }; Spacer(); Button("Settings", action: settings); Button("Quit", action: quit) }.font(.system(size: 11)).buttonStyle(.borderless).padding(13)
            }
        }.frame(width: 370).background(Color(nsColor: .windowBackgroundColor)).clipShape(RoundedRectangle(cornerRadius: notch ? 16 : 8))
            .preferredColorScheme(notch ? .dark : .light)
    }
}
