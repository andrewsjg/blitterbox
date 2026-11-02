// The iPad's half of the configuration screen shared with the Mac
// (macos/Sources/goAmigaLauncher: LauncherView, LauncherModel, the tabs):
// its Bridge asks the Go code in-process, its MachineRunner runs the
// machine in this app, and chooseFile shows the document picker.
import Goamiga
import SwiftUI
import UIKit
import UniformTypeIdentifiers

/// Bridge answers the launcher API's requests in-process (launch.Handle).
final class Bridge {
    func call(_ req: Request) -> Response {
        do {
            let json = String(decoding: try JSONEncoder().encode(req), as: UTF8.self)
            return try JSONDecoder().decode(Response.self, from: Data(GoamigaRequest(json).utf8))
        } catch {
            return Response(error: "goAmiga: \(error.localizedDescription)")
        }
    }

    /// settingsPath is how settings name a chosen file: relative to
    /// Documents, so that it survives iOS moving the app's container.
    func settingsPath(_ url: URL) -> String {
        let docs = Documents.url.standardizedFileURL.path + "/"
        let path = url.standardizedFileURL.path
        return path.hasPrefix(docs) ? String(path.dropFirst(docs.count)) : path
    }
}

enum Documents {
    static let url = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0]
}

/// MachineRunner runs the machine in this app: the settings screen covers
/// it while running is false (RootView).
@MainActor
final class MachineRunner {
    func run(config: String, name: String, model: LauncherModel) {
        var error: NSError?
        guard GoamigaStart(config, name, &error) else {
            model.say("Can't start: \(error?.localizedDescription ?? "unknown error")", error: true)
            return
        }
        model.running = true
    }

    func stop() {
        GoamigaStop()
    }
}

/// chooseFile shows the document picker for a file (of these extensions)
/// or a folder. One from outside the app's Documents folder is copied in,
/// where the emulator can open it (and write to it: hardfiles, ADFs).
@MainActor
func chooseFile(title: String, folder: Bool, types: [String] = [], start: String = "",
                cancelled: @escaping () -> Void = {}, then: @escaping (URL) -> Void) {
    let content: [UTType] = folder ? [.folder] : (types.isEmpty ? [.item] : types.compactMap { UTType(filenameExtension: $0) } + [.data])
    let picker = UIDocumentPickerViewController(forOpeningContentTypes: content, asCopy: false)
    picker.directoryURL = Documents.url
    picker.allowsMultipleSelection = false
    let delegate = PickerDelegate(cancelled: cancelled) { url in
        do {
            then(try importIntoDocuments(url))
        } catch {
            print("goAmiga: can't use \(url.lastPathComponent): \(error.localizedDescription)")
            cancelled()
        }
    }
    picker.delegate = delegate
    objc_setAssociatedObject(picker, &PickerDelegate.key, delegate, .OBJC_ASSOCIATION_RETAIN)
    topViewController()?.present(picker, animated: true)
}

/// importIntoDocuments gives a picked file's place in Documents: where it
/// is, or a copy of it there (one already copied is used again).
private func importIntoDocuments(_ url: URL) throws -> URL {
    let docs = Documents.url.standardizedFileURL.path + "/"
    if url.standardizedFileURL.path.hasPrefix(docs) {
        return url
    }
    let scoped = url.startAccessingSecurityScopedResource()
    defer { if scoped { url.stopAccessingSecurityScopedResource() } }
    let dest = Documents.url.appendingPathComponent(url.lastPathComponent)
    let fm = FileManager.default
    if fm.fileExists(atPath: dest.path) {
        let size = { (u: URL) in (try? fm.attributesOfItem(atPath: u.path)[.size] as? Int) ?? -1 }
        if size(dest) == size(url) {
            return dest
        }
        try fm.removeItem(at: dest)
    }
    try fm.copyItem(at: url, to: dest)
    return dest
}

private final class PickerDelegate: NSObject, UIDocumentPickerDelegate {
    static var key = 0
    let cancelled: () -> Void
    let picked: (URL) -> Void

    init(cancelled: @escaping () -> Void, picked: @escaping (URL) -> Void) {
        (self.cancelled, self.picked) = (cancelled, picked)
    }

    func documentPicker(_ controller: UIDocumentPickerViewController, didPickDocumentsAt urls: [URL]) {
        if let url = urls.first { picked(url) } else { cancelled() }
    }

    func documentPickerWasCancelled(_ controller: UIDocumentPickerViewController) {
        cancelled()
    }
}

@MainActor
private func topViewController() -> UIViewController? {
    let scene = UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first
    var top = scene?.keyWindow?.rootViewController
    while let presented = top?.presentedViewController {
        top = presented
    }
    return top
}

/// AppHost answers what the running machine asks of the app (GoamigaHost),
/// on the main thread.
final class AppHost: NSObject, GoamigaHostProtocol {
    let model: LauncherModel
    weak var root: RootController?

    init(model: LauncherModel) { self.model = model }

    func settingsRequested() {
        DispatchQueue.main.async { [model] in
            model.running = false
            model.say("The machine has been switched off. Change the settings and Start it again.")
        }
    }

    func pointerLock(_ locked: Bool) {
        DispatchQueue.main.async { [weak self] in self?.root?.lockPointer(locked) }
    }

    func chooseDisk(_ unit: Int) {
        DispatchQueue.main.async { [model] in
            chooseFile(title: "Insert disk in DF\(unit):", folder: false, types: diskTypes,
                       cancelled: { GoamigaDiskChosen("") }) { url in
                GoamigaDiskChosen(model.bridge.settingsPath(url))
            }
        }
    }
}

/// RootView is the Amiga's screen, covered by the configuration screen
/// whenever the machine isn't running.
struct RootView: View {
    @Environment(LauncherModel.self) private var model

    var body: some View {
        AmigaScreen()
            .ignoresSafeArea()
            .fullScreenCover(isPresented: .constant(!model.running)) {
                LauncherView()
            }
    }
}

/// AmigaScreen is AmigaViewController (Ebitengine's view and the
/// keyboard) in SwiftUI.
struct AmigaScreen: UIViewControllerRepresentable {
    func makeUIViewController(context: Context) -> AmigaViewController { AmigaViewController() }
    func updateUIViewController(_ vc: AmigaViewController, context: Context) {}
}
