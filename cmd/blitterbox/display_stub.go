//go:build noebitengine

package main

import (
	"errors"

	"github.com/andrewsjg/blitterbox/internal/config"
	"github.com/andrewsjg/blitterbox/internal/launch"
)

var errNoDisplay = errors.New("built with -tags noebitengine: no display; use --headless")

// runMachine is unavailable in a build without Ebitengine: use --headless.
func runMachine(*launch.Machine, *config.Settings, string) error { return errNoDisplay }

// runLauncher is unavailable in a build without Ebitengine.
func runLauncher() error { return errNoDisplay }
