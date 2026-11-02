//go:build !noebitengine

package main

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/andrewsjg/blitterbox/icons"
)

// setIcon gives the window BlitterBox's icon (Windows and Linux use it).
func setIcon() {
	ebiten.SetWindowIcon(icons.Window())
}

// setDockIconOnce puts the icon in the macOS Dock, which ignores window
// icons. It is called from the first Update, once the game loop (and so
// the application) is running: touching AppKit before Ebitengine has set
// it up can leave the window blank.
func (g *app) setDockIconOnce() {
	if g.dockIconSet {
		return
	}
	g.dockIconSet = true
	setDockIcon(icons.Dock())
}
