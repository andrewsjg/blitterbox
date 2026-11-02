//go:build !noebitengine

package launch

import (
	"slices"

	"github.com/andrewsjg/blitterbox/amiga"
	"github.com/andrewsjg/blitterbox/internal/config"
)

// NewDisplay makes the machine's display as the settings say, with its
// drives' disks listed for swapping. name is the configuration's, for the
// title. The caller Starts it.
func (m *Machine) NewDisplay(s *config.Settings, name string) *amiga.Display {
	d := amiga.NewDisplay(m.A)
	d.Mute = s.Mute
	d.Scale = s.Scale
	d.StatusBar = s.StatusBar
	d.StartWarp = s.Warp
	d.JoyKeys = s.JoyKeys
	d.JoyAnalog = s.JoyAnalog
	d.Name = name
	d.ScreenshotDir = s.ScreenshotDir
	d.StateDir = s.StateDir
	for n, list := range m.Drives {
		d.Disks[n] = list
		if len(list) > 0 {
			d.DiskIndex[n] = 0
		}
	}
	return d
}

// PowerCycleSettings is s with the disks now in d's drives, for switching
// the machine off and on again (the Power button) as it is.
func (m *Machine) PowerCycleSettings(s config.Settings, d *amiga.Display) config.Settings {
	for n, list := range d.Disks {
		if i := d.DiskIndex[n]; i >= 0 && i < len(list) {
			// The disk in the drive goes in first; the rest stay listed.
			s.Drives[n] = append(config.List{list[i]}, slices.Delete(slices.Clone(list), i, i+1)...)
		} else {
			s.Drives[n] = nil
		}
	}
	// The drives as they are now (DiskIndex is -1 for an empty one).
	if s.NumDrives < len(d.Disks) {
		for n := len(d.Disks) - 1; n > 0; n-- {
			if m.A.Disk.DF(n) != nil {
				s.NumDrives = max(s.NumDrives, n+1)
				break
			}
		}
	}
	return s
}
