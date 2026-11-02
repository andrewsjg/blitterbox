import UIKit

/// AmigaKeyboardView is an on-screen Amiga keyboard. Keys go down when
/// touched and up when let go, as real ones do, so they can be held (for
/// games) and several at once. Shift, Ctrl, Alt and the Amiga keys latch
/// when tapped and let go after the next key; tapping a latched one lets
/// it go. Caps Lock stays down until tapped again, like the real key.
final class AmigaKeyboardView: UIView {
    /// onKey reports an Amiga raw key code going down or up.
    var onKey: (Int, Bool) -> Void = { _, _ in }

    private struct Key {
        let label: String
        let code: Int // Amiga raw key code; -1 is a gap
        let width: CGFloat // in key widths
    }

    private static func k(_ label: String, _ code: Int, _ width: CGFloat = 1) -> Key {
        Key(label: label, code: code, width: width)
    }

    private static func gap(_ width: CGFloat) -> Key { Key(label: "", code: -1, width: width) }

    private static func run(_ labels: String, from code: Int) -> [Key] {
        labels.enumerated().map { k(String($1), code + $0) }
    }

    // The A500/A1200 layout, 15 key widths across. Codes are from the
    // Amiga Hardware Reference Manual's raw key map.
    private static let rows: [[Key]] = [
        [k("Esc", 0x45), gap(0.5)] + (1...10).map { k("F\($0)", 0x4F + $0) } + [gap(0.5), k("Del", 0x46), k("Help", 0x5F)],
        [k("`", 0x00)] + run("1234567890", from: 0x01) + [k("-", 0x0B), k("=", 0x0C), k("\\", 0x0D), k("⌫", 0x41)],
        [k("Tab", 0x42, 1.5)] + run("QWERTYUIOP", from: 0x10) + [k("[", 0x1A), k("]", 0x1B), k("Return", 0x44, 1.5)],
        [k("Ctrl", 0x63), k("Caps", 0x62, 1.5)] + run("ASDFGHJKL", from: 0x20) + [k(";", 0x29), k("'", 0x2A)],
        [k("Shift", 0x60, 2)] + run("ZXCVBNM", from: 0x31) + [k(",", 0x38), k(".", 0x39), k("/", 0x3A), k("Shift", 0x61), k("↑", 0x4C), gap(1)],
        [k("Alt", 0x64, 1.25), k("Amiga", 0x66, 1.25), k("", 0x40, 6), k("Amiga", 0x67, 1.25), k("Alt", 0x65, 1.25), gap(1),
         k("←", 0x4F), k("↓", 0x4D), k("→", 0x4E)],
    ]
    private static let across: CGFloat = 15
    private static let modifiers: Set<Int> = [0x60, 0x61, 0x63, 0x64, 0x65, 0x66, 0x67]
    private static let capsLock = 0x62

    private var buttons: [(UIButton, Key)] = []
    private var latched: Set<Int> = [] // modifiers held by a tap
    private var held: Set<Int> = [] // every key the Amiga has down

    override init(frame: CGRect) {
        super.init(frame: frame)
        backgroundColor = UIColor(white: 0.12, alpha: 1)
        isMultipleTouchEnabled = true
        for row in Self.rows {
            for key in row where key.code >= 0 {
                let b = UIButton(type: .custom)
                b.setTitle(key.label, for: .normal)
                b.titleLabel?.font = .systemFont(ofSize: 15, weight: .medium)
                b.titleLabel?.adjustsFontSizeToFitWidth = true
                b.setTitleColor(.white, for: .normal)
                b.layer.cornerRadius = 5
                b.isExclusiveTouch = false
                b.accessibilityLabel = Self.accessibilityName(key)
                b.addAction(UIAction { [weak self] _ in self?.press(key) }, for: .touchDown)
                b.addAction(UIAction { [weak self] _ in self?.release(key) }, for: [.touchUpInside, .touchUpOutside, .touchCancel])
                addSubview(b)
                buttons.append((b, key))
            }
        }
        refresh()
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// heightConstraint keeps the keys three quarters as tall as they are
    /// wide, whatever the width.
    func heightConstraint() -> NSLayoutConstraint {
        heightAnchor.constraint(equalTo: widthAnchor, multiplier: 0.75 * CGFloat(Self.rows.count) / Self.across, constant: 12)
    }

    override func layoutSubviews() {
        super.layoutSubviews()
        let unit = (bounds.width - 8) / Self.across
        let height = unit * 0.75
        var i = 0
        for (r, row) in Self.rows.enumerated() {
            var x: CGFloat = 4
            for key in row {
                if key.code >= 0 {
                    buttons[i].0.frame = CGRect(x: x, y: 6 + CGFloat(r) * height, width: key.width * unit, height: height)
                        .insetBy(dx: 2, dy: 2)
                    i += 1
                }
                x += key.width * unit
            }
        }
    }

    private func press(_ key: Key) {
        if Self.modifiers.contains(key.code) || key.code == Self.capsLock {
            return // these act when let go: a tap toggles them
        }
        send(key.code, true)
        refresh()
    }

    private func release(_ key: Key) {
        if Self.modifiers.contains(key.code) {
            if latched.remove(key.code) != nil {
                send(key.code, false)
            } else {
                latched.insert(key.code)
                send(key.code, true)
            }
        } else if key.code == Self.capsLock {
            send(key.code, !held.contains(key.code))
        } else {
            send(key.code, false)
            for code in latched {
                send(code, false)
            }
            latched.removeAll()
        }
        refresh()
    }

    /// releaseAll lets go of every key, as when the keyboard is hidden
    /// (Caps Lock stays as it is).
    func releaseAll() {
        for code in held where code != Self.capsLock {
            send(code, false)
        }
        latched.removeAll()
        refresh()
    }

    private func send(_ code: Int, _ down: Bool) {
        guard held.contains(code) != down else { return }
        if down {
            held.insert(code)
        } else {
            held.remove(code)
        }
        onKey(code, down)
    }

    private func refresh() {
        for (b, key) in buttons {
            b.backgroundColor = held.contains(key.code) ? .systemOrange : UIColor(white: 0.3, alpha: 1)
        }
    }

    private static func accessibilityName(_ key: Key) -> String {
        switch key.code {
        case 0x40: "Space"
        case 0x41: "Backspace"
        case 0x67: "Right Amiga"
        case 0x4C: "Up"
        case 0x4D: "Down"
        case 0x4E: "Right"
        case 0x4F: "Left"
        default: key.label
        }
    }
}
