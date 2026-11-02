//go:build !noebitengine

package amiga

import "github.com/hajimehoshi/ebiten/v2"

const (
	touchSlop   = 10 // points a tap may move (a finger is not still)
	touchTarget = 44 // points: the status bar's least height for a finger
)

// pollTouch reads the screen's touches into d.touch for pollMouse, as a
// trackpad (touchpad.go). A tap on the status bar works its buttons and
// drive lights instead of clicking the Amiga.
//
// Touches measure in points, and a logical pixel can be far smaller than
// one (hires and RTG modes), so the tap slop and the bar's target are
// sized in points: the bar reaches up into the picture for a finger when
// it is too thin. (The picture's own taps click wherever the Amiga's
// pointer is, so none of it is lost.)
func (d *Display) pollTouch() {
	ids := ebiten.AppendTouchIDs(nil)
	touches := make([]touchPoint, len(ids))
	for i, id := range ids {
		x, y := ebiten.TouchPosition(id)
		touches[i] = touchPoint{id: int(id), x: x, y: y}
	}
	w, h := d.amiga.PictureSize()
	pts := d.pointScale(w, h+d.statusBarHeight())
	d.touchpad.slop = max(1, int(touchSlop/pts))
	d.touch = d.touchpad.update(touches)
	if !d.touch.tap {
		return
	}
	x, y := d.touch.tapAt.x, d.touch.tapAt.y
	if reach := int(touchTarget/pts) - statusHeight; d.StatusBar && y < h && y >= h-reach {
		y = h
	}
	if d.statusClick(x, y, false) {
		d.touchpad.lastTapEnd = 0 // the next touch is not a drag in the Amiga
	} else {
		d.touchpad.click()
	}
}

// PointerMove moves the Amiga's mouse by dx, dy points, for a mouse or
// trackpad the app reads itself (the iPad's, through GCMouse).
func (d *Display) PointerMove(dx, dy float64) {
	w, h := d.amiga.PictureSize()
	s := d.pointScale(w, h+d.statusBarHeight())
	d.pointer.dx += dx / s
	d.pointer.dy += dy / s
}

// PointerButton presses or releases that pointer's button: 0 left,
// 1 right, 2 middle.
func (d *Display) PointerButton(button int, down bool) {
	if button >= 0 && button < len(d.pointer.buttons) {
		d.pointer.buttons[button] = down
	}
}

// pollPointer gives the Amiga's mouse what PointerMove and PointerButton
// did since the last frame, keeping fractions of a pixel for the next.
func (d *Display) pollPointer() {
	m, p := d.amiga.Mouse, &d.pointer
	m.LeftButton = m.LeftButton || p.buttons[0]
	m.RightButton = m.RightButton || p.buttons[1]
	m.MiddleButton = p.buttons[2]
	dx, dy := int(p.dx), int(p.dy)
	p.dx -= float64(dx)
	p.dy -= float64(dy)
	d.moveMouse(dx, dy)
}
