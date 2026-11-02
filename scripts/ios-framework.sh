#!/bin/sh
# Builds ios/Blitterbox.xcframework (device and simulator) from ./mobile for
# the iPad app in ios/BlitterBox. Needs Xcode and ebitenmobile:
#   go install github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@v2.10.4
set -e
cd "$(dirname "$0")/.."
rm -rf ios/Blitterbox.xcframework
"$(go env GOPATH)/bin/ebitenmobile" bind -target ios -o ios/Blitterbox.xcframework ./mobile
