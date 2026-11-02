#if os(macOS)
import AppKit
#endif
import SwiftUI
import UniformTypeIdentifiers

// The configuration screen: the saved configurations in the sidebar, the
// settings in tabs, the status line with Start below.
struct LauncherView: View {
    @Environment(LauncherModel.self) private var model
    @State private var confirmDelete = false
    @State private var deleting: String? // the configuration confirmDelete asks about

    var body: some View {
        @Bindable var model = model
        NavigationSplitView {
            // The footer is below the list rather than inset into it,
            // where the list's styling took the buttons' bezels.
            VStack(spacing: 0) {
                configurations
                    .overlay {
                        if model.saved.isEmpty {
                            ContentUnavailableView("No Configurations", systemImage: "tray",
                                                   description: Text("Set up a machine, name it below and Save it."))
                        }
                    }
                configurationFooter
            }
            #if os(macOS)
            .navigationSplitViewColumnWidth(min: 190, ideal: 220)
            #else
            .navigationSplitViewColumnWidth(min: 240, ideal: 270)
            #endif
            .navigationTitle("Configurations")
        } detail: {
            VStack(spacing: 0) {
                #if os(macOS)
                TabView(selection: $model.tab) {
                    ForEach(LauncherModel.Tab.allCases) { t in
                        tabContent(t).tabItem { Text(t.rawValue) }.tag(t)
                    }
                }
                .padding([.horizontal, .top])
                #else
                // A segmented control rather than the iPad's tab bar, which
                // hid the last three tabs behind a ›.
                Picker("Settings", selection: $model.tab) {
                    ForEach(LauncherModel.Tab.allCases) { t in
                        Text(t.rawValue).tag(t)
                    }
                }
                .pickerStyle(.segmented)
                .padding(.horizontal, 20).padding(.top, 4).padding(.bottom, 8)
                tabContent(model.tab)
                #endif
                Divider().padding(.top, 10)
                HStack(alignment: .center) {
                    // The status is a quiet note, unless it is an error.
                    Group {
                        if model.statusIsError {
                            Label(model.status, systemImage: "exclamationmark.triangle.fill")
                                .foregroundStyle(.red)
                        } else {
                            Text(model.status).foregroundStyle(.secondary)
                        }
                    }
                    .font(.callout)
                    .lineLimit(3)
                    .textSelection(.enabled)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    #if os(macOS)
                    Button("Quit", systemImage: "power") { NSApp.terminate(nil) }
                        .controlSize(.large)
                    #endif
                    Button { model.start() } label: {
                        Label("Start", systemImage: "play.fill").padding(.horizontal, 8)
                    }
                    .buttonStyle(.borderedProminent)
                    .keyboardShortcut(.defaultAction)
                    .controlSize(.large)
                    .disabled(model.running)
                }
                .padding()
            }
            .navigationTitle(model.name.isEmpty ? "BlitterBox" : "BlitterBox — \(model.name)")
        }
        #if os(macOS)
        .task { await saveTabShots(model) }
        #endif
        .confirmationDialog("Delete the configuration \(deleting ?? "")?", isPresented: $confirmDelete) {
            Button("Delete", systemImage: "trash", role: .destructive) {
                if let n = deleting { model.delete(n) }
            }
        }
    }
}

extension LauncherView {
    @ViewBuilder
    private func tabContent(_ t: LauncherModel.Tab) -> some View {
        switch t {
        case .machine: MachineTab()
        case .memory: MemoryTab()
        case .floppies: FloppyTab()
        case .hardDisks: HardDiskTab()
        case .whdload: WHDLoadTab()
        case .ports: PortsTab()
        case .display: DisplayTab()
        }
    }

    #if os(macOS)
    // configurations is the saved configurations: a click loads one, a
    // double-click starts it.
    private var configurations: some View {
        @Bindable var model = model
        return List(model.saved, id: \.self, selection: $model.selection) { name in
            ConfigRow(name: name)
        }
        .contextMenu(forSelectionType: String.self) { names in
            if let n = names.first {
                Button("Start", systemImage: "play.fill") {
                    model.load(n)
                    model.start()
                }
                Button("Delete…", systemImage: "trash", role: .destructive) {
                    model.selection = n
                    confirmDeleting(n)
                }
            }
        } primaryAction: { names in
            if let n = names.first { // double-click: load and start
                model.load(n)
                model.start()
            }
        }
        .onChange(of: model.selection) { _, n in
            if let n { model.load(n) }
        }
    }
    #else
    // configurations is the saved configurations: a tap loads one, to be
    // started with Start or its ▶. The one loaded is marked with a capsule
    // (the List's own selection is a squared-off highlight, and comes and
    // goes).
    private var configurations: some View {
        List(model.saved, id: \.self) { name in
            let loaded = name == model.name
            HStack(spacing: 4) {
                Button { model.load(name) } label: {
                    ConfigRow(name: name)
                        .padding(.leading, 8) // clear of the capsule's curve
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .contentShape(Rectangle())
                }
                .accessibilityAddTraits(loaded ? .isSelected : [])
                Button {
                    model.load(name)
                    model.start()
                } label: {
                    Image(systemName: "play.circle.fill")
                        .font(.title2)
                        .symbolRenderingMode(.palette)
                        .foregroundStyle(.white, Color.accentColor)
                        .frame(width: 44, height: 44)
                        .contentShape(Rectangle())
                }
                .accessibilityLabel("Start \(name)")
            }
            .buttonStyle(.plain) // each its own target
            .listRowBackground(loaded
                ? Capsule().fill(Color.accentColor.opacity(0.2)).padding(.horizontal, 10).padding(.vertical, 2)
                : nil)
            .swipeActions {
                Button("Delete", systemImage: "trash", role: .destructive) { confirmDeleting(name) }
            }
            .contextMenu {
                Button("Start", systemImage: "play.fill") {
                    model.load(name)
                    model.start()
                }
                Button("Delete…", systemImage: "trash", role: .destructive) { confirmDeleting(name) }
            }
        }
    }
    #endif

    // configurationFooter is below the list: the configuration's name,
    // Save, New and Delete.
    private var configurationFooter: some View {
        @Bindable var model = model
        return VStack(spacing: 0) {
            Divider()
            VStack(alignment: .leading, spacing: 10) {
                HStack {
                    Text("Name").font(.caption.weight(.semibold)).foregroundStyle(.secondary)
                    Spacer()
                    if model.unsaved && !model.name.isEmpty {
                        Text("Unsaved changes").font(.caption).foregroundStyle(.orange)
                    }
                }
                TextField("Name", text: $model.name, prompt: Text("Untitled"))
                    .textFieldStyle(.roundedBorder)
                    .onSubmit { if canSave { model.save() } }
                    #if !os(macOS)
                    .submitLabel(.done)
                    #endif
                HStack(spacing: 8) {
                    Button { model.save() } label: {
                        Label("Save", systemImage: "square.and.arrow.down").frame(maxWidth: .infinity)
                    }
                    .buttonStyle(FooterButton(prominent: true))
                    .disabled(!canSave)
                    Button { model.newConfiguration() } label: {
                        Label("New", systemImage: "plus").frame(maxWidth: .infinity)
                    }
                    .buttonStyle(FooterButton())
                    Button(role: .destructive) {
                        if let n = model.current { confirmDeleting(n) }
                    } label: {
                        Label("Delete…", systemImage: "trash").labelStyle(.iconOnly) // no room for its title
                    }
                    .buttonStyle(FooterButton())
                    .help("Delete the configuration")
                    .accessibilityLabel("Delete the configuration")
                    .disabled(model.current == nil)
                }
                .lineLimit(1)
                .fixedSize(horizontal: false, vertical: true)
                #if os(macOS)
                Text("Double-click a configuration to start it.")
                    .font(.caption).foregroundStyle(.secondary)
                #else
                Text("Tap a configuration to load it; ▶ starts it.")
                    .font(.caption).foregroundStyle(.secondary)
                #endif
            }
            .padding(12)
        }
        .background(.bar)
    }

    /// canSave: there is a name, and something to save under it.
    private var canSave: Bool { !model.name.isEmpty && model.unsaved }

    private func confirmDeleting(_ n: String) {
        deleting = n
        confirmDelete = true
    }
}

// ConfigRow is a saved configuration in the list: an icon for its kind
// of Amiga, its name (with a dot while the one loaded has unsaved
// changes), and its model and Kickstart.
struct ConfigRow: View {
    @Environment(LauncherModel.self) private var model
    let name: String

    var body: some View {
        let summary = model.summaries[name]
        let loaded = name == model.name
        HStack(spacing: 10) {
            Image(systemName: icon(summary?.model))
                .font(.body)
                .foregroundStyle(loaded ? Color.accentColor : .secondary)
                .frame(width: 24)
            VStack(alignment: .leading, spacing: 2) {
                HStack(spacing: 6) {
                    Text(name)
                        .fontWeight(loaded ? .semibold : .regular)
                        .lineLimit(1)
                    if loaded && model.unsaved {
                        Circle().fill(.orange).frame(width: 7, height: 7)
                            .accessibilityLabel("Unsaved changes")
                    }
                }
                if let line = summary?.line, !line.isEmpty {
                    Text(line).font(.caption).foregroundStyle(.secondary).lineLimit(2)
                }
            }
        }
        .padding(.vertical, 2)
    }

    // icon: the big-box Amigas are desktops; the rest are a keyboard.
    private func icon(_ model: String?) -> String {
        switch model {
        case "A3000", "A4000": "desktopcomputer"
        case nil, "": "cpu"
        default: "keyboard"
        }
    }
}

// displayName is how a file in the settings is shown: its name, and for
// a disk inside a zip ("games.zip#Disk 1.adf") that disk's, with the zip's.
func displayName(_ path: String) -> String {
    let parts = path.split(separator: "#", maxSplits: 1).map(String.init)
    let file = (parts.first.map { ($0 as NSString).lastPathComponent }) ?? path
    if parts.count == 2 {
        return "\((parts[1] as NSString).lastPathComponent) (in \(file))"
    }
    return file
}

// MARK: - Controls

// FooterButton is the sidebar footer's buttons: a capsule, filled with
// the accent colour when prominent, else tinted. (The iPad's sidebar
// drew .bordered buttons with no bezel at all.)
struct FooterButton: ButtonStyle {
    var prominent = false
    @Environment(\.isEnabled) private var enabled

    func makeBody(configuration: Configuration) -> some View {
        let tint: Color = configuration.role == .destructive ? .red : .accentColor
        configuration.label
            .font(.subheadline.weight(.medium))
            .foregroundStyle(prominent ? .white : tint)
            .padding(.horizontal, 12)
            .padding(.vertical, 7)
            .background(prominent ? tint : tint.opacity(0.12), in: Capsule())
            .opacity(enabled ? (configuration.isPressed ? 0.7 : 1) : 0.4)
            .contentShape(Capsule())
    }
}

extension View {
    // settingsForm is how each tab's Form looks. On iOS a row's buttons
    // need a style of their own to be tapped one at a time (plain ones
    // all fire together: Choose… with Clear, every model at once), and
    // their icons one too (InlineLabel).
    func settingsForm() -> some View {
        #if os(macOS)
        formStyle(.grouped)
        #else
        formStyle(.grouped).buttonStyle(.bordered).labelStyle(InlineLabel())
        #endif
    }
}

#if !os(macOS)
// InlineLabel is a button's icon and title side by side. An iOS Form lays
// out a row holding a standard Label as a list item with an icon: the row
// grew tall, and a disabled button's title stayed dark.
struct InlineLabel: LabelStyle {
    func makeBody(configuration: Configuration) -> some View {
        HStack(spacing: 6) {
            configuration.icon
            configuration.title
        }
    }
}
#endif

// Choice is a pop-up menu of values with their labels. A value from a
// settings file that isn't on the menu is shown as it is.
struct Choice<T: Hashable>: View {
    let title: String
    @Binding var value: T
    let options: [(String, T)]
    var other: (T) -> String = { "\($0)" }

    var body: some View {
        Picker(title, selection: $value) {
            ForEach(options, id: \.1) { label, v in
                Text(label).tag(v)
            }
            if !options.contains(where: { $0.1 == value }) {
                Text(other(value)).tag(value)
            }
        }
    }
}

// FileRow shows a file or folder with Choose… and Clear, and takes one
// dropped from the Finder.
struct FileRow: View {
    @Environment(LauncherModel.self) private var model
    let title: String
    let target: String
    let path: String
    var folder = false
    var types: [String] = []
    var empty = "None"
    let clear: () -> Void

    var body: some View {
        LabeledContent(title) {
            HStack {
                // Its name; the whole path on hovering (Mac) or touching
                // and holding (iPad).
                Text(path.isEmpty ? empty : displayName(path))
                    .foregroundStyle(path.isEmpty ? .secondary : .primary)
                    .lineLimit(1).truncationMode(.middle)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .help(path)
                    .accessibilityValue(path)
                    #if !os(macOS)
                    .contextMenu {
                        if !path.isEmpty {
                            Text(path)
                            Button("Copy Path", systemImage: "doc.on.doc") { UIPasteboard.general.string = path }
                        }
                    }
                    #endif
                Button("Choose…", systemImage: "folder") {
                    chooseFile(title: title, folder: folder, types: types, start: path) { url in
                        model.choose(target, url)
                    }
                }
                Button("Clear", systemImage: "xmark.circle", action: clear).disabled(path.isEmpty)
            }
        }
        .dropDestination(for: URL.self) { urls, _ in
            guard let url = urls.first else { return false }
            model.choose(target, url)
            return true
        }
    }
}

let romTypes = ["rom", "bin"]
let diskTypes = ["adf", "ipf", "zip", "adz", "gz"]
let hdfTypes = ["hdf", "hdz"]
let whdloadTypes = ["lha", "lzh", "slave"]

#if os(macOS)
// chooseFile asks for a file (of these extensions) or a folder, and gives
// it to then, or calls cancelled. (The iPad app's shows the document
// picker.)
@MainActor
func chooseFile(title: String, folder: Bool, types: [String] = [], start: String = "",
                cancelled: () -> Void = {}, then: (URL) -> Void) {
    if let url = openPanel(title: title, folder: folder, types: types, start: start) {
        then(url)
    } else {
        cancelled()
    }
}

// openPanel asks for a file (of these extensions) or a folder.
@MainActor
func openPanel(title: String, folder: Bool, types: [String] = [], start: String = "") -> URL? {
    let p = NSOpenPanel()
    p.title = title
    p.message = title
    p.canChooseFiles = !folder
    p.canChooseDirectories = folder
    p.allowsMultipleSelection = false
    if !types.isEmpty {
        p.allowedContentTypes = types.compactMap { UTType(filenameExtension: $0) }
        p.allowsOtherFileTypes = true
    }
    if !start.isEmpty {
        let url = URL(fileURLWithPath: start)
        p.directoryURL = folder ? url : url.deletingLastPathComponent()
    }
    return p.runModal() == .OK ? p.url : nil
}
#endif

struct Note: View {
    let text: String
    init(_ text: String) { self.text = text }
    var body: some View {
        Text(text).font(.callout).foregroundStyle(.secondary)
            .multilineTextAlignment(.leading)
            .frame(maxWidth: .infinity, alignment: .leading)
            .fixedSize(horizontal: false, vertical: true)
    }
}

struct Warning: View {
    let text: String
    init(_ text: String) { self.text = text }
    var body: some View {
        Label(text, systemImage: "exclamationmark.triangle.fill")
            .foregroundStyle(.red).font(.callout)
            .fixedSize(horizontal: false, vertical: true)
    }
}
