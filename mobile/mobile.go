// Package blitterbox is BlitterBox for iPadOS: ebitenmobile binds it into an
// xcframework (scripts/ios-framework.sh) that the app in ios/ hosts.
//
// The app's settings screen asks Request (the launcher API, as the macOS
// one asks blitterbox --launcher-api), then Starts the machine from the file
// "start" wrote. Paths in settings are relative to the app's Documents
// folder (SetDocuments), which the Files app shows, so they survive the
// app moving container on an update.
package blitterbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/mobile"

	"github.com/andrewsjg/blitterbox/amiga"
	"github.com/andrewsjg/blitterbox/internal/config"
	"github.com/andrewsjg/blitterbox/internal/launch"
)

// Host is the app, for what the machine asks of it. Its methods are
// called on the emulation's thread and must not call back into this
// package before returning.
type Host interface {
	// SettingsRequested: the machine was switched off (ChangeSettings),
	// to change its settings.
	SettingsRequested()
	// ChooseDisk asks for a disk image for DFunit: (InsertDisk); the app
	// answers with DiskChosen.
	ChooseDisk(unit int)
	// PointerLock: the pointer was trapped (locked: hide and hold it) or
	// freed.
	PointerLock(locked bool)
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
// paths in settings are found; settings files keep paths there relative
// to it, as iOS can move the app's container. BlitterBox's own files (saved
// configurations, WHDLoad's) go in the app's Library beside it: there is
// no writable ~/.config on iOS.
func SetDocuments(dir string) error {
	config.SetBase(dir)
	support := filepath.Join(filepath.Dir(dir), "Library", "Application Support")
	own := filepath.Join(support, "BlitterBox")
	config.MoveOld(filepath.Join(support, "goAmiga"), own)
	config.SetDir(own)
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
// reports its keys to the display like a desktop one, once the app has
// made its view controller first responder.
func Key(code int, down bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.m != nil && g.m.A.Keyboard != nil {
		g.m.A.Keyboard.EnqueueKey(byte(code), down)
	}
}

// Touch gives the machine a finger's event: phase 0 down, 1 moved, 2 up
// or cancelled, at x, y points in Ebitengine's view; id tells fingers
// apart. The app sends every touch here rather than to Ebitengine, which
// misses a tap begun and ended between two frames.
func Touch(id, phase int, x, y float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.Touch(id, phase, x, y)
	}
}

// The app's trackpad or mouse (GCMouse: Ebitengine sees only a pointer's
// clicks, as touches) works as the desktop's mouse does: free, it clicks
// the status bar, and a click anywhere else traps it, for the Amiga, till
// Esc or F12. Host.PointerLock hears which.

// PointerClick: the pointer pressed (down) or released, with its right
// button or not, at x, y points in Ebitengine's view.
func PointerClick(x, y float64, right, down bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.PointerClick(x, y, right, down)
	}
}

// PointerMove moves the Amiga's mouse by dx, dy points, while trapped.
func PointerMove(dx, dy float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.PointerMove(dx, dy)
	}
}

// PointerButton presses (down) or releases the pointer's button: 0 left,
// 1 right, 2 middle. The Amiga's, while trapped.
func PointerButton(button int, down bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.PointerButton(button, down)
	}
}

// JoystickStick moves the on-screen joystick's stick to x, y: -1 to 1
// each way, y down, 0, 0 centred. It works as a gamepad's stick does,
// in game port 2.
func JoystickStick(x, y float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.TouchStick(x, y)
	}
}

// JoystickButton presses (down) or releases the on-screen joystick's
// fire button (0) or its second button (1).
func JoystickButton(button int, down bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.TouchButton(button, down)
	}
}

// Say shows a note in the status bar (Status's message).
func Say(text string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.Say(text)
	}
}

// The app's own status bar: what to show (Status), and its buttons.

// Status is the machine's status bar as JSON (amiga.Status): its
// activity lights and message. "" while the machine is off.
func Status() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp == nil {
		return ""
	}
	b, err := json.Marshal(g.disp.Status())
	if err != nil {
		return ""
	}
	return string(b)
}

// Reset is the keyboard reset: Kickstart restarts.
func Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.Reset()
	}
}

// ToggleWarp turns warp mode on or off: the machine runs as fast as it
// can, silent.
func ToggleWarp() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.ToggleWarp()
	}
}

// PowerCycle switches the machine off and on again, the disks still in.
func PowerCycle() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.PowerRequested = true
	}
}

// ChangeSettings switches the machine off for its settings to be changed
// (Host.SettingsRequested follows).
func ChangeSettings() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.ConfigRequested = true
	}
}

// InsertDisk asks for a disk for DFunit: (Host.ChooseDisk).
func InsertDisk(unit int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.InsertDisk(unit)
	}
}

// Eject takes the disk out of DFunit:.
func Eject(unit int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.disp != nil {
		g.disp.Eject(unit)
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
	s.Scale = 1         // the window is the screen
	s.StatusBar = false // the app draws its own (Status)
	d := m.NewDisplay(&s, name)
	d.ChooseDiskFile = g.chooseDisk
	d.GrabChanged = g.pointerLock
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

// pointerLock tells the app the pointer was trapped or freed. g.mu is
// held.
func (g *game) pointerLock(locked bool) {
	if g.host != nil {
		g.host.PointerLock(locked)
	}
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
