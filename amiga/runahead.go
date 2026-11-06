package amiga

// Running ahead: the CPU running on, an instruction at a time, while the
// rest of the machine waits, and catching the machine up later in one go.
//
// Step runs the machine on after each instruction (advance), the beam,
// the Copper, the blitter and every device, by the instruction's time. A
// fast CPU (the 68040 takes under 2 cycles of the 7 MHz clock for most
// instructions) running from fast RAM takes as long over that as over the
// instruction (The Secret of Monkey Island draws its actors so: a few
// instructions in fast RAM for each access to its chip RAM buffer). Yet
// nothing such an instruction does sees the rest of the machine, nor
// does the machine see it: fast RAM and the ROM are the CPU's alone. So
// the machine is run on only when the CPU accesses something else, which
// it brings up to the instruction's start, as it would be (catchUp); or
// when something in it may change what the CPU sees unasked, an
// interrupt: at the next event cyclesToNextEvent knows of, or the end of
// the line, whose start ticks CIA-B's TOD and the pots.

// aheadRecheck is the least time, in CPU cycles, between looks for the
// next event: while the Copper runs there is none to run ahead to, and
// looking costs.
const aheadRecheck = 16

// runAhead is how far the CPU has run ahead of the rest of the machine.
type runAhead struct {
	n       int    // CPU cycles ahead
	valid   bool   // the CPU may run ahead to machine cycle to
	to      uint64 // the next event, or the end of the line
	recheck uint64 // the machine cycle from which to look for it again
}

// runOn runs the machine on by n cycles, the CPU's last instruction, or
// lets the CPU run ahead by them.
func (a *Amiga) runOn(n int) {
	r := &a.ahead
	if a.cpuStopped || a.idle.watch {
		// A STOPped CPU idles a frame at a time (Step), and a loop
		// watched for idling compares the beam's line with the CPU's.
		r.valid = false
		a.catchUp()
		a.advance(n)
		return
	}
	if !r.valid && a.cycles >= r.recheck {
		r.recheck = a.cycles + aheadRecheck
		if c := min(a.cyclesToNextEvent(), a.lineCycles-a.hpos); c > 0 {
			r.valid, r.to = true, a.cycles+uint64(c)
		}
	}
	r.n += n
	if !r.valid || a.cycles+uint64(r.n) >= r.to {
		r.valid = false
		a.catchUp()
	}
}

// catchUp brings the rest of the machine up to the CPU.
func (a *Amiga) catchUp() {
	if n := a.ahead.n; n > 0 {
		a.ahead.n = 0
		a.advance(n)
	}
}

// cpuReaches is called before each CPU access, to addr: one to anything
// but fast RAM or the ROM catches the machine up first, and one to a
// device (not chip or slow RAM) may change when its next event is.
func (a *Amiga) cpuReaches(addr int) {
	r := &a.ahead
	if r.n == 0 && !r.valid {
		return
	}
	if a.chipBusAddr(addr) {
		a.catchUp()
		if a := addr & 0xFFFFFF; a >= CustomBase && a <= CustomEnd {
			r.valid = false
		}
		return
	}
	if a.isMemory(a.Mapper.Device(addr)) {
		return
	}
	a.catchUp()
	r.valid = false
}
