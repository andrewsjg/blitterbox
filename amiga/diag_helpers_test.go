package amiga

import (
	"os"
	"testing"
)

// requireDiag skips print-heavy diagnostic tests unless BLITTERBOX_DIAG is set.
// These tests trace boot/rendering internals to stdout rather than assert
// behaviour, so they are opt-in to keep `go test` output readable:
//
//	BLITTERBOX_DIAG=1 go test -run TestHotPCs ./amiga/
func requireDiag(t *testing.T) {
	t.Helper()
	if os.Getenv("BLITTERBOX_DIAG") == "" {
		t.Skip("diagnostic trace; set BLITTERBOX_DIAG=1 to run")
	}
}
