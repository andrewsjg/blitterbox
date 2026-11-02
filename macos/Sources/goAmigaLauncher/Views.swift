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
            List(model.saved, id: \.self, selection: $model.selection) { name in
                Text(name)
                    #if !os(macOS)
                    // A tap starts a configuration rather than selecting
                    // it, and the List can clear the selection after: the
                    // one loaded is marked instead.
                    .fontWeight(name == model.name ? .semibold : .regular)
                    .listRowBackground(name == model.name
                        ? Capsule().fill(Color.accentColor.opacity(0.2)).padding(.horizontal, 10).padding(.vertical, 2)
                        : nil)
                    #endif
            }
            .contextMenu(forSelectionType: String.self) { names in
                if let n = names.first {
                    Button("Start", systemImage: "play.fill") {
                        model.load(n)
                        model.start()
                    }
                    #if !os(macOS)
                    // A tap starts it on iOS (primaryAction), rather than
                    // selecting it as a click does on the Mac.
                    Button("Edit", systemImage: "pencil") { model.load(n) }
                    #endif
                    Button("Delete…", systemImage: "trash", role: .destructive) {
                        model.selection = n
                        deleting = n
                        confirmDelete = true
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
            .safeAreaInset(edge: .bottom) {
                VStack(alignment: .leading, spacing: 8) {
                    TextField("Name", text: $model.name)
                        .onSubmit { model.save() }
                    HStack {
                        Button("Save", systemImage: "square.and.arrow.down") { model.save() }
                        Button("New", systemImage: "plus") { model.newConfiguration() }
                        Spacer()
                        Button("Delete…", systemImage: "trash") {
                            deleting = model.current
                            confirmDelete = true
                        }
                        .disabled(model.current == nil)
                        #if !os(macOS)
                        .labelStyle(.iconOnly) // no room for its title
                        #endif
                    }
                    .lineLimit(1)
                    #if !os(macOS)
                    .controlSize(.small) // the sidebar is narrow
                    #endif
                    #if os(macOS)
                    Text("Double-click a configuration to start it.")
                        .font(.caption).foregroundStyle(.secondary)
                    #else
                    Text("Tap a configuration to start it; touch and hold to edit it.")
                        .font(.caption).foregroundStyle(.secondary)
                    #endif
                }
                .padding(10)
            }
            .navigationSplitViewColumnWidth(min: 190, ideal: 220)
            .navigationTitle("Configurations")
        } detail: {
            VStack(spacing: 0) {
                TabView(selection: $model.tab) {
                    MachineTab().tabItem { Text("Machine") }.tag(LauncherModel.Tab.machine)
                    MemoryTab().tabItem { Text("Memory") }.tag(LauncherModel.Tab.memory)
                    FloppyTab().tabItem { Text("Floppies") }.tag(LauncherModel.Tab.floppies)
                    HardDiskTab().tabItem { Text("Hard disks") }.tag(LauncherModel.Tab.hardDisks)
                    WHDLoadTab().tabItem { Text("WHDLoad") }.tag(LauncherModel.Tab.whdload)
                    PortsTab().tabItem { Text("Ports") }.tag(LauncherModel.Tab.ports)
                    DisplayTab().tabItem { Text("Display") }.tag(LauncherModel.Tab.display)
                }
                .padding([.horizontal, .top])
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
            .navigationTitle(model.name.isEmpty ? "goAmiga" : "goAmiga — \(model.name)")
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

// MARK: - Controls

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
                Text(path.isEmpty ? empty : path)
                    .foregroundStyle(path.isEmpty ? .secondary : .primary)
                    .lineLimit(1).truncationMode(.middle)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .help(path)
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
