package asr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeModelDir creates a sherpa-onnx model directory with the given files.
func writeModelDir(t *testing.T, dir string, files ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", file, err)
		}
	}
}

func TestDiscoverEnginesFindsWhisperAndSherpa(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "tools", "Release")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir tools: %v", err)
	}
	for _, bin := range []string{"whisper-cli.exe", "sherpa-onnx-offline.exe", "sherpa-onnx-offline-audio-tagging.exe"} {
		if err := os.WriteFile(filepath.Join(binDir, bin), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", bin, err)
		}
	}

	modelsDir := filepath.Join(root, "models")
	writeModelDir(t, filepath.Join(modelsDir, "parakeet-tdt-0.6b-v3-int8"),
		"encoder.int8.onnx", "decoder.int8.onnx", "joiner.int8.onnx", "tokens.txt")
	writeModelDir(t, filepath.Join(modelsDir, "moonshine-tiny-en-int8"),
		"preprocess.onnx", "encode.int8.onnx", "uncached_decode.int8.onnx",
		"cached_decode.int8.onnx", "tokens.txt")
	// A directory with tokens but neither family layout must be skipped.
	writeModelDir(t, filepath.Join(modelsDir, "unknown-model"), "tokens.txt")
	if err := os.WriteFile(filepath.Join(modelsDir, "ggml-tiny.en.bin"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write ggml model: %v", err)
	}

	engines := DiscoverEngines(filepath.Join(root, "does-not-exist"), root)
	if len(engines) != 3 {
		t.Fatalf("discovered %d engines, want 3: %+v", len(engines), engines)
	}

	byName := make(map[string]string, len(engines))
	for _, engine := range engines {
		byName[engine.Name] = engine.Spec
	}

	whisper, ok := byName["Whisper tiny.en"]
	if !ok {
		t.Fatalf("whisper engine missing: %v", engines)
	}
	want := "whisper=" + filepath.Join(binDir, "whisper-cli.exe") +
		" -m " + filepath.Join(modelsDir, "ggml-tiny.en.bin") + " -np -f {audio}"
	if whisper != want {
		t.Fatalf("whisper spec =\n%s\nwant\n%s", whisper, want)
	}

	parakeet, ok := byName["Parakeet tdt 0.6b v3 int8"]
	if !ok {
		t.Fatalf("parakeet engine missing: %v", engines)
	}
	if !strings.Contains(parakeet, "--encoder="+filepath.Join(modelsDir, "parakeet-tdt-0.6b-v3-int8", "encoder.int8.onnx")) ||
		!strings.Contains(parakeet, "--tokens="+filepath.Join(modelsDir, "parakeet-tdt-0.6b-v3-int8", "tokens.txt")) ||
		!strings.HasSuffix(parakeet, " {audio}") {
		t.Fatalf("parakeet spec malformed: %s", parakeet)
	}

	moonshine, ok := byName["Moonshine tiny en int8"]
	if !ok {
		t.Fatalf("moonshine engine missing: %v", engines)
	}
	if !strings.Contains(moonshine, "--moonshine-uncached-decoder=") {
		t.Fatalf("moonshine spec malformed: %s", moonshine)
	}

	// Every discovered spec must survive the parser the live pipeline uses.
	for _, engine := range engines {
		if _, err := ParseEngineSpec(engine.Spec); err != nil {
			t.Fatalf("spec %q does not parse: %v", engine.Spec, err)
		}
	}
}

func TestDiscoverEnginesQuotesPathsWithSpaces(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "my tools")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "whisper-cli"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "ggml-small.en.bin"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write model: %v", err)
	}

	engines := DiscoverEngines(root)
	if len(engines) != 1 {
		t.Fatalf("discovered %d engines, want 1", len(engines))
	}
	spec := engines[0].Spec
	if !strings.Contains(spec, `"`) {
		t.Fatalf("spec with spaces was not quoted: %s", spec)
	}
	engine, err := ParseEngineSpec(spec)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if engine.Name() != "whisper" {
		t.Fatalf("engine name = %q", engine.Name())
	}
}

func TestDiscoverEnginesEmptyRoots(t *testing.T) {
	if engines := DiscoverEngines(filepath.Join(t.TempDir(), "nothing")); len(engines) != 0 {
		t.Fatalf("expected no engines, got %v", engines)
	}
	if engines := DiscoverEngines(); len(engines) != 0 {
		t.Fatalf("expected no engines without roots, got %v", engines)
	}
}
