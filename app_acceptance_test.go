package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zowe-worship/internal/media"
)

// wallSlide is the wall's observable content, compared by value: the
// controller hands out frozen snapshots, and the acceptance claim is about
// what the room sees, not which pointer it came from.
type wallSlide struct {
	label     string
	lines     string
	reference string
	source    string
}

func wallOf(t *testing.T, app *App) wallSlide {
	t.Helper()

	state, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("read presentation state: %v", err)
	}
	if state.Live == nil {
		t.Fatal("the wall is empty")
	}
	return wallSlide{
		label:     state.Live.Label,
		lines:     strings.Join(state.Live.Lines, "\n"),
		reference: state.Live.Reference,
		source:    state.Live.Source,
	}
}

func assertWall(t *testing.T, app *App, want wallSlide, after string) {
	t.Helper()

	if got := wallOf(t, app); got != want {
		t.Fatalf("wall changed after %s:\n got %+v\nwant %+v", after, got, want)
	}
}

// TestNavigatingAndEditingLeaveTheWallOnItsSlide is the acceptance evidence
// that operating the studio around a live slide never disturbs it: browsing
// the schedule, previewing other content, rejected commands and even editing
// the song that is currently projected all leave the room on exactly the
// slide it was shown, until something is deliberately sent live.
func TestNavigatingAndEditingLeaveTheWallOnItsSlide(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	library, err := app.CreateSong(NewSongRequest{
		Title:     "Amazing Grace",
		Author:    "John Newton",
		Copyright: "Public domain",
		Sections: []NewSongSectionRequest{
			{Label: "Verse 1", Text: "Amazing grace! how sweet the sound,\nThat saved a wretch like me!"},
			{Label: "Chorus", Text: "My chains are gone, I am free now."},
		},
	})
	if err != nil {
		t.Fatalf("create song: %v", err)
	}
	song := library.Songs[0]
	verse, chorus := song.Sections[0], song.Sections[1]

	if _, err := app.AddSongSectionToService(SongSectionRequest{SongID: song.ID, SectionID: chorus.ID}); err != nil {
		t.Fatalf("add song item: %v", err)
	}
	if _, err := app.AddScriptureToService(ScriptureRequest{
		TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 16,
	}); err != nil {
		t.Fatalf("add scripture item: %v", err)
	}
	source := t.TempDir()
	imported, err := app.ImportMedia([]string{writeMediaFixture(t, source, "cross.png", 320, 240)})
	if err != nil || len(imported.Added) != 1 {
		t.Fatalf("import picture: %+v %v", imported.Added, err)
	}
	imageID := imported.Added[0].ID
	if _, err := app.AddMediaToService(imageID); err != nil {
		t.Fatalf("add media item: %v", err)
	}
	order, err := app.GetServiceOrder()
	if err != nil || len(order.Items) != 3 {
		t.Fatalf("schedule = %+v %v", order, err)
	}

	// The wall starts on the chorus, mid-service.
	if _, err := app.ProjectSongSection(SongSectionRequest{SongID: song.ID, SectionID: chorus.ID}); err != nil {
		t.Fatalf("project chorus: %v", err)
	}
	want := wallOf(t, app)
	if !strings.Contains(want.label, "Chorus") {
		t.Fatalf("wall = %+v, want the chorus projected", want)
	}

	// Every browsing move updates only the preview pane.
	steps := []struct {
		name string
		run  func() (PresentationView, error)
	}{
		{"previewing the schedule's song item", func() (PresentationView, error) {
			return app.PreviewServiceItem(order.Items[0].ID)
		}},
		{"advancing to the next item", app.NextServiceItem},
		{"stepping back to the previous item", app.PreviousServiceItem},
		{"previewing another passage", func() (PresentationView, error) {
			return app.PreviewScripture(ScriptureRequest{TranslationID: "kjv", BookID: 19, Chapter: 23, VerseStart: 1})
		}},
		{"previewing an image", func() (PresentationView, error) { return app.PreviewMedia(imageID) }},
		{"previewing the song's verse", func() (PresentationView, error) {
			return app.PreviewSongSection(SongSectionRequest{SongID: song.ID, SectionID: verse.ID})
		}},
	}
	for _, step := range steps {
		if _, err := step.run(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		assertWall(t, app, want, step.name)
	}

	// A rejected command cannot reach the wall either: the error is the
	// operator's feedback, and the room keeps what it had.
	failing := []struct {
		name string
		run  func() error
	}{
		{"previewing a schedule item that does not exist", func() error {
			_, err := app.PreviewServiceItem("no-such-item")
			return err
		}},
		{"projecting an asset that does not exist", func() error {
			_, err := app.ProjectMedia("no-such-asset")
			return err
		}},
		{"previewing a book the translation does not carry", func() error {
			_, err := app.PreviewScripture(ScriptureRequest{TranslationID: "kjv", BookID: 999, Chapter: 1, VerseStart: 1})
			return err
		}},
	}
	for _, step := range failing {
		if err := step.run(); err == nil {
			t.Fatalf("%s: the bad command was accepted", step.name)
		}
		assertWall(t, app, want, step.name)
	}

	// Editing the song under the wall changes the next preview, never the
	// slide already projected: the operator sees the new words in the pane
	// and pushes them when ready.
	view, err := app.ListSongs("")
	if err != nil || len(view.Songs) != 1 {
		t.Fatalf("list songs: %+v %v", view, err)
	}
	current := view.Songs[0]
	edit := EditSongRequest{
		ID:         current.ID,
		Title:      current.Title,
		Author:     current.Author,
		Copyright:  current.Copyright,
		CCLINumber: current.CCLINumber,
		Source:     current.Source,
		Sections:   make([]SongSectionEdit, 0, len(current.Sections)),
	}
	for _, section := range current.Sections {
		edit.Sections = append(edit.Sections, SongSectionEdit{ID: section.ID, Label: section.Label, Text: section.Text})
	}
	edit.Sections[0].Text = "A brand new line the wall must not show."
	if _, err := app.UpdateSong(edit); err != nil {
		t.Fatalf("edit song: %v", err)
	}
	previewed, err := app.PreviewSongSection(SongSectionRequest{SongID: song.ID, SectionID: verse.ID})
	if err != nil {
		t.Fatalf("preview edited verse: %v", err)
	}
	if previewed.Preview == nil || !strings.Contains(strings.Join(previewed.Preview.Lines, "\n"), "brand new line") {
		t.Fatalf("preview did not pick up the edit: %+v", previewed.Preview)
	}
	assertWall(t, app, want, "editing the projected song")

	// And the wall still moves when someone means it to.
	if _, err := app.SendPreviewLive(); err != nil {
		t.Fatalf("send preview live: %v", err)
	}
	if got := wallOf(t, app); got == want {
		t.Fatal("SendPreviewLive did not move the wall, so the checks above proved nothing")
	}
}

// TestMediaFailuresAreToldAndTheWallSurvives is the acceptance evidence for
// the media failure rows: unusable files are reported rather than raised, a
// library copy that vanished from disk refuses to project instead of blanking
// the room, and a relink puts the slide back in service.
func TestMediaFailuresAreToldAndTheWallSurvives(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)
	source := t.TempDir()

	// An unsupported container is a per-file problem, not a failed import.
	mkv := filepath.Join(source, "sermon.mkv")
	if err := os.WriteFile(mkv, []byte("not really an mkv"), 0o600); err != nil {
		t.Fatalf("write mkv: %v", err)
	}
	result, err := app.ImportMedia([]string{mkv})
	if err != nil {
		t.Fatalf("import unsupported file: %v", err)
	}
	if len(result.Added) != 0 || len(result.Problems) != 1 {
		t.Fatalf("import result = %+v, want no asset and one problem", result)
	}
	if result.Problems[0].Path != mkv || result.Problems[0].Reason == "" {
		t.Fatalf("problem = %+v, want the chosen file named with a reason", result.Problems[0])
	}

	// The wall holds a passage while the picture's file goes missing.
	if _, err := app.ProjectScripture(ScriptureRequest{
		TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 16,
	}); err != nil {
		t.Fatalf("project passage: %v", err)
	}
	want := wallOf(t, app)

	imported, err := app.ImportMedia([]string{writeMediaFixture(t, source, "cross.png", 320, 240)})
	if err != nil || len(imported.Added) != 1 {
		t.Fatalf("import picture: %+v %v", imported.Added, err)
	}
	imageID := imported.Added[0].ID
	asset, err := app.media.ByID(app.context(), imageID)
	if err != nil {
		t.Fatalf("read asset: %v", err)
	}
	if err := os.Remove(app.media.Path(asset)); err != nil {
		t.Fatalf("remove library copy: %v", err)
	}

	// The row survives, the file does not: both surfaces refuse rather than
	// put a broken slide on the wall.
	if _, err := app.ProjectMedia(imageID); !errors.Is(err, media.ErrFileGone) {
		t.Fatalf("project missing file err = %v, want ErrFileGone", err)
	}
	if _, err := app.PreviewMedia(imageID); !errors.Is(err, media.ErrFileGone) {
		t.Fatalf("preview missing file err = %v, want ErrFileGone", err)
	}
	assertWall(t, app, want, "a library copy going missing")

	// Relinking with the file restored puts the slide back in service.
	restored := writeMediaFixture(t, t.TempDir(), "cross.png", 320, 240)
	views, err := app.RelinkMedia(imageID, restored)
	if err != nil {
		t.Fatalf("relink: %v", err)
	}
	for _, view := range views {
		if view.ID == imageID && view.Missing {
			t.Fatalf("asset still marked missing after relink: %+v", view)
		}
	}
	live, err := app.ProjectMedia(imageID)
	if err != nil {
		t.Fatalf("project after relink: %v", err)
	}
	if live.Live == nil || live.Live.Source != "/media/file/"+imageID {
		t.Fatalf("live after relink = %+v, want the picture on the wall", live.Live)
	}
}

// TestDisplayFailuresKeepTheWallUp is the acceptance evidence that display
// trouble never costs the service its slide: with no projector attached -
// the window closed, the machine that hosts it gone - display commands fail
// loudly while the wall keeps its content, and the studio keeps projecting.
func TestDisplayFailuresKeepTheWallUp(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	if _, err := app.ProjectScripture(ScriptureRequest{
		TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 16,
	}); err != nil {
		t.Fatalf("project passage: %v", err)
	}
	want := wallOf(t, app)

	// Display commands report their failure instead of pretending to work.
	if _, err := app.OpenAudienceDisplay("any"); !errors.Is(err, errNoApplication) {
		t.Fatalf("open display err = %v, want errNoApplication", err)
	}
	if _, err := app.ReconnectAudienceDisplay(); !errors.Is(err, errNoApplication) {
		t.Fatalf("reconnect err = %v, want errNoApplication", err)
	}
	assertWall(t, app, want, "display commands failing")

	// Closing the display is a route out, not a command against the wall.
	view, err := app.CloseAudienceDisplay()
	if err != nil {
		t.Fatalf("close display: %v", err)
	}
	if view.Open {
		t.Fatalf("display still open after close: %+v", view)
	}
	assertWall(t, app, want, "the display closing")

	// The studio keeps projecting exactly as before - and still refuses a
	// bad command without touching the room.
	if _, err := app.PreviewScripture(ScriptureRequest{TranslationID: "kjv", BookID: 19, Chapter: 23, VerseStart: 1}); err != nil {
		t.Fatalf("preview psalm: %v", err)
	}
	if _, err := app.SendPreviewLive(); err != nil {
		t.Fatalf("send live with no display: %v", err)
	}
	if got := wallOf(t, app); !strings.Contains(got.reference, "Psalms 23") {
		t.Fatalf("wall = %+v, want the previewed psalm after an explicit send live", got)
	}
	stillThere := wallOf(t, app)
	if _, err := app.PreviewScripture(ScriptureRequest{TranslationID: "kjv", BookID: 999, Chapter: 1, VerseStart: 1}); err == nil {
		t.Fatal("preview of an unknown book succeeded")
	}
	assertWall(t, app, stillThere, "a rejected command with no display attached")
}
