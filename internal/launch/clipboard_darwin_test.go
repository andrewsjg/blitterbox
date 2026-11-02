//go:build !noebitengine

package launch

import (
	"os"
	"testing"
)

// The Mac's pasteboard: text set comes back, and the change count moves.
// It changes the real clipboard, so it only runs with BLITTERBOX_PASTEBOARD.
func TestMacClipboard(t *testing.T) {
	if os.Getenv("BLITTERBOX_PASTEBOARD") == "" {
		t.Skip("set BLITTERBOX_PASTEBOARD to use the real clipboard")
	}
	c := hostClipboard()
	before, _ := c.Text()
	defer c.SetText(before)
	n := c.ChangeCount()
	c.SetText("BlitterBox café ✓")
	if got, ok := c.Text(); !ok || got != "BlitterBox café ✓" {
		t.Errorf("read back %q, %v", got, ok)
	}
	if c.ChangeCount() == n {
		t.Error("the change count didn't move")
	}
}
