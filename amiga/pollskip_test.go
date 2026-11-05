package amiga

import (
	"encoding/binary"
	"testing"
)

// A raster wait for line 100 is run on through a line at a time, not a
// poll at a time, and leaves the loop at the start of line 100 as a
// polled one does: the store after it reads VHPOSR within a poll or two
// (one takes 15 colour clocks on the 68000; polled, it read 28).
func TestRasterWaitRunsOn(t *testing.T) {
	a := createTestAmiga(t)
	a.wireMemoryMap(false)
	code := []uint16{
		0x0C39, 0x0064, 0x00DF, 0xF006, // CMPI.B #100,$DFF006
		0x66F6,                                 // BNE.S back to the CMPI
		0x33F9, 0x00DF, 0xF006, 0x0000, 0x4000, // MOVE.W $DFF006,$4000
		0x60FE, // BRA.S to itself
	}
	buf := make([]byte, 2*len(code))
	for i, w := range code {
		binary.BigEndian.PutUint16(buf[2*i:], w)
	}
	a.ChipRAM.Write(0x1000, buf)
	a.CPU.PC = 0x1000
	a.cpuStopped = false
	steps := 0
	for a.CPU.PC != 0x1014 {
		if err := a.Step(); err != nil {
			t.Fatal(err)
		}
		if steps++; steps > 100000 {
			t.Fatalf("never left the loop: PC %X, line %d, hpos %d, model %v", a.CPU.PC, a.vpos, a.hpos, a.CPU.Model)
		}
	}
	got := make([]byte, 2)
	a.ChipRAM.Read(0x4000, got)
	t.Logf("%d steps, left at line %d, colour clock %d", steps, got[0], got[1])
	if got[0] != 100 || got[1] > 32 {
		t.Errorf("left the wait at line %d, colour clock %d: want line 100, its start", got[0], got[1])
	}
	if steps > 2000 {
		t.Errorf("%d steps for 100 lines: the wait was polled, not run on", steps)
	}
}
