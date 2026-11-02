import UIKit

/// AmigaKeyboardView is an on-screen Amiga keyboard. Keys go down when
/// touched and up when let go, as real ones do, so they can be held (for
/// games) and several at once. Shift, Ctrl, Alt and the Amiga keys latch
/// when tapped and let go after the next key; tapping a latched one lets
/// it go. Caps Lock stays down until tapped again, like the real key.
///
/// It looks like the A500's: light character keys, darker special ones,
/// on a beige case (charcoal in dark mode). A key drops when pressed, a
/// held one is lit, Caps Lock has its green light, and a character key
/// pops up above the finger.
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
        [k("Esc", 0x45), gap(0.5)] + (1...10).map { k("F\($0)", 0x4F + $0) } + [gap(0.5), k("Del", 0x46), k("Help", 0x5F),
         k("PrtSc", 0x5D)], // keypad *: WHDLoad's usual quit key
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

    private var buttons: [(KeyCap, Key)] = []
    private let preview = KeyPreview()
    private var previewing: Int? // the key the preview shows
    private var latched: Set<Int> = [] // modifiers held by a tap
    private var held: Set<Int> = [] // every key the Amiga has down

    override init(frame: CGRect) {
        super.init(frame: frame)
        backgroundColor = Palette.chassis
        isMultipleTouchEnabled = true
        for row in Self.rows {
            for key in row where key.code >= 0 {
                let b = KeyCap(label: key.label, special: Self.isSpecial(key), light: key.code == Self.capsLock)
                b.isExclusiveTouch = false
                b.accessibilityLabel = Self.accessibilityName(key)
                b.addAction(UIAction { [weak self] _ in self?.press(key) }, for: .touchDown)
                b.addAction(UIAction { [weak self] _ in self?.release(key) }, for: [.touchUpInside, .touchUpOutside, .touchCancel])
                addSubview(b)
                buttons.append((b, key))
            }
        }
        addSubview(preview)
        refresh()
    }

    /// isSpecial: the keys the A500 has in darker grey, everything but the
    /// characters and the space bar.
    private static func isSpecial(_ key: Key) -> Bool {
        key.code != 0x40 && (key.label.count != 1 || (0x4C...0x4F).contains(key.code))
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
                    // The cap, with room below for its side; placed by
                    // bounds and centre, as a pressed key is moved.
                    var f = CGRect(x: x, y: 6 + CGFloat(r) * height, width: key.width * unit, height: height)
                        .insetBy(dx: 2, dy: 2)
                    f.size.height -= KeyCap.lip
                    let b = buttons[i].0
                    b.bounds = CGRect(origin: .zero, size: f.size)
                    b.center = CGPoint(x: f.midX, y: f.midY)
                    i += 1
                }
                x += key.width * unit
            }
        }
    }

    private func press(_ key: Key) {
        showPreview(key)
        if Self.modifiers.contains(key.code) || key.code == Self.capsLock {
            return // these act when let go: a tap toggles them
        }
        send(key.code, true)
        refresh()
    }

    private func release(_ key: Key) {
        if previewing == key.code {
            preview.isHidden = true
            previewing = nil
        }
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
            b.isDown = held.contains(key.code)
        }
    }

    /// showPreview pops a character key up above the finger.
    private func showPreview(_ key: Key) {
        guard !Self.isSpecial(key), key.code != 0x40,
              let cap = buttons.first(where: { $0.1.code == key.code })?.0 else { return }
        previewing = key.code
        preview.show(key.label, above: cap.frame, in: bounds)
        bringSubviewToFront(preview)
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

/// Palette is the keyboard's colours: the A500's beige, or charcoal in
/// dark mode.
private enum Palette {
    static func dynamic(_ light: UIColor, _ dark: UIColor) -> UIColor {
        UIColor { $0.userInterfaceStyle == .dark ? dark : light }
    }
    static func rgb(_ hex: UInt32) -> UIColor {
        UIColor(red: CGFloat(hex >> 16 & 0xFF) / 255, green: CGFloat(hex >> 8 & 0xFF) / 255,
                blue: CGFloat(hex & 0xFF) / 255, alpha: 1)
    }
    static let chassis = dynamic(rgb(0xCFC8B3), rgb(0x141517))
    static let key = dynamic(rgb(0xF1EDE2), rgb(0x3B3C41))
    static let special = dynamic(rgb(0xA9A496), rgb(0x2A2B2F))
    static let side = dynamic(rgb(0x8C8676), rgb(0x0B0B0C)) // a key's lower edge
    static let legend = dynamic(rgb(0x2A2925), UIColor(white: 0.92, alpha: 1))
    static let held = dynamic(rgb(0xF0A43A), rgb(0xD9822B))
}

/// KeyCap is one key: its cap sits on its side, and drops onto it when
/// pressed or held.
private final class KeyCap: UIButton {
    private let special: Bool
    private let led: UIView?
    static let lip: CGFloat = 3 // the side showing below the cap

    var isDown = false { didSet { update() } }
    override var isHighlighted: Bool { didSet { update() } }

    init(label: String, special: Bool, light: Bool) {
        self.special = special
        self.led = light ? UIView() : nil
        super.init(frame: .zero)
        setTitle(label, for: .normal)
        titleLabel?.font = label.count == 1 && !special
            ? .systemFont(ofSize: 19, weight: .regular)
            : .systemFont(ofSize: 14, weight: .medium)
        titleLabel?.adjustsFontSizeToFitWidth = true
        titleLabel?.minimumScaleFactor = 0.6
        layer.cornerRadius = 6
        layer.cornerCurve = .continuous
        layer.shadowOpacity = 1
        layer.shadowRadius = 0
        if let led {
            led.layer.cornerRadius = 3
            led.isUserInteractionEnabled = false
            addSubview(led)
        }
        update()
        // The layer's colours aren't dynamic.
        registerForTraitChanges([UITraitUserInterfaceStyle.self]) { (cap: KeyCap, _) in cap.update() }
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError() }

    override func layoutSubviews() {
        super.layoutSubviews()
        led?.frame = CGRect(x: 8, y: 7, width: 12, height: 6)
        layer.shadowPath = UIBezierPath(roundedRect: bounds, cornerRadius: 6).cgPath
    }


    private func update() {
        let down = isDown || isHighlighted
        let t = traitCollection
        backgroundColor = isDown ? Palette.held : (special ? Palette.special : Palette.key)
        setTitleColor(Palette.legend.resolvedColor(with: t), for: .normal)
        layer.shadowColor = Palette.side.resolvedColor(with: t).cgColor
        layer.shadowOffset = CGSize(width: 0, height: down ? 1 : Self.lip)
        transform = CGAffineTransform(translationX: 0, y: down ? Self.lip - 1 : 0)
        led?.backgroundColor = isDown ? UIColor(red: 0.3, green: 0.95, blue: 0.35, alpha: 1)
            : UIColor(white: 0, alpha: 0.18)
        led?.layer.shadowColor = UIColor.green.cgColor
        led?.layer.shadowOpacity = isDown ? 0.8 : 0
        led?.layer.shadowRadius = 3
        led?.layer.shadowOffset = .zero
    }
}

/// KeyPreview is a character key popped up above the finger, as the
/// iPad's own keyboard does on a phone.
private final class KeyPreview: UIView {
    private let label = UILabel()

    init() {
        super.init(frame: .zero)
        isHidden = true
        isUserInteractionEnabled = false
        backgroundColor = Palette.key
        layer.cornerRadius = 10
        layer.cornerCurve = .continuous
        layer.shadowColor = UIColor.black.cgColor
        layer.shadowOpacity = 0.35
        layer.shadowRadius = 6
        layer.shadowOffset = CGSize(width: 0, height: 2)
        label.font = .systemFont(ofSize: 34, weight: .regular)
        label.textAlignment = .center
        label.textColor = Palette.legend
        addSubview(label)
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError() }

    /// show puts the label in a bubble above key (in the keyboard, whose
    /// bounds keep it on screen).
    func show(_ text: String, above key: CGRect, in bounds: CGRect) {
        label.text = text
        let w = max(key.width * 1.4, 54), h = key.height * 1.4
        var f = CGRect(x: key.midX - w / 2, y: key.minY - h - 6, width: w, height: h)
        f.origin.x = min(max(f.minX, bounds.minX + 2), bounds.maxX - w - 2)
        f.origin.y = max(f.minY, bounds.minY - h) // may rise over the picture
        frame = f
        label.frame = CGRect(origin: .zero, size: f.size)
        isHidden = false
    }
}
