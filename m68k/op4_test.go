package m68k_test

import (
	"testing"

	. "github.com/andrewsjg/blitterbox/m68k/m68ktest"
)

func TestLea(t *testing.T) {
	t.Run("WithAbsoluteShort", func(t *testing.T) {
		p := LoadBytes(t, []byte{
			0x41, 0xF8, 0x7F, 0xFF, // lea $7FFF.w,A0
		})
		AssertRun(t, p)
		AssertAddressRegister(t, p, 0, 0x7FFF)
	})

	t.Run("WithAbsoluteLong", func(t *testing.T) {
		p := LoadBytes(t, []byte{
			0x41, 0xF9, 0xFF, 0xFF, 0xFF, 0xFF, // lea $FFFFFFFF.l,A0
		})
		AssertRun(t, p)
		AssertAddressRegister(t, p, 0, 0xFFFFFFFF)
	})
}

func TestClr(t *testing.T) {
	p := LoadBytes(t, []byte{
		0x42, 0x00, // clr.b D0
		0x42, 0x41, // clr.w D1
		0x42, 0x82, // clr.l D2
		0x00, 0x00, // EOF
		0xAA, 0xAA, 0xAA, 0xAA, // dc.l $AAAAAAAA
		0xBB, 0xBB, 0xBB, 0xBB, // dc.l $BBBBBBBB
		0xCC, 0xCC, 0xCC, 0xCC, // dc.l $CCCCCCCC
	})
	p.D[0] = 0xFFFFFFFF
	p.D[1] = 0xFFFFFFFF
	p.D[2] = 0xFFFFFFFF

	AssertRun(t, p)
	AssertDataRegister(t, p, 0, 0xFFFFFF00)
	AssertDataRegister(t, p, 1, 0xFFFF0000)
	AssertDataRegister(t, p, 2, 0)
}

func TestMovem(t *testing.T) {
	// TODO: improve movem tests
	p := LoadBytes(t, []byte{
		0x4c, 0x9d, 0x00, 0x03, // movem.w (A5)+,$E0
	})
	p.A[5] = p.PC
	AssertRun(t, p)
	AssertDataRegister(t, p, 0, 0x4C9D)
	AssertDataRegister(t, p, 1, 0x0003)
}

// MOVEM.L (d16,PC),regs — used by AmigaDOS/CLI jump tables.
// Without PC-relative support it raised Illegal Instruction.
func TestMovemPCRelative(t *testing.T) {
	// Register mask $6400 = bits 10,13,14 → A2, A5, A6
	// The disp word is at offset 4 and the table at 10, so d = 6.
	p := LoadBytes(t, []byte{
		0x4C, 0xFA, 0x64, 0x00, 0x00, 0x06, // movem.l (6,PC), a2/a5-a6
		0x4E, 0x71, // nop
		0x00, 0x00, // EOF
		0x11, 0x22, 0x33, 0x44, // A2 @ offset 10; PC_at_disp=4; 4+6=10
		0x55, 0x66, 0x77, 0x88, // A5
		0x99, 0xAA, 0xBB, 0xCC, // A6
	})
	AssertRun(t, p)
	AssertAddressRegister(t, p, 2, 0x11223344)
	AssertAddressRegister(t, p, 5, 0x55667788)
	AssertAddressRegister(t, p, 6, 0x99AABBCC)
}
