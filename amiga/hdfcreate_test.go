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
