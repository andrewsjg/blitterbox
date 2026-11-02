package icons

import "testing"

func TestIcons(t *testing.T) {
	if len(Dock()) == 0 {
		t.Error("no Dock icon")
	}
	if got := len(Window()); got != 4 {
		t.Errorf("%d window icons, want 4", got)
	}
}
