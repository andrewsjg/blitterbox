import AppKit
import SwiftUI

@main
struct BlitterBoxApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) var delegate
    @State private var model = LauncherModel()

    var body: some Scene {
        Window("BlitterBox", id: "launcher") {
            LauncherView()
                .environment(model)
                .frame(minWidth: 860, minHeight: 560)
        }
        .defaultSize(width: 920, height: 700)
        .windowResizability(.contentMinSize)
        .commands {
            CommandGroup(replacing: .newItem) {
                Button("New Configuration", systemImage: "plus") { model.newConfiguration() }
                    .keyboardShortcut("n")
            }
            CommandGroup(replacing: .saveItem) {
                Button("Save Configuration", systemImage: "square.and.arrow.down") { model.save() }
                    .keyboardShortcut("s")
            }
        }
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationWillFinishLaunching(_ notification: Notification) {
        signal(SIGPIPE, SIG_IGN) // a blitterbox that has gone away is an error, not a crash
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        // Hiding the window while the machine runs isn't closing it.
        NSApp.activationPolicy() == .regular
    }
}
