// The Magic Keyboard's trackpad (or any mouse) as the Amiga's mouse.
// Ebitengine sees a trackpad only as touches where its clicks land, so
// the app reads it through GameController's GCMouse instead (relative
// motion and buttons, as the desktop's grabbed mouse is) and keeps its
// touches from Ebitengine. While the Amiga runs, the pointer is locked:
// hidden and held, so that it never stops at the screen's edge.
import GameController
import Goamiga
import SwiftUI
import UIKit

/// Pointer hands every connected mouse's motion and buttons to the Amiga.
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
        input.mouseMovedHandler = { _, dx, dy in GoamigaPointerMove(Double(dx), Double(-dy)) }
        input.leftButton.pressedChangedHandler = { _, _, down in GoamigaPointerButton(0, down) }
        input.rightButton?.pressedChangedHandler = { _, _, down in GoamigaPointerButton(1, down) }
        input.middleButton?.pressedChangedHandler = { _, _, down in GoamigaPointerButton(2, down) }
    }
}

/// RootController is the window's root: the Amiga's screen under the
/// settings, locking the pointer while the machine runs.
final class RootController: UIHostingController<AnyView> {
    private let model: LauncherModel

    init(model: LauncherModel) {
        self.model = model
        super.init(rootView: AnyView(RootView().environment(model)))
        watchRunning()
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { fatalError() }

    override var prefersPointerLocked: Bool { model.running }

    private func watchRunning() {
        withObservationTracking { _ = model.running } onChange: { [weak self] in
            DispatchQueue.main.async {
                self?.setNeedsUpdateOfPrefersPointerLocked()
                self?.watchRunning()
            }
        }
    }
}

/// EbitenController is Ebitengine's view controller without a pointer's
/// touches (a trackpad's clicks): Pointer gives the Amiga those.
final class EbitenController: GoamigaEbitenViewController {
    private func fingers(_ touches: Set<UITouch>) -> Set<UITouch> {
        touches.filter { $0.type != .indirectPointer }
    }

    override func touchesBegan(_ touches: Set<UITouch>, with event: UIEvent?) {
        super.touchesBegan(fingers(touches), with: event)
    }

    override func touchesMoved(_ touches: Set<UITouch>, with event: UIEvent?) {
        super.touchesMoved(fingers(touches), with: event)
    }

    override func touchesEnded(_ touches: Set<UITouch>, with event: UIEvent?) {
        super.touchesEnded(fingers(touches), with: event)
    }

    override func touchesCancelled(_ touches: Set<UITouch>, with event: UIEvent?) {
        super.touchesCancelled(fingers(touches), with: event)
    }
}
