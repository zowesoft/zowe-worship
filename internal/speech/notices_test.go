package speech

import (
	"strings"
	"testing"
)

// The app redistributes three separate licensed pieces; every one must be
// listed with real license text, or the shipped notices would be incomplete.
func TestComponentsCoverEveryRedistributedPiece(t *testing.T) {
	want := map[string]string{
		"sherpa-onnx":       "Apache-2.0",
		"ONNX Runtime":      "MIT",
		"Moonshine (models)": "MIT",
	}

	got := map[string]string{}
	for _, c := range Components() {
		got[c.Name] = c.License
		if len(strings.TrimSpace(c.Text)) < 500 {
			t.Errorf("component %q has suspiciously short license text (%d bytes)", c.Name, len(c.Text))
		}
		if c.Homepage == "" {
			t.Errorf("component %q has no homepage", c.Name)
		}
	}

	for name, license := range want {
		if got[name] != license {
			t.Errorf("component %q license = %q, want %q", name, got[name], license)
		}
	}
	if len(got) != len(want) {
		t.Errorf("components = %d, want %d: %v", len(got), len(want), got)
	}
}

func TestNoticesIncludeEachComponentAndItsFullText(t *testing.T) {
	notices := Notices()

	for _, c := range Components() {
		if !strings.Contains(notices, c.Name) {
			t.Errorf("notices missing component name %q", c.Name)
		}
		if !strings.Contains(notices, c.License) {
			t.Errorf("notices missing license name %q", c.License)
		}
		// The whole license must be present, not just a header line.
		marker := strings.TrimSpace(strings.SplitN(c.Text, "\n", 2)[0])
		if marker != "" && !strings.Contains(notices, marker) {
			t.Errorf("notices missing license text for %q (no %q)", c.Name, marker)
		}
	}

	// The Apache text's operative clause must survive the assembly.
	if !strings.Contains(notices, "Licensed under the Apache License, Version 2.0") {
		t.Error("notices missing the Apache-2.0 grant clause")
	}
}
