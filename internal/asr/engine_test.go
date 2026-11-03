package asr

import (
	"strings"
	"testing"
)

func TestParseEngineSpec(t *testing.T) {
	engine, err := ParseEngineSpec(`whisper-cli="C:\tools\whisper cli.exe" -m model.bin -np -f {audio}`)
	if err != nil {
		t.Fatalf("spec parse failed: %v", err)
	}
	if engine.Name() != "whisper-cli" {
		t.Errorf("name = %q", engine.Name())
	}
	if engine.bin != `C:\tools\whisper cli.exe` {
		t.Errorf("bin = %q", engine.bin)
	}
	want := []string{"-m", "model.bin", "-np", "-f", "{audio}"}
	if len(engine.args) != len(want) {
		t.Fatalf("args = %v", engine.args)
	}
	for i := range want {
		if engine.args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, engine.args[i], want[i])
		}
	}
}

func TestParseEngineSpecUnnamed(t *testing.T) {
	engine, err := ParseEngineSpec("/usr/local/bin/whisper-cli -m m.bin {audio}")
	if err != nil {
		t.Fatalf("spec parse failed: %v", err)
	}
	if !strings.HasPrefix(engine.Name(), "whisper-cli") {
		t.Errorf("default name = %q", engine.Name())
	}
}

func TestCleanTranscriptStripsStampsAndNoise(t *testing.T) {
	raw := "[00:00:00.000 --> 00:00:04.320]  Let us go to Romans 8:28.\n" +
		"[00:00:04.320 --> 00:00:07.000]  Twenty eight is a promise.\n" +
		"whisper_init_state: loaded model\n"
	cleaned := CleanTranscript(raw)
	if strings.Contains(cleaned, "[00:00") {
		t.Errorf("timestamp stamps survived: %q", cleaned)
	}
	if strings.Contains(cleaned, "whisper_init") {
		t.Errorf("engine noise survived: %q", cleaned)
	}
	if !strings.Contains(cleaned, "Romans 8:28") || !strings.Contains(cleaned, "promise") {
		t.Errorf("transcript content lost: %q", cleaned)
	}
}

func TestNormalizeAndTokenize(t *testing.T) {
	// "chapter" and "verse" connectors are removed after number composition;
	// bare digit pairs ("8 28") still resolve to verse spans in the scanner.
	tokens := tokenizeForDetection(normalizeSpokenText(
		"Let’s turn tonight, to Romans chapter 8, verse 28."))
	got := strings.Join(tokens, "|")
	if !strings.Contains(got, "romans|8|28") {
		t.Errorf("tokens = %v", tokens)
	}
}
