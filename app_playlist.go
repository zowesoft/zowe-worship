package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"zopresent/internal/database"
	"zopresent/internal/media"
	"zopresent/internal/presentation"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// PlaylistRunEvent reports which playlist is playing and where, so the media
// workspace can show the run without polling - and can clear the badge the
// moment the run finishes or the operator projects something else.
const PlaylistRunEvent = "playlist-run"

type MediaPlaylistView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	MediaIDs []string `json:"mediaIds"`
}

// MediaPlaylistInput is a create-or-update: an empty id makes a new playlist.
type MediaPlaylistInput struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	MediaIDs []string `json:"mediaIds"`
}

// PlaylistRunView is the run in progress. Active is false for the empty
// payload that ends it, so a listener clears rather than invents a position.
type PlaylistRunView struct {
	PlaylistID string `json:"playlistId"`
	Index      int    `json:"index"`
	Length     int    `json:"length"`
	Active     bool   `json:"active"`
}

// playlistRun is a snapshot of the playlist as it started: editing the list
// mid-run changes the stored copy, never the clip the room is watching.
type playlistRun struct {
	id    string
	items []string
	index int
}

func init() {
	application.RegisterEvent[PlaylistRunView](PlaylistRunEvent)
}

func (a *App) ListMediaPlaylists() ([]MediaPlaylistView, error) {
	if a.media == nil {
		return nil, errStorageUnavailable
	}
	return a.playlistViews(a.context())
}

func (a *App) playlistViews(ctx context.Context) ([]MediaPlaylistView, error) {
	playlists, err := a.media.ListPlaylists(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]MediaPlaylistView, 0, len(playlists))
	for _, playlist := range playlists {
		views = append(views, MediaPlaylistView{
			ID:       playlist.ID,
			Name:     playlist.Name,
			MediaIDs: playlist.Items,
		})
	}
	return views, nil
}

// SaveMediaPlaylist creates or rewrites a playlist and reports the stored
// result, so a create knows the id it got without a second read.
func (a *App) SaveMediaPlaylist(input MediaPlaylistInput) (MediaPlaylistView, error) {
	if a.media == nil {
		return MediaPlaylistView{}, errStorageUnavailable
	}
	ctx := a.context()
	playlist := media.Playlist{
		ID:        strings.TrimSpace(input.ID),
		Name:      input.Name,
		Items:     input.MediaIDs,
		CreatedAt: time.Now().UTC(),
	}
	if playlist.ID == "" {
		id, err := database.NewID()
		if err != nil {
			return MediaPlaylistView{}, fmt.Errorf("make a playlist id: %w", err)
		}
		playlist.ID = id
	} else if _, err := a.media.PlaylistByID(ctx, playlist.ID); err != nil {
		// An update has to keep the original creation time and must not
		// resurrect a deleted playlist under an id the operator typed.
		return MediaPlaylistView{}, err
	}
	if err := a.media.SavePlaylist(ctx, playlist); err != nil {
		return MediaPlaylistView{}, err
	}
	stored, err := a.media.PlaylistByID(ctx, playlist.ID)
	if err != nil {
		return MediaPlaylistView{}, err
	}
	return MediaPlaylistView{ID: stored.ID, Name: stored.Name, MediaIDs: stored.Items}, nil
}

func (a *App) DeleteMediaPlaylist(id string) ([]MediaPlaylistView, error) {
	if a.media == nil {
		return nil, errStorageUnavailable
	}
	ctx := a.context()
	if err := a.media.DeletePlaylist(ctx, strings.TrimSpace(id)); err != nil {
		return nil, err
	}
	return a.playlistViews(ctx)
}

// PlayMediaPlaylist starts the run at the first clip. Every file is checked
// up front: refusing at the start is a message the operator can act on,
// where stopping halfway through a run because clip three walked off the
// disk is a service disruption.
func (a *App) PlayMediaPlaylist(id string) (PresentationView, error) {
	if a.media == nil || a.presentation == nil {
		return PresentationView{}, errStorageUnavailable
	}
	ctx := a.context()
	playlist, err := a.media.PlaylistByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return PresentationView{}, err
	}
	if len(playlist.Items) == 0 {
		return PresentationView{}, media.ErrPlaylistEmpty
	}
	for _, mediaID := range playlist.Items {
		asset, err := a.media.ByID(ctx, mediaID)
		if err != nil {
			return PresentationView{}, err
		}
		if !a.media.Exists(asset) {
			return PresentationView{}, fmt.Errorf("%s: %w", asset.Title, media.ErrFileGone)
		}
	}

	a.run = &playlistRun{id: playlist.ID, items: playlist.Items, index: 0}
	view, err := a.projected(a.presentation.ProjectMedia(ctx, playlist.Items[0]))
	if err != nil {
		a.run = nil
		a.emitRun()
		return view, err
	}
	a.emitRun()
	return view, nil
}

// advancePlaylist takes the run to the next clip when the clip that ended is
// its own, and reports whether the run continued: the studio's transport
// hears `media-ended` only when the run is over, because the next clip has
// already started the moment it advances.
func (a *App) advancePlaylist() bool {
	run := a.run
	if run == nil {
		return false
	}
	index := a.liveRunIndex(run)
	switch {
	case index < 0:
		// The live slide is no longer this run's clip; the run ended
		// earlier without notice and nothing should advance now.
		a.stopRun()
		return false
	case index+1 >= len(run.items):
		// The last clip finished. Live keeps it - the wall goes dark only
		// when someone clears it - and the transport hears the end.
		a.stopRun()
		return false
	}

	run.index = index + 1
	if _, err := a.projected(a.presentation.ProjectMedia(a.context(), run.items[run.index])); err != nil {
		// The next clip would not come up (its file went missing, say).
		// Keep the wall on the clip that is already there and end the run.
		a.stopRun()
		return false
	}
	a.emitRun()
	return true
}

// liveRunIndex finds the run clip the wall is showing, or -1 when the wall
// shows something else. Matching is on the media URL the slide was built
// from, which the presentation state already carries.
func (a *App) liveRunIndex(run *playlistRun) int {
	if a.presentation == nil {
		return -1
	}
	live := a.presentation.State().Live
	if live == nil {
		return -1
	}
	for index, mediaID := range run.items {
		if live.Source == media.FileURL(mediaID) {
			return index
		}
	}
	return -1
}

// syncRunToLive follows the wall: a run whose clip is still up keeps going,
// a run whose wall moved to one of its own clips steps to that position
// (the operator drove the list by hand), and a run whose wall shows anything
// else is over - projecting a song must not later treat a stray clip end as
// its cue.
func (a *App) syncRunToLive(state presentation.State) {
	if a.run == nil {
		return
	}
	index := -1
	if state.Live != nil {
		for i, mediaID := range a.run.items {
			if state.Live.Source == media.FileURL(mediaID) {
				index = i
				break
			}
		}
	}
	switch {
	case index < 0:
		a.stopRun()
	case index != a.run.index:
		a.run.index = index
		a.emitRun()
	}
}

// stopRun ends the run without touching the wall: what is live stays live.
func (a *App) stopRun() {
	if a.run == nil {
		return
	}
	a.run = nil
	a.emitRun()
}

func (a *App) emitRun() {
	desktop := application.Get()
	if desktop == nil {
		return
	}
	view := PlaylistRunView{}
	if a.run != nil {
		view = PlaylistRunView{
			PlaylistID: a.run.id,
			Index:      a.run.index,
			Length:     len(a.run.items),
			Active:     true,
		}
	}
	desktop.Event.Emit(PlaylistRunEvent, view)
}
