package main

import (
	"testing"

	"zopresent/internal/asr"
	"zopresent/internal/detection"
	"zopresent/internal/scripture"
)

// autoReadyApp has a Bible installed and auto-projection switched on and
// resumed, so a test only has to break the one gate it is about.
func autoReadyApp(t *testing.T) *App {
	t.Helper()

	app := newTestApp(t)
	installTestBible(t, app)
	if err := app.SetAutoProjection(true); err != nil {
		t.Fatalf("enable auto-projection: %v", err)
	}
	if err := app.SetAutoProjectionPaused(false); err != nil {
		t.Fatalf("resume auto-projection: %v", err)
	}
	return app
}

func autoCandidate(t *testing.T, app *App, transcript, text string) detection.Candidate {
	t.Helper()

	reference, err := scripture.Parse(text)
	if err != nil {
		t.Fatalf("parse %q: %v", text, err)
	}
	added, err := app.detection.Add(transcript, []asr.Detection{{Reference: reference, Raw: text}}, app.currentServiceID(app.context()))
	if err != nil {
		t.Fatalf("queue %q: %v", text, err)
	}
	if len(added) != 1 {
		t.Fatalf("queue %q added %d candidates, want 1", text, len(added))
	}
	return added[0]
}

func autoHistoryDecision(t *testing.T, app *App, id string) string {
	t.Helper()

	var decision string
	if err := app.core.QueryRow(`SELECT decision FROM detection_history WHERE id = ?`, id).Scan(&decision); err != nil {
		t.Fatalf("read history for %s: %v", id, err)
	}
	return decision
}

func TestAutoProjectionProjectsWhenEveryGatePasses(t *testing.T) {
	app := autoReadyApp(t)
	candidate := autoCandidate(t, app, "Turn with me to John 3:16", "John 3:16")

	projected := app.autoProject([]detection.Candidate{candidate})
	if len(projected) != 1 || projected[0] != "John 3:16" {
		t.Fatalf("projected = %v, want John 3:16", projected)
	}

	state, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("presentation state: %v", err)
	}
	if state.Live == nil || state.Live.Reference != "John 3:16" {
		t.Fatalf("live = %+v, want John 3:16 on air", state.Live)
	}
	if pending := app.detection.List(app.currentServiceID(app.context())); len(pending) != 0 {
		t.Fatalf("queue holds %d cards, want the projected one removed", len(pending))
	}
	if decision := autoHistoryDecision(t, app, candidate.ID); decision != "auto_projected" {
		t.Errorf("history decision = %q, want auto_projected", decision)
	}
}

func TestAutoProjectionDefaultsOffAndPaused(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	config, err := app.GetDetectionConfig()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if config.AutoProjection {
		t.Error("auto-projection is enabled on a fresh install; it must be opt-in")
	}
	if !config.AutoProjectionPaused {
		t.Error("a fresh start must boot paused so nothing projects unwatched")
	}
}

func TestAutoProjectionGatesKeepCandidatesQueued(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(t *testing.T, app *App) []detection.Candidate
	}{
		{"switched off", func(t *testing.T, app *App) []detection.Candidate {
			if err := app.SetAutoProjection(false); err != nil {
				t.Fatalf("switch off: %v", err)
			}
			return []detection.Candidate{autoCandidate(t, app, "John 3:16", "John 3:16")}
		}},
		{"paused", func(t *testing.T, app *App) []detection.Candidate {
			if err := app.SetAutoProjectionPaused(true); err != nil {
				t.Fatalf("pause: %v", err)
			}
			return []detection.Candidate{autoCandidate(t, app, "John 3:16", "John 3:16")}
		}},
		{"emergency disable", func(t *testing.T, app *App) []detection.Candidate {
			if err := app.SetDetectionDisabled(true); err != nil {
				t.Fatalf("disable detection: %v", err)
			}
			return []detection.Candidate{autoCandidate(t, app, "John 3:16", "John 3:16")}
		}},
		{"medium confidence", func(t *testing.T, app *App) []detection.Candidate {
			// Bare numbers after the book name are the weakest explicit form.
			return []detection.Candidate{autoCandidate(t, app, "John three sixteen", "John 3:16")}
		}},
		{"passage missing from translation", func(t *testing.T, app *App) []detection.Candidate {
			return []detection.Candidate{autoCandidate(t, app, "John 3:99", "John 3:99")}
		}},
		{"repeated hearing", func(t *testing.T, app *App) []detection.Candidate {
			candidate := autoCandidate(t, app, "John 3:16", "John 3:16")
			candidate.Repeats = 1
			return []detection.Candidate{candidate}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := autoReadyApp(t)
			candidates := tc.prepare(t, app)

			if projected := app.autoProject(candidates); len(projected) != 0 {
				t.Fatalf("projected = %v, want the gate to keep it queued", projected)
			}
			if pending := app.detection.List(app.currentServiceID(app.context())); len(pending) == 0 {
				t.Fatal("the blocked candidate left the queue; the operator would never see it")
			}
			state, err := app.GetPresentationState()
			if err != nil {
				t.Fatalf("presentation state: %v", err)
			}
			if state.Live != nil {
				t.Fatalf("live = %+v, want the display untouched", state.Live)
			}
		})
	}
}

func TestAutoProjectionRefusesCompetingCandidates(t *testing.T) {
	app := autoReadyApp(t)

	john, err := scripture.Parse("John 3:16")
	if err != nil {
		t.Fatalf("parse John: %v", err)
	}
	romans, err := scripture.Parse("Romans 8:28")
	if err != nil {
		t.Fatalf("parse Romans: %v", err)
	}
	added, err := app.detection.Add("John 3:16 and Romans 8:28", []asr.Detection{
		{Reference: john, Raw: "John 3:16"},
		{Reference: romans, Raw: "Romans 8:28"},
	}, app.currentServiceID(app.context()))
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	if projected := app.autoProject(added); len(projected) != 0 {
		t.Fatalf("projected = %v, want two competing passages left for the operator", projected)
	}
	if pending := app.detection.List(app.currentServiceID(app.context())); len(pending) != 2 {
		t.Fatalf("queue holds %d cards, want both kept", len(pending))
	}
}

func TestAutoProjectionFailurePreservesLiveSlide(t *testing.T) {
	app := autoReadyApp(t)

	app.autoProject([]detection.Candidate{autoCandidate(t, app, "John 3:16", "John 3:16")})
	before, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("presentation state: %v", err)
	}
	if before.Live == nil {
		t.Fatal("setup did not project a slide")
	}

	// A verse that does not exist fails the passage gate and never reaches the
	// controller, so the wall keeps showing what it showed.
	missing := autoCandidate(t, app, "John 3:99", "John 3:99")
	if projected := app.autoProject([]detection.Candidate{missing}); len(projected) != 0 {
		t.Fatalf("projected = %v, want the missing passage refused", projected)
	}
	after, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("presentation state: %v", err)
	}
	if after.Live == nil || after.Live.Reference != before.Live.Reference {
		t.Fatalf("live changed from %+v to %+v", before.Live, after.Live)
	}
}
