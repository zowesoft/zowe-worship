package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"zopresent/internal/settings"
)

// shortcutsConfigEvent tells the studio its operator shortcuts changed. They
// are edited in the Settings window, a different webview, so without this the
// open studio keeps firing the old keys until the app restarts.
const shortcutsConfigEvent = "shortcuts:config"

// ShortcutsView is the operator key map: one combo per action. Combos are
// stored in a canonical "Ctrl+Alt+Shift+Key" form that the frontend builds from
// a KeyboardEvent, so Go only ever stores and compares strings — it never has
// to know what a key looks like.
type ShortcutsView struct {
	SendPreviewLive      string `json:"sendPreviewLive"`
	FocusBibleSearch     string `json:"focusBibleSearch"`
	ToggleListening      string `json:"toggleListening"`
	ToggleAutoProjection string `json:"toggleAutoProjection"`
	EmergencyDisable     string `json:"emergencyDisable"`
	SchedulePrevious     string `json:"schedulePrevious"`
	ScheduleNext         string `json:"scheduleNext"`
}

func defaultShortcuts() ShortcutsView {
	return ShortcutsView{
		SendPreviewLive:      "Ctrl+L",
		FocusBibleSearch:     "Ctrl+F",
		ToggleListening:      "F8",
		ToggleAutoProjection: "F9",
		EmergencyDisable:     "F12",
		SchedulePrevious:     "ArrowLeft",
		ScheduleNext:         "ArrowRight",
	}
}

// GetShortcuts reads the stored key map. Unmarshalling into a defaults-filled
// struct means a blob saved before an action existed still gets that action's
// default, and a missing or corrupt blob falls back to the full default set
// rather than leaving the operator with no keys.
func (a *App) GetShortcuts() (ShortcutsView, error) {
	view := defaultShortcuts()
	if a.settings == nil {
		return view, nil
	}
	raw := a.settings.GetDefault(a.context(), settings.KeyShortcuts, "")
	if strings.TrimSpace(raw) == "" {
		return view, nil
	}
	if err := json.Unmarshal([]byte(raw), &view); err != nil {
		return defaultShortcuts(), nil
	}
	return view, nil
}

// SetShortcuts validates and stores a full key map, then nudges the studio to
// re-read it so the change takes effect without a restart.
func (a *App) SetShortcuts(view ShortcutsView) error {
	if a.settings == nil {
		return errStorageUnavailable
	}
	if err := validateShortcuts(view); err != nil {
		return err
	}
	blob, err := json.Marshal(view)
	if err != nil {
		return fmt.Errorf("encode shortcuts: %w", err)
	}
	if err := a.settings.Set(a.context(), settings.KeyShortcuts, string(blob)); err != nil {
		return err
	}
	a.notifyShortcuts()
	return nil
}

// ResetShortcuts drops the stored map and returns the defaults, so the studio
// picks them straight back up.
func (a *App) ResetShortcuts() (ShortcutsView, error) {
	if a.settings == nil {
		return defaultShortcuts(), nil
	}
	if err := a.settings.Delete(a.context(), settings.KeyShortcuts); err != nil {
		return ShortcutsView{}, err
	}
	a.notifyShortcuts()
	return defaultShortcuts(), nil
}

// validateShortcuts rejects an empty combo and two actions sharing one combo,
// so a saved map always resolves a keypress to a single action.
func validateShortcuts(v ShortcutsView) error {
	entries := []struct{ combo, label string }{
		{v.SendPreviewLive, "Send preview live"},
		{v.FocusBibleSearch, "Focus Bible search"},
		{v.ToggleListening, "Toggle listening"},
		{v.ToggleAutoProjection, "Pause/resume auto-projection"},
		{v.EmergencyDisable, "Emergency disable detection"},
		{v.SchedulePrevious, "Previous schedule item"},
		{v.ScheduleNext, "Next schedule item"},
	}
	seen := make(map[string]string, len(entries))
	for _, entry := range entries {
		combo := strings.TrimSpace(entry.combo)
		if combo == "" {
			return fmt.Errorf("%s needs a key combination", entry.label)
		}
		if other, dup := seen[combo]; dup {
			return fmt.Errorf("%s and %s both use %s", other, entry.label, combo)
		}
		seen[combo] = entry.label
	}
	return nil
}

// notifyShortcuts nudges the studio to re-read its key map. Best-effort: a
// missing window just means nobody is listening.
func (a *App) notifyShortcuts() {
	desktop := application.Get()
	if desktop == nil {
		return
	}
	if studio, ok := desktop.Window.GetByName(studioWindow); ok {
		studio.DispatchWailsEvent(&application.CustomEvent{Name: shortcutsConfigEvent})
	}
}
