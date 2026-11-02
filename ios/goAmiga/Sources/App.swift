// goAmiga for iPadOS: the Go emulator (../../mobile, built into
// Goamiga.xcframework by scripts/ios-framework.sh) runs inside Ebitengine's
// view controller. It boots the first .rom and .adf in the app's Documents
// folder, which the Files app shows as "On My iPad > goAmiga".
import Goamiga
import UIKit

@main
final class AppDelegate: UIResponder, UIApplicationDelegate {}

// SceneDelegate makes the window: apps built with the iOS 27 SDK must use
// the scene life cycle (Info.plist's UIApplicationSceneManifest names it).
final class SceneDelegate: UIResponder, UIWindowSceneDelegate {
    var window: UIWindow?

    func scene(_ scene: UIScene, willConnectTo session: UISceneSession,
               options connectionOptions: UIScene.ConnectionOptions) {
        guard let scene = scene as? UIWindowScene else { return }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = AmigaViewController()
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
    private let message = UILabel()
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

        message.textColor = .white
        message.numberOfLines = 0
        message.textAlignment = .center
        view.addSubview(message)

        for v in [amiga.view!, keyboard, keyboardButton, message] {
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
            message.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            message.centerYAnchor.constraint(equalTo: view.centerYAnchor),
            message.widthAnchor.constraint(lessThanOrEqualTo: view.widthAnchor, constant: -40),
        ])
        if UserDefaults.standard.bool(forKey: "keyboardShown") {
            showKeyboard(true)
        }
        boot()
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

    private func boot() {
        let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
        let files = ((try? FileManager.default.contentsOfDirectory(at: docs, includingPropertiesForKeys: nil)) ?? [])
            .sorted { $0.lastPathComponent < $1.lastPathComponent }
        guard let rom = files.first(where: { $0.pathExtension.lowercased() == "rom" }) else {
            message.text = "Put a Kickstart .rom (and optionally an .adf) in\nOn My iPad › goAmiga, then reopen the app."
            return
        }
        let adf = files.first(where: { ["adf", "zip"].contains($0.pathExtension.lowercased()) })
        var error: NSError?
        if GoamigaBoot(rom.path, adf?.path ?? "", &error) {
            print("goAmiga: booted \(rom.lastPathComponent) with \(adf?.lastPathComponent ?? "no disk")")
        } else {
            message.text = "Could not start the Amiga:\n\(error?.localizedDescription ?? "unknown error")"
        }
    }
}
