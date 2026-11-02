package amiga

// touchpad turns touches on the screen into the Amiga mouse, the way a
// laptop trackpad does:
//
//   - one finger moving moves the pointer (relative, not where it touches);
//   - a tap clicks the left button;
//   - a tap, then touching again and moving, drags with the left button
//     held (a double tap without moving is a double click);
//   - two fingers down hold the right button, so a menu is pressing with
//     two fingers, sliding to the item and lifting; a two-finger tap is a
//     right click.
//
// It knows nothing of Ebitengine: Display feeds it each frame's touches
// in logical screen pixels (display_touch.go).
type touchpad struct {
	prev map[int]touchPoint // last frame's touches by id

	active     bool       // a gesture is under way (fingers down)
	start      int        // its first frame
	startAt    touchPoint // where its first finger landed
	moved      bool       // it went further than tapSlop
	maxFingers int        // most fingers down at once
	dragging   bool       // tap-then-touch: left held for the gesture

	slop int // how far a tap may move, logical pixels (0: tapSlop)

	frame      int
	lastTapEnd int // frame the last one-finger tap ended (0: none yet)
	leftHold   int // frames left of a tap's click
	rightHold  int // frames left of a two-finger tap's click
}

// touchPoint is one finger, at logical screen pixels.
type touchPoint struct {
	id   int
	x, y int
}

// touchpadOut is what one frame of touches does to the mouse.
type touchpadOut struct {
	dx, dy      int  // pointer motion, logical screen pixels
	left, right bool // buttons held this frame
	tap         bool // a one-finger tap ended this frame, at tapAt
	tapAt       touchPoint
}

const (
	tapFrames   = 15 // a tap is shorter than this (0.3 s at 50 Hz)
	tapSlop     = 4  // and moves no further (logical pixels), unless slop says
	dragGap     = 15 // a touch this soon after a tap drags
	clickFrames = 3  // how long a tap holds its button: the Amiga reads it once a frame
)

// update takes this frame's touches and says what the mouse does.
func (t *touchpad) update(touches []touchPoint) touchpadOut {
	t.frame++
	var out touchpadOut

	switch {
	case len(touches) > 0 && !t.active:
		t.active, t.start, t.startAt, t.moved = true, t.frame, touches[0], false
		t.maxFingers = 0
		t.dragging = t.lastTapEnd > 0 && t.frame-t.lastTapEnd <= dragGap
		t.leftHold = 0 // a click still held ends, so a double click is two
	case len(touches) == 0 && t.active:
		t.active = false
		if !t.moved && t.frame-t.start < tapFrames {
			switch {
			case t.maxFingers >= 2:
				t.rightHold = clickFrames
			case !t.dragging:
				// A double tap's second tap is its own click (dragging held
				// the button already), so it does not start another drag.
				out.tap, out.tapAt = true, t.startAt
				t.lastTapEnd = t.frame
			}
		}
		t.dragging = false
	}
	t.maxFingers = max(t.maxFingers, len(touches))

	// Motion: the average of the fingers that were down last frame too, so
	// a finger landing or lifting does not jump the pointer.
	var sx, sy, n int
	cur := make(map[int]touchPoint, len(touches))
	for _, p := range touches {
		cur[p.id] = p
		if q, ok := t.prev[p.id]; ok {
			sx += p.x - q.x
			sy += p.y - q.y
			n++
		}
	}
	t.prev = cur
	if n > 0 {
		out.dx, out.dy = sx/n, sy/n
	}
	if t.active && !t.moved {
		slop := t.slop
		if slop <= 0 {
			slop = tapSlop
		}
		if p, ok := cur[t.startAt.id]; ok && (abs(p.x-t.startAt.x) > slop || abs(p.y-t.startAt.y) > slop) {
			t.moved = true
		}
	}

	if t.leftHold > 0 {
		t.leftHold--
	}
	if t.rightHold > 0 {
		t.rightHold--
	}
	// A drag presses from its second frame: the button is up for at least
	// one frame between a tap's click and it.
	out.left = t.leftHold > 0 || (t.dragging && len(touches) == 1 && t.frame > t.start)
	out.right = t.rightHold > 0 || len(touches) >= 2
	return out
}

// click holds the left button for a tap's click, from the next frame.
func (t *touchpad) click() { t.leftHold = clickFrames + 1 }

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
