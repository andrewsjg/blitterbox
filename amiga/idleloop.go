package amiga

import "math"

// An idle loop: the CPU going round a loop that waits for something to
// change, as games wait for a frame:
//
//	wait: tst.b  (a0)            ; a flag the interrupt clears (Bubba 'n' Stix)
//	      bne.s  wait
//
//	wait: cmpi.b #line,$DFF006   ; a raster wait (Bubble Bobble)
//	      bne.s  wait
//
// Emulated an instruction at a time, a loop like this can take most of a
// frame's emulation time while the machine does nothing. If one round of
// it starts and ends at the same instruction with the same registers,
// has changed no memory (a write may store what is there already) and
// has read only memory, the next round will be the same, and the one
// after, until something else changes what it reads: an interrupt,
// which the events cyclesToNextEvent knows of raise. So the machine runs
// on to the next of those in one go, as an idle (STOPped) CPU's does,
// and the loop then goes round again on what it finds. While the Copper
// runs it may raise one at any time, so the machine runs on a little at
// a time, as long as none is raised (runIdle). A loop that reads the
// beam's line (VPOSR, VHPOSR's V7-V0) can see it change at the end of
// the line, so it runs on that far.
//
// A round is found alike in two steps, so that most code pays only for
// the first: a backward jump to where the last one landed, with the
// same registers, has the CPU's accesses watched through the next
// round (watchRead, watchWrite, cpuBus).

// idleLoopSpan is the longest loop looked for, in bytes back.
const idleLoopSpan = 256

// cpuRegs is the CPU state a loop's round must leave as it found it.
type cpuRegs struct {
	D, A [8]uint32
	SR   uint32
	FP   [8]uint64 // bits: NaN, as a 68040's reset leaves them, equals itself
	FPSR uint32
}

// idleLoop follows a loop that may be idle.
type idleLoop struct {
	pc     uint32  // where the last backward jump landed: the loop's start
	regs   cpuRegs // the CPU's registers then
	watch  bool    // the registers came round alike: this round's accesses are watched
	clean  bool    // the watched round has changed nothing and read only memory
	line   bool    // it read the beam's line, on line lineAt
	lineAt int
	armed  bool // idle: Step may run on (runIdle)
	toLine bool // as far as the line's end only
}

func (a *Amiga) cpuRegs() cpuRegs {
	c := a.CPU
	r := cpuRegs{D: c.D, A: c.A, SR: c.SR, FPSR: c.FPSR}
	for i, f := range c.FP {
		r.FP[i] = math.Float64bits(f)
	}
	return r
}

// noteLoop follows the loop: from is where the instruction just run was,
// the CPU now at its next.
func (a *Amiga) noteLoop(from uint32) {
	l := &a.idle
	l.armed = false
	to := a.CPU.PC
	if to >= from || from-to > idleLoopSpan {
		return // not a jump back
	}
	r := a.cpuRegs()
	if to != l.pc || r != l.regs {
		l.pc, l.regs, l.watch = to, r, false
		return
	}
	if l.watch && l.clean && (!l.line || a.vpos == l.lineAt) {
		l.armed, l.toLine = true, l.line
	}
	l.watch, l.clean, l.line = true, true, false
}

// watchRead notes a CPU read in a watched round: memory, or the beam's
// line, which stays as it is till the line ends.
func (a *Amiga) watchRead(addr, n int) {
	l := &a.idle
	if a.CPU.M.Fetch || !l.clean {
		return
	}
	dev := a.Mapper.Device(addr)
	if a.isMemory(dev) {
		return
	}
	if m, ok := dev.(*busCounter); ok && m.m == a.Custom {
		switch off := addr & 0x1FF; {
		case off == 0x004 && n <= 2, off == 0x005 && n == 1, off == 0x006 && n == 1:
			if !l.line {
				l.line, l.lineAt = true, a.vpos
			}
			if a.vpos == l.lineAt {
				return
			}
		}
	}
	l.clean = false
}

// watchWrite notes a CPU write in a watched round, before it is made: one
// that stores in memory what is there already changes nothing (Monkey
// Island 2 keeps the counter it waits on in a local variable, stored
// again each round).
func (a *Amiga) watchWrite(addr int, p []byte) {
	l := &a.idle
	if !l.clean {
		return
	}
	var old [16]byte
	if len(p) > len(old) || !a.isMemory(a.Mapper.Device(addr)) {
		l.clean = false
		return
	}
	n := a.cpuChipAccesses // the emulator's read, not the CPU's
	a.Mapper.Read(addr, old[:len(p)])
	a.cpuChipAccesses = n
	if string(old[:len(p)]) != string(p) {
		l.clean = false
	}
}

// isMemory reports whether dev, mapped in the CPU's address space, is
// RAM or ROM: reading it changes nothing.
func (a *Amiga) isMemory(dev any) bool {
	switch m := dev.(type) {
	case *ram:
		return true
	case *busCounter:
		return m.m == a.ChipRAM || m.m == a.SlowRAM
	}
	return dev != nil && dev == a.ROM
}

// idleCopperChunk is how far, in CPU cycles, the machine runs on through
// an idle loop at a time while the Copper runs, which may raise an
// interrupt at any time: about a round of a short loop, the time the
// CPU takes an interrupt in it.
const idleCopperChunk = 16

// runIdle runs the machine on through an idle loop, the CPU about to go
// round it again: to the next event, or while the Copper runs, till it
// raises an interrupt. It reports whether it did.
func (a *Amiga) runIdle() bool {
	l := &a.idle
	if !l.armed || a.CPU.PC != l.pc {
		return false
	}
	l.armed = false
	if a.interruptDue() {
		return false // the CPU takes it first
	}
	skip := a.cyclesToDeviceEvent()
	chunk := skip
	switch c := a.Copper.idleCycles(a.hpos, a.vpos); {
	case c == 0:
		chunk = idleCopperChunk
	case c > 0:
		skip = min(skip, c)
	}
	if l.toLine {
		skip = min(skip, a.lineCycles-a.hpos)
	}
	if skip < 16 {
		return false
	}
	for skip > 0 {
		n := min(skip, chunk)
		a.advance(n)
		if skip -= n; a.interruptDue() {
			break
		}
	}
	return true
}

// interruptDue reports whether the CPU takes an interrupt before its next
// instruction.
func (a *Amiga) interruptDue() bool {
	lvl := a.CPU.PendingIPL()
	return lvl > a.CPU.IPL() || lvl == 7
}
