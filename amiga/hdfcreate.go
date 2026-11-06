package amiga

import (
	"encoding/binary"
	"fmt"
	"os"
)

// A new, blank hardfile is a whole disk, as a real one is: a Rigid Disk
// Block (RDSK, block 0) and one partition (PART, block 1), DH0, bootable
// after everything else (newHDBootPri), for the Fast File System (DOS\3)
// Kickstart 2.0 and later have in ROM, on cylinders 2 to the end. So it
// needs only formatting (Format, or Workbench's Format Disk on DH0:NDOS),
// on goamiga.device, on IDE (whose scsi.device, Kickstart's own, reads
// the RDB as it reads a real disk's), on the A2091, and on a real Amiga's
// controller, from an SD card in a BlueSCSI or the like.

// New hardfiles' geometry: 16 heads of 63 sectors, the usual for a disk
// of this kind (and so for HDToolBox), and the least and most sizes, in
// MB. Kickstart's scsi.device and FFS address the first 4 GB only.
const (
	newHDHeads   = 16
	newHDSectors = 63
	NewHDMinMB   = 8
	NewHDMaxMB   = 4000

	// It boots only when nothing else will: blank, it can't, and the
	// disks and folders beside it boot at -10 and up.
	newHDBootPri = 0xFFFFFFEC // -20
)

// CreateHardfile makes path a blank, partitioned hard disk of about mb
// MB (whole cylinders). It does not replace a file already there.
func CreateHardfile(path string, mb int) error {
	if mb < NewHDMinMB || mb > NewHDMaxMB {
		return fmt.Errorf("a new hardfile is %d to %d MB, not %d", NewHDMinMB, NewHDMaxMB, mb)
	}
	img := newRDB(mb)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(img.blocks)
	if err == nil {
		err = f.Truncate(img.size)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
	}
	return err
}

// rdbImage is a new hardfile: its first blocks, and its whole size.
type rdbImage struct {
	blocks []byte
	size   int64
}

func newRDB(mb int) rdbImage {
	const cylBlocks = newHDHeads * newHDSectors
	cyls := uint32(int64(mb) << 20 / hdBlockSize / cylBlocks)
	b := make([]byte, 2*hdBlockSize)
	be := binary.BigEndian
	put := func(blk, off int, v uint32) { be.PutUint32(b[blk*hdBlockSize+off:], v) }
	none := uint32(0xFFFFFFFF)

	// The Rigid Disk Block (devices/hardblocks.h, struct RigidDiskBlock).
	copy(b, "RDSK")
	put(0, 4, 64) // rdb_SummedLongs
	put(0, 12, 7) // rdb_HostID
	put(0, 16, hdBlockSize)
	put(0, 20, 2) // rdb_Flags: RDBFF_LASTLUN (other IDs are still looked at)
	for off := 24; off < 64; off += 4 {
		put(0, off, none) // no bad blocks, filesystems, drive init or boot code
	}
	put(0, 28, 1) // rdb_PartitionList
	put(0, 64, cyls)
	put(0, 68, newHDSectors)
	put(0, 72, newHDHeads)
	put(0, 76, 1)              // rdb_Interleave
	put(0, 80, cyls)           // rdb_Park
	put(0, 96, cyls)           // rdb_WritePreComp
	put(0, 100, cyls)          // rdb_ReducedWrite
	put(0, 104, 3)             // rdb_StepRate
	put(0, 128, 0)             // rdb_RDBBlocksLo
	put(0, 132, 2*cylBlocks-1) // rdb_RDBBlocksHi: cylinders 0 and 1
	put(0, 136, 2)             // rdb_LoCylinder
	put(0, 140, cyls-1)        // rdb_HiCylinder
	put(0, 144, cylBlocks)
	put(0, 152, 1) // rdb_HighRDSKBlock
	copy(b[160:], "BLITTERB")
	copy(b[168:], "HARDFILE")
	copy(b[184:], "1.0")

	// Its partition (struct PartitionBlock).
	p := hdBlockSize
	copy(b[p:], "PART")
	put(1, 4, 64)
	put(1, 12, 7)
	put(1, 16, none) // pb_Next
	put(1, 20, 1)    // pb_Flags: PBFF_BOOTABLE
	put(1, 24, none)
	put(1, 28, none)
	b[p+36] = 3
	copy(b[p+37:], "DH0")
	env := []uint32{
		deTableSize:      deDosType,
		deSizeBlock:      hdBlockSize / 4,
		deSurfaces:       newHDHeads,
		deSectorPerBlock: 1,
		deBlocksPerTrack: newHDSectors,
		deReserved:       2,
		deLowCyl:         2,
		deHighCyl:        cyls - 1,
		deNumBuffers:     30,
		deMaxTransfer:    0x1FE00, // what controllers of every kind take
		deMask:           0x7FFFFFFE,
		deBootPri:        newHDBootPri,
		deDosType:        0x444F5303, // DOS\3: FFS, international
	}
	for i, v := range env {
		put(1, 128+4*i, v)
	}

	for blk := range 2 {
		put(blk, 8, -rdbSum(b[blk*hdBlockSize:], 64))
	}
	return rdbImage{blocks: b, size: int64(cyls) * cylBlocks * hdBlockSize}
}

// rdbSum is the sum of a hard block's first n longs.
func rdbSum(b []byte, n int) uint32 {
	var s uint32
	for i := range n {
		s += binary.BigEndian.Uint32(b[4*i:])
	}
	return s
}
