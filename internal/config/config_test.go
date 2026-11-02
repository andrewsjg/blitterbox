package config

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type diskList []string

func (l *diskList) String() string { return strings.Join(*l, ",") }
func (l *diskList) Set(v string) error {
	*l = append(*l, v)
	return nil
}

func TestApplyConfig(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "blitterbox.conf")
	os.WriteFile(conf, []byte(`# my A500
rom = ROMS/Kick32.rom
df0 = "disks/a.adf"   # first disk
df0 = /abs/b.adf
fastram = 8192
scale = 3
mute
`), 0o644)

	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	rom := fs.String("rom", "", "")
	var df0 diskList
	fs.Var(&df0, "df0", "")
	fastram := fs.Int("fastram", 2048, "")
	scale := fs.Int("scale", 2, "")
	mute := fs.Bool("mute", false, "")
	fs.Parse([]string{"--scale", "1"}) // the command line wins

	if err := Apply(fs, conf); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "ROMS/Kick32.rom"); *rom != want {
		t.Errorf("rom = %q, want %q (relative to the config file)", *rom, want)
	}
	if want := (diskList{filepath.Join(dir, "disks/a.adf"), "/abs/b.adf"}); !reflect.DeepEqual(df0, want) {
		t.Errorf("df0 = %v, want %v", df0, want)
	}
	if *fastram != 8192 || !*mute {
		t.Errorf("fastram %d, mute %v; want 8192, true", *fastram, *mute)
	}
	if *scale != 1 {
		t.Errorf("scale = %d, want the command line's 1", *scale)
	}
}

func TestApplyConfigErrors(t *testing.T) {
	for _, tc := range []string{"nosuch = 1", "fastram", "fastram = lots"} {
		conf := filepath.Join(t.TempDir(), "c.conf")
		os.WriteFile(conf, []byte(tc+"\n"), 0o644)
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.Int("fastram", 0, "")
		if err := Apply(fs, conf); err == nil {
			t.Errorf("%q: no error", tc)
		}
	}
}

// goAmiga's folder from before the rename, its settings file and saved
// configurations, becomes BlitterBox's.
func TestMoveOld(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	old := filepath.Join(home, ".config", "goamiga")
	os.MkdirAll(filepath.Join(old, "configs"), 0o755)
	os.WriteFile(filepath.Join(old, "goamiga.conf"), []byte("chipram = 1024\n"), 0o644)
	os.WriteFile(filepath.Join(old, "configs", "A4000.conf"), []byte("cpu = 68040\n"), 0o644)
	t.Chdir(t.TempDir())

	if got := Find(""); got != filepath.Join(home, ".config", "blitterbox", "blitterbox.conf") {
		t.Errorf("Find found %q", got)
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("the old folder is still there")
	}
	if got := Saved(); len(got) != 1 || got[0] != "A4000" {
		t.Errorf("saved configurations %v, want [A4000]", got)
	}
	// Once moved, a new goamiga folder is left alone.
	os.MkdirAll(old, 0o755)
	Dir()
	if _, err := os.Stat(old); err != nil {
		t.Error("a later goamiga folder was moved")
	}
}
