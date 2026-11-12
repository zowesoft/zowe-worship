package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"testing"

	"zowe-worship/internal/asr"
)

// The test binary doubles as a fake engine: invoked with the sentinel argument
// it reads the chunk's WAV and speaks the reference whose number the test
// encoded in the first sample, so one long recording yields several hearings.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "detection-stub-engine" {
		stubDetectionEngineMain()
		return
	}
	os.Exit(m.Run())
}

func stubDetectionEngineMain() {
	spec, err := asr.ReadWAV(os.Args[len(os.Args)-1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	chunk := 0
	if len(spec.Data) >= 2 {
		chunk = int(int16(binary.LittleEndian.Uint16(spec.Data[:2])))
	}
	lines := map[int]string{
		1: "Turn with me to John chapter three verse sixteen.",
		2: "Now the book of Psalms, chapter twenty three verse one.",
		3: "And Hebrews chapter eleven verse one.",
	}
	if line, ok := lines[chunk]; ok {
		fmt.Println("[00:00:00.000 --> 00:00:02.000]  " + line)
	}
}

// A whole recording must queue every passage it names, not just the one in the
// opening 30 seconds: long audio is split into engine-sized chunks and each
// chunk settles its own segment.
func TestDetectScriptureHearsEveryChunkOfALongRecording(t *testing.T) {
	app := newTestApp(t)
	installTestBible(t, app)

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("test binary path: %v", err)
	}
	if err := app.SetDetectionEngine(fmt.Sprintf(`stub="%s" detection-stub-engine {audio}`, exe)); err != nil {
		t.Fatalf("set engine: %v", err)
	}

	// 70 seconds spans three chunks; the first sample of each marks which
	// reference the stub engine should speak for it.
	pcm := make([]byte, 70*detectionAudioRate*2)
	for chunk := 1; chunk <= 3; chunk++ {
		start := (chunk - 1) * maxDetectionAudioBytes
		binary.LittleEndian.PutUint16(pcm[start:start+2], uint16(chunk))
	}

	result, err := app.DetectScripture(pcm)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if result.Segment != 3 {
		t.Errorf("segment = %d, want one per chunk (3)", result.Segment)
	}
	want := map[string]bool{"John 3:16": false, "Psalms 23:1": false, "Hebrews 11:1": false}
	for _, candidate := range result.Candidates {
		if _, ok := want[candidate.Reference]; !ok {
			t.Errorf("unexpected candidate %q", candidate.Reference)
			continue
		}
		want[candidate.Reference] = true
		if !candidate.Finalized {
			t.Errorf("candidate %q is not finalized", candidate.Reference)
		}
	}
	for reference, found := range want {
		if !found {
			t.Errorf("missing candidate %q — a chunk of the recording was never heard", reference)
		}
	}
	for _, phrase := range []string{"John chapter three", "Psalms", "Hebrews chapter eleven"} {
		if !strings.Contains(result.Transcript, phrase) {
			t.Errorf("transcript lost %q: %q", phrase, result.Transcript)
		}
	}
}