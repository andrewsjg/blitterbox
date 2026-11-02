// goAmiga for iPadOS: the Go emulator (../../mobile, built into
// Goamiga.xcframework by scripts/ios-framework.sh) runs inside Ebitengine's
// view controller, set up on the configuration screen it shares with the
// Mac (Launcher.swift). ROMs, disks and hardfiles live in the app's
// Documents folder, which the Files app shows as "On My iPad > goAmiga".
import Goamiga
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
        if !GoamigaSetDocuments(Documents.url.path, &error) {
            print("goAmiga: \(error?.localizedDescription ?? "can't use Documents")")
        }
        let model = MainActor.assumeIsolated { LauncherModel() }
        let host = AppHost(model: model)
        GoamigaSetHost(host)
        self.host = host

        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: RootView().environment(model))
        window.makeKeyAndVisible()
        self.window = window
    }
}

/// AmigaViewController shows the Amiga's display (Ebitengine's view
/// controller, as a child) with the on-screen keyboard docked below it
/// when shown; the picture shrinks to fit above the keyboard.
final class AmigaViewController: UIViewController {
    private let amiga = GoamigaEbitenViewController()
    private let keyboard = AmigaKeyboardView()
    private let keyboardButton = UIButton(type: .system)
    private var keyboardShown: NSLayoutConstraint!
    private var keyboardHidden: NSLayoutConstraint!

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .black

        addChild(amiga)
        view.addSubview(amiga.view)
        amiga.didMove(toParent: self)
        view.addSubview(keyboard)
        keyboard.onKey = { code, down in GoamigaKey(code, down) }

        var config = UIButton.Configuration.filled()
        config.image = UIImage(systemName: "keyboard")
        config.baseBackgroundColor = UIColor(white: 0.2, alpha: 0.7)
        config.baseForegroundColor = .white
        config.cornerStyle = .capsule
        keyboardButton.configuration = config
        keyboardButton.accessibilityLabel = "Keyboard"
        keyboardButton.addAction(UIAction { [weak self] _ in self?.toggleKeyboard() }, for: .primaryActionTriggered)
        view.addSubview(keyboardButton)

        for v in [amiga.view!, keyboard, keyboardButton] {
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
            keyboardButton.topAnchor.constraint(equalTo: safe.topAnchor, constant: 8),
            keyboardButton.trailingAnchor.constraint(equalTo: safe.trailingAnchor, constant: -8),
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
        UserDefaults.standard.set(show, forKey: "keyboardShown")
    }
}
