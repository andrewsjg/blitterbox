#!/bin/sh
# Builds BlitterBox.app: the blitterbox binary in a macOS application bundle with
# the Boing Ball icon, so it shows in Finder and the Dock and can be put in
# /Applications. Run from the repository root; the app lands in the root.
#
# With Swift (Xcode or its command-line tools) the app opens on the native
# SwiftUI configuration screen in macos/, which starts blitterbox beside it;
# without, or with BLITTERBOX_NO_SWIFTUI=1, blitterbox itself is the app and
# shows its own.
#
#   scripts/macos-app.sh
set -e
APP=BlitterBox.app
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

go build -o "$WORK/blitterbox" ./cmd/blitterbox
EXE=blitterbox
MINOS=11.0
if [ -z "$BLITTERBOX_NO_SWIFTUI" ] && command -v swift >/dev/null 2>&1; then
	swift build -c release --package-path macos
	cp "$(swift build -c release --package-path macos --show-bin-path)/BlitterBoxLauncher" "$WORK/"
	EXE=BlitterBoxLauncher
	MINOS=14.0
fi
go run ./internal/boing/mkiconset "$WORK/blitterbox.iconset"
iconutil -c icns -o "$WORK/blitterbox.icns" "$WORK/blitterbox.iconset"

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$WORK/blitterbox" "$APP/Contents/MacOS/blitterbox"
if [ "$EXE" = BlitterBoxLauncher ]; then
	cp "$WORK/BlitterBoxLauncher" "$APP/Contents/MacOS/BlitterBoxLauncher"
fi
cp "$WORK/blitterbox.icns" "$APP/Contents/Resources/blitterbox.icns"
VERSION=$(git describe --tags --always 2>/dev/null || echo dev)
cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key><string>BlitterBox</string>
	<key>CFBundleDisplayName</key><string>BlitterBox</string>
	<key>CFBundleIdentifier</key><string>com.github.andrewsjg.blitterbox</string>
	<key>CFBundleExecutable</key><string>$EXE</string>
	<key>CFBundleIconFile</key><string>blitterbox</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleShortVersionString</key><string>$VERSION</string>
	<key>CFBundleVersion</key><string>$VERSION</string>
	<key>NSHighResolutionCapable</key><true/>
	<key>LSMinimumSystemVersion</key><string>$MINOS</string>
</dict>
</plist>
PLIST
echo "built $APP"
