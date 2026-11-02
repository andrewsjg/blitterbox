//go:build !noebitengine

package amiga

import "testing"

// TestFedTouchQuickTap: a tap that begins and ends between two frames
// (a slow frame) is still a finger for one frame, then gone: a tap.
func TestFedTouchQuickTap(t *testing.T) {
	a := &Amiga{ChipRAM: newChipRAM(ChipRAMSize)}
	a.Custom = NewCustomChips(nil, a)
	a.Video = NewVideo(a)
	d := &Display{amiga: a}
	w, h := a.PictureSize()
	d.outsideW, d.outsideH = w*2, h*2+100 // 2 points a pixel, 50 above and below

	d.Touch(1, TouchDown, 100, 150)
	d.Touch(1, TouchUp, 100, 150)
	if got := d.fedTouches(); len(got) != 1 || got[0] != (touchPoint{1, 50, 50}) {
		t.Fatalf("first frame: %v, want finger 1 at 50,50", got)
	}
	if got := d.fedTouches(); len(got) != 0 {
		t.Fatalf("second frame: %v, want none", got)
	}
}
