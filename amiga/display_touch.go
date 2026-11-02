//go:build !noebitengine

package amiga

import "github.com/hajimehoshi/ebiten/v2"

// pollTouch reads the screen's touches into d.touch for pollMouse, as a
// trackpad (touchpad.go). A tap on the status bar works its buttons and
// drive lights instead of clicking the Amiga.
func (d *Display) pollTouch() {
	ids := ebiten.AppendTouchIDs(nil)
	touches := make([]touchPoint, len(ids))
	for i, id := range ids {
		x, y := ebiten.TouchPosition(id)
		touches[i] = touchPoint{id: int(id), x: x, y: y}
	}
	d.touch = d.touchpad.update(touches)
	if d.touch.tap && !d.statusClick(d.touch.tapAt.x, d.touch.tapAt.y, false) {
		d.touchpad.click()
	}
}
