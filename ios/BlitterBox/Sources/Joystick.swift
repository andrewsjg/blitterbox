// The on-screen joystick, for game port 2: a stick or a D-pad (touch and
// hold the machine bar's joystick button to choose) on the left and its
// two buttons on the right, slightly see-through over the picture. Each part
// has a grip to drag it somewhere else, and stays there.
import Blitterbox
import UIKit

/// MovablePad is one part of the on-screen joystick. Its grip, above it,
/// drags it about; where it was left (as fractions of the space it moves
/// in, so a rotation keeps it in place) is remembered under its name.
/// Only its controls and grip take touches: the rest reaches the picture.
class MovablePad: UIView {
    private let name: String
    private let home: CGPoint // where it starts, as fractions
    private let grip = UIImageView(image: UIImage(systemName: "arrow.up.and.down.and.arrow.left.and.right"))
    private var space = CGRect.zero // where it may go
    private var spot: CGPoint       // where it is, as fractions of space
    static let gripSize: CGFloat = 32

    /// content is the controls' area, below the grip.
    var content: CGRect { CGRect(x: 0, y: Self.gripSize + 6, width: bounds.width, height: bounds.height - Self.gripSize - 6) }

    init(name: String, home: CGPoint, size: CGSize) {
        self.name = name
        self.home = home
        let saved = UserDefaults.standard.string(forKey: "pad." + name).map(NSCoder.cgPoint(for:))
        spot = saved ?? home
        super.init(frame: CGRect(origin: .zero, size: CGSize(width: size.width, height: size.height + Self.gripSize + 6)))
        alpha = 0.8
        isMultipleTouchEnabled = true

        grip.contentMode = .center
        grip.preferredSymbolConfiguration = .init(pointSize: 14, weight: .semibold)
        grip.tintColor = Palette.legend
        grip.backgroundColor = Palette.chassis
        grip.layer.cornerRadius = Self.gripSize / 2
        grip.frame = CGRect(x: (bounds.width - Self.gripSize) / 2, y: 0, width: Self.gripSize, height: Self.gripSize)
        grip.isUserInteractionEnabled = true
        grip.addGestureRecognizer(UIPanGestureRecognizer(target: self, action: #selector(drag)))
        grip.addGestureRecognizer({
            let t = UITapGestureRecognizer(target: self, action: #selector(goHome))
            t.numberOfTapsRequired = 2
            return t
        }())
        grip.accessibilityLabel = "Move"
        grip.accessibilityHint = "Drag to move; double-tap to put back"
        addSubview(grip)
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError() }

    /// place puts the pad where it was left, within space.
    func place(in space: CGRect) {
        self.space = space
        center = clamp(CGPoint(x: space.minX + spot.x * space.width, y: space.minY + spot.y * space.height))
    }

    private func clamp(_ p: CGPoint) -> CGPoint {
        let w = bounds.width / 2, h = bounds.height / 2
        return CGPoint(x: min(max(p.x, space.minX + w), max(space.maxX - w, space.minX + w)),
                       y: min(max(p.y, space.minY + h), max(space.maxY - h, space.minY + h)))
    }

    @objc private func drag(_ pan: UIPanGestureRecognizer) {
        center = clamp(CGPoint(x: center.x + pan.translation(in: superview).x, y: center.y + pan.translation(in: superview).y))
        pan.setTranslation(.zero, in: superview)
        guard space.width > 0, space.height > 0 else { return }
        // Kept as it goes, as a layout meanwhile places the pad again.
        spot = CGPoint(x: (center.x - space.minX) / space.width, y: (center.y - space.minY) / space.height)
        if pan.state == .ended || pan.state == .cancelled {
            UserDefaults.standard.set(NSCoder.string(for: spot), forKey: "pad." + name)
        }
    }

    @objc private func goHome() {
        spot = home
        UserDefaults.standard.removeObject(forKey: "pad." + name)
        UIView.animate(withDuration: 0.25) { self.place(in: self.space) }
    }

    /// controlsContain: whether p (in the pad) is on one of its controls.
    func controlsContain(_ p: CGPoint) -> Bool { content.contains(p) }

    override func point(inside p: CGPoint, with event: UIEvent?) -> Bool {
        grip.frame.insetBy(dx: -6, dy: -6).contains(p) || controlsContain(p)
    }

    /// mine: the touches on the controls, not the grip's.
    func mine(_ touches: Set<UITouch>) -> [UITouch] { touches.filter { $0.view === self } }

    /// letGo lets go of everything held, as when the pad is hidden.
    func letGo() {}
}

/// StickPad is the joystick's left part, in either of two styles. As a
/// stick, a knob dragged from the middle of its well leans the Amiga's
/// joystick past half way. As a D-pad, a cross whose eight ways (the
/// arms and between them) push it all the way; a finger can roll round.
final class StickPad: MovablePad {
    private let well = UIView()
    private let knob = UIView()
    private let arms = [UIView(), UIView(), UIView(), UIView()] // up, right, down, left
    private let hub = UIView()
    private var finger: UITouch?
    private static let diameter: CGFloat = 160

    /// dpad: the D-pad rather than the stick, remembered for next time.
    var dpad = UserDefaults.standard.bool(forKey: "joystickDPad") {
        didSet {
            letGo()
            UserDefaults.standard.set(dpad, forKey: "joystickDPad")
            showStyle()
        }
    }

    init() {
        super.init(name: "stick", home: CGPoint(x: 0.12, y: 0.62), size: CGSize(width: Self.diameter, height: Self.diameter))
        well.frame = content
        well.backgroundColor = Palette.chassis
        well.layer.cornerRadius = Self.diameter / 2
        well.layer.borderWidth = 1
        well.layer.borderColor = UIColor.black.withAlphaComponent(0.25).cgColor
        knob.bounds = CGRect(x: 0, y: 0, width: 70, height: 70)
        knob.center = well.center
        knob.backgroundColor = Palette.special
        knob.layer.cornerRadius = 35
        knob.layer.shadowColor = UIColor.black.cgColor
        knob.layer.shadowOpacity = 0.35
        knob.layer.shadowRadius = 4
        knob.layer.shadowOffset = CGSize(width: 0, height: 2)

        // The cross: a hub and four arms out to the well's edge, each with
        // its arrow.
        let c = well.center, r = Self.diameter / 2 - 6, t: CGFloat = 54
        hub.frame = CGRect(x: c.x - t / 2, y: c.y - t / 2, width: t, height: t)
        let frames = [CGRect(x: c.x - t / 2, y: c.y - r, width: t, height: r - t / 2 + 1),
                      CGRect(x: c.x + t / 2 - 1, y: c.y - t / 2, width: r - t / 2 + 1, height: t),
                      CGRect(x: c.x - t / 2, y: c.y + t / 2 - 1, width: t, height: r - t / 2 + 1),
                      CGRect(x: c.x - r, y: c.y - t / 2, width: r - t / 2 + 1, height: t)]
        for (i, a) in arms.enumerated() {
            a.frame = frames[i]
            a.layer.cornerRadius = 8
            let arrow = UIImageView(image: UIImage(systemName: ["arrowtriangle.up.fill", "arrowtriangle.right.fill",
                                                                "arrowtriangle.down.fill", "arrowtriangle.left.fill"][i]))
            arrow.tintColor = Palette.legend
            arrow.preferredSymbolConfiguration = .init(pointSize: 13)
            arrow.contentMode = .center
            arrow.frame = a.bounds
            a.addSubview(arrow)
        }
        for v in [well, knob, hub] + arms {
            v.isUserInteractionEnabled = false
            addSubview(v)
        }
        showStyle()
    }

    private func showStyle() {
        knob.isHidden = dpad
        for v in arms + [hub] { v.isHidden = !dpad }
        lightArms(x: 0, y: 0)
    }

    private func lightArms(x: CGFloat, y: CGFloat) {
        let on = [y < 0, x > 0, y > 0, x < 0]
        for (i, a) in arms.enumerated() { a.backgroundColor = on[i] ? Palette.held : Palette.special }
        hub.backgroundColor = Palette.special
    }

    override func controlsContain(_ p: CGPoint) -> Bool {
        hypot(p.x - well.center.x, p.y - well.center.y) <= Self.diameter / 2
    }

    private func lean(to p: CGPoint) {
        let r = Self.diameter / 2
        var v = CGPoint(x: p.x - well.center.x, y: p.y - well.center.y)
        let d = hypot(v.x, v.y)
        if dpad {
            // Eight ways, 45 degrees each, past a small dead middle.
            var x: CGFloat = 0, y: CGFloat = 0
            if d > r * 0.2 {
                let way = (atan2(v.y, v.x) / (.pi / 4)).rounded() * .pi / 4
                x = cos(way).rounded()
                y = sin(way).rounded()
            }
            lightArms(x: x, y: y)
            BlitterboxJoystickStick(Double(x), Double(y))
            return
        }
        if d > r { v = CGPoint(x: v.x * r / d, y: v.y * r / d) }
        knob.center = CGPoint(x: well.center.x + v.x, y: well.center.y + v.y)
        BlitterboxJoystickStick(Double(v.x / r), Double(v.y / r))
    }

    override func letGo() {
        finger = nil
        UIView.animate(withDuration: 0.1) { self.knob.center = self.well.center }
        lightArms(x: 0, y: 0)
        BlitterboxJoystickStick(0, 0)
    }

    override func touchesBegan(_ touches: Set<UITouch>, with event: UIEvent?) {
        guard finger == nil, let t = mine(touches).first else { return }
        finger = t
        lean(to: t.location(in: self))
    }
    override func touchesMoved(_ touches: Set<UITouch>, with event: UIEvent?) {
        if let f = finger, touches.contains(f) { lean(to: f.location(in: self)) }
    }
    override func touchesEnded(_ touches: Set<UITouch>, with event: UIEvent?) {
        if let f = finger, touches.contains(f) { letGo() }
    }
    override func touchesCancelled(_ touches: Set<UITouch>, with event: UIEvent?) { touchesEnded(touches, with: event) }
}

/// ButtonsPad is the two buttons, fire low on the left and the second
/// button high on the right, as a gamepad's. A finger can slide from one
/// to the other, and two fingers hold both.
final class ButtonsPad: MovablePad {
    private let buttons: [UILabel] = [UILabel(), UILabel()]
    private var fingers: [ObjectIdentifier: (UITouch, Int?)] = [:] // each finger and the button under it
    private var down = [false, false]

    init() {
        super.init(name: "buttons", home: CGPoint(x: 0.88, y: 0.62), size: CGSize(width: 190, height: 160))
        let c = content
        let frames = [CGRect(x: c.minX, y: c.maxY - 96, width: 96, height: 96),
                      CGRect(x: c.maxX - 84, y: c.minY, width: 84, height: 84)]
        for (i, b) in buttons.enumerated() {
            b.frame = frames[i]
            b.text = i == 0 ? "FIRE" : "2"
            b.font = .systemFont(ofSize: i == 0 ? 17 : 22, weight: .bold)
            b.textAlignment = .center
            b.textColor = Palette.legend
            b.layer.cornerRadius = frames[i].width / 2
            b.layer.masksToBounds = true
            b.layer.borderWidth = 1
            b.layer.borderColor = UIColor.black.withAlphaComponent(0.25).cgColor
            b.isUserInteractionEnabled = false
            addSubview(b)
        }
        update()
    }

    private func button(at p: CGPoint) -> Int? {
        buttons.firstIndex { hypot(p.x - $0.center.x, p.y - $0.center.y) <= $0.bounds.width / 2 + 8 }
    }

    override func controlsContain(_ p: CGPoint) -> Bool { button(at: p) != nil }

    private func update() {
        var now = [false, false]
        for (_, b) in fingers.values { if let b { now[b] = true } }
        for i in 0..<2 {
            if now[i] != down[i] { BlitterboxJoystickButton(i, now[i]) }
            buttons[i].backgroundColor = now[i] ? Palette.held : Palette.key
        }
        down = now
    }

    override func letGo() {
        fingers = [:]
        update()
    }

    override func touchesBegan(_ touches: Set<UITouch>, with event: UIEvent?) {
        for t in mine(touches) { fingers[ObjectIdentifier(t)] = (t, button(at: t.location(in: self))) }
        update()
    }
    override func touchesMoved(_ touches: Set<UITouch>, with event: UIEvent?) {
        for t in touches where fingers[ObjectIdentifier(t)] != nil {
            fingers[ObjectIdentifier(t)] = (t, button(at: t.location(in: self)))
        }
        update()
    }
    override func touchesEnded(_ touches: Set<UITouch>, with event: UIEvent?) {
        for t in touches { fingers[ObjectIdentifier(t)] = nil }
        update()
    }
    override func touchesCancelled(_ touches: Set<UITouch>, with event: UIEvent?) { touchesEnded(touches, with: event) }
}
