package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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
