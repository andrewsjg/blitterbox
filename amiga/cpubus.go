package amiga

import m68k "github.com/andrewsjg/blitterbox/m68k"

// Exact chip bus timing (ExactBus).
//
// Normally an instruction runs whole and its chip accesses are charged
// afterwards (chipBusWait) against a forecast of the slots DMA will take,
// the Copper's read ahead from its list (Copper.busyWords). With ExactBus
// the CPU's accesses are placed in time instead. Each bus cycle of an
// instruction starts 4 CPU cycles after the last, and before a chip
// access (chip RAM, slow RAM, the custom chips) the rest of the machine
// is brought up to that moment: the beam, the Copper, the blitter and the
// devices run to it (advance). The access then waits there for an even
// colour clock that neither the bitplanes nor the Copper take, as on the
// real bus, and happens at that time, after any Copper write or DMA
// before it and before anything after it. What remains of the
// instruction's time runs after it.
//
// A faster CPU (a 68000 above 7 MHz, the 68020 and up) runs its
// instructions in less time than the chip bus takes, so only its chip
// accesses are placed: each waits for a free slot as above and then
// takes its bus cycle, and a read gets the chips' state at the end of
// it. Accesses elsewhere take no bus time. A CPU polling a custom
// register so gets one read per bus cycle, and sees a change made during
// the read before it takes the interrupt that change raises (Cruise for
// a Corpse polls INTREQR for VERTB with the VERTB interrupt on).

// cpuBus is the CPU's view of memory: the Mapper, with each access timed
// first. Emulator code reads and writes through the Mapper directly, so
// only the CPU's own accesses come here.
type cpuBus struct{ a *Amiga }

func (b *cpuBus) Read(addr int, p []byte) (int, error) {
	b.a.cpuAccess(addr, len(p))
	return b.a.Mapper.Read(addr, p)
}

func (b *cpuBus) Write(addr int, p []byte) (int, error) {
	b.a.cpuAccess(addr, len(p))
	return b.a.Mapper.Write(addr, p)
}

func (b *cpuBus) Reset() error { return b.a.Mapper.Reset() }

// instrTiming follows the instruction being run under ExactBus, in CPU
// cycles from its start.
type instrTiming struct {
	on     bool
	fast   bool // a faster CPU than the 68000: only chip accesses take time
	busT   int  // when the next bus cycle can start
	synced int  // how far the rest of the machine has been run
	waits  int  // cycles spent waiting for the chip bus
	chip   int  // chip accesses timed so (not to be charged again)
}

// exactBus reports whether instructions are timed access by access (the
// 68000 at its own speed). Under ExactBus a faster CPU has its chip
// accesses timed alone.
func (a *Amiga) exactBus() bool {
	return a.ExactBus && a.CPU.Model == m68k.Model68000 && a.CPUSpeed <= 100 // 0: not set, 100%
}

// chipBusAddr reports whether a CPU access to addr goes over the chip
// bus: chip RAM (once the ROM overlay is off), slow RAM, the custom chips.
func (a *Amiga) chipBusAddr(addr int) bool {
	addr &= 0xFFFFFF
	switch {
	case addr < ChipRAMBase+a.ChipRAMSize():
		return !a.overlay
	case addr >= CustomBase && addr <= CustomEnd:
		return true
	case a.SlowRAM != nil && addr >= SlowRAMBase && addr < SlowRAMBase+len(a.SlowRAM.b):
		return true
	}
	return false
}

// cpuAccess times a CPU access of n bytes at addr: a word is one bus
// cycle, a longword two (one in the AGA machines' 32-bit chip RAM).
func (a *Amiga) cpuAccess(addr, n int) {
	in := &a.instr
	if !in.on {
		return
	}
	if !a.chipBusAddr(addr) {
		if !in.fast {
			in.busT += 4 * ((n + 1) / 2)
		}
		return
	}
	for range a.busCycles(n, addr&0xFFFFFF < ChipRAMBase+a.ChipRAMSize()) {
		a.chipSlot()
		in.chip++
		if in.fast {
			a.advance(in.busT - in.synced)
			in.synced = in.busT
		}
	}
}

// chipSlot brings the machine up to the CPU's next bus cycle and waits
// for a chip bus slot for it.
func (a *Amiga) chipSlot() {
	in := &a.instr
	if d := in.busT - in.synced; d > 0 {
		a.advance(d)
		in.synced += d
	}
	wait := 0
	step := func(n int) {
		a.advance(n)
		wait += n
	}
	dmacon := a.Custom.ReadReg16(0x096)
	blit := a.Blitter != nil && a.Blitter.busy && dmacon&0x0240 == 0x0240
	switch {
	case blit && dmacon&0x0400 != 0 && a.Blitter.hogsBus():
		// Nasty mode: a blitter using every slot keeps the bus until it is
		// done. (The idle cycles of other blits are the CPU's.)
		for a.Blitter.busy && a.Custom.ReadReg16(0x096)&0x0640 == 0x0640 {
			step(CPUCyclesPerColorClock * 2)
		}
	case blit && a.Blitter.hogsBus():
		// A blit using every slot lets the CPU in after 3 refusals, and
		// loses that slot.
		step(3 * CPUCyclesPerColorClock)
		a.Blitter.completionCycles += CPUCyclesPerColorClock
	}
	for {
		// The 68000 starts its bus cycles on even colour clocks.
		for a.hpos%(2*CPUCyclesPerColorClock) != 0 {
			step(1)
		}
		if !a.slotTaken() {
			break
		}
		step(2 * CPUCyclesPerColorClock)
	}
	in.waits += wait
	in.synced += wait
	in.busT = in.synced + 4
}

// slotTaken reports whether DMA has the colour clock the beam is at: the
// bitplanes, or the Copper fetching a word in it.
func (a *Amiga) slotTaken() bool {
	if a.bitplaneDMA(a.vpos).uses(a.hpos / CPUCyclesPerColorClock) {
		return true
	}
	c := a.Copper
	return c.fetched && c.fetchV == a.vpos && c.fetchH == a.hpos
}
