#if os(macOS)
import AppKit

// MachineRunner runs the machine on the Mac: blitterbox --config, a process
// beside this app, whose window becomes the app while the configuration
// screen hides. (The iPad app's runs it in-process.)
@MainActor
final class MachineRunner {
    private weak var model: LauncherModel?
    private var machine: Process?
    private var machineOutput: [String] = []
    private var hidden: [NSWindow] = [] // the windows hidden while it runs

    func run(config conf: String, name: String, model: LauncherModel) {
        self.model = model
        let back = URL(fileURLWithPath: conf).deletingLastPathComponent().appendingPathComponent("return.conf")
        try? FileManager.default.removeItem(at: back)

        let p = Process()
        p.executableURL = model.bridge.blitterbox
        p.arguments = ["--config", conf] + (name.isEmpty ? [] : ["--name", name])
        p.currentDirectoryURL = Bridge.workingDirectory
        var env = ProcessInfo.processInfo.environment
        env["BLITTERBOX_RETURN_CONFIG"] = back.path
        p.environment = env
        let pipe = Pipe()
        p.standardOutput = pipe
        p.standardError = pipe
        machineOutput = []
        pipe.fileHandleForReading.readabilityHandler = { [weak self] h in
            let data = h.availableData
            guard !data.isEmpty, let text = String(data: data, encoding: .utf8) else { return }
            FileHandle.standardOutput.write(data) // still on the terminal, if any
            Task { @MainActor in self?.keepOutput(text) }
        }
        p.terminationHandler = { [weak self] p in
            let code = p.terminationStatus
            Task { @MainActor in self?.machineEnded(code, returnFile: back) }
        }
        do {
            try p.run()
        } catch {
            model.say("Can't run blitterbox: \(error.localizedDescription)", error: true)
            return
        }
        machine = p
        model.running = true
        // The machine's window is the app now: the configuration screen
        // hides, and its Dock icon with it.
        hidden = NSApp.windows.filter(\.isVisible)
        for w in hidden { w.orderOut(nil) }
        NSApp.setActivationPolicy(.accessory)
    }

    private func keepOutput(_ text: String) {
        machineOutput.append(contentsOf: text.split(separator: "\n").map(String.init))
        if machineOutput.count > 50 {
            machineOutput.removeFirst(machineOutput.count - 50)
        }
    }

    private func machineEnded(_ code: Int32, returnFile: URL) {
        machine = nil
        guard let model else { return }
        model.running = false
        switch code {
        case 0:
            NSApp.terminate(nil) // closing the machine's window quits BlitterBox
            return
        case 3: // Config (Cmd+,): the settings come back, as they are now
            let r = model.bridge.call(Request(op: "loadFile", path: returnFile.path))
            if let s = r.settings {
                model.settings = s
            }
            model.say("The machine has been switched off. Change the settings and Start it again.")
        default:
            let err = machineOutput.last(where: { $0.hasPrefix("Error:") || $0.contains("error") }) ?? machineOutput.last
            model.say("The machine stopped (exit \(code))" + (err.map { ": " + $0 } ?? ""), error: true)
        }
        NSApp.setActivationPolicy(.regular)
        for w in hidden { w.makeKeyAndOrderFront(nil) }
        hidden = []
        NSApp.activate(ignoringOtherApps: true)
    }

    func stop() {
        machine?.terminate()
    }
}
#endif
