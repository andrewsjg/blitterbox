package amiga

import (
	"testing"
	"time"
)

// TestRunFrameNoHangWhenStoppedAtVBlank ensures STOP idle at vpos==0 cannot
// wedge RunFrame (the leave-line-0 infinite loop).
func TestRunFrameNoHangWhenStoppedAtVBlank(t *testing.T) {
	rom := make([]byte, ROMSize)
	// Minimal reset vectors
	rom[0] = 0x00
	rom[1] = 0x04
	rom[2] = 0x00
	rom[3] = 0x00 // SSP
	rom[4] = 0x00
	rom[5] = 0xFC
	rom[6] = 0x00
	rom[7] = 0x08 // PC in ROM

	a, err := New(rom, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Reset(); err != nil {
		t.Fatal(err)
	}

	// Force STOP-at-VBlank condition: beam at 0, CPU stopped, no VERTB wake.
	a.vpos = 0
	a.hpos = 0
	a.cpuStopped = true
	// Disable all interrupts so VERTB cannot wake the CPU.
	a.Custom.WriteReg16(0x09A, 0x7FFF) // INTENA clear all (bit 15 = 0 → clear)

	done := make(chan struct{})
	go func() {
		_ = a.RunFrame()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("RunFrame hung for 2s with CPU STOP'd at vpos==0 (leave-line-0 loop)")
	}

	if a.cycles == 0 {
		t.Error("expected cycle progress")
	}
}

// A step that carries the beam past line 0 ends the frame: here the CPU
// waits behind a blit that hogs the chip bus (BLTPRI) from line 300 into
// the next frame. RunFrame used to look for the beam on line 0 after a
// step, missed it, and ran on through another frame.
func TestRunFrameEndsOnABlankPassedMidStep(t *testing.T) {
	a := createTestAmiga(t)
	a.wireMemoryMap(false)
	w := func(reg int, v uint16) { a.Mapper.Write(CustomBase+reg, []byte{byte(v >> 8), byte(v)}) }
	code := []byte{
		0x30, 0x39, 0x00, 0x00, 0x20, 0x00, // MOVE.W $2000,D0: waits for the blit
		0x60, 0xFE, // BRA.S to itself
	}
	a.ChipRAM.Write(0x1000, code)
	a.CPU.PC = 0x1000
	a.cpuStopped = false
	a.vpos, a.hpos = 300, 0
	w(0x096, 0x8640) // DMACON: DMAEN, BLTEN, BLTPRI
	w(0x040, 0x09F0) // BLTCON0: A to D
	w(0x042, 0x0000) // BLTCON1
	w(0x044, 0xFFFF) // BLTAFWM
	w(0x046, 0xFFFF) // BLTALWM
	w(0x050, 0x0001) // BLTAPT $10000
	w(0x052, 0x0000)
	w(0x054, 0x0002) // BLTDPT $20000
	w(0x056, 0x0000)
	w(0x058, 400<<6|0) // BLTSIZE: 400 lines of 64 words
	if !a.Blitter.busy {
		t.Fatal("the blit didn't start")
	}
	start := a.frames
	if err := a.RunFrame(); err != nil {
		t.Fatal(err)
	}
	if n := a.frames - start; n != 1 {
		t.Errorf("RunFrame ran %d frames (now line %d), want 1", n, a.vpos)
	}
}
