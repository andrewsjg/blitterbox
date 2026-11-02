// Package icons holds BlitterBox's icons: iPadOS/ for the iPad app (its
// asset catalog has a copy), macOS/ for the Mac app (BlitterBox.icns, which
// scripts/macos-app.sh puts in the bundle), and Masters/ they were made
// from. For Go, it has the macOS ones, for window and Dock icons.
package icons

import (
	"bytes"
	"embed"
	"image"
	"image/png"
)

//go:embed macOS/AppIcon.appiconset/*.png
var mac embed.FS

// Dock is the 1024-pixel macOS icon, as PNG.
func Dock() []byte {
	b, _ := mac.ReadFile("macOS/AppIcon.appiconset/icon_512x512@2x.png")
	return b
}

// Window is the icon at the sizes window icons are drawn.
func Window() []image.Image {
	var imgs []image.Image
	for _, n := range []string{"16x16", "32x32", "128x128", "256x256"} {
		b, err := mac.ReadFile("macOS/AppIcon.appiconset/icon_" + n + ".png")
		if err != nil {
			continue
		}
		if img, err := png.Decode(bytes.NewReader(b)); err == nil {
			imgs = append(imgs, img)
		}
	}
	return imgs
}
