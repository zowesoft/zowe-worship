package media

import (
	"bytes"
	"context"
	"errors"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"zowe-worship/internal/database"
)

func newTestLibrary(t *testing.T) (*Library, context.Context) {
	t.Helper()

	root := filepath.Join(t.TempDir(), "Zowe Worship")
	paths := database.Paths{Root: root}
	if err := paths.Ensure(); err != nil {
		t.Fatalf("prepare directories: %v", err)
	}
	db, err := database.Open(context.Background(), paths.CoreDatabase())
	if err != nil {
		t.Fatalf("open core database: %v", err)
	}
	if err := database.Migrate(context.Background(), db, paths.Backups()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo := NewSQLiteRepository(db)
	return NewLibrary(repo, paths.Media(), paths.MediaThumbnails()), context.Background()
}

func writeTestImage(t *testing.T, dir, name string, width, height int) string {
	t.Helper()

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			canvas.Set(x, y, color.NRGBA{R: 30, G: uint8(x % 200), B: 120, A: 255})
		}
	}

	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	defer file.Close()

	if filepath.Ext(name) == ".png" {
		if err := png.Encode(file, canvas); err != nil {
			t.Fatalf("encode png: %v", err)
		}
	} else if err := jpeg.Encode(file, canvas, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return path
}

func TestImportImagesBuildsThumbnails(t *testing.T) {
	library, ctx := newTestLibrary(t)
	source := t.TempDir()

	imported, problems, err := library.Import(ctx, []string{
		writeTestImage(t, source, "upper room.jpg", 1600, 900),
		writeTestImage(t, source, "welcome.png", 500, 400),
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %+v", problems)
	}
	if len(imported) != 2 {
		t.Fatalf("imported = %d, want 2", len(imported))
	}

	first := imported[0]
	if first.Kind != KindImage || first.Width != 1600 || first.Height != 900 {
		t.Fatalf("first asset = %+v", first)
	}
	if !first.HasThumbnail {
		t.Fatal("no thumbnail was recorded")
	}
	if _, err := os.Stat(library.Path(first)); err != nil {
		t.Fatalf("library copy missing: %v", err)
	}
	thumb, err := os.Stat(library.ThumbnailPath(first.ID))
	if err != nil {
		t.Fatalf("thumbnail missing: %v", err)
	}
	if thumb.Size() == 0 {
		t.Fatal("thumbnail is empty")
	}
	if first.Filename == "upper room.jpg" {
		t.Fatal("the library kept the original name instead of its own")
	}

	list, err := library.repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 || list[0].ID != imported[1].ID {
		t.Fatalf("newest first ordering broken: %+v", list)
	}
}

func TestImportRejectsDuplicatesAndUnsupportedFiles(t *testing.T) {
	library, ctx := newTestLibrary(t)
	source := t.TempDir()

	original := writeTestImage(t, source, "cross.jpg", 800, 600)
	if _, _, err := library.Import(ctx, []string{original}); err != nil {
		t.Fatalf("first import: %v", err)
	}

	copy := filepath.Join(source, "cross v2 FINAL.jpg")
	if data, err := os.ReadFile(original); err != nil {
		t.Fatalf("read fixture: %v", err)
	} else if err := os.WriteFile(copy, data, 0o600); err != nil {
		t.Fatalf("write copy: %v", err)
	}

	notes := filepath.Join(source, "reading.txt")
	if err := os.WriteFile(notes, []byte("not media"), 0o600); err != nil {
		t.Fatalf("write notes: %v", err)
	}

	imported, problems, err := library.Import(ctx, []string{copy, notes})
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if len(imported) != 0 {
		t.Fatalf("imported = %+v, want none", imported)
	}
	if len(problems) != 2 {
		t.Fatalf("problems = %+v, want two", problems)
	}
	if !problems[0].Duplicate || problems[0].Reason != "already in the library" {
		t.Fatalf("duplicate problem = %+v", problems[0])
	}
	if problems[1].Duplicate || problems[1].Reason != "unsupported format" {
		t.Fatalf("unsupported problem = %+v", problems[1])
	}

	list, err := library.repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("library holds %d assets, want 1", len(list))
	}
}

func TestDeleteRemovesRecordAndFiles(t *testing.T) {
	library, ctx := newTestLibrary(t)
	source := t.TempDir()

	imported, _, err := library.Import(ctx, []string{writeTestImage(t, source, "altar.png", 640, 480)})
	if err != nil || len(imported) != 1 {
		t.Fatalf("import: %+v %v", imported, err)
	}
	asset := imported[0]

	if err := library.Delete(ctx, asset.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(library.Path(asset)); !os.IsNotExist(err) {
		t.Fatal("the library copy survived deletion")
	}
	if _, err := os.Stat(library.ThumbnailPath(asset.ID)); !os.IsNotExist(err) {
		t.Fatal("the thumbnail survived deletion")
	}
	if _, err := library.repo.ByID(ctx, asset.ID); err != ErrAssetNotFound {
		t.Fatalf("record after delete: %v", err)
	}
	if err := library.Delete(ctx, asset.ID); err != ErrAssetNotFound {
		t.Fatalf("second delete: %v", err)
	}
}

func TestMissingDetectsDeletedLibraryFiles(t *testing.T) {
	library, ctx := newTestLibrary(t)
	source := t.TempDir()

	imported, _, err := library.Import(ctx, []string{writeTestImage(t, source, "lantern.jpg", 300, 300)})
	if err != nil || len(imported) != 1 {
		t.Fatalf("import: %+v %v", imported, err)
	}

	if missing, err := library.Missing(ctx); err != nil || len(missing) != 0 {
		t.Fatalf("missing = %+v err = %v, want none", missing, err)
	}

	if err := os.Remove(library.Path(imported[0])); err != nil {
		t.Fatalf("remove library copy: %v", err)
	}
	missing, err := library.Missing(ctx)
	if err != nil {
		t.Fatalf("missing: %v", err)
	}
	if len(missing) != 1 || missing[0].ID != imported[0].ID {
		t.Fatalf("missing = %+v", missing)
	}
}

func TestDurationProbes(t *testing.T) {
	dir := t.TempDir()

	t.Run("mp4", func(t *testing.T) {
		path := filepath.Join(dir, "clip.mp4")
		if err := os.WriteFile(path, buildTestMP4(1000, 9500), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if got := durationMS(path, KindVideo); got != 9500 {
			t.Fatalf("mp4 duration = %d, want 9500", got)
		}
	})

	t.Run("wav", func(t *testing.T) {
		path := filepath.Join(dir, "track.wav")
		if err := os.WriteFile(path, buildTestWAV(44100, 2, 16, 44100*2*2), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if got := durationMS(path, KindAudio); got != 1000 {
			t.Fatalf("wav duration = %d, want 1000", got)
		}
	})

	t.Run("unprobeable", func(t *testing.T) {
		path := filepath.Join(dir, "loop.webm")
		if err := os.WriteFile(path, []byte("not really a webm"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if got := durationMS(path, KindVideo); got != 0 {
			t.Fatalf("duration = %d, want 0", got)
		}
	})
}

// buildTestMP4 writes the smallest container the walker should accept: an ftyp
// box followed by moov > mvhd carrying a timescale and duration.
func buildTestMP4(timescale, duration uint32) []byte {
	mvhd := make([]byte, 32)
	binary.BigEndian.PutUint32(mvhd[12:16], timescale)
	binary.BigEndian.PutUint32(mvhd[16:20], duration)

	box := func(name string, body []byte) []byte {
		out := make([]byte, 8+len(body))
		binary.BigEndian.PutUint32(out[0:4], uint32(len(out)))
		copy(out[4:8], name)
		copy(out[8:], body)
		return out
	}

	var out bytes.Buffer
	out.Write(box("ftyp", []byte("isom")))
	out.Write(box("moov", box("mvhd", mvhd)))
	return out.Bytes()
}

func buildTestWAV(sampleRate uint32, channels, bits uint16, dataBytes int) []byte {
	byteRate := sampleRate * uint32(channels) * uint32(bits) / 8

	out := make([]byte, 0, 44+dataBytes)
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(36+dataBytes))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], channels)
	binary.LittleEndian.PutUint32(header[24:28], sampleRate)
	binary.LittleEndian.PutUint32(header[28:32], byteRate)
	binary.LittleEndian.PutUint16(header[32:34], channels*bits/8)
	binary.LittleEndian.PutUint16(header[34:36], bits)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(dataBytes))

	out = append(out, header...)
	return append(out, make([]byte, dataBytes)...)
}

func TestPresentationRoundTripAndDefaults(t *testing.T) {
	library, ctx := newTestLibrary(t)
	source := t.TempDir()

	imported, _, err := library.Import(ctx, []string{writeTestImage(t, source, "frame.jpg", 800, 600)})
	if err != nil || len(imported) != 1 {
		t.Fatalf("import: %+v %v", imported, err)
	}
	asset := imported[0]
	if asset.Aspect != AspectAuto || asset.Fit != FitContain {
		t.Fatalf("a new asset should show all of itself: %+v", asset.Presentation)
	}

	saved, err := library.SetPresentation(ctx, asset.ID, Presentation{Aspect: "16:9", Fit: FitCover})
	if err != nil {
		t.Fatalf("save presentation: %v", err)
	}
	if saved.Ratio == 0 || saved.Fit != FitCover {
		t.Fatalf("reply should carry the resolved ratio: %+v", saved)
	}

	stored, err := library.Slide(ctx, asset.ID)
	if err != nil {
		t.Fatalf("read slide: %v", err)
	}
	if stored.Ratio != saved.Ratio || stored.Fit != saved.Fit {
		t.Fatalf("persisted %+v, saved %+v", stored, saved)
	}

	// An aspect or fit nobody recognises shows the whole picture rather than
	// failing the save or drawing something distorted.
	garbage, err := library.SetPresentation(ctx, asset.ID, Presentation{Aspect: "7:banana", Fit: "stretchy"})
	if err != nil {
		t.Fatalf("unknown values should not fail: %v", err)
	}
	if garbage.Ratio != 0 || garbage.Fit != FitContain {
		t.Fatalf("fallback = %+v", garbage)
	}
}

func TestPresentationRefusesNonImages(t *testing.T) {
	library, ctx := newTestLibrary(t)
	source := t.TempDir()

	path := filepath.Join(source, "clip.mp4")
	if err := os.WriteFile(path, []byte("not really a video"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	imported, _, err := library.Import(ctx, []string{path})
	if err != nil || len(imported) != 1 {
		t.Fatalf("import: %+v %v", imported, err)
	}

	if _, err := library.SetPresentation(ctx, imported[0].ID, FullFrame()); !errors.Is(err, ErrNotImage) {
		t.Fatalf("err = %v, want ErrNotImage", err)
	}
	// Framing is an image idea, but a clip still has to be projectable, and the
	// slide has to say which player the viewer needs.
	slide, err := library.Slide(ctx, imported[0].ID)
	if err != nil {
		t.Fatalf("a video should still resolve to a slide: %v", err)
	}
	if slide.Kind != string(KindVideo) || slide.URL == "" {
		t.Fatalf("slide = %+v", slide)
	}
}

func TestAspectListIsUsable(t *testing.T) {
	seen := map[string]bool{}
	for _, aspect := range Aspects() {
		if aspect.Value == "" || aspect.Label == "" {
			t.Fatalf("incomplete aspect: %+v", aspect)
		}
		if seen[aspect.Value] {
			t.Fatalf("duplicate aspect %s", aspect.Value)
		}
		seen[aspect.Value] = true
		if aspect.Value != AspectAuto && aspect.Value != AspectScreen && aspect.Ratio <= 0 {
			t.Fatalf("aspect %s has no ratio", aspect.Value)
		}
	}
	if !seen["16:9"] || !seen["1:1"] {
		t.Fatalf("expected the common shapes to be offered: %v", seen)
	}
	if !seen[AspectScreen] {
		t.Fatal("operators need a way to fill the whole screen")
	}
}

func TestScreenAspectFillsWithoutAShape(t *testing.T) {
	library, ctx := newTestLibrary(t)
	source := t.TempDir()

	imported, _, err := library.Import(ctx, []string{writeTestImage(t, source, "banner.png", 1000, 500)})
	if err != nil || len(imported) != 1 {
		t.Fatalf("import: %+v %v", imported, err)
	}

	slide, err := library.SetPresentation(ctx, imported[0].ID, Presentation{Aspect: AspectScreen, Fit: FitContain})
	if err != nil {
		t.Fatalf("set screen: %v", err)
	}
	// Filling the screen is its own instruction: it has no ratio, and the edges
	// choice is decided for it so the picture cannot come back letterboxed.
	if slide.Aspect != AspectScreen || slide.Ratio != 0 {
		t.Fatalf("slide = %+v", slide)
	}
}
