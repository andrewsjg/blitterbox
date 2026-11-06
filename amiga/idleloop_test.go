package amiga

import (
	"encoding/binary"
	"testing"
)

// idleTestAmiga is a machine with code at $1000 to run, the CPU in
// supervisor mode with interrupts on.
func idleTestAmiga(t *testing.T, code []uint16) *Amiga {
	t.Helper()
	a := createTestAmiga(t)
	a.wireMemoryMap(false)
	buf := make([]byte, 2*len(code))
	for i, w := range code {
		binary.BigEndian.PutUint16(buf[2*i:], w)
	}
	a.ChipRAM.Write(0x1000, buf)
	a.CPU.PC, a.CPU.SR, a.CPU.A[7] = 0x1000, 0x2000, 0x8000
	a.cpuStopped = false
	return a
}

// runTo steps till the CPU is at pc, failing after max steps.
func runTo(t *testing.T, a *Amiga, pc uint32, max int) int {
	t.Helper()
	steps := 0
	for a.CPU.PC != pc {
		if err := a.Step(); err != nil {
			t.Fatal(err)
		}
		if steps++; steps > max {
			t.Fatalf("not at $%X after %d steps: PC $%X, line %d", pc, max, a.CPU.PC, a.vpos)
		}
	}
	return steps
}

// A loop waiting for the VERTB interrupt's handler to clear a flag in
// RAM runs on to the interrupt, not a test at a time, and leaves the
// loop just after it on line 0, within a round of where the polled loop
// left (polled): run on, the interrupt can come at another point in the
// round. So too one that stores what it reads each round, changing
// nothing (Monkey Island 2).
func TestIdleLoopRunsOnToTheInterrupt(t *testing.T) {
	for _, c := range []struct {
		name   string
		polled int // the colour clock the polled loop left at
		loop   []uint16
	}{
		{"tests the flag", 62, []uint16{
			0x4A39, 0x0000, 0x3000, // loop: TST.B $3000
			0x66F8, //                     BNE.S loop
		}},
		{"stores the flag again", 90, []uint16{
			0x1039, 0x0000, 0x3000, // loop: MOVE.B $3000,D0
			0x13C0, 0x0000, 0x3002, //       MOVE.B D0,$3002
			0x66F2, //                     BNE.S loop
		}},
	} {
		a := idleTestAmiga(t, append(c.loop,
			0x33F9, 0x00DF, 0xF006, 0x0000, 0x4000, // MOVE.W $DFF006,$4000
			0x60FE, // BRA.S to itself
		))
		// The level 3 handler clears the flag and VERTB.
		a.ChipRAM.Write(0x2000, []byte{
			0x42, 0x39, 0x00, 0x00, 0x30, 0x00, // CLR.B $3000
			0x33, 0xFC, 0x00, 0x20, 0x00, 0xDF, 0xF0, 0x9C, // MOVE.W #$20,$DFF09C
			0x4E, 0x73, // RTE
		})
		a.ChipRAM.Write(0x6C, []byte{0x00, 0x00, 0x20, 0x00})
		a.ChipRAM.Write(0x3000, []byte{1})
		a.Custom.Write(0x09A, []byte{0xC0, 0x20}) // INTEN + VERTB
		a.vpos, a.hpos = 100, 0
		steps := runTo(t, a, 0x1000+2*uint32(len(c.loop))+10, 100000)
		got := make([]byte, 2)
		a.ChipRAM.Read(0x4000, got)
		t.Logf("%s: %d steps, left at line %d, colour clock %d", c.name, steps, got[0], got[1])
		if got[0] != 0 || int(got[1]) < c.polled-20 || int(got[1]) > c.polled+20 {
			t.Errorf("%s: left the loop at line %d, colour clock %d: want line 0, near %d", c.name, got[0], got[1], c.polled)
		}
		if steps > 100 {
			t.Errorf("%s: %d steps to the next frame: the loop was polled, not run on", c.name, steps)
		}
	}
}

// A loop that writes, or reads anything but memory, is run as it is: a
// count down in RAM, and a wait for a CIA's port to change.
func TestBusyLoopsAreRun(t *testing.T) {
	for _, c := range []struct {
		name string
		loop []uint16
		n    int
	}{
		{"counts in RAM", []uint16{0x5379, 0x0000, 0x3000, 0x66F8}, 2000},                         // SUBQ.W #1,$3000; BNE.S
		{"reads a CIA", []uint16{0x4A39, 0x00BF, 0xE001, 0x5381, 0x66F6}, 2000},                   // TST.B $BFE001; SUBQ.L #1,D1; BNE.S
		{"reads the beam's colour clock", []uint16{0x4A39, 0x00DF, 0xF007, 0x5381, 0x66F6}, 2000}, // TST.B $DFF007; ...
	} {
		a := idleTestAmiga(t, append(c.loop, 0x60FE))
		a.ChipRAM.Write(0x3000, []byte{byte(c.n >> 8), byte(c.n)})
		a.CPU.D[1] = uint32(c.n)
		end := 0x1000 + 2*uint32(len(c.loop))
		if steps := runTo(t, a, end, 100000); steps < c.n {
			t.Errorf("%s: %d steps for %d rounds", c.name, steps, c.n)
		}
	}
}

// A raster wait for line 100 is run on through a line at a time, not a
// poll at a time, and leaves the loop at the start of line 100 as a
// polled one does: the store after it reads VHPOSR within a poll or two
// (one takes 15 colour clocks on the 68000; polled, it read 28).
func TestRasterWaitRunsOn(t *testing.T) {
	a := idleTestAmiga(t, []uint16{
		0x0C39, 0x0064, 0x00DF, 0xF006, // CMPI.B #100,$DFF006
		0x66F6,                                 // BNE.S back to the CMPI
		0x33F9, 0x00DF, 0xF006, 0x0000, 0x4000, // MOVE.W $DFF006,$4000
		0x60FE, // BRA.S to itself
	})
	steps := runTo(t, a, 0x1014, 100000)
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
