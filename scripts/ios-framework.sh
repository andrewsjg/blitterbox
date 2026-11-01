#!/bin/sh
# Builds ios/Goamiga.xcframework (device and simulator) from ./mobile for
# the iPad app in ios/goAmiga. Needs Xcode and ebitenmobile:
#   go install github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@v2.10.4
set -e
cd "$(dirname "$0")/.."
rm -rf ios/Goamiga.xcframework
"$(go env GOPATH)/bin/ebitenmobile" bind -target ios -o ios/Goamiga.xcframework ./mobile
