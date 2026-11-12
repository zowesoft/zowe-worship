package main

import (
	"encoding/base64"
	"encoding/binary"
	"strings"

	"zowe-worship/internal/audio"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The audio service's application events. The frontend listens by these
// names, so they are constants rather than strings scattered over the code.
const (
	// AudioStartedEvent says a capture is running, and names the device that
	// was actually opened.
	AudioStartedEvent = "audio.input.started"

	// AudioHealthEvent says the state of the stream changed: ok, stalled,
	// failed or stopped.
	AudioHealthEvent = "audio.input.health_changed"

	// AudioLevelEvent carries the readings the segmenter lives on. It is not
	// one of the PRD's two audio events, but the PRD's AudioService owns the
	// levels and the panel must not poll for them.
	AudioLevelEvent = "audio.input.level"
)

// audioService returns the capture service, building it on first use. Startup
// wires it, so this only builds one for a headless run or a test.
func (a *App) audioService() *audio.Service {
	a.audioMu.Lock()
	defer a.audioMu.Unlock()
	if a.audio == nil {
		a.wireAudio()
	}
	return a.audio
}

// wireAudio builds the capture service and points its three publishers at the
// PRD's events, so the events fire from the backend rather than from the
// webview.
func (a *App) wireAudio() {
	service := audio.New()
	service.OnStarted(func(started audio.Started) { a.emitAudio(AudioStartedEvent, started) })
	service.OnHealth(func(health audio.Health) { a.emitAudio(AudioHealthEvent, health) })
	service.OnLevel(func(features audio.Features) { a.emitAudio(AudioLevelEvent, features) })
	a.audio = service
}

// emitAudio publishes one audio event to every window. A missing desktop (as
// in a test) is not an error: there is simply nobody listening.
func (a *App) emitAudio(name string, data any) {
	desktop := application.Get()
	if desktop == nil {
		return
	}
	desktop.Event.Emit(name, data)
}

// AudioInputs lists the capture devices the operator can choose from, with
// the default marked.
func (a *App) AudioInputs() ([]audio.Device, error) {
	return a.audioService().Inputs()
}

// AudioStart opens the input device - the default when the id is empty - and
// begins publishing levels and health. It emits audio.input.started with the
// device that was actually opened, which is not always the one that was
// asked for.
func (a *App) AudioStart(deviceID string) error {
	return a.audioService().Start(strings.TrimSpace(deviceID))
}

// AudioStop ends the capture. The buffered audio stays readable, so a
// segment that closed just before the operator stopped the microphone can
// still be transcribed.
func (a *App) AudioStop() error {
	a.audioService().Stop()
	return nil
}

// AudioTakeLast returns the last seconds of captured audio as base64 PCM, the
// encoding the bridge and DetectScripture speak.
func (a *App) AudioTakeLast(seconds float64) string {
	return encodePCM(a.audioService().TakeLast(seconds))
}

// AudioTakeRange returns the absolute sample span a segment closed over, as
// base64 PCM. A span that has already wrapped out of the ring comes back
// clamped to what is still held.
func (a *App) AudioTakeRange(from, to int64) string {
	return encodePCM(a.audioService().TakeRange(from, to))
}

// encodePCM turns 16-bit samples into the base64 the bridge hands over.
func encodePCM(samples []int16) string {
	if len(samples) == 0 {
		return ""
	}
	raw := make([]byte, len(samples)*2)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(raw[i*2:], uint16(sample))
	}
	return base64.StdEncoding.EncodeToString(raw)
}
