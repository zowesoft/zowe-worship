//go:build windows

package asr

import (
	"os/exec"
	"syscall"
)

// hideConsole keeps engine sidecars from flashing a console window. The app
// is a windowsgui binary, so Windows would otherwise allocate a brand-new
// console for every console-subprocess transcription run.
const createNoWindow = 0x08000000

func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}
