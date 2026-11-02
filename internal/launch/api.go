// Package launch turns settings into a running Amiga, for every front
// end: Build makes the machine, Presets and ApplyChoice edit settings, and
// the launcher API (Handle) is what a native configuration screen asks of
// Go, so that settings files, presets and Kickstart checks stay here. The
// macOS screen (macos/) talks to goamiga --launcher-api, which Serves one
// JSON request per line on stdin with one JSON line on stdout; the iPad
// app calls HandleJSON in-process.
package launch

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/andrewsjg/goamiga/amiga"
	"github.com/andrewsjg/goamiga/internal/config"
	"github.com/andrewsjg/goamiga/internal/whdload"
)

// Request is one question from a configuration screen; Op says which.
type Request struct {
	Op       string           `json:"op"`
	Name     string           `json:"name,omitempty"`
	Settings *config.Settings `json:"settings,omitempty"`
	Preset   string           `json:"preset,omitempty"`
	Target   string           `json:"target,omitempty"` // choose: what a file is for (ApplyChoice)
	Path     string           `json:"path,omitempty"`
}

// ROMReport is what the chosen Kickstart ROM is, and whether it can run.
type ROMReport struct {
	Description string `json:"description,omitempty"` // "Kickstart 3.2 (47.96)"
	Error       string `json:"error,omitempty"`       // unreadable
	Mismatch    string `json:"mismatch,omitempty"`    // these settings can't run it
	Model       string `json:"model,omitempty"`       // the preset that can (A3000, A4000)
	Version     int    `json:"version"`
}

// WHDLoadReport is what WHDLoad games have to run with.
type WHDLoadReport struct {
	Kicks     []string `json:"kicks"` // Kickstart images beside the ROM, by WHDLoad's names
	Installed bool     `json:"installed"`
	Home      string   `json:"home"`
}

// Response answers a Request.
type Response struct {
	OK       bool             `json:"ok"`
	Error    string           `json:"error,omitempty"`
	Status   string           `json:"status,omitempty"` // a line for the status bar
	Settings *config.Settings `json:"settings,omitempty"`
	Name     *string          `json:"name,omitempty"`
	Saved    []string         `json:"saved,omitempty"`
	Presets  []string         `json:"presets,omitempty"`
	ROM      *ROMReport       `json:"rom,omitempty"`
	WHDLoad  *WHDLoadReport   `json:"whdload,omitempty"`
	Config   string           `json:"config,omitempty"` // start: the settings file to run
	Limits   map[string]int   `json:"limits,omitempty"`
}

// Serve answers requests from r on w until r ends.
func Serve(r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		if _, err := w.Write(append(HandleJSON(sc.Bytes()), '\n')); err != nil {
			return err
		}
	}
	return sc.Err()
}

// HandleJSON answers one request, given and answered as JSON.
func HandleJSON(line []byte) []byte {
	var req Request
	var resp Response
	if err := json.Unmarshal(line, &req); err != nil {
		resp.Error = err.Error()
	} else {
		resp = Handle(req)
	}
	if resp.Settings != nil {
		resp.Settings = jsonSettings(*resp.Settings)
	}
	b, err := json.Marshal(resp)
	if err != nil {
		b, _ = json.Marshal(Response{Error: err.Error()})
	}
	return b
}

// jsonSettings gives empty lists rather than nulls.
func jsonSettings(s config.Settings) *config.Settings {
	for i := range s.Drives {
		if s.Drives[i] == nil {
			s.Drives[i] = config.List{}
		}
	}
	for _, l := range []*config.List{&s.Hardfiles, &s.Dirs, &s.IDE, &s.SCSI} {
		if *l == nil {
			*l = config.List{}
		}
	}
	return &s
}

// Handle answers one request.
func Handle(req Request) (resp Response) {
	s := config.Defaults()
	if req.Settings != nil {
		s = *req.Settings
	}
	fail := func(format string, args ...any) Response {
		return Response{Error: fmt.Sprintf(format, args...)}
	}
	switch req.Op {
	case "startup":
		resp = apiStartup()
	case "info":
		names := make([]string, len(Presets))
		for i, p := range Presets {
			names[i] = p.Name
		}
		resp.Presets = names
		resp.Limits = map[string]int{"hardfiles": amiga.HardfileLimit, "partitions": 64, "scsi": 7}
	case "defaults":
		resp.Settings = &s
		*resp.Settings = config.Defaults()
	case "saved":
		resp.Saved = config.Saved()
	case "load":
		ls, err := config.Load(config.SavedPath(req.Name))
		if err != nil {
			return fail("%v", err)
		}
		resp.Settings, resp.Name = ls, &req.Name
		resp.Status = "Loaded " + req.Name
	case "loadFile": // a running machine's settings, given back (returnEnv)
		ls, err := config.Load(req.Path)
		if err != nil {
			return fail("%v", err)
		}
		resp.Settings = ls
	case "save":
		if err := config.ValidName(req.Name); err != nil {
			return fail("%v", err)
		}
		if err := s.Write(config.SavedPath(req.Name)); err != nil {
			return fail("Saving: %v", err)
		}
		resp.Saved = config.Saved()
		resp.Status = fmt.Sprintf("Saved %s (start it from the command line with --config %q)", req.Name, req.Name)
	case "delete":
		if err := os.Remove(config.SavedPath(req.Name)); err != nil {
			return fail("Deleting: %v", err)
		}
		resp.Saved = config.Saved()
		resp.Status = "Deleted " + req.Name
	case "preset":
		i := slices.IndexFunc(Presets, func(p Preset) bool { return p.Name == req.Preset })
		if i < 0 {
			return fail("no model %q", req.Preset)
		}
		Presets[i].Apply(&s)
		resp.Settings, resp.Status = &s, PresetMessage(req.Preset)
	case "choose":
		ApplyChoice(&s, req.Target, req.Path)
		resp.Settings = &s
	case "rom":
		resp.ROM = apiROMInfo(&s)
	case "whdload":
		home := filepath.Join(config.Dir(), "whdload")
		resp.WHDLoad = &WHDLoadReport{
			Kicks:     slices.Sorted(maps.Keys(whdload.KickImages(whdload.ROMsBeside(s.ROM)))),
			Installed: whdload.Installed(home),
			Home:      home,
		}
		if resp.WHDLoad.Kicks == nil {
			resp.WHDLoad.Kicks = []string{}
		}
	case "start":
		path, err := apiStart(&s, req.Name)
		if err != nil {
			return fail("%v", err)
		}
		resp.Config = path
	default:
		return fail("unknown request %q", req.Op)
	}
	resp.OK = true
	return resp
}

// apiStartup is the configuration to open with: the one used last, else
// the settings file, else the defaults (as startupLauncher).
func apiStartup() Response {
	resp := Response{Saved: config.Saved()}
	name := ""
	resp.Name = &name
	if last := config.LastUsed(); last != "" {
		if s, err := config.Load(config.SavedPath(last)); err == nil {
			resp.Settings, name = s, last
			resp.Status = "Loaded " + last + ", the configuration used last"
			return resp
		}
	}
	if conf := config.Find(""); conf != "" {
		if s, err := config.Load(conf); err == nil {
			resp.Settings = s
			resp.Status = "Settings from " + conf
			return resp
		}
	}
	d := config.Defaults()
	resp.Settings = &d
	if len(resp.Saved) == 0 {
		resp.Status = "Choose a Kickstart ROM on the Machine tab, then Start (and Save, to keep it)"
	}
	return resp
}

func apiROMInfo(s *config.Settings) *ROMReport {
	r := &ROMReport{}
	if s.ROM == "" {
		return r
	}
	data, err := amiga.LoadROM(s.ROM)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	info := amiga.IdentifyROM(data)
	r.Description, r.Version = info.String(), info.Version
	if m := info.Mismatch(s.CPU, s.Motherboard); m != "" {
		r.Mismatch = m
		r.Model = map[string]string{"a3000": "A3000", "a4000": "A4000"}[info.Machine]
	}
	return r
}

// apiStart checks the settings can run and writes them to launch.conf, the
// file the machine is started from.
func apiStart(s *config.Settings, name string) (string, error) {
	if s.ROM == "" {
		return "", errors.New("choose a Kickstart ROM on the Machine tab first")
	}
	if r := apiROMInfo(s); r.Mismatch != "" {
		return "", fmt.Errorf("%s. Click its model on the Machine tab, then Start", r.Mismatch)
	}
	if name != "" && slices.Contains(config.Saved(), name) {
		config.SetLastUsed(name)
	}
	path := filepath.Join(config.Dir(), "launch.conf")
	if err := s.Write(path); err != nil {
		return "", err
	}
	return path, nil
}
