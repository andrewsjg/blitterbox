// goAmiga for iPadOS, a spike: the Go emulator (../../mobile, built into
// Goamiga.xcframework by scripts/ios-framework.sh) runs inside Ebitengine's
// view controller. It boots the first .rom and .adf in the app's Documents
// folder, which the Files app shows as "On My iPad > goAmiga".
import Goamiga
import UIKit

@main
final class AppDelegate: UIResponder, UIApplicationDelegate {
    var window: UIWindow?

    func application(_ application: UIApplication,
                     didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?) -> Bool {
        let window = UIWindow(frame: UIScreen.main.bounds)
        window.rootViewController = AmigaViewController()
        window.makeKeyAndVisible()
        self.window = window
        return true
    }
}

final class AmigaViewController: GoamigaEbitenViewController {
    private let message = UILabel()

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .black
        message.textColor = .white
        message.numberOfLines = 0
        message.textAlignment = .center
        message.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(message)
        NSLayoutConstraint.activate([
            message.centerXAnchor.constraint(equalTo: view.centerXAnchor),
            message.centerYAnchor.constraint(equalTo: view.centerYAnchor),
            message.widthAnchor.constraint(lessThanOrEqualTo: view.widthAnchor, constant: -40),
        ])
        boot()
    }

    override var prefersStatusBarHidden: Bool { true }
    override var prefersHomeIndicatorAutoHidden: Bool { true }

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
