// Package goamiga is goAmiga for iPadOS: ebitenmobile binds it into an
// xcframework (scripts/ios-framework.sh) that the app in ios/ hosts.
//
// The app's settings screen asks Request (the launcher API, as the macOS
// one asks goamiga --launcher-api), then Starts the machine from the file
// "start" wrote. Paths in settings are relative to the app's Documents
// folder (SetDocuments), which the Files app shows, so they survive the
// app moving container on an update.
package goamiga

import (
	"fmt"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/mobile"

	"github.com/andrewsjg/goamiga/amiga"
	"github.com/andrewsjg/goamiga/internal/config"
	"github.com/andrewsjg/goamiga/internal/launch"
)

// Host is the app, for what the machine asks of it. Its methods are
// called on the emulation's thread and must not call back into this
// package before returning.
type Host interface {
	// SettingsRequested: the status bar's Config button switched the
	// machine off, to change its settings.
	SettingsRequested()
	// ChooseDisk asks for a disk image for DFunit: (a drive's light was
	// tapped); the app answers with DiskChosen.
	ChooseDisk(unit int)
}

// game is the running machine and its display, or nothing (black) while
// the settings screen shows. Ebitengine calls Update, Draw and Layout on
// its own thread; the app calls in on its main thread, hence the lock.
type game struct {
	mu   sync.Mutex
	m    *launch.Machine
	disp *amiga.Display
	s    config.Settings // the running machine's settings
	name string          // and its configuration's name, if saved
	host Host
}

var (
	g         = &game{}
	diskReply = make(chan string, 1) // DiskChosen's answer to ChooseDisk
)

func init() {
	mobile.SetGame(g)
}

// SetDocuments makes dir, the app's Documents folder, where relative
// paths in settings are found.
func SetDocuments(dir string) error {
	return os.Chdir(dir)
}

// SetHost gives the machine the app to ask for things.
func SetHost(h Host) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.host = h
}

// Request answers one launcher API request (JSON, see launch.Handle).
func Request(json string) string {
	return string(launch.HandleJSON([]byte(json)))
}

// Start builds the machine from the settings file the launcher API's
// "start" wrote and runs it, in place of any running one. name is the
// configuration's, if saved.
func Start(configPath, name string) error {
	s, err := config.Load(configPath)
	if err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stop()
	return g.start(*s, name)
}

// Stop switches the machine off.
func Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stop()
}

// Key presses (down) or releases an Amiga key by its raw code, for the
// app's on-screen keyboard. A hardware keyboard needs no help: Ebitengine
// reports its keys to the display like a desktop one.
func Key(code int, down bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.m != nil && g.m.A.Keyboard != nil {
		g.m.A.Keyboard.EnqueueKey(byte(code), down)
	}
}

// DiskChosen answers ChooseDisk: the disk image's path, or "" if none
// was chosen.
func DiskChosen(path string) {
	select {
	case diskReply <- path:
	default: // nobody asked
	}
}

// start builds and runs a machine from s. g.mu is held.
func (g *game) start(s config.Settings, name string) error {
	m, err := launch.Build(&s, "", nil, os.Stdout)
	if err != nil {
		return err
	}
	s.Scale = 1 // the window is the screen
	d := m.NewDisplay(&s, name)
	d.ChooseDiskFile = g.chooseDisk
	fmt.Printf("Starting display (%.2f fps %s)...\n", m.A.FrameRate(), m.A.VideoStandard())
	g.m, g.disp, g.s, g.name = m, d, s, name
	d.Start()
	return nil
}

// stop switches the machine off. g.mu is held.
func (g *game) stop() {
	if g.disp != nil {
		g.disp.Stop()
	}
	if g.m != nil {
		g.m.Close()
	}
	g.m, g.disp = nil, nil
}

// chooseDisk is the display's disk dialog: the app's picker, waited for
// on the display's chooser goroutine.
func (g *game) chooseDisk(unit int, _ string) (string, error) {
	g.mu.Lock()
	h := g.host
	g.mu.Unlock()
	if h == nil {
		return "", nil
	}
	select {
	case <-diskReply: // a stale answer
	default:
	}
	h.ChooseDisk(unit)
	return <-diskReply, nil
}

func (g *game) Update() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp == nil {
		return nil
	}
	if err := g.disp.Update(); err != nil {
		return err
	}
	switch {
	case g.disp.PowerRequested:
		s := g.m.PowerCycleSettings(g.s, g.disp)
		g.stop()
		if err := g.start(s, g.name); err != nil {
			fmt.Printf("Can't switch it on again: %v\n", err)
			g.settingsRequested()
		}
	case g.disp.ConfigRequested:
		g.stop()
		g.settingsRequested()
	}
	return nil
}

func (g *game) settingsRequested() {
	if g.host != nil {
		g.host.SettingsRequested()
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.Draw(screen)
	}
}

// DrawFinalScreen scales the picture the way the desktop window does.
func (g *game) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.DrawFinalScreen(screen, offscreen, geoM)
		return
	}
	screen.DrawImage(offscreen, &ebiten.DrawImageOptions{GeoM: geoM})
}

func (g *game) Layout(w, h int) (int, int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp == nil {
		return w, h
	}
	return g.disp.Layout(w, h)
}
