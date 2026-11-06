package amiga

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// A 68040 counting in fast RAM runs ahead of the rest of the machine, and
// sees it as one that does not: the beam it reads after each count, and
// where it takes each VERTB interrupt (the beam, and the count, the
// handler stores), are as they are with the machine caught up after every
// instruction.
func TestRunAheadSeesTheMachineAsItIs(t *testing.T) {
	code := []uint16{
		0x41F9, 0x0000, 0x3000, //       LEA $3000,A0
		0x721E,         //               loop:  MOVEQ #30,D1
		0x5280,         //               count: ADDQ.L #1,D0
		0x51C9, 0xFFFC, //              DBF D1,count
		0x30F9, 0x00DF, 0xF006, //      MOVE.W $DFF006,(A0)+ (VHPOSR)
		0x60F0, //                      BRA.S loop
	}
	handler := []uint16{
		0x30F9, 0x00DF, 0xF006, //      MOVE.W $DFF006,(A0)+
		0x30C0,                         //                      MOVE.W D0,(A0)+
		0x33FC, 0x0020, 0x00DF, 0xF09C, // MOVE.W #$20,INTREQ
		0x4E73, //                      RTE
	}
	run := func(everyStep bool) (log []byte, ahead int) {
		a := fastCPU(t, nil)
		fast := &ram{b: make([]byte, 0x1000)}
		if err := a.Mapper.Map(fast, 0x200000, 0x200FFF); err != nil {
			t.Fatal(err)
		}
		for i, w := range code {
			binary.BigEndian.PutUint16(fast.b[2*i:], w)
		}
		for i, w := range handler {
			binary.BigEndian.PutUint16(fast.b[0x100+2*i:], w)
		}
		a.ChipRAM.Write(0x6C, []byte{0x00, 0x20, 0x01, 0x00})
		a.Custom.Write(0x09A, []byte{0xC0, 0x20}) // INTEN + VERTB
		a.CPU.PC = 0x200000
		for range 300000 {
			if err := a.Step(); err != nil {
				t.Fatal(err)
			}
			ahead = max(ahead, a.ahead.n)
			if everyStep {
				a.catchUp()
			}
		}
		log = make([]byte, a.CPU.A[0]-0x3000)
		a.ChipRAM.Read(0x3000, log)
		return log, ahead
	}
	got, ahead := run(false)
	want, _ := run(true)
	if ahead == 0 {
		t.Error("the CPU never ran ahead")
	}
	if len(want) < 1000 {
		t.Fatalf("%d bytes logged: the program did not run", len(want))
	}
	if !bytes.Equal(got, want) {
		i := 0
		for i < min(len(got), len(want)) && got[i] == want[i] {
			i++
		}
		t.Errorf("run ahead, the CPU's log differs from byte %d of %d (%d logged)", i, len(want), len(got))
	}
}
