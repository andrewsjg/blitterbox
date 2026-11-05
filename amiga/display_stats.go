//go:build !noebitengine

package amiga

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// The performance overlay (Cmd+I): what keeps the sound flowing, over the
// last second. A frame has 20 ms (PAL) to be emulated in; each host tick
// runs 0, 1 or 2 frames to keep the audio buffer near audioTarget, so
// ticks running 0 or 2 show the pacing correcting. "Dry" is sound the
// player asked for that the buffer didn't have, played as silence: a
// glitch.
type perfStats struct {
	on      bool
	start   time.Time
	ticks   int
	ran     [3]int // ticks that ran 0, 1 or 2 frames
	frames  int
	emu     time.Duration // emulating those frames
	emuMax  time.Duration // the slowest tick's frames, per frame
	minFill int           // the buffer's lowest level seen, in frames
	line    string
}

// note records a tick that ran n frames in d, with the audio buffer
// holding fill frames before them (-1: no sound).
func (s *perfStats) note(n int, d time.Duration, fill int, out *AudioBuffer) {
	if !s.on {
		return
	}
	now := time.Now()
	if s.start.IsZero() {
		s.start, s.minFill = now, -1
	}
	s.ticks++
	s.ran[min(n, 2)]++
	if n > 0 {
		s.frames += n
		s.emu += d
		s.emuMax = max(s.emuMax, d/time.Duration(n))
	}
	if fill >= 0 && (s.minFill < 0 || fill < s.minFill) {
		s.minFill = fill
	}
	if el := now.Sub(s.start); el >= time.Second {
		ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
		emu := 0.0
		if s.frames > 0 {
			emu = ms(s.emu) / float64(s.frames)
		}
		audio := "no sound"
		if out != nil {
			audio = fmt.Sprintf("buffer min %d ms, dry %d ms", s.minFill*1000/AudioSampleRate, out.TakeDry()*1000/AudioSampleRate)
		}
		s.line = fmt.Sprintf("emulation %.1f ms/frame, slowest %.1f\n%d frames, %d ticks: ran 0:%d 1:%d 2:%d\n%s\nTPS %.1f  FPS %.1f",
			emu, ms(s.emuMax), s.frames, s.ticks, s.ran[0], s.ran[1], s.ran[2], audio, ebiten.ActualTPS(), ebiten.ActualFPS())
		*s = perfStats{on: true, start: now, minFill: -1, line: s.line}
	}
}

// toggle switches the overlay on or off.
func (s *perfStats) toggle(out *AudioBuffer) {
	*s = perfStats{on: !s.on, line: "measuring..."}
	if out != nil {
		out.TakeDry()
	}
}

func (s *perfStats) draw(screen *ebiten.Image) {
	if s.on {
		ebitenutil.DebugPrintAt(screen, s.line, 8, 8)
	}
}
