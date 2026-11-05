package amiga

// A raster wait, polling the beam's line in a loop till it changes:
//
//	loop: cmpi.b #line,$DFF006   (or cmp.b $DFF006,Dn)
//	      bne    loop             (any Bcc back to it)
//
// reads the same byte, VHPOSR's V7-V0, for a whole line, and while it
// does the loop goes round and round alike, changing nothing. Bubble
// Bobble spends nearly every frame in one, a read every colour clock and
// a half: emulated a poll at a time, each with its chip bus slot, that
// took most of a frame's emulation time. Once the loop has gone round
// twice on one line it can only go round again till the line changes,
// so the machine runs on to the next line in one go instead, as an
// idle (STOPped) CPU's does, and the loop then reads the new line. It
// stops short for anything that may interrupt the CPU
// (cyclesToNextEvent), and doesn't start with an interrupt to take.

// pollLoop follows a raster wait.
type pollLoop struct {
	pc    uint32   // the loop's start: its compare
	line  int      // the line when the loop last came round
	armed bool     // round twice on that line: Step may run on
	not   uint32   // a backward branch's target found not to be one
	buf   [12]byte // the loop's code, read to check it
}

// notePoll follows the loop: from is where the instruction just run
// was, the CPU now at its next.
func (a *Amiga) notePoll(from uint32) {
	p := &a.poll
	p.armed = false
	to := a.CPU.PC
	if d := from - to; d != 6 && d != 8 {
		return
	}
	switch {
	case to == p.pc && to != 0:
		p.armed = a.vpos == p.line
		p.line = a.vpos
	case to == p.not:
	case a.isRasterWait(to, from):
		p.pc, p.line = to, a.vpos
	default:
		p.not = to
	}
}

// pollSkip is how far the machine can run on through a raster wait, the
// CPU about to go round it again; 0 if not.
func (a *Amiga) pollSkip() int {
	p := &a.poll
	if !p.armed || a.CPU.PC != p.pc || a.vpos != p.line {
		return 0
	}
	p.armed = false
	if lvl := a.CPU.PendingIPL(); lvl > a.CPU.IPL() || lvl == 7 {
		return 0 // the CPU takes an interrupt first
	}
	skip := min(a.lineCycles-a.hpos, a.cyclesToNextEvent())
	if skip < 16 {
		return 0
	}
	return skip
}

// isRasterWait reports whether the code at pc, up to the branch at br,
// is a raster wait: a byte compare with $DFF006 and a branch back.
func (a *Amiga) isRasterWait(pc, br uint32) bool {
	p := &a.poll
	n := int(br-pc) + 4
	if _, err := a.CPU.M.Read(int(pc), p.buf[:n]); err != nil {
		return false
	}
	w := func(i int) uint16 { return uint16(p.buf[i])<<8 | uint16(p.buf[i+1]) }
	l := func(i int) uint32 { return uint32(w(i))<<16 | uint32(w(i+2)) }
	switch br - pc {
	case 8: // CMPI.B #imm,(abs).L
		if w(0) != 0x0C39 || w(2)&0xFF00 != 0 || l(4) != 0x00DFF006 {
			return false
		}
	case 6: // CMP.B (abs).L,Dn
		if w(0)&0xF1FF != 0xB039 || l(2) != 0x00DFF006 {
			return false
		}
	default:
		return false
	}
	b := w(int(br - pc))
	if b&0xF000 != 0x6000 || b&0x0F00 == 0x0100 { // Bcc, not BSR
		return false
	}
	target := br + 2
	switch d := int8(b); d {
	case 0:
		target += uint32(int32(int16(w(int(br-pc) + 2))))
	case -1:
		return false // a 32-bit displacement: not this short a loop
	default:
		target += uint32(int32(d))
	}
	return target == pc
}
