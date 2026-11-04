// Touches, and the Magic Keyboard's trackpad (or any mouse), for the
// Amiga. The app gives Go every touch itself (EbitenController), and
// reads the trackpad through GameController's GCMouse, which Ebitengine
// can't: it sees a trackpad only as touches where it clicks.
//
// The trackpad works as the desktop's mouse does. Free, it is the iPad's
// pointer and clicks the status bar; a click in the picture traps it,
// and the Amiga has it (hidden and locked, so it never stops at the
// screen's edge) until Esc or F12.
import GameController
import Blitterbox
import SwiftUI
import UIKit

/// Pointer hands every connected mouse's motion and buttons to Go, which
/// uses them while the pointer is trapped.
enum Pointer {
    private static var observer: NSObjectProtocol?

    static func start() {
        GCMouse.mice().forEach(attach)
        observer = NotificationCenter.default.addObserver(forName: .GCMouseDidConnect, object: nil, queue: .main) { note in
            if let mouse = note.object as? GCMouse { attach(mouse) }
        }
    }

    private static func attach(_ mouse: GCMouse) {
        guard let input = mouse.mouseInput else { return }
        // GCMouse's y grows upward; the screen's downward.
        input.mouseMovedHandler = { _, dx, dy in BlitterboxPointerMove(Double(dx), Double(-dy)) }
        input.leftButton.pressedChangedHandler = { _, _, down in BlitterboxPointerButton(0, down) }
        input.rightButton?.pressedChangedHandler = { _, _, down in BlitterboxPointerButton(1, down) }
        input.middleButton?.pressedChangedHandler = { _, _, down in BlitterboxPointerButton(2, down) }
    }
}

/// RootController is the window's root: the Amiga's screen under the
/// settings. It locks the pointer while the Amiga has it trapped.
final class RootController: UIHostingController<AnyView> {
    private let model: LauncherModel
    private var trapped = false

    init(model: LauncherModel) {
        self.model = model
        super.init(rootView: AnyView(RootView().environment(model)))
        watchRunning()
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError() }

    override var prefersPointerLocked: Bool { model.running && trapped }

    /// lockPointer hides and holds the pointer (trapped) or lets it go.
    func lockPointer(_ lock: Bool) {
        trapped = lock
        setNeedsUpdateOfPrefersPointerLocked()
        guard lock else { return }
        // The iPad locks it only for an app that fills the screen.
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) { [weak self] in
            guard let self, self.trapped, self.view.window?.windowScene?.pointerLockState?.isLocked == false else { return }
            BlitterboxSay("The iPad hides the pointer only with BlitterBox full screen. Esc or F12 frees it")
        }
    }

    private func watchRunning() {
        withObservationTracking { _ = model.running } onChange: { [weak self] in
            DispatchQueue.main.async {
                guard let self else { return }
                if !self.model.running { self.trapped = false }
                self.setNeedsUpdateOfPrefersPointerLocked()
                self.watchRunning()
            }
        }
    }
}

/// EbitenController is Ebitengine's view controller with its touches
/// given to Go by the app: fingers as touches (every event, so none is
/// missed between frames), the pointer's as clicks.
///
/// It takes the hardware keyboard by being first responder: UIKit gives
/// key presses to the first responder and its parents, and Ebitengine
/// reads them in its pressesBegan. Nested under SwiftUI's hosting
/// controller, it was never in that chain, so a Magic Keyboard typed
/// nothing on the Amiga.
final class EbitenController: BlitterboxEbitenViewController {
    private var ids: [ObjectIdentifier: Int] = [:]
    private var nextID = 1

    override var canBecomeFirstResponder: Bool { true }

    /// takeKeys makes the hardware keyboard the Amiga's.
    func takeKeys() {
        if !isFirstResponder { becomeFirstResponder() }
    }

    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        takeKeys()
    }

    private func handle(_ touches: Set<UITouch>, _ event: UIEvent?) {
        for touch in touches {
            let p = touch.location(in: touch.view)
            let up = touch.phase == .ended || touch.phase == .cancelled
            if touch.type == .indirectPointer {
                guard touch.phase == .began || up else { continue }
                let right = event?.buttonMask.contains(.secondary) ?? false
                BlitterboxPointerClick(p.x, p.y, right, !up)
                continue
            }
            let key = ObjectIdentifier(touch)
            let id = ids[key] ?? nextID
            if ids[key] == nil {
                ids[key] = id
                nextID += 1
            }
            BlitterboxTouch(id, touch.phase == .began ? 0 : up ? 2 : 1, p.x, p.y)
            if up { ids[key] = nil }
        }
    }

    override func touchesBegan(_ touches: Set<UITouch>, with event: UIEvent?) {
        takeKeys() // back from wherever the settings or controls took them
        handle(touches, event)
    }
    override func touchesMoved(_ touches: Set<UITouch>, with event: UIEvent?) { handle(touches, event) }
    override func touchesEnded(_ touches: Set<UITouch>, with event: UIEvent?) { handle(touches, event) }
    override func touchesCancelled(_ touches: Set<UITouch>, with event: UIEvent?) { handle(touches, event) }
}
