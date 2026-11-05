// MachineBar is the running machine's controls on the iPad, in place of
// the desktop's status bar (amiga/display_status.go): the activity
// lights, the keyboard, the joystick, Reset, Power and Settings, floating over the
// bottom of the picture. It fades to a small handle when left alone, and
// the machine's notes show above it.
import Blitterbox
import SwiftUI

/// MachineStatus is the machine's status bar (amiga.Status, from
/// BlitterboxStatus).
struct MachineStatus: Decodable, Equatable {
    struct Light: Decodable, Equatable, Identifiable {
        var label: String
        var unit: Int
        var track: Int
        var disk: String?
        var state: String? // power, dim, read, write; nil off
        var id: String { label }
    }
    var lights: [Light] = []
    var message: String?
    var paused = false
    var warp = false
}

@MainActor @Observable
final class MachineBarModel {
    private(set) var status: MachineStatus?
    private(set) var shown = true
    private(set) var fading = false // the controls fading out, before the handle replaces them
    var keyboardShown = false
    var toggleKeyboard: () -> Void = {}
    var joystickShown = false
    var toggleJoystick: () -> Void = {}
    var holding = false { // a dialog is up: stay
        didSet { holding ? hide?.cancel() : stayAWhile() }
    }

    private var timer: Timer?
    private var hide: Task<Void, Never>?

    /// start polls the machine's status ten times a second.
    func start() {
        timer = Timer.scheduledTimer(withTimeInterval: 0.1, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated { self?.poll() }
        }
    }

    private func poll() {
        let json = BlitterboxStatus()
        let s = json.isEmpty ? nil : try? JSONDecoder().decode(MachineStatus.self, from: Data(json.utf8))
        if s == status { return }
        if status == nil, s != nil {
            show() // switched on: the controls show a while
        }
        withAnimation(.easeOut(duration: 0.2)) { status = s }
    }

    func show() {
        withAnimation(.snappy) { (shown, fading) = (true, false) }
        stayAWhile()
    }

    /// stayAWhile keeps the controls for a few seconds more.
    func stayAWhile() {
        hide?.cancel()
        hide = Task { [weak self] in
            try? await Task.sleep(for: .seconds(5))
            guard !Task.isCancelled, let self, !self.holding else { return }
            // The controls fade where they are; only then does the handle
            // take their place, as the bar's size changing mid-fade made
            // them jump.
            withAnimation(.easeInOut(duration: 0.4)) {
                self.fading = true
            } completion: {
                guard self.fading else { return } // shown again meanwhile
                withAnimation(.easeIn(duration: 0.2)) { (self.shown, self.fading) = (false, false) }
            }
        }
    }
}

struct MachineBar: View {
    @Bindable var bar: MachineBarModel
    @State private var confirming: Confirm?

    enum Confirm { case power, settings }

    var body: some View {
        VStack(spacing: 10) {
            if let s = bar.status {
                if let note = note(s) {
                    Text(note)
                        .font(.callout.weight(.medium))
                        .padding(.horizontal, 14).padding(.vertical, 8)
                        .background(.regularMaterial, in: Capsule())
                        .transition(.opacity.combined(with: .move(edge: .bottom)))
                }
                if bar.shown {
                    controls(s)
                        .opacity(bar.fading ? 0 : 1)
                        .scaleEffect(bar.fading ? 0.95 : 1, anchor: .bottom)
                        .transition(.opacity.combined(with: .scale(scale: 0.9, anchor: .bottom)))
                } else {
                    Button { bar.show() } label: {
                        Image(systemName: "chevron.compact.up")
                            .font(.title2.weight(.semibold))
                            .frame(width: 64, height: 28)
                            .background(.ultraThinMaterial.opacity(0.7), in: Capsule())
                            .frame(width: 88, height: 44) // a finger's target
                            .contentShape(Rectangle())
                    }
                    .accessibilityLabel("Show the controls")
                    .transition(.opacity)
                }
            }
        }
        // Centred at the foot of the space it is given, while that is
        // catching up with its size.
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .bottom)
        .buttonStyle(.plain)
        .foregroundStyle(.white)
        .environment(\.colorScheme, .dark)
        .confirmationDialog(confirming == .power ? "Switch the Amiga off and on?" : "Switch the Amiga off and change its settings?",
                            isPresented: Binding(get: { confirming != nil }, set: { if !$0 { confirming = nil } }),
                            titleVisibility: .visible) {
            switch confirming {
            case .power:
                Button("Power Cycle", systemImage: "power", role: .destructive) { BlitterboxPowerCycle() }
            case .settings:
                Button("Switch Off", systemImage: "gearshape", role: .destructive) { BlitterboxChangeSettings() }
            case nil:
                EmptyView()
            }
        }
        .onChange(of: confirming) { _, c in bar.holding = c != nil }
    }

    private func note(_ s: MachineStatus) -> String? {
        let notes = [s.paused ? "Paused" : nil, s.warp ? "Warp" : nil, s.message].compactMap { $0 }
        return notes.isEmpty ? nil : notes.joined(separator: " · ")
    }

    private func controls(_ s: MachineStatus) -> some View {
        HStack(spacing: 16) {
            ForEach(s.lights) { l in
                if l.unit >= 0 {
                    // A drive: tap to insert a disk, touch and hold to eject it.
                    Button { tapped { BlitterboxInsertDisk(l.unit) } } label: { DriveLight(light: l) }
                        .contextMenu {
                            Button("Insert Disk…", systemImage: "square.and.arrow.down") { BlitterboxInsertDisk(l.unit) }
                            Button("Eject", systemImage: "eject") { BlitterboxEject(l.unit) }
                                .disabled(l.disk == nil)
                        }
                        .accessibilityLabel("DF\(l.unit): \(l.disk ?? "empty")")
                        .accessibilityHint("Insert a disk")
                } else {
                    ActivityLight(light: l)
                }
            }
            Rectangle().fill(.white.opacity(0.25)).frame(width: 1, height: 28)
            ControlButton(bar.keyboardShown ? "keyboard.chevron.compact.down" : "keyboard",
                          bar.keyboardShown ? "Hide the keyboard" : "Show the keyboard") { tapped(bar.toggleKeyboard) }
            ControlButton(bar.joystickShown ? "gamecontroller.fill" : "gamecontroller",
                          bar.joystickShown ? "Hide the joystick" : "Show the joystick") { tapped(bar.toggleJoystick) }
            ControlButton("arrow.counterclockwise", "Reset") { tapped { BlitterboxReset() } }
            ControlButton("power", "Power cycle", tint: .red) { confirming = .power }
            ControlButton("gearshape", "Settings") { confirming = .settings }
        }
        .padding(.leading, 18).padding(.trailing, 8).padding(.vertical, 6)
        .background(.regularMaterial, in: Capsule())
        .overlay(Capsule().strokeBorder(.white.opacity(0.12)))
        .shadow(color: .black.opacity(0.35), radius: 12, y: 4)
    }

    /// tapped does what a control does, and keeps the controls a while.
    private func tapped(_ action: () -> Void) {
        action()
        bar.stayAWhile()
    }
}

/// ControlButton is one of the bar's round buttons.
private struct ControlButton: View {
    let symbol: String
    let label: String
    var tint: Color = .white
    let action: () -> Void

    init(_ symbol: String, _ label: String, tint: Color = .white, action: @escaping () -> Void) {
        (self.symbol, self.label, self.tint, self.action) = (symbol, label, tint, action)
    }

    var body: some View {
        Button(action: action) {
            Image(systemName: symbol)
                .font(.title3.weight(.medium))
                .foregroundStyle(tint)
                .frame(width: 44, height: 44)
                .background(.white.opacity(0.08), in: Circle())
                .contentShape(Circle())
        }
        .accessibilityLabel(label)
    }
}

/// LED is an activity light's lamp.
private struct LED: View {
    let state: String?

    var body: some View {
        let c: Color? = switch state {
        case "power": .red
        case "dim": Color(red: 0.55, green: 0.13, blue: 0.11)
        case "read": .green
        case "write": .orange
        default: nil
        }
        Capsule()
            .fill(c ?? .white.opacity(0.12))
            .overlay(Capsule().strokeBorder(.white.opacity(c == nil ? 0.2 : 0)))
            .frame(width: 16, height: 6)
            .shadow(color: c ?? .clear, radius: c == nil ? 0 : 4)
    }
}

private struct ActivityLight: View {
    let light: MachineStatus.Light

    var body: some View {
        VStack(spacing: 5) {
            LED(state: light.state)
            Text(light.label == "POWER" ? "Power" : light.label)
                .font(.caption2.weight(.semibold))
                .foregroundStyle(.secondary)
        }
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(light.label)
    }
}

private struct DriveLight: View {
    let light: MachineStatus.Light

    var body: some View {
        HStack(spacing: 8) {
            VStack(spacing: 5) {
                LED(state: light.state)
                Text(String(format: "%02d", light.track))
                    .font(.caption2.monospacedDigit())
                    .foregroundStyle(.secondary)
            }
            VStack(alignment: .leading, spacing: 1) {
                Text(light.label).font(.caption.weight(.bold))
                Text(light.disk ?? "Empty")
                    .font(.caption2)
                    .foregroundStyle(light.disk == nil ? .tertiary : .secondary)
                    .lineLimit(1).truncationMode(.middle)
                    .frame(maxWidth: 120, alignment: .leading)
            }
        }
        .padding(.horizontal, 10).frame(height: 44)
        .background(.white.opacity(0.08), in: Capsule())
        .contentShape(Capsule())
    }
}
