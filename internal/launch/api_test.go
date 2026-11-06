package launch

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrewsjg/blitterbox/amiga"
	"github.com/andrewsjg/blitterbox/internal/config"
)

// apiCall sends requests to the launcher API and decodes the answers.
func apiCall(t *testing.T, reqs ...Request) []Response {
	t.Helper()
	var in bytes.Buffer
	for _, r := range reqs {
		b, _ := json.Marshal(r)
		in.Write(append(b, '\n'))
	}
	var out bytes.Buffer
	if err := Serve(&in, &out); err != nil {
		t.Fatal(err)
	}
	var resps []Response
	dec := json.NewDecoder(&out)
	for dec.More() {
		var r Response
		if err := dec.Decode(&r); err != nil {
			t.Fatal(err)
		}
		resps = append(resps, r)
	}
	if len(resps) != len(reqs) {
		t.Fatalf("%d answers to %d requests", len(resps), len(reqs))
	}
	return resps
}

func TestLauncherAPI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(t.TempDir()) // no ./blitterbox.conf

	r := apiCall(t, Request{Op: "startup"})[0]
	if !r.OK || r.Settings == nil || r.Settings.CPU != "68000" || len(r.Saved) != 0 {
		t.Fatalf("startup: %+v", r)
	}

	s := config.Defaults()
	s.ROM = filepath.Join(home, "kick.rom")
	rom := make([]byte, 512*1024)
	copy(rom[12:], []byte{0, 40, 0, 68}) // Kickstart 40.68 (3.1)
	if err := os.WriteFile(s.ROM, rom, 0o644); err != nil {
		t.Fatal(err)
	}
	rs := apiCall(t,
		Request{Op: "preset", Preset: "A1200", Settings: &s},
		Request{Op: "save", Name: "Test", Settings: &s},
		Request{Op: "load", Name: "Test"},
		Request{Op: "choose", Target: "hdf", Path: "/tmp/a.hdf", Settings: &s},
		Request{Op: "rom", Settings: &s},
		Request{Op: "start", Name: "Test", Settings: &s},
		Request{Op: "save", Name: "bad/name", Settings: &s},
		Request{Op: "delete", Name: "Test"},
	)
	for i, r := range rs[:6] {
		if !r.OK {
			t.Fatalf("request %d: %s", i, r.Error)
		}
	}
	if rs[0].Settings.Chipset != "aga" || rs[0].Settings.ROM != s.ROM {
		t.Errorf("A1200 preset: %+v", rs[0].Settings)
	}
	if len(rs[1].Saved) != 1 || rs[1].Saved[0] != "Test" {
		t.Errorf("saved: %v", rs[1].Saved)
	}
	if rs[2].Settings.ROM != s.ROM || rs[2].Name == nil || *rs[2].Name != "Test" {
		t.Errorf("load: %+v", rs[2])
	}
	if h := rs[3].Settings.Hardfiles; len(h) != 1 || h[0] != "/tmp/a.hdf" {
		t.Errorf("choose hdf: %v", h)
	}
	if !strings.Contains(rs[4].ROM.Description, "3.1") || rs[4].ROM.Mismatch != "" {
		t.Errorf("rom: %+v", rs[4].ROM)
	}
	if got, err := config.Load(rs[5].Config); err != nil || got.ROM != s.ROM {
		t.Errorf("start wrote %s: %v %+v", rs[5].Config, err, got)
	}
	if config.LastUsed() != "Test" {
		t.Errorf("last used %q", config.LastUsed())
	}
	if rs[6].OK || rs[6].Error == "" {
		t.Errorf("a bad name saved: %+v", rs[6])
	}
	if !rs[7].OK || len(config.Saved()) != 0 {
		t.Errorf("delete: %+v", rs[7])
	}

	// Lists are [] rather than null, for the Swift side's decoding.
	var out bytes.Buffer
	Serve(strings.NewReader(`{"op":"defaults"}`+"\n"), &out)
	if strings.Contains(out.String(), "null") {
		t.Errorf("defaults have nulls: %s", out.String())
	}

	// Starting needs a ROM.
	if r := apiCall(t, Request{Op: "start"})[0]; r.OK {
		t.Error("started with no ROM")
	}
}

// TestHandleJSONKeys checks the answers' JSON keys, as the Swift screens
// decode them (macos/Sources/BlitterBoxLauncher/Bridge.swift's Response):
// the Go names are not on the wire, so a rename must not change these.
func TestHandleJSONKeys(t *testing.T) {
	var got map[string]json.RawMessage
	if err := json.Unmarshal(HandleJSON([]byte(`{"op":"info"}`)), &got); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"ok", "presets", "limits"} {
		if _, ok := got[k]; !ok {
			t.Errorf(`info's answer has no %q: %v`, k, got)
		}
	}
	if err := json.Unmarshal(HandleJSON([]byte(`{"op":"defaults"}`)), &got); err != nil {
		t.Fatal(err)
	}
	var s map[string]json.RawMessage
	if err := json.Unmarshal(got["settings"], &s); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"rom", "drives", "hardfiles", "cpu", "chipRAM", "fastRAM", "motherboard"} {
		if _, ok := s[k]; !ok {
			t.Errorf(`defaults' settings have no %q`, k)
		}
	}
}

// TestSetDir: with BlitterBox's folder set (the iPad's, where ~/.config
// can't be written), configurations are saved there.
func TestSetDir(t *testing.T) {
	t.Setenv("HOME", "/nonexistent")
	dir := filepath.Join(t.TempDir(), "BlitterBox")
	config.SetDir(dir)
	defer config.SetDir("")
	r := apiCall(t, Request{Op: "save", Name: "A4000", Settings: &config.Settings{}})[0]
	if r.Error != "" {
		t.Fatal(r.Error)
	}
	if _, err := os.Stat(filepath.Join(dir, "configs", "A4000.conf")); err != nil {
		t.Fatal(err)
	}
}

func TestMatchPreset(t *testing.T) {
	for _, p := range Presets {
		s := config.Defaults()
		p.Apply(&s)
		if got := MatchPreset(s); got != p.Name {
			t.Errorf("%s settings: MatchPreset = %q", p.Name, got)
		}
	}
	s := config.Defaults()
	Presets[len(Presets)-1].Apply(&s) // A4000
	s.Z3RAM, s.RTG = 65536, 16
	if got := MatchPreset(s); got != "A4000" {
		t.Errorf("an A4000 with more memory: MatchPreset = %q", got)
	}
	s.CPU = "68030"
	if got := MatchPreset(s); got != "" {
		t.Errorf("an A4000 with a 68030: MatchPreset = %q", got)
	}
}

func TestMatchPresetDefaults(t *testing.T) {
	if got := MatchPreset(config.Defaults()); got == "" {
		t.Error("the default settings match no model")
	}
}

func TestSummarize(t *testing.T) {
	s := config.Defaults()
	Presets[len(Presets)-1].Apply(&s) // A4000
	if got := Summarize(s); got.Model != "A4000" || got.Line != "A4000" {
		t.Errorf("an A4000 with no ROM: %+v", got)
	}
	s.CPU = "68030"
	s.WHDLoad = "game.lha"
	if got := Summarize(s); got.Model != "" || got.Line != "68030\u00a0· WHDLoad" {
		t.Errorf("no model: %+v", got)
	}
	if rom := "../../ROMS/Kick13.rom"; fileExists(rom) {
		s = config.Defaults()
		s.ROM = rom
		if got := Summarize(s).Line; !strings.HasSuffix(got, "\u00a0· Kickstart\u00a01.3") {
			t.Errorf("with Kickstart 1.3: %q", got)
		}
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// newHardfile makes a blank disk and adds it to the hardfiles; it won't
// make one over a file already there.
func TestNewHardfile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Blank 100 MB.hdf")
	r := Handle(Request{Op: "newHardfile", Path: p, MB: 100})
	if !r.OK || r.Settings == nil || len(r.Settings.Hardfiles) != 1 || r.Settings.Hardfiles[0] != p {
		t.Fatalf("response %+v", r)
	}
	if h, err := amiga.OpenHardfile(p, true, 0, 0); err != nil || !h.HasRDB() {
		t.Errorf("the new hardfile: %v", err)
	} else {
		h.Close()
	}
	if r := Handle(Request{Op: "newHardfile", Path: p, MB: 100}); r.OK || r.Error == "" {
		t.Error("made a hardfile over one already there")
	}
}
