// BlitterBox for iPadOS: the Go emulator (../../mobile, built into
// Blitterbox.xcframework by scripts/ios-framework.sh) runs inside Ebitengine's
// view controller, set up on the configuration screen it shares with the
// Mac (Launcher.swift). ROMs, disks and hardfiles live in the app's
// Documents folder, which the Files app shows as "On My iPad > BlitterBox".
import Blitterbox
import SwiftUI
import UIKit

@main
final class AppDelegate: UIResponder, UIApplicationDelegate {}

// SceneDelegate makes the window: apps built with the iOS 27 SDK must use
// the scene life cycle (Info.plist's UIApplicationSceneManifest names it).
final class SceneDelegate: UIResponder, UIWindowSceneDelegate {
    var window: UIWindow?
    private var host: AppHost?

    func scene(_ scene: UIScene, willConnectTo session: UISceneSession,
               options connectionOptions: UIScene.ConnectionOptions) {
        guard let scene = scene as? UIWindowScene else { return }
        // Settings name files relative to Documents; Go must be there
        // before the model asks it anything.
        var error: NSError?
        if !BlitterboxSetDocuments(Documents.url.path, &error) {
            print("BlitterBox: \(error?.localizedDescription ?? "can't use Documents")")
        }
        let model = MainActor.assumeIsolated { LauncherModel() }
        let host = AppHost(model: model)
        BlitterboxSetHost(host)
        self.host = host
        Pointer.start()

        let window = UIWindow(windowScene: scene)
        let root = RootController(model: model)
        host.root = root
        window.rootViewController = root
        window.makeKeyAndVisible()
        self.window = window
    }
}

/// AmigaViewController shows the Amiga's display (Ebitengine's view
/// controller, as a child) with the on-screen keyboard, slightly
/// see-through, over the picture's foot when shown, and the machine's
/// controls (MachineBar) floating above it; the picture keeps its size.
/// The on-screen joystick (Joystick.swift) lies over the picture too,
/// when shown, where it was left.
final class AmigaViewController: UIViewController {
    private let amiga = EbitenController()
    private let keyboard = AmigaKeyboardView()
    private let stick = StickPad()
    private lazy var pads: [MovablePad] = [stick, ButtonsPad()]
    private let bar = MachineBarModel()
    private lazy var barHost = UIHostingController(rootView: MachineBar(bar: bar))
    private var keyboardShown: NSLayoutConstraint!
    private var keyboardHidden: NSLayoutConstraint!

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .black

        addChild(amiga)
        view.addSubview(amiga.view)
        amiga.didMove(toParent: self)
        for p in pads {
            p.isHidden = true
            view.addSubview(p)
        }
        view.addSubview(keyboard)
        keyboard.alpha = 0.8
        keyboard.layer.cornerRadius = 12 // it no longer reaches the edges
        keyboard.layer.maskedCorners = [.layerMinXMinYCorner, .layerMaxXMinYCorner]
        keyboard.onKey = { code, down in BlitterboxKey(code, down) }

        bar.toggleKeyboard = { [weak self] in self?.toggleKeyboard() }
        bar.toggleJoystick = { [weak self] in self?.showJoystick(!(self?.bar.joystickShown ?? true)) }
        bar.joystickDPad = stick.dpad
        bar.chooseDPad = { [weak self] dpad in
            guard let self else { return }
            stick.dpad = dpad
            bar.joystickDPad = dpad
            showJoystick(true)
        }
        barHost.sizingOptions = .intrinsicContentSize
        barHost.view.backgroundColor = .clear
        addChild(barHost)
        view.addSubview(barHost.view)
        barHost.didMove(toParent: self)
        bar.start()

        for v in [amiga.view!, keyboard, barHost.view!] {
            v.translatesAutoresizingMaskIntoConstraints = false
        }
        let safe = view.safeAreaLayoutGuide
        keyboardShown = keyboard.bottomAnchor.constraint(equalTo: safe.bottomAnchor)
        keyboardHidden = keyboard.topAnchor.constraint(equalTo: view.bottomAnchor)
        NSLayoutConstraint.activate([
            amiga.view.topAnchor.constraint(equalTo: view.topAnchor),
            amiga.view.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            amiga.view.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            amiga.view.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            // A little narrower than the screen, so it hides less of the
            // picture; the keys keep their shape.
            keyboard.centerXAnchor.constraint(equalTo: safe.centerXAnchor),
            keyboard.widthAnchor.constraint(equalTo: safe.widthAnchor, multiplier: 0.85),
            keyboard.heightConstraint(),
            keyboardHidden,
            // The controls: centred above the keyboard, clear of the home
            // indicator.
            barHost.view.centerXAnchor.constraint(equalTo: safe.centerXAnchor),
            barHost.view.widthAnchor.constraint(lessThanOrEqualTo: safe.widthAnchor, constant: -16),
            barHost.view.bottomAnchor.constraint(lessThanOrEqualTo: keyboard.topAnchor, constant: -8),
            barHost.view.bottomAnchor.constraint(lessThanOrEqualTo: safe.bottomAnchor, constant: -8),
            {
                let c = barHost.view.bottomAnchor.constraint(equalTo: keyboard.topAnchor, constant: -8)
                c.priority = .defaultHigh
                return c
            }(),
        ])
        if UserDefaults.standard.bool(forKey: "keyboardShown") {
            showKeyboard(true)
        }
        showJoystick(UserDefaults.standard.bool(forKey: "joystickShown"))
    }

    override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        let space = view.bounds.inset(by: view.safeAreaInsets)
        for p in pads { p.place(in: space) }
    }

    /// takeKeys gives the hardware keyboard to the Amiga.
    func takeKeys() { amiga.takeKeys() }

    override var prefersStatusBarHidden: Bool { true }
    override var prefersHomeIndicatorAutoHidden: Bool { true }

    private func toggleKeyboard() {
        showKeyboard(!keyboardShown.isActive)
        UIView.animate(withDuration: 0.25) { self.view.layoutIfNeeded() }
    }

    /// showKeyboard slides the keyboard up over the picture or hides it, and
    /// remembers which for next time.
    private func showKeyboard(_ show: Bool) {
        if !show {
            keyboard.releaseAll()
        }
        keyboardHidden.isActive = !show
        keyboardShown.isActive = show
        bar.keyboardShown = show
        UserDefaults.standard.set(show, forKey: "keyboardShown")
    }

    /// showJoystick shows the on-screen joystick or hides it, letting go of
    /// it, and remembers which for next time.
    private func showJoystick(_ show: Bool) {
        for p in pads {
            if !show { p.letGo() }
            p.isHidden = !show
        }
        bar.joystickShown = show
        UserDefaults.standard.set(show, forKey: "joystickShown")
    }
}
