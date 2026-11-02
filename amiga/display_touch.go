//go:build !noebitengine

package amiga

import (
	"math"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	touchSlop   = 10 // points a tap may move (a finger is not still)
	touchTarget = 44 // points: the status bar's least height for a finger
)

// Touch phases, for Touch.
const (
	TouchDown = iota
	TouchMoved
	TouchUp // or cancelled
)

// touchEvent is one finger's event from the app, at points in the view.
type touchEvent struct {
	id, phase int
	x, y      float64
}

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
	var touches []touchPoint
	if d.touchFed {
		touches = d.fedTouches()
	} else {
		for _, id := range ebiten.AppendTouchIDs(nil) {
			x, y := ebiten.TouchPosition(id)
			touches = append(touches, touchPoint{id: int(id), x: x, y: y})
		}
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

// Touch gives the display a finger's event (phase TouchDown, TouchMoved
// or TouchUp) at x, y points in the view, from an app that reads the
// touches itself. Once it has, the display takes all its touches from
// here: Ebitengine's own say only which fingers are down at each frame,
// and when a frame is slow (a 68040 in an RTG mode) a quick tap begins
// and ends between two and is never seen.
func (d *Display) Touch(id, phase int, x, y float64) {
	d.touchFed = true
	d.touchQueue = append(d.touchQueue, touchEvent{id, phase, x, y})
}

// fedTouches plays Touch's events into the fingers down now. A finger
// that came down since the last frame stays for this one even if it has
// gone up already; its going up, and what follows, wait for the next.
func (d *Display) fedTouches() []touchPoint {
	if d.fingers == nil {
		d.fingers = map[int]touchPoint{}
	}
	fresh := map[int]bool{}
	n := 0
	for _, e := range d.touchQueue {
		if e.phase == TouchUp && fresh[e.id] {
			break
		}
		n++
		if e.phase == TouchUp {
			delete(d.fingers, e.id)
			continue
		}
		if _, ok := d.fingers[e.id]; !ok {
			fresh[e.id] = true
		}
		x, y := d.toLogical(e.x, e.y)
		d.fingers[e.id] = touchPoint{e.id, x, y}
	}
	d.touchQueue = append(d.touchQueue[:0], d.touchQueue[n:]...)
	touches := make([]touchPoint, 0, len(d.fingers))
	for _, p := range d.fingers {
		touches = append(touches, p)
	}
	slices.SortFunc(touches, func(a, b touchPoint) int { return a.id - b.id })
	return touches
}

// toLogical turns points in the view into logical pixels: Ebitengine
// fits the logical screen in the view, centred.
func (d *Display) toLogical(x, y float64) (int, int) {
	w, h := d.amiga.PictureSize()
	h += d.statusBarHeight()
	s := d.pointScale(w, h)
	ox := (float64(d.outsideW) - float64(w)*s) / 2
	oy := (float64(d.outsideH) - float64(h)*s) / 2
	return int(math.Floor((x - ox) / s)), int(math.Floor((y - oy) / s))
}

// A pointer the app reads itself (the iPad's trackpad or mouse, through
// GCMouse) works as the desktop's mouse does: free, it clicks the status
// bar, and a click anywhere else traps it (the app hides and locks it,
// GrabChanged); trapped, it is the Amiga's mouse until Esc or F12.

// PointerClick says the free pointer was pressed (down) or released at
// x, y points in the view, with the right button or not.
func (d *Display) PointerClick(x, y float64, right, down bool) {
	if d.mouseGrabbed || !down {
		return
	}
	if lx, ly := d.toLogical(x, y); d.statusClick(lx, ly, right) {
		return
	}
	if !right {
		d.pendingGrab = true // once the buttons are up (pollMouseGrab)
	}
}

// PointerMove moves the Amiga's mouse by dx, dy points, while trapped.
func (d *Display) PointerMove(dx, dy float64) {
	if !d.mouseGrabbed {
		return
	}
	w, h := d.amiga.PictureSize()
	s := d.pointScale(w, h+d.statusBarHeight())
	d.pointer.dx += dx / s
	d.pointer.dy += dy / s
}

// PointerButton presses or releases the pointer's button (0 left,
// 1 right, 2 middle): the Amiga's, while trapped.
func (d *Display) PointerButton(button int, down bool) {
	if button >= 0 && button < len(d.pointer.buttons) {
		d.pointer.buttons[button] = down && d.mouseGrabbed
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

// Say shows a note from the app in the status bar.
func (d *Display) Say(text string) { d.message("%s", text) }
