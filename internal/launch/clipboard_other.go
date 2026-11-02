//go:build noebitengine || !darwin || ios

package launch

import "github.com/andrewsjg/blitterbox/amiga"

// hostClipboard: clipboard sharing is for macOS for now (not iOS).
func hostClipboard() amiga.HostClipboard { return nil }
