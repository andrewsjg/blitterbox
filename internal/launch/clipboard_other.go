//go:build noebitengine || !darwin || ios

package launch

import "github.com/andrewsjg/goamiga/amiga"

// hostClipboard: clipboard sharing is for macOS for now (not iOS).
func hostClipboard() amiga.HostClipboard { return nil }
