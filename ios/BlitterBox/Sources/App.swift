// BlitterBox (BlitterBox) for iPadOS: the Go emulator (../../mobile, built into
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
/// controller, as a child) with the on-screen keyboard docked below it
/// when shown, and the machine's controls (MachineBar) floating over the
/// picture's foot; the picture shrinks to fit above the keyboard.
final class AmigaViewController: UIViewController {
    private let amiga = EbitenController()
    private let keyboard = AmigaKeyboardView()
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
        view.addSubview(keyboard)
        keyboard.onKey = { code, down in BlitterboxKey(code, down) }

        bar.toggleKeyboard = { [weak self] in self?.toggleKeyboard() }
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
            amiga.view.bottomAnchor.constraint(equalTo: keyboard.topAnchor),
            keyboard.leadingAnchor.constraint(equalTo: safe.leadingAnchor),
            keyboard.trailingAnchor.constraint(equalTo: safe.trailingAnchor),
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
    }

    override var prefersStatusBarHidden: Bool { true }
    override var prefersHomeIndicatorAutoHidden: Bool { true }

    private func toggleKeyboard() {
        showKeyboard(!keyboardShown.isActive)
        UIView.animate(withDuration: 0.25) { self.view.layoutIfNeeded() }
    }

    /// showKeyboard docks the keyboard below the picture or hides it, and
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
}
