package amiga

import "testing"

// touchRun feeds a touchpad one frame per entry and returns its outputs.
// After each tap it clicks, as pollTouch does off the status bar.
func touchRun(t *touchpad, frames ...[]touchPoint) []touchpadOut {
	var outs []touchpadOut
	for _, f := range frames {
		o := t.update(f)
		if o.tap {
			t.click()
		}
		outs = append(outs, o)
	}
	return outs
}

func finger(id, x, y int) []touchPoint { return []touchPoint{{id, x, y}} }

func hold(n int, f []touchPoint) [][]touchPoint {
	out := make([][]touchPoint, n)
	for i := range out {
		out[i] = f
	}
	return out
}

func held(outs []touchpadOut, right bool) int {
	n := 0
	for _, o := range outs {
		if (!right && o.left) || (right && o.right) {
			n++
		}
	}
	return n
}

func TestTouchpadDragMovesWithoutClicking(t *testing.T) {
	var tp touchpad
	outs := touchRun(&tp, finger(1, 100, 100), finger(1, 110, 95), finger(1, 130, 90), nil, nil, nil, nil, nil)
	dx, dy := 0, 0
	for _, o := range outs {
		dx += o.dx
		dy += o.dy
		if o.tap {
			t.Fatal("a drag tapped")
		}
	}
	if dx != 30 || dy != -10 {
		t.Errorf("moved %d,%d, want 30,-10", dx, dy)
	}
	if n := held(outs, false); n != 0 {
		t.Errorf("left held %d frames, want 0", n)
	}
}

func TestTouchpadTapClicksLeft(t *testing.T) {
	var tp touchpad
	outs := touchRun(&tp, append(hold(3, finger(1, 50, 60)), hold(8, nil)...)...)
	if !outs[3].tap || outs[3].tapAt != (touchPoint{1, 50, 60}) {
		t.Fatalf("no tap at 50,60 on lifting: %+v", outs[3])
	}
	if n := held(outs, false); n != clickFrames {
		t.Errorf("left held %d frames, want %d", n, clickFrames)
	}
	if held(outs, true) != 0 {
		t.Error("a tap pressed the right button")
	}
}

func TestTouchpadLongPressIsNoTap(t *testing.T) {
	var tp touchpad
	outs := touchRun(&tp, append(hold(tapFrames+2, finger(1, 50, 60)), nil)...)
	if held(outs, false) != 0 || outs[len(outs)-1].tap {
		t.Error("a long press clicked")
	}
}

func TestTouchpadTapThenDragHoldsLeft(t *testing.T) {
	var tp touchpad
	frames := append(hold(2, finger(1, 50, 60)), hold(5, nil)...) // tap, click done
	frames = append(frames, finger(2, 50, 60), finger(2, 70, 60), finger(2, 90, 60), nil)
	outs := touchRun(&tp, frames...)
	if outs[7].left {
		t.Error("left held on the drag's first frame: no release after the tap's click")
	}
	drag := outs[8:10]
	for i, o := range drag {
		if !o.left {
			t.Errorf("drag frame %d: left not held", i)
		}
	}
	if outs[10].left || outs[10].tap {
		t.Errorf("after the drag: %+v, want left up and no tap", outs[10])
	}
}

func TestTouchpadDoubleTapClicksTwice(t *testing.T) {
	var tp touchpad
	frames := append(hold(2, finger(1, 50, 60)), hold(4, nil)...)
	frames = append(frames, append(hold(2, finger(2, 50, 60)), hold(6, nil)...)...)
	outs := touchRun(&tp, frames...)
	presses := 0
	for i := 1; i < len(outs); i++ {
		if outs[i].left && !outs[i-1].left {
			presses++
		}
	}
	if presses != 2 {
		t.Errorf("%d presses of the left button, want 2", presses)
	}
}

func TestTouchpadTwoFingersHoldRight(t *testing.T) {
	var tp touchpad
	two := func(x int) []touchPoint { return []touchPoint{{1, x, 50}, {2, x + 40, 50}} }
	outs := touchRun(&tp, finger(1, 10, 50), two(10), two(30), two(50), finger(2, 90, 50), nil, nil, nil, nil)
	for i := 1; i <= 3; i++ {
		if !outs[i].right {
			t.Errorf("frame %d: two fingers down, right not held", i)
		}
	}
	if outs[2].dx != 20 || outs[3].dx != 20 {
		t.Errorf("two fingers moved %d, %d, want 20 each", outs[2].dx, outs[3].dx)
	}
	if outs[4].dx != 0 {
		t.Errorf("a finger lifting moved the pointer %d", outs[4].dx)
	}
	if held(outs, false) != 0 {
		t.Error("a two-finger drag pressed the left button")
	}
}

func TestTouchpadTwoFingerTapClicksRight(t *testing.T) {
	var tp touchpad
	two := []touchPoint{{1, 10, 50}, {2, 50, 50}}
	outs := touchRun(&tp, two, two, nil, nil, nil, nil, nil)
	if n := held(outs, true); n < clickFrames {
		t.Errorf("right held %d frames, want at least %d", n, clickFrames)
	}
	if held(outs, false) != 0 || outs[2].tap {
		t.Error("a two-finger tap clicked left")
	}
}

// TestTouchpadSlop: a finger wobbling within slop still taps (hires
// modes, where a point is several logical pixels), and beyond it moves.
func TestTouchpadSlop(t *testing.T) {
	wobble := []([]touchPoint){finger(1, 50, 60), finger(1, 57, 55), finger(1, 53, 62), nil}
	tp := touchpad{slop: 8}
	if outs := touchRun(&tp, wobble...); !outs[3].tap {
		t.Error("a 7-pixel wobble within slop 8 did not tap")
	}
	tp = touchpad{}
	if outs := touchRun(&tp, wobble...); outs[3].tap {
		t.Error("a 7-pixel wobble tapped with the default slop")
	}
}
