import Foundation
import Observation

// The configuration screen's state, on the Mac and the iPad. Everything
// about settings files, presets and Kickstart ROMs is asked of Go
// (Bridge); this keeps what is on screen and starts the machine (each
// platform's MachineRunner).
@MainActor @Observable
final class LauncherModel {
    var settings = Settings() {
        didSet { if settings != oldValue { settingsChanged(from: oldValue) } }
    }
    var name = ""
    var saved: [String] = []
    var selection: String?
    var status = ""
    var statusIsError = false
    var tab = Tab.machine

    var presets: [String] = []
    var preset = "" // the model the settings are, of presets ("" none)
    var limits: [String: Int] = [:]
    var rom = ROMDescription()
    var whdload = WHDLoadInfo()

    var running = false

    let bridge = Bridge()
    private let runner = MachineRunner()

    enum Tab: String, CaseIterable, Identifiable {
        case machine = "Machine", memory = "Memory", floppies = "Floppies", hardDisks = "Hard disks"
        case whdload = "WHDLoad", ports = "Ports", display = "Display"
        var id: String { rawValue }
    }

    init() {
        let info = bridge.call(Request(op: "info"))
        (presets, limits) = (info.presets ?? [], info.limits ?? [:])
        if let e = info.error {
            say(e, error: true)
        }
        let r = bridge.call(Request(op: "startup"))
        if r.ok {
            apply(r)
        }
        refreshROM()
        refreshPreset() // didSet doesn't run in init
    }

    func say(_ message: String, error: Bool = false) {
        (status, statusIsError) = (message, error)
    }

    // apply takes a response's settings, name, saved list and status.
    private func apply(_ r: Response) {
        if let s = r.settings {
            settings = s
        }
        if let n = r.name {
            name = n
        }
        if let s = r.saved {
            saved = s
        }
        selection = saved.contains(name) ? name : nil
        if let e = r.error {
            say(e, error: true)
        } else if let s = r.status {
            say(s)
        }
    }

    private func settingsChanged(from old: Settings) {
        refreshPreset()
        if settings.rom != old.rom || settings.cpu != old.cpu || settings.motherboard != old.motherboard {
            refreshROM()
        }
    }

    func refreshPreset() {
        preset = bridge.call(Request(op: "model", settings: settings)).model ?? ""
    }

    func refreshROM() {
        rom = bridge.call(Request(op: "rom", settings: settings)).rom ?? ROMDescription()
        whdload = bridge.call(Request(op: "whdload", settings: settings)).whdload ?? WHDLoadInfo()
    }

    // MARK: Saved configurations

    /// current is the saved configuration being edited: the one selected,
    /// else the one loaded.
    var current: String? { selection ?? (saved.contains(name) ? name : nil) }

    func load(_ n: String) {
        guard n != name || !saved.contains(n) else { return }
        apply(bridge.call(Request(op: "load", name: n)))
    }

    func save() {
        let r = bridge.call(Request(op: "save", name: name, settings: settings))
        apply(r)
    }

    func newConfiguration() {
        let r = bridge.call(Request(op: "defaults"))
        if let s = r.settings {
            settings = s
        }
        name = ""
        selection = nil
        say("New configuration: the default A500 settings")
    }

    func delete(_ n: String) {
        let r = bridge.call(Request(op: "delete", name: n))
        if r.ok, name == n {
            name = ""
        }
        apply(r)
    }

    func applyPreset(_ p: String) {
        apply(bridge.call(Request(op: "preset", settings: settings, preset: p)))
    }

    // choose puts a file or folder where target says (applyChoice in Go).
    func choose(_ target: String, _ url: URL) {
        apply(bridge.call(Request(op: "choose", settings: settings, target: target, path: bridge.settingsPath(url))))
    }

    // MARK: Running the machine

    func start() {
        guard !running else { return }
        let r = bridge.call(Request(op: "start", name: name, settings: settings))
        guard r.ok, let conf = r.config else {
            say(r.error ?? "Can't start", error: true)
            if rom.mismatch != nil || settings.rom.isEmpty {
                tab = .machine
            }
            return
        }
        runner.run(config: conf, name: name, model: self)
    }

    func stopMachine() {
        runner.stop()
    }
}
