package main

import "errors"

// A machine started by a native configuration screen, with returnEnv
// naming a file, writes its settings there when Config (Cmd+,) is chosen
// and exits with exitReturnToLauncher, for the screen to show them again.
// (The screen's questions are answered by launch.Serve, --launcher-api.)
const (
	returnEnv            = "BLITTERBOX_RETURN_CONFIG"
	exitReturnToLauncher = 3
)

var errReturnToLauncher = errors.New("back to the configuration screen")
