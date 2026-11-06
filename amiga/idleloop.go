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
// has left memory as it was and has read only memory, the next round
// will be the same, and the one after, until something else changes what
// it reads: an interrupt, which the events cyclesToNextEvent knows of
// raise. So the machine runs on to the next of those in one go, as an
// idle (STOPped) CPU's does, and the loop then goes round again on what
// it finds. While the Copper runs it may raise one at any time, so the
// machine runs on a little at a time, as long as none is raised
// (runIdle). A loop that reads the beam's line
// (VPOSR, VHPOSR's V7-V0) can see it change at the end of the line, so
// it runs on that far.
//
// A round is found alike in two steps, so that most code pays only for
// the first: a backward jump to one of the last places jumped back to,
// with the same registers as then, has the CPU's accesses watched
// through the next round (watchRead, watchWrite, cpuBus).

// idleLoopSpan is the longest loop looked for, in bytes back; a loop is
// looked for at the last idleLoopStarts places jumped back to, so that
// one with loops inside it is found as well as they are.
const (
	idleLoopSpan   = 1024
	idleLoopStarts = 8
)

// idleRoundSteps is the most instructions a watched round may take.
const idleRoundSteps = 4096

// idleWrites is how many bytes a round may write, each to be as it was
// when the round ends: Eye of the Beholder 2 writes over 64, most of them
// on the stack.
const idleWrites = 512

// cpuRegs is the CPU state a loop's round must leave as it found it.
type cpuRegs struct {
	D, A [8]uint32
	SR   uint32
	FP   [8]uint64 // bits: NaN, as a 68040's reset leaves them, equals itself
	FPSR uint32
}

// idleBackoff is the most times a loop's registers may come round alike
// unwatched after its watched rounds have found it busy: watching a
// round costs, and a busy loop with the same registers each round
// (Dungeon Master's main loop) would otherwise be watched every round.
const idleBackoff = 64

// loopStart is a place a backward jump landed, with the registers then.
type loopStart struct {
	pc      uint32
	regs    cpuRegs
	wait    int // rounds alike to let go unwatched
	backoff int // how many the last watch found busy set wait to
}

// idleLoop follows a loop that may be idle.
type idleLoop struct {
	starts [idleLoopStarts]loopStart
	next   int // the start to be replaced next

	loopStart      // the loop being watched
	at        int  // its place in starts
	watch     bool // its registers came round alike, and this round has accessed only memory
	steps     int  // instructions into the watched round
	line      bool // it read the beam's line, on line lineAt
	lineAt    int
	writes    map[int]byte // the bytes the round has written, as they were
	armed     bool         // idle: Step may run on (runIdle)
	toLine    bool         // as far as the line's end only
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
	if l.watch {
		if l.steps++; l.steps > idleRoundSteps {
			a.busy()
		}
	}
	to := a.CPU.PC
	if to >= from || from-to > idleLoopSpan {
		return // not a jump back
	}
	r := a.cpuRegs()
	if l.watch && to == l.pc {
		if r == l.regs {
			if (!l.line || a.vpos == l.lineAt) && a.writesUndone() {
				l.armed, l.toLine = true, l.line
				l.starts[l.at].backoff = 0
				a.watchRound() // and the next round
			} else {
				a.busy()
			}
			return
		}
		a.busy()
	}
	for i := range l.starts {
		st := &l.starts[i]
		if st.pc != to {
			continue
		}
		switch {
		case st.regs != r:
			st.regs = r
		case l.watch:
		case st.wait > 0:
			st.wait--
		default:
			l.loopStart, l.at, l.watch = *st, i, true
			a.watchRound()
		}
		return
	}
	l.starts[l.next] = loopStart{pc: to, regs: r}
	l.next = (l.next + 1) % idleLoopStarts
}

// watchRead notes a CPU read in a watched round: memory, or the beam's
// line, which stays as it is till the line ends.
func (a *Amiga) watchRead(addr, n int) {
	l := &a.idle
	if a.CPU.M.Fetch {
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
	a.busy()
}

// busy ends the watch: the loop is busy, and its next few rounds alike go
// unwatched (idleBackoff).
func (a *Amiga) busy() {
	l := &a.idle
	l.watch = false
	if st := &l.starts[l.at]; st.pc == l.pc {
		st.backoff = min(max(1, 2*st.backoff), idleBackoff)
		st.wait = st.backoff
	}
}

// watchRound starts watching a round of the loop.
func (a *Amiga) watchRound() {
	l := &a.idle
	l.steps, l.line = 0, false
	if l.writes == nil {
		l.writes = make(map[int]byte, idleWrites)
	}
	clear(l.writes)
}

// watchWrite notes a CPU write in a watched round, before it is made:
// to memory, where what it changes must be as it was when the round ends
// (writesUndone). Monkey Island 2 keeps the counter it waits on in a
// local variable, stored again each round; Eye of the Beholder 2 calls a
// subroutine for each part of the screen the mouse may be over, which
// uses the stack, and sets a variable and puts it back.
func (a *Amiga) watchWrite(addr int, p []byte) {
	l := &a.idle
	if !a.isMemory(a.Mapper.Device(addr)) {
		a.busy()
		return
	}
	for i := range p {
		if _, ok := l.writes[addr+i]; ok {
			continue
		}
		if len(l.writes) == idleWrites {
			a.busy()
			return
		}
		l.writes[addr+i] = a.peek(addr + i)
	}
}

// writesUndone reports whether the bytes the round has written are as
// they were before it.
func (a *Amiga) writesUndone() bool {
	for addr, old := range a.idle.writes {
		if a.peek(addr) != old {
			return false
		}
	}
	return true
}

// peek reads a byte of memory for the emulator, not the CPU: the read
// takes no bus time.
func (a *Amiga) peek(addr int) byte {
	var b [1]byte
	n := a.cpuChipAccesses
	a.Mapper.Read(addr, b[:])
	a.cpuChipAccesses = n
	return b[0]
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
