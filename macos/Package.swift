// swift-tools-version:5.10
// BlitterBox's native configuration screen for macOS. scripts/macos-app.sh
// builds it as BlitterBox.app's executable, with the blitterbox binary beside
// it; it asks blitterbox --launcher-api for everything about settings and
// starts the machine as a blitterbox run.
import PackageDescription

let package = Package(
    name: "BlitterBoxLauncher",
    platforms: [.macOS(.v14)],
    targets: [
        .executableTarget(name: "BlitterBoxLauncher", path: "Sources/BlitterBoxLauncher")
    ]
)
