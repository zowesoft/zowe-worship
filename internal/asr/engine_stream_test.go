package asr

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// The test binary doubles as a fake engine: invoked with the sentinel argument
// it prints whisper.cpp-style segments with pauses between them, so the
// streaming reader can be observed mid-run.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "asr-stub-engine" {
		stubEngineMain()
		return
	}
	os.Exit(m.Run())
}

func stubEngineMain() {
	fmt.Println("whisper_init_state: loaded model")
	fmt.Println("[00:00:00.000 --> 00:00:02.000]  Turn with me to Philippians chapter 4.")
	time.Sleep(300 * time.Millisecond)
	fmt.Println("[00:00:02.000 --> 00:00:04.000]  verses 6 and 7.")
	fmt.Println("system_info: stub engine")
}

func TestTranscribeStreamReportsPartialsBeforeFinish(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("test binary path: %v", err)
	}
	engine := &ExecutableEngine{
		name:    "stub",
		bin:     exe,
		args:    []string{"asr-stub-engine"},
		timeout: 30 * time.Second,
	}

	var partials []string
	transcript, err := engine.TranscribeStream("ignored.wav", func(partial string) {
		partials = append(partials, partial)
	})
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}

	if len(partials) != 2 {
		t.Fatalf("partials = %v, want 2 incremental updates", partials)
	}
	if partials[0] != "Turn with me to Philippians chapter 4." {
		t.Errorf("first partial = %q", partials[0])
	}
	if !strings.Contains(partials[1], "verses 6 and 7") {
		t.Errorf("second partial = %q", partials[1])
	}

	if strings.Contains(transcript, "[00:00") || strings.Contains(transcript, "whisper_init") {
		t.Errorf("final transcript kept engine noise: %q", transcript)
	}
	if !strings.Contains(transcript, "Philippians") || !strings.Contains(transcript, "verses 6 and 7") {
		t.Errorf("final transcript lost speech: %q", transcript)
	}
}

func TestSegmentTextOnlyAcceptsStampedSpeech(t *testing.T) {
	if text, ok := segmentText("[00:00:01.000 --> 00:00:02.000]  spoken words"); !ok || text != "spoken words" {
		t.Errorf("segment line = %q, %v", text, ok)
	}
	if _, ok := segmentText("whisper_init_state: loaded"); ok {
		t.Error("engine noise streamed as speech")
	}
	if _, ok := segmentText(""); ok {
		t.Error("blank line streamed as speech")
	}
	if _, ok := segmentText("result: {\"text\": \"sherpa json\"}"); ok {
		t.Error("end-of-run JSON streamed as speech")
	}
}
