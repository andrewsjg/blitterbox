package amiga

import "os"

// BLITTERBOX_EXACTBUS=0 runs the tests with the chip bus forecast instead of
// exact timing.
func init() {
	if os.Getenv("BLITTERBOX_EXACTBUS") == "0" {
		defaultExactBus = false
	}
}
