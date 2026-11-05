package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"zowe-worship/internal/asr"
	"zowe-worship/internal/bible"
)

// F07 end-to-end over the real engines on disk: a spoken clip goes through the
// same pipeline the studio uses — engine spec from settings -> DetectScripture
// -> candidate queue -> approve (preview only) / dismiss / clear — with a real
// engine and model. Each case is skipped when its binaries or the fixture are
// absent, so the suite still runs on a bare checkout.
func TestDetectionEndToEndWithRealEngines(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	clip := filepath.Join(root, "benchmarks", "smoke-set", "smoke-speech.wav")
	if _, err := os.Stat(clip); err != nil {
		t.Skipf("fixture missing: %s", clip)
	}

	whisperBin := filepath.Join(root, "benchmarks", "tools", "Release", "whisper-cli.exe")
	whisperModel := filepath.Join(root, "benchmarks", "models", "ggml-tiny.en.bin")

	sherpaBin := filepath.Join(root, "benchmarks", "tools",
		"sherpa-onnx-v2.0.1-win-x64-shared-MT-Release", "bin", "sherpa-onnx-offline.exe")
	moonshine := filepath.Join(root, "benchmarks", "models", "moonshine-tiny-en-int8")

	cases := []struct {
		name string
		spec string
		need []string
	}{
		{
			name: "whisper",
			spec: fmt.Sprintf(`whisper="%s" -m "%s" -np -f {audio}`, whisperBin, whisperModel),
			need: []string{whisperBin, whisperModel},
		},
		{
			name: "moonshine",
			spec: fmt.Sprintf("moonshine=%s --moonshine-preprocessor=%s --moonshine-encoder=%s "+
				"--moonshine-uncached-decoder=%s --moonshine-cached-decoder=%s --tokens=%s {audio}",
				sherpaBin,
				filepath.Join(moonshine, "preprocess.onnx"),
				filepath.Join(moonshine, "encode.int8.onnx"),
				filepath.Join(moonshine, "uncached_decode.int8.onnx"),
				filepath.Join(moonshine, "cached_decode.int8.onnx"),
				filepath.Join(moonshine, "tokens.txt"),
			),
			need: []string{
				sherpaBin,
				filepath.Join(moonshine, "preprocess.onnx"),
				filepath.Join(moonshine, "encode.int8.onnx"),
				filepath.Join(moonshine, "uncached_decode.int8.onnx"),
				filepath.Join(moonshine, "cached_decode.int8.onnx"),
				filepath.Join(moonshine, "tokens.txt"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, p := range append(tc.need, clip) {
				if _, err := os.Stat(p); err != nil {
					t.Skipf("fixture missing: %s", p)
				}
			}
			verifyDetectionWithEngine(t, clip, tc.spec)
		})
	}
}

// verifyDetectionWithEngine drives one engine through the whole detection
// pipeline and asserts the reference lands, that approval only previews, and
// that the queue and the F12 switch behave.
func verifyDetectionWithEngine(t *testing.T, clip, spec string) {
	t.Helper()

	app := newTestApp(t)
	installTestBible(t, app)
	if _, err := app.bible.Install(context.Background(), bibleRomans8()); err != nil {
		t.Fatalf("install romans: %v", err)
	}

	if err := app.SetDetectionEngine(spec); err != nil {
		t.Fatalf("set engine: %v", err)
	}
	config, err := app.GetDetectionConfig()
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if config.EngineSpec == "" || config.Disabled {
		t.Fatalf("config = %+v, want a spec and not disabled", config)
	}

	raw, err := os.ReadFile(clip)
	if err != nil {
		t.Fatalf("read clip: %v", err)
	}
	src := filepath.Join(t.TempDir(), "in.wav")
	if err := os.WriteFile(src, raw, 0o644); err != nil {
		t.Fatalf("write clip: %v", err)
	}
	spec16, err := asr.ReadWAV(src)
	if err != nil {
		t.Fatalf("read wav: %v", err)
	}
	// The browser hands the app 16 kHz mono PCM; mirror that here.
	mono := asr.ToMono16k16bit(spec16)

	result, err := app.DetectScripture(mono.Data)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	t.Logf("transcript=%q candidates=%d", result.Transcript, len(result.Candidates))
	if result.Transcript == "" {
		t.Fatal("engine returned an empty transcript")
	}

	// The clip says "Let us turn tonight to Romans 8, verse 28."
	var romans *DetectionView
	for i := range result.Candidates {
		if result.Candidates[i].Reference == "Romans 8:28" {
			romans = &result.Candidates[i]
		}
	}
	if romans == nil {
		t.Fatalf("no Romans 8:28 candidate in %+v", result.Candidates)
	}
	if romans.BookID != 45 || romans.Chapter != 8 || romans.VerseStart != 28 {
		t.Fatalf("candidate geometry = %+v", romans)
	}

	// Approve previews the passage; it must NOT go live on its own.
	state, err := app.ApproveDetection(romans.ID)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if state.Live != nil {
		t.Fatalf("approval put something live: %+v", state.Live)
	}
	remaining, err := app.ListDetections()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, c := range remaining {
		if c.ID == romans.ID {
			t.Fatal("approved candidate still in the queue")
		}
	}

	// Dismiss then clear.
	if _, err := app.DetectScripture(mono.Data); err != nil {
		t.Fatalf("detect again: %v", err)
	}
	again, err := app.ListDetections()
	if err != nil || len(again) == 0 {
		t.Fatalf("second detect produced no candidates: %v", err)
	}
	if _, err := app.DismissDetection(again[0].ID); err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	cleared, err := app.ClearDetections()
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if len(cleared) != 0 {
		t.Fatalf("clear left %d candidates", len(cleared))
	}

	// F12: the emergency switch refuses new detections.
	if err := app.SetDetectionDisabled(true); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := app.DetectScripture(mono.Data); err == nil {
		t.Fatal("detection ran while disabled")
	}
	if err := app.SetDetectionDisabled(false); err != nil {
		t.Fatalf("re-enable: %v", err)
	}
}

// bibleRomans8 installs just the verse the smoke clip names, so Approve can
// preview it through real storage.
func bibleRomans8() bible.TranslationData {
	return bible.TranslationData{
		TranslationMeta: bible.TranslationMeta{
			ID: "kjv", Name: "King James Version", Abbreviation: "KJV", Language: "en", Version: "1.0.0",
		},
		Verses: []bible.Verse{
			{BookID: 45, Chapter: 8, Number: 28, Text: "And we know that all things work together for good to them that love God."},
		},
	}
}
