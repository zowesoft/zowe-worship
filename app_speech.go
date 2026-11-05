package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"zowe-worship/internal/settings"
	"zowe-worship/internal/speech"
)

// speechProgressEvent streams an engine download to the settings window. A
// model is ~140 MB, so the operator needs to see it moving rather than watch a
// frozen button for a minute.
const speechProgressEvent = "speech:progress"

// speechConfigEvent tells both windows the engine situation changed — an
// installed or removed engine should appear without reopening the app.
const speechConfigEvent = "speech:config"

var (
	errEngineUnsupported  = errors.New("no runtime is available for this platform")
	errEngineNotInstalled = errors.New("that engine is not installed yet")
	errInstallInProgress  = errors.New("another engine download is already running")
)

// SpeechEngineView is one row in the Settings engine list: what it is, whether
// it is already on disk, and whether it is the one currently selected.
type SpeechEngineView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Installed is true when the engine's files are present.
	Installed bool `json:"installed"`
	// Bundled is true when the engine's files come from beside the app rather
	// than a download.
	Bundled bool `json:"bundled"`
	// Active is true when this engine is the one Live Detection uses.
	Active bool `json:"active"`
	// Supported is false when no runtime exists for this machine's platform.
	Supported bool `json:"supported"`
}

// SpeechProgressView is one download update for the Settings progress bar.
type SpeechProgressView struct {
	EngineID   string `json:"engineId"`
	File       string `json:"file"`
	Done       int    `json:"done"`
	Total      int    `json:"total"`
	Bytes      int64  `json:"bytes"`
	TotalBytes int64  `json:"totalBytes"`
}

// speechInstall tracks an install in flight so a second click cannot start a
// competing download.
var speechInstall struct {
	mu      sync.Mutex
	running bool
}

// ListSpeechEngines returns the engine catalogue with the state each row needs:
// installed, bundled, active and platform support.
func (a *App) ListSpeechEngines() ([]SpeechEngineView, error) {
	layout := a.speechLayout()
	active := ""
	if a.settings != nil {
		active = a.settings.GetDefault(a.context(), settings.KeyAsrEngine, "")
	}

	views := make([]SpeechEngineView, 0)
	for _, engine := range speech.Catalog() {
		_, supported := engine.HostRuntime()
		bundled := speech.Present(layout.BundledRoot, engine)
		views = append(views, SpeechEngineView{
			ID:          engine.ID,
			Name:        engine.Name,
			Description: engine.Description,
			Installed:   bundled || speech.Present(layout.DownloadRoot, engine),
			Bundled:     bundled,
			Active:      engine.Matches(active),
			Supported:   supported,
		})
	}
	return views, nil
}

// InstallSpeechEngine downloads an engine into the user's data root and streams
// progress to the settings window. It returns as soon as the download starts so
// the window stays responsive; the outcome arrives as speech:config.
func (a *App) InstallSpeechEngine(id string) error {
	if a.settings == nil {
		return errStorageUnavailable
	}
	engine, err := speech.Find(id)
	if err != nil {
		return err
	}
	if _, supported := engine.HostRuntime(); !supported {
		return errEngineUnsupported
	}

	speechInstall.mu.Lock()
	if speechInstall.running {
		speechInstall.mu.Unlock()
		return errInstallInProgress
	}
	speechInstall.running = true
	speechInstall.mu.Unlock()

	go a.runSpeechInstall(engine)
	return nil
}

// runSpeechInstall does the download off the UI thread and reports the result.
func (a *App) runSpeechInstall(engine speech.Engine) {
	defer func() {
		speechInstall.mu.Lock()
		speechInstall.running = false
		speechInstall.mu.Unlock()
	}()

	layout := a.speechLayout()
	installer := speech.NewInstaller()
	installer.Progress = func(p speech.Progress) {
		a.emitSpeechProgress(SpeechProgressView{
			EngineID:   engine.ID,
			File:       p.File,
			Done:       p.Done,
			Total:      p.Total,
			Bytes:      p.Bytes,
			TotalBytes: p.TotalBytes,
		})
	}

	err := installer.Install(layout.DownloadRoot, engine)
	if err == nil {
		// Point the detector at what we just installed, so "Get" also selects.
		if spec, specErr := engine.Spec(layout.DownloadRoot); specErr == nil {
			err = a.SetDetectionEngine(spec)
		}
	}
	a.finishSpeechInstall(engine.ID, err)
}

// emitSpeechProgress forwards one update to the settings window.
func (a *App) emitSpeechProgress(view SpeechProgressView) {
	a.dispatchToWindow(settingsWindow, speechProgressEvent, view)
}

// finishSpeechInstall reports the outcome and nudges both windows to refresh.
func (a *App) finishSpeechInstall(id string, err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	data := map[string]any{"engineId": id, "ok": err == nil, "error": message}
	a.dispatchToWindow(settingsWindow, speechConfigEvent, data)
	a.dispatchToWindow(studioWindow, speechConfigEvent, data)
}

// dispatchToWindow sends one custom event to a named window, best-effort: a
// window that is not open simply means nobody is listening.
func (a *App) dispatchToWindow(window, name string, data any) {
	desktop := application.Get()
	if desktop == nil {
		return
	}
	if target, ok := desktop.Window.GetByName(window); ok {
		target.DispatchWailsEvent(&application.CustomEvent{Name: name, Data: data})
	}
}

// SelectSpeechEngine makes an already-installed engine the one Live Detection
// uses. It looks the engine up under both roots — the bundled one beside the
// app or a downloaded one — builds the command line and stores it, so the
// operator never handles a path.
func (a *App) SelectSpeechEngine(id string) (string, error) {
	if a.settings == nil {
		return "", errStorageUnavailable
	}
	engine, err := speech.Find(id)
	if err != nil {
		return "", err
	}

	layout := a.speechLayout()
	root := ""
	switch {
	case speech.Present(layout.BundledRoot, engine):
		root = layout.BundledRoot
	case speech.Present(layout.DownloadRoot, engine):
		root = layout.DownloadRoot
	default:
		return "", errEngineNotInstalled
	}

	spec, err := engine.Spec(root)
	if err != nil {
		return "", err
	}
	if err := a.SetDetectionEngine(spec); err != nil {
		return "", err
	}
	return spec, nil
}

// speechLayout is the two engine roots: the bundled, read-only home beside the
// app, and the writable home under the data root.
func (a *App) speechLayout() speech.Layout {
	layout := speech.Layout{}
	if exe, err := os.Executable(); err == nil {
		layout.BundledRoot = filepath.Join(filepath.Dir(exe), "engines")
	}
	if paths, err := a.paths(); err == nil {
		layout.DownloadRoot = filepath.Join(paths.Root, "engines")
	}
	return layout
}
