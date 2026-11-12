package main

import "testing"

// A service holds its own working history: what went on the wall while it
// ran, and what the operator decided about the speech they heard in it. One
// service's work never leaks into another's lists.
func TestHistoryBelongsToTheServiceItHappenedIn(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	first, err := app.CreateService("First service")
	if err != nil {
		t.Fatalf("create the first service: %v", err)
	}
	if _, err := app.ProjectScripture(ScriptureRequest{
		TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 16,
	}); err != nil {
		t.Fatalf("project passage: %v", err)
	}

	// The service that was running keeps what happened in it.
	own, err := app.RecentProjections()
	if err != nil {
		t.Fatalf("recent projections: %v", err)
	}
	if len(own) != 1 {
		t.Fatalf("the first service holds %d projections, want its own one", len(own))
	}

	// The next service starts with nothing of its own: the projection
	// belongs to the service before it, not to this one.
	second, err := app.CreateService("Second service")
	if err != nil {
		t.Fatalf("create the second service: %v", err)
	}
	if second.Service == nil || second.Service.Name != "Second service" {
		t.Fatalf("current after create = %+v", second.Service)
	}

	other, err := app.RecentProjections()
	if err != nil {
		t.Fatalf("recent projections in the second service: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("the second service lists %d projections of the first one", len(other))
	}

	// Back to the first service, and its history is where it was left.
	if _, err := app.UseService(first.Service.ID); err != nil {
		t.Fatalf("switch back: %v", err)
	}
	back, err := app.RecentProjections()
	if err != nil {
		t.Fatalf("recent projections after switching back: %v", err)
	}
	if len(back) != 1 {
		t.Fatalf("the first service holds %d projections after switching back, want one", len(back))
	}
}

// The cards the panel shows belong to the service being worked on: hearing
// something in one service does not put its card in another's list.
func TestPendingCardsBelongToTheServiceTheyWereHeardIn(t *testing.T) {
	app := newTestApp(t)

	first, err := app.CreateService("First service")
	if err != nil {
		t.Fatalf("create the first service: %v", err)
	}
	queueDetection(t, app, "John 3:16")

	cards, err := app.ListDetections()
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("the first service lists %d cards, want its own one", len(cards))
	}

	if _, err := app.CreateService("Second service"); err != nil {
		t.Fatalf("create the second service: %v", err)
	}
	other, err := app.ListDetections()
	if err != nil {
		t.Fatalf("list candidates in the second service: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("the second service lists %d cards of the first one", len(other))
	}

	if _, err := app.UseService(first.Service.ID); err != nil {
		t.Fatalf("switch back: %v", err)
	}
	back, err := app.ListDetections()
	if err != nil {
		t.Fatalf("list candidates after switching back: %v", err)
	}
	if len(back) != 1 {
		t.Fatalf("the first service lists %d cards after switching back, want one", len(back))
	}
}

// The decisions the operator took about what they heard belong to the service
// they were taken in.
func TestDetectionDecisionsBelongToTheServiceTheyWereTakenIn(t *testing.T) {
	app := newTestApp(t)

	first, err := app.CreateService("First service")
	if err != nil {
		t.Fatalf("create the first service: %v", err)
	}
	candidate := queueDetection(t, app, "John 3:16")
	if _, err := app.DismissDetection(candidate); err != nil {
		t.Fatalf("dismiss: %v", err)
	}

	own, err := app.ListDetectionHistory(20)
	if err != nil {
		t.Fatalf("detection history: %v", err)
	}
	if len(own) != 1 {
		t.Fatalf("the first service holds %d decisions, want its own one", len(own))
	}

	if _, err := app.CreateService("Second service"); err != nil {
		t.Fatalf("create the second service: %v", err)
	}
	other, err := app.ListDetectionHistory(20)
	if err != nil {
		t.Fatalf("detection history in the second service: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("the second service lists %d decisions of the first one", len(other))
	}

	if _, err := app.UseService(first.Service.ID); err != nil {
		t.Fatalf("switch back: %v", err)
	}
	back, err := app.ListDetectionHistory(20)
	if err != nil {
		t.Fatalf("detection history after switching back: %v", err)
	}
	if len(back) != 1 {
		t.Fatalf("the first service holds %d decisions after switching back, want one", len(back))
	}
}
