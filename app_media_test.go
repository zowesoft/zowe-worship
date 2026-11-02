package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"zowe-worship/internal/media"
)

func writeMediaFixture(t *testing.T, dir, name string, width, height int) string {
	t.Helper()

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			canvas.Set(x, y, color.NRGBA{R: 200, G: uint8(y % 250), B: 90, A: 255})
		}
	}

	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, canvas); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return path
}

func TestMediaImportListingAndDeletion(t *testing.T) {
	app := newTestApp(t)
	source := t.TempDir()

	empty, err := app.ListMedia()
	if err != nil {
		t.Fatalf("list before import: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("library started non-empty: %+v", empty)
	}

	result, err := app.ImportMedia([]string{
		writeMediaFixture(t, source, "stage.png", 1200, 800),
		filepath.Join(source, "notes.txt"),
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(result.Added) != 1 {
		t.Fatalf("added = %+v", result.Added)
	}
	if len(result.Problems) != 1 || result.Problems[0].Reason != "unsupported format" {
		t.Fatalf("problems = %+v", result.Problems)
	}

	added := result.Added[0]
	if added.Kind != "image" || added.Width != 1200 || added.Height != 800 {
		t.Fatalf("added view = %+v", added)
	}
	if added.Missing {
		t.Fatal("a freshly imported asset reports itself missing")
	}
	if added.URL != "/media/file/"+added.ID {
		t.Fatalf("url = %s", added.URL)
	}

	again, err := app.ImportMedia([]string{filepath.Join(source, "stage.png")})
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if len(again.Added) != 0 || len(again.Problems) != 1 || !again.Problems[0].Duplicate {
		t.Fatalf("duplicate import = %+v / %+v", again.Added, again.Problems)
	}

	list, err := app.ListMedia()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("library = %+v, want one asset", list)
	}

	remaining, err := app.DeleteMedia(added.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("library after delete = %+v", remaining)
	}
}

func TestServeMediaOnlyResolvesKnownIDs(t *testing.T) {
	app := newTestApp(t)
	source := t.TempDir()

	result, err := app.ImportMedia([]string{writeMediaFixture(t, source, "cross.png", 600, 400)})
	if err != nil || len(result.Added) != 1 {
		t.Fatalf("import: %+v %v", result.Added, err)
	}
	id := result.Added[0].ID

	response := httptest.NewRecorder()
	app.serveMedia(response, httptest.NewRequest(http.MethodGet, "/media/file/"+id, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("content type = %s", got)
	}
	if _, err := png.Decode(bytes.NewReader(response.Body.Bytes())); err != nil {
		t.Fatalf("served bytes are not the image: %v", err)
	}

	rangeResponse := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/media/file/"+id, nil)
	request.Header.Set("Range", "bytes=0-99")
	app.serveMedia(rangeResponse, request)
	if rangeResponse.Code != http.StatusPartialContent {
		t.Fatalf("range status = %d, want 206", rangeResponse.Code)
	}
	if rangeResponse.Header().Get("Content-Range") == "" {
		t.Fatal("range response has no Content-Range, so video seeking would break")
	}
	if rangeResponse.Body.Len() != 100 {
		t.Fatalf("range body = %d bytes, want 100", rangeResponse.Body.Len())
	}

	thumb := httptest.NewRecorder()
	app.serveMedia(thumb, httptest.NewRequest(http.MethodGet, "/media/thumb/"+id, nil))
	if thumb.Code != http.StatusOK || thumb.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("thumbnail = %d %s", thumb.Code, thumb.Header().Get("Content-Type"))
	}

	// A request cannot name a path: anything that is not a library id is refused
	// before the filesystem is consulted.
	for _, path := range []string{
		"/media/file/nope",
		"/media/file/../../windows/win.ini",
		"/media/thumb/0000000000000000000000000000000f",
	} {
		recorder := httptest.NewRecorder()
		app.serveMedia(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s served with status %d", path, recorder.Code)
		}
	}
}

func TestMediaSlidesFlowThroughScheduleAndLive(t *testing.T) {
	app := newTestApp(t)
	source := t.TempDir()

	imported, err := app.ImportMedia([]string{writeMediaFixture(t, source, "cross.png", 800, 600)})
	if err != nil || len(imported.Added) != 1 {
		t.Fatalf("import: %+v %v", imported.Added, err)
	}
	id := imported.Added[0].ID

	preview, err := app.PreviewMedia(id)
	if err != nil {
		t.Fatalf("preview media: %v", err)
	}
	if preview.Preview == nil || preview.Preview.Kind != "image" || preview.Preview.Source != "/media/file/"+id {
		t.Fatalf("preview = %+v", preview.Preview)
	}
	if preview.Live != nil {
		t.Fatal("previewing an image sent it live")
	}

	live, err := app.ProjectMedia(id)
	if err != nil {
		t.Fatalf("project media: %v", err)
	}
	if live.Live == nil || live.Live.Label != "cross" {
		t.Fatalf("live = %+v", live.Live)
	}

	order, err := app.AddMediaToService(id)
	if err != nil {
		t.Fatalf("add to service: %v", err)
	}
	if len(order.Items) != 1 {
		t.Fatalf("schedule = %+v", order.Items)
	}
	item := order.Items[0]
	if item.Kind != "media" || item.MediaID != id || item.Title != "cross" {
		t.Fatalf("schedule item = %+v", item)
	}

	// Advancing the schedule has to rebuild the same slide from the reference.
	if _, err := app.ClearLive(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	fromItem, err := app.PreviewServiceItem(item.ID)
	if err != nil {
		t.Fatalf("preview schedule item: %v", err)
	}
	if fromItem.Preview == nil || fromItem.Preview.Source != "/media/file/"+id {
		t.Fatalf("preview from schedule = %+v", fromItem.Preview)
	}

	// Removing the asset out from under a live slide must not blank the room.
	if _, err := app.SendPreviewLive(); err != nil {
		t.Fatalf("send live: %v", err)
	}
	if _, err := app.DeleteMedia(id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := app.ProjectMedia(id); err == nil {
		t.Fatal("projecting a deleted asset succeeded")
	}
	after, err := app.GetPresentationState()
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if after.Live == nil || after.Live.Source != "/media/file/"+id {
		t.Fatalf("live output was not preserved: %+v", after.Live)
	}
}

func TestVideoAndAudioProjectAsSlides(t *testing.T) {
	app := newTestApp(t)
	source := t.TempDir()

	write := func(name string) string {
		path := filepath.Join(source, name)
		if err := os.WriteFile(path, []byte("placeholder bytes of "+name), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return path
	}

	imported, err := app.ImportMedia([]string{write("sermon.mp4"), write("anthem.mp3")})
	if err != nil || len(imported.Added) != 2 {
		t.Fatalf("import: %+v %v", imported.Added, err)
	}

	ids := map[string]string{}
	for _, asset := range imported.Added {
		ids[asset.Kind] = asset.ID
	}
	videoID, audioID := ids["video"], ids["audio"]
	if videoID == "" || audioID == "" {
		t.Fatalf("both kinds should be imported: %+v", imported.Added)
	}

	video, err := app.ProjectMedia(videoID)
	if err != nil {
		t.Fatalf("project video: %v", err)
	}
	if video.Live == nil || video.Live.Kind != "video" || video.Live.Source != "/media/file/"+videoID {
		t.Fatalf("live video = %+v", video.Live)
	}

	audio, err := app.PreviewMedia(audioID)
	if err != nil {
		t.Fatalf("preview audio: %v", err)
	}
	if audio.Preview == nil || audio.Preview.Kind != "audio" {
		t.Fatalf("preview audio = %+v", audio.Preview)
	}

	// Framing is an image idea, so a clip is refused rather than opened blank.
	if _, err := app.OpenMediaEditor(videoID); !errors.Is(err, media.ErrNotImage) {
		t.Fatalf("editor err = %v, want ErrNotImage", err)
	}
}
