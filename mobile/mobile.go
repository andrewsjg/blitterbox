// Package goamiga is goAmiga for iPadOS: ebitenmobile binds it into an
// xcframework (scripts/ios-framework.sh) that the app in ios/ hosts.
//
// This is a spike. It boots one fixed machine, an A500 with 1 MB of chip
// RAM and 2 MB of fast RAM, from a Kickstart ROM and an optional ADF that
// the app names. The full settings live in cmd/goamiga's buildMachine,
// which a real port would move out of package main to share.
package goamiga

import (
	"fmt"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/mobile"

	"github.com/andrewsjg/goamiga/amiga"
)

// game shows nothing until Boot gives it a machine, then the machine's
// display. Ebitengine calls Update, Draw and Layout on its own thread;
// Boot is called from the app's main thread, hence the lock.
type game struct {
	mu   sync.Mutex
	a    *amiga.Amiga
	disp *amiga.Display
}

var g = &game{}

func init() {
	mobile.SetGame(g)
}

// Boot builds the A500, inserts adfPath (if not empty) in DF0: and starts
// it. A second call replaces the running machine.
func Boot(romPath, adfPath string) error {
	rom, err := amiga.LoadROM(romPath)
	if err != nil {
		return err
	}
	a, err := amiga.New(rom, nil)
	if err != nil {
		return fmt.Errorf("creating the Amiga: %w", err)
	}
	for _, step := range []func() error{
		func() error { return a.SetChipRAM(1024) },
		func() error { return a.SetCPU("68000", 0) },
		func() error { return a.SetFPU("auto") },
		func() error { return a.SetChipset("ecs-agnus") },
		func() error { return a.SetVideoStandard("pal") },
		func() error { return a.ConfigureMemory(0, 2048) },
		func() error {
			if adfPath == "" {
				return nil
			}
			return a.LoadDiskUnit(0, adfPath)
		},
		a.Reset,
	} {
		if err := step(); err != nil {
			a.Shutdown()
			return err
		}
	}

	d := amiga.NewDisplay(a)
	d.Scale = 1
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.Stop()
		g.a.Shutdown()
	}
	g.a, g.disp = a, d
	d.Start()
	return nil
}

// Key presses (down) or releases an Amiga key by its raw code, for the
// app's on-screen keyboard. A hardware keyboard needs no help: Ebitengine
// reports its keys to the display like a desktop one.
func Key(code int, down bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.a != nil && g.a.Keyboard != nil {
		g.a.Keyboard.EnqueueKey(byte(code), down)
	}
}

func (g *game) Update() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp == nil {
		return nil
	}
	return g.disp.Update()
}

func (g *game) Draw(screen *ebiten.Image) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.Draw(screen)
	}
}

// DrawFinalScreen scales the picture the way the desktop window does.
func (g *game) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.DrawFinalScreen(screen, offscreen, geoM)
		return
	}
	screen.DrawImage(offscreen, &ebiten.DrawImageOptions{GeoM: geoM})
}

func (g *game) Layout(w, h int) (int, int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp == nil {
		return w, h
	}
	return g.disp.Layout(w, h)
}
