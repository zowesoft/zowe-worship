//go:build !windows

package asr

import "os/exec"

// hideConsole is a no-op off Windows: console subprocesses attach to the
// caller's terminal, which is where their output belongs anyway.
func hideConsole(cmd *exec.Cmd) {}
