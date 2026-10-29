package main

import (
	"testing"
)

func TestSongLibraryLyricSlidesAndSchedule(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	if _, err := app.CreateSong(NewSongRequest{Title: "No sections"}); err == nil {
		t.Fatal("song without sections was accepted")
	}

	library, err := app.CreateSong(NewSongRequest{
		Title:     "Amazing Grace",
		Author:    "John Newton",
		Copyright: "Public domain",
		Sections: []NewSongSectionRequest{
			{Label: "Verse 1", Text: "Amazing grace! how sweet the sound,\nThat saved a wretch like me!"},
			{Label: "Chorus", Text: "My chains are gone, and I am free."},
		},
	})
	if err != nil {
		t.Fatalf("create song: %v", err)
	}
	if len(library.Songs) != 1 || len(library.Songs[0].Sections) != 2 {
		t.Fatalf("library = %+v", library.Songs)
	}
	songID := library.Songs[0].ID
	chorus := library.Songs[0].Sections[1]

	hits, err := app.ListSongs("chains gone")
	if err != nil {
		t.Fatalf("search lyrics: %v", err)
	}
	if len(hits.Hits) != 1 || hits.Hits[0].SongTitle != "Amazing Grace" || hits.Hits[0].Label != "Chorus" {
		t.Fatalf("hits = %+v", hits.Hits)
	}

	state, err := app.PreviewSongSection(SongSectionRequest{SongID: songID, SectionID: chorus.ID})
	if err != nil {
		t.Fatalf("preview song: %v", err)
	}
	if state.Preview == nil || state.Preview.Kind != "song" || state.Preview.Label != "Amazing Grace · Chorus" {
		t.Fatalf("preview = %+v", state.Preview)
	}
	if len(state.Preview.Lines) != 1 || state.Live != nil {
		t.Fatalf("state after song preview = %+v", state)
	}

	state, err = app.ProjectSongSection(SongSectionRequest{SongID: songID, SectionID: chorus.ID})
	if err != nil {
		t.Fatalf("project song: %v", err)
	}
	if state.Live == nil || state.Live.Label != "Amazing Grace · Chorus" {
		t.Fatalf("live = %+v", state.Live)
	}

	order, err := app.AddSongSectionToService(SongSectionRequest{SongID: songID, SectionID: chorus.ID})
	if err != nil {
		t.Fatalf("add song to service: %v", err)
	}
	if len(order.Items) != 1 || order.Items[0].Kind != "song" || order.Items[0].SongID != songID {
		t.Fatalf("schedule = %+v", order.Items)
	}

	order, err = app.AddScriptureToService(ScriptureRequest{TranslationID: "kjv", BookID: 43, Chapter: 3, VerseStart: 16})
	if err != nil {
		t.Fatalf("add scripture to service: %v", err)
	}
	if len(order.Items) != 2 || order.Items[1].Kind != "scripture" {
		t.Fatalf("mixed schedule = %+v", order.Items)
	}

	state, err = app.PreviewServiceItem(order.Items[0].ID)
	if err != nil {
		t.Fatalf("preview song item: %v", err)
	}
	if state.Preview.Kind != "song" {
		t.Fatalf("expected a song slide, got %+v", state.Preview)
	}
	state, err = app.NextServiceItem()
	if err != nil {
		t.Fatalf("next item: %v", err)
	}
	if state.Preview.Kind != "scripture" || state.Preview.Reference != "John 3:16" {
		t.Fatalf("next did not move to scripture: %+v", state.Preview)
	}

	remaining, err := app.DeleteSong(songID)
	if err != nil {
		t.Fatalf("delete song: %v", err)
	}
	if len(remaining.Songs) != 0 {
		t.Fatalf("songs remain: %+v", remaining.Songs)
	}
	if _, err := app.PreviewSongSection(SongSectionRequest{SongID: songID, SectionID: chorus.ID}); err == nil {
		t.Fatal("deleted song still previews")
	}
}
