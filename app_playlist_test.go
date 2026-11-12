package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"zopresent/internal/media"
)

// importPlaylistClip imports one placeholder clip and hands back its id.
// The bytes are distinct per name, so each import is its own asset.
func importPlaylistClip(t *testing.T, app *App, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("placeholder bytes of "+name), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	result, err := app.ImportMedia([]string{path})
	if err != nil || len(result.Added) != 1 {
		t.Fatalf("import %s: %+v %v", name, result.Added, err)
	}
	return result.Added[0].ID
}

func TestPlaylistRunsClipByClipUntilTheRunIsOver(t *testing.T) {
	app := newTestApp(t)
	first := importPlaylistClip(t, app, "opening.mp4")
	second := importPlaylistClip(t, app, "hymn.mp3")
	third := importPlaylistClip(t, app, "sermon.mp4")

	playlist, err := app.SaveMediaPlaylist(MediaPlaylistInput{
		Name: "Morning", MediaIDs: []string{first, second, third},
	})
	if err != nil {
		t.Fatalf("save playlist: %v", err)
	}
	if playlist.ID == "" || len(playlist.MediaIDs) != 3 {
		t.Fatalf("saved playlist = %+v", playlist)
	}

	view, err := app.PlayMediaPlaylist(playlist.ID)
	if err != nil {
		t.Fatalf("play: %v", err)
	}
	if view.Live == nil || view.Live.Source != "/media/file/"+first {
		t.Fatalf("live after play = %+v, want the first clip", view.Live)
	}
	if app.run == nil || app.run.index != 0 || app.run.id != playlist.ID {
		t.Fatalf("run = %+v, want the playlist at its first clip", app.run)
	}

	// Each clip's end is the cue for the next one; the wall follows.
	for step, want := range []struct {
		clip  string
		index int
	}{{second, 1}, {third, 2}} {
		app.MediaEnded()
		state, err := app.GetPresentationState()
		if err != nil {
			t.Fatalf("state after step %d: %v", step, err)
		}
		if state.Live == nil || state.Live.Source != "/media/file/"+want.clip {
			t.Fatalf("live after step %d = %+v, want clip %s", step, state.Live, want.clip)
		}
		if app.run == nil || app.run.index != want.index {
			t.Fatalf("run after step %d = %+v, want index %d", step, app.run, want.index)
		}
	}

	// The last clip's end is the end of the run. Live keeps the clip - only
	// a clear empties the wall - and a stray end with no run is no cue.
	app.MediaEnded()
	if app.run != nil {
		t.Fatalf("run outlived its last clip: %+v", app.run)
	}
	state, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("state after the run: %v", err)
	}
	if state.Live == nil || state.Live.Source != "/media/file/"+third {
		t.Fatalf("live after the run = %+v, want the last clip kept", state.Live)
	}
	app.MediaEnded()
}

func TestPlaylistRunEndsTheMomentTheWallShowsSomethingElse(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)
	first := importPlaylistClip(t, app, "processional.mp4")
	second := importPlaylistClip(t, app, "offertory.mp3")

	playlist, err := app.SaveMediaPlaylist(MediaPlaylistInput{
		Name: "Prelude", MediaIDs: []string{first, second},
	})
	if err != nil {
		t.Fatalf("save playlist: %v", err)
	}
	if _, err := app.PlayMediaPlaylist(playlist.ID); err != nil {
		t.Fatalf("play: %v", err)
	}

	// The operator projects a passage by hand: the run is over at once, so
	// the passage's own clip ends (there are none) - or any later stray
	// clip end - can never be mistaken for its cue.
	if _, err := app.applied(app.presentation.ProjectScripture(app.context(), "kjv", 43, 3, 16, 16, false)); err != nil {
		t.Fatalf("project scripture: %v", err)
	}
	if app.run != nil {
		t.Fatalf("run = %+v, want it ended when the wall moved", app.run)
	}
	app.MediaEnded()
	state, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Live == nil || state.Live.Reference != "John 3:16" {
		t.Fatalf("live after a stray clip end = %+v, want the passage kept", state.Live)
	}
}

func TestPlaylistRunFollowsAHandDrivenClipAndStopsWithTransportStop(t *testing.T) {
	app := newTestApp(t)
	first := importPlaylistClip(t, app, "entrance.mp4")
	second := importPlaylistClip(t, app, "anthem.mp3")
	third := importPlaylistClip(t, app, "closing.mp4")

	playlist, err := app.SaveMediaPlaylist(MediaPlaylistInput{
		Name: "Full run", MediaIDs: []string{first, second, third},
	})
	if err != nil {
		t.Fatalf("save playlist: %v", err)
	}
	if _, err := app.PlayMediaPlaylist(playlist.ID); err != nil {
		t.Fatalf("play: %v", err)
	}

	// Driving the list by hand steps the run to that clip rather than
	// ending it: the wall is still showing the run's own material, and the
	// next end should still cue what follows it.
	if _, err := app.ProjectMedia(third); err != nil {
		t.Fatalf("project third clip: %v", err)
	}
	if app.run == nil || app.run.index != 2 {
		t.Fatalf("run = %+v, want it stepped to the clip on the wall", app.run)
	}

	// Stopping the clip stops the run with it: playing the clip again
	// later must not cue the next clip of a run the operator ended.
	if err := app.MediaControl(MediaCommand{Action: "stop"}); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if app.run != nil {
		t.Fatalf("run = %+v, want it ended by stop", app.run)
	}
	app.MediaEnded()
	state, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if state.Live == nil || state.Live.Source != "/media/file/"+third {
		t.Fatalf("live after stop and a stray end = %+v, want the clip kept", state.Live)
	}
}

func TestPlaylistPlayRefusesWhatCannotStart(t *testing.T) {
	app := newTestApp(t)
	source := t.TempDir()

	imageID := ""
	if result, err := app.ImportMedia([]string{writeMediaFixture(t, source, "cross.png", 80, 60)}); err != nil || len(result.Added) != 1 {
		t.Fatalf("import picture: %+v %v", result.Added, err)
	} else {
		imageID = result.Added[0].ID
	}
	clipID := importPlaylistClip(t, app, "clip.mp4")

	// An image can never go into a playlist: it never ends, so the run
	// would strand on it.
	if _, err := app.SaveMediaPlaylist(MediaPlaylistInput{
		Name: "Stills", MediaIDs: []string{imageID},
	}); !errors.Is(err, media.ErrPlaylistImage) {
		t.Fatalf("image in playlist err = %v, want ErrPlaylistImage", err)
	}

	// A playlist with nothing in it says so rather than going live blank.
	empty, err := app.SaveMediaPlaylist(MediaPlaylistInput{Name: "Empty"})
	if err != nil {
		t.Fatalf("save empty playlist: %v", err)
	}
	if _, err := app.PlayMediaPlaylist(empty.ID); !errors.Is(err, media.ErrPlaylistEmpty) {
		t.Fatalf("play empty err = %v, want ErrPlaylistEmpty", err)
	}

	// A clip whose library copy walked off the disk is refused before the
	// run starts, not halfway through the service.
	full, err := app.SaveMediaPlaylist(MediaPlaylistInput{
		Name: "One clip", MediaIDs: []string{clipID},
	})
	if err != nil {
		t.Fatalf("save playlist: %v", err)
	}
	assets, err := app.media.List(app.context())
	if err != nil {
		t.Fatalf("list library: %v", err)
	}
	for _, asset := range assets {
		if asset.ID != clipID {
			continue
		}
		if err := os.Remove(app.media.Path(asset)); err != nil {
			t.Fatalf("remove library copy: %v", err)
		}
	}
	if _, err := app.PlayMediaPlaylist(full.ID); !errors.Is(err, media.ErrFileGone) {
		t.Fatalf("play with the file gone err = %v, want ErrFileGone", err)
	}
	if app.run != nil {
		t.Fatalf("a refused play started a run: %+v", app.run)
	}
}
