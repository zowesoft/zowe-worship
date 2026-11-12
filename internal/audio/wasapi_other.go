//go:build !windows

package audio

import "errors"

// enumerateInputs has no WASAPI off Windows. The app is a Windows build; the
// stub keeps the package compiling so tests and tooling can still run there.
func enumerateInputs() ([]Device, error) {
	return nil, errors.New("audio capture is only implemented on Windows")
}

// openDevice is the non-Windows counterpart of the WASAPI capture loop.
func openDevice(deviceID string, sink sink) (stream, Device, error) {
	return nil, Device{}, errors.New("audio capture is only implemented on Windows")
}
