package amiga

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A new hardfile is a disk with an RDB: its blocks' checksums hold, and
// it opens as one partition, DH0, FFS, on cylinders 2 to its last.
func TestCreateHardfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.hdf")
	if err := CreateHardfile(path, 100); err != nil {
		t.Fatal(err)
	}
	if err := CreateHardfile(path, 100); err == nil {
		t.Error("made a hardfile over one already there")
	}
	if err := CreateHardfile(filepath.Join(t.TempDir(), "big.hdf"), NewHDMaxMB+1); err == nil {
		t.Error("made a hardfile past 4 GB")
	}
	b, _ := os.ReadFile(path)
	const cyl = newHDHeads * newHDSectors * hdBlockSize
	if len(b)%cyl != 0 || len(b) > 100<<20 || len(b) < 100<<20-cyl {
		t.Errorf("%d bytes: want 100 MB in whole cylinders", len(b))
	}
	for blk, id := range []string{"RDSK", "PART"} {
		h := b[blk*hdBlockSize:]
		if string(h[:4]) != id || rdbSum(h, 64) != 0 {
			t.Errorf("block %d: %q, sum %08X, want %s summing to 0", blk, h[:4], rdbSum(h, 64), id)
		}
	}
	h, err := OpenHardfile(path, true, 3, 0x444F5300)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	cyls := len(b) / cyl
	if !h.HasRDB() || h.Cylinders != cyls || h.Heads != newHDHeads || h.Sectors != newHDSectors {
		t.Errorf("RDB %v, geometry %d/%d/%d", h.HasRDB(), h.Cylinders, h.Heads, h.Sectors)
	}
	if len(h.Partitions) != 1 {
		t.Fatalf("%d partitions, want 1", len(h.Partitions))
	}
	p := h.Partitions[0]
	if p.Name != "DH0" || !p.Bootable || p.Env[deLowCyl] != 2 || p.Env[deHighCyl] != uint32(cyls-1) || p.Env[deDosType] != 0x444F5303 {
		t.Errorf("partition %+v", p)
	}
}

// A new hardfile on the A1200's IDE interface: Kickstart 3.2's own
// scsi.device, as on a real Amiga, finds its partition in the RDB, and
// Format makes it a volume a file can be written to.
func TestCreateHardfileFormatsOnIDE(t *testing.T) {
	rom, err := LoadROM("../ROMS/kick32a1200.rom")
	if err != nil {
		t.Skip(err)
	}
	hdf := filepath.Join(t.TempDir(), "new.hdf")
	if err := CreateHardfile(hdf, 20); err != nil {
		t.Fatal(err)
	}
	boot := filepath.Join(t.TempDir(), "Boot")
	adfExtract(t, readDisk(t, "../Workbench/Install3.2.adf"), boot)
	os.WriteFile(filepath.Join(boot, "yes"), []byte("\ny\ny\n"), 0o644)
	ss := filepath.Join(boot, "S", "Startup-Sequence")
	s, _ := os.ReadFile(ss)
	script := strings.Join([]string{
		"Info >SYS:info.log",
		"SYS:System/Format DRIVE DH0: NAME Fresh FFS INTL QUICK NOICONS <SYS:yes >SYS:format.log",
		"Echo hello >Fresh:hello",
		"List Fresh: >SYS:list.log",
		"Echo done >SYS:done",
	}, "\n")
	os.WriteFile(ss, []byte(strings.Replace(string(s), "LoadWB", script+"\nLoadWB", 1)), 0o644)

	a, _ := New(rom, nil)
	a.SetCPU("68020", 0)
	a.SetChipset("aga")
	a.SetChipRAM(2048)
	a.ConfigureMemory(0, 8192)
	if err := a.AttachDirectory(boot, "", false); err != nil {
		t.Fatal(err)
	}
	if err := a.AttachIDE(hdf, false); err != nil {
		t.Fatal(err)
	}
	a.Reset()
	m := &whdMachine{a: a}
	for i := 0; i < 120; i++ {
		if _, err := os.Stat(filepath.Join(boot, "done")); err == nil {
			break
		}
		m.frames(25)
	}
	for _, f := range []string{"info.log", "format.log", "list.log"} {
		b, _ := os.ReadFile(filepath.Join(boot, f))
		t.Logf("%s: %s", f, b)
	}
	if list, _ := os.ReadFile(filepath.Join(boot, "list.log")); !bytes.Contains(list, []byte("hello")) {
		t.Fatal("no file on the formatted drive")
	}
	// The partition's first block, after the RDB's two cylinders, is FFS's.
	img, _ := os.ReadFile(hdf)
	root := img[2*newHDHeads*newHDSectors*hdBlockSize:]
	if dt := binary.BigEndian.Uint32(root); dt != 0x444F5303 {
		t.Errorf("partition starts %08X, want DOS\\3", dt)
	}
}

// Two new hardfiles, both DH0 in their RDBs, are DH0 and DH1, and a
// folder beside them DH2.
func TestNewHardfilesNamedApart(t *testing.T) {
	a := createTestAmiga(t)
	dir := t.TempDir()
	for _, n := range []string{"a.hdf", "b.hdf"} {
		p := filepath.Join(dir, n)
		if err := CreateHardfile(p, 10); err != nil {
			t.Fatal(err)
		}
		if err := a.AttachHardfile(p, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.AttachDirectory(t.TempDir(), "Folder", true); err != nil {
		t.Fatal(err)
	}
	a.HardDisk.buildROM(0) // as autoconfig does
	var names []string
	for _, p := range a.HardDisk.parts {
		names = append(names, p.p.Name)
	}
	if got := strings.Join(names, " "); got != "DH0 DH1 DH2" {
		t.Errorf("devices %s, want DH0 DH1 DH2", got)
	}
}

// HDToolBox reaches a disk on goamiga.device as it would a SCSI drive
// (HD_SCSICMD): it asks what the drive is, reads the RDB, and writes the
// partition back changed, here its Mask, which the partition has from
// the next reset on.
func TestHDSCSICmd(t *testing.T) {
	a := createTestAmiga(t)
	path := filepath.Join(t.TempDir(), "new.hdf")
	if err := CreateHardfile(path, 10); err != nil {
		t.Fatal(err)
	}
	if err := a.AttachHardfile(path, false); err != nil {
		t.Fatal(err)
	}
	h := a.HardDisk
	const base = 0xE90000
	h.base = base
	h.configured(base)
	a.wireMemoryMap(false) // chip RAM at 0, not the ROM's overlay

	const io, sc, cdbAt, buf, sense = 0x1000, 0x1100, 0x1200, 0x2000, 0x1300
	run := func(read bool, n int, cdb ...byte) (status byte, ioErr byte) {
		t.Helper()
		a.poke32(io+ioUnit, base+hdUnits)
		a.poke16(io+ioCommand, 28)
		a.poke8(io+ioFlags, iofQuick)
		a.poke32(io+ioData, sc)
		a.poke32(io+ioLength, scsiCmdSize)
		a.pokeBytes(cdbAt, cdb)
		a.poke32(sc+scsiData, buf)
		a.poke32(sc+scsiLength, uint32(n))
		a.poke32(sc+scsiCommand, cdbAt)
		a.poke16(sc+scsiCmdLength, uint16(len(cdb)))
		flags := byte(scsifAutoSense)
		if read {
			flags |= scsifRead
		}
		a.poke8(sc+scsiFlags, flags)
		a.poke32(sc+scsiSenseData, sense)
		a.poke16(sc+scsiSenseLength, 18)
		h.beginIO(io)
		return a.peek8(sc + scsiStatus), a.peek8(io + ioError)
	}
	got := func(n int) []byte { b := make([]byte, n); a.peekBytes(buf, b); return b }

	if s, e := run(true, 36, 0x12, 0, 0, 0, 36, 0); s != 0 || e != 0 || !bytes.Contains(got(36), []byte("BLITTERB")) {
		t.Errorf("INQUIRY: status %d, error %d, %q", s, e, got(36))
	}
	if s, _ := run(true, 8, 0x25, 0, 0, 0, 0, 0, 0, 0, 0, 0); s != 0 || binary.BigEndian.Uint32(got(4)) != uint32(h.Files[0].blocks-1) {
		t.Errorf("READ CAPACITY: status %d, last block %d", s, binary.BigEndian.Uint32(got(4)))
	}
	// The RDB and its partition: READ(10) of blocks 0 and 1.
	if s, _ := run(true, 1024, 0x28, 0, 0, 0, 0, 0, 0, 0, 2, 0); s != 0 || string(got(4)) != "RDSK" {
		t.Fatalf("READ: status %d, %q", s, got(4))
	}
	part := got(1024)[512:]
	if string(part[:4]) != "PART" {
		t.Fatalf("block 1 is %q", part[:4])
	}
	binary.BigEndian.PutUint32(part[128+4*deMask:], 0x00FFFFFE)
	binary.BigEndian.PutUint32(part[8:], 0)
	binary.BigEndian.PutUint32(part[8:], -rdbSum(part, 64))
	a.pokeBytes(buf, part)
	if s, e := run(false, 512, 0x2A, 0, 0, 0, 0, 1, 0, 0, 1, 0); s != 0 || e != 0 {
		t.Fatalf("WRITE: status %d, error %d", s, e)
	}
	// A command the drive doesn't know: CHECK CONDITION, and the sense.
	if s, e := run(true, 0, 0xFF, 0, 0, 0, 0, 0); s != scsiCheckCondition || e != hferrBadStatus || a.peek8(sense+2) != senseIllegalRequest {
		t.Errorf("unknown command: status %d, error %d, sense key %d", s, e, a.peek8(sense+2))
	}

	if m := h.Files[0].Partitions[0].Env[deMask]; m != 0x7FFFFFFE {
		t.Errorf("Mask %08X before the reset", m)
	}
	h.configured(base) // the reset HDToolBox asks for
	if m := h.Files[0].Partitions[0].Env[deMask]; m != 0x00FFFFFE {
		t.Errorf("Mask %08X after the reset, want 00FFFFFE", m)
	}
}
