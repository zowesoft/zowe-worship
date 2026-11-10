package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"zowe-worship/internal/settings"
	"zowe-worship/internal/speech"
)

// speechProgressEvent exists so a ~140 MB download shows movement instead of a
// frozen button.
const speechProgressEvent = "speech:progress"

// speechConfigEvent refreshes both windows when an engine is installed or
// removed, without an app restart.
const speechConfigEvent = "speech:config"

var (
	errEngineUnsupported  = errors.New("no runtime is available for this platform")
	errEngineNotInstalled = errors.New("that engine is not installed yet")
	errInstallInProgress  = errors.New("another engine download is already running")
)

// SpeechEngineView is one row in the Settings engine list.
type SpeechEngineView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
	// Bundled means the files come from beside the app, not a download.
	Bundled bool `json:"bundled"`
	Active  bool `json:"active"`
	// Supported is false when no runtime exists for this platform.
	Supported bool `json:"supported"`
}

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

// InstallSpeechEngine returns once the download starts, so the window stays
// responsive; the outcome arrives as speech:config.
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

func (a *App) emitSpeechProgress(view SpeechProgressView) {
	a.dispatchToWindow(settingsWindow, speechProgressEvent, view)
}

func (a *App) finishSpeechInstall(id string, err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	data := map[string]any{"engineId": id, "ok": err == nil, "error": message}
	a.dispatchToWindow(settingsWindow, speechConfigEvent, data)
	a.dispatchToWindow(studioWindow, speechConfigEvent, data)
}

// dispatchToWindow is best-effort: a missing window means nobody is listening.
func (a *App) dispatchToWindow(window, name string, data any) {
	desktop := application.Get()
	if desktop == nil {
		return
	}
	if target, ok := desktop.Window.GetByName(window); ok {
		target.DispatchWailsEvent(&application.CustomEvent{Name: name, Data: data})
	}
}

// SelectSpeechEngine looks under both roots, so the operator never handles a
// path.
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

// SpeechComponentView is one redistributed component and its license.
type SpeechComponentView struct {
	Name     string `json:"name"`
	License  string `json:"license"`
	Homepage string `json:"homepage"`
	Text     string `json:"text"`
}

// SpeechAttributionView is the same text as the shipped notices file, plus its
// location so an operator can open it.
type SpeechAttributionView struct {
	Components  []SpeechComponentView `json:"components"`
	NoticesPath string                `json:"noticesPath"`
}

// GetSpeechAttribution surfaces the licenses in Settings so nobody has to hunt
// through the install folder.
func (a *App) GetSpeechAttribution() SpeechAttributionView {
	components := speech.Components()
	views := make([]SpeechComponentView, 0, len(components))
	for _, c := range components {
		views = append(views, SpeechComponentView{
			Name:     c.Name,
			License:  c.License,
			Homepage: c.Homepage,
			Text:     c.Text,
		})
	}

	// The notices file lands beside the engine in whichever root holds it.
	layout := a.speechLayout()
	noticesPath := filepath.Join(layout.BundledRoot, "THIRD-PARTY-NOTICES.txt")
	if _, err := os.Stat(noticesPath); err != nil {
		noticesPath = filepath.Join(layout.DownloadRoot, "THIRD-PARTY-NOTICES.txt")
		if _, err := os.Stat(noticesPath); err != nil {
			noticesPath = ""
		}
	}

	return SpeechAttributionView{Components: views, NoticesPath: noticesPath}
}

// OpenPath is how an operator reaches the notices without knowing the install
// path.
func (a *App) OpenPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("no path to open")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	return application.Get().Env.OpenFileManager(path, false)
}

func (a *App) speechLayout() speech.Layout {
	layout := speech.Layout{}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		layout.BundledRoot = filepath.Join(dir, "engines")
		// Linux packages install the binary to /usr/local/bin and the engine
		// tree to /usr/local/lib/zowe-worship/engines (FHS); fall back to the
		// library location when nothing sits beside the executable.
		if _, err := os.Stat(layout.BundledRoot); err != nil {
			lib := filepath.Join(dir, "..", "lib", "zowe-worship", "engines")
			if info, err := os.Stat(lib); err == nil && info.IsDir() {
				layout.BundledRoot = lib
			}
		}
	}
	if paths, err := a.paths(); err == nil {
		layout.DownloadRoot = filepath.Join(paths.Root, "engines")
	}
	return layout
}
