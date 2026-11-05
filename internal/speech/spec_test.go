package speech

import (
	"strings"
	"testing"

	"zowe-worship/internal/asr"
)

func TestSpecBuildsParsableMoonshineCommand(t *testing.T) {
	root := "/data/engines"
	spec, err := Moonshine().Spec(root)
	if err != nil {
		t.Fatalf("spec: %v", err)
	}
	t.Logf("spec = %s", spec)

	if !strings.HasPrefix(spec, "moonshine=") {
		t.Fatalf("spec must be named by the engine command, got %q", spec)
	}
	if !strings.Contains(spec, "--moonshine-preprocessor=") || !strings.Contains(spec, "--tokens=") {
		t.Fatalf("spec missing model flags: %s", spec)
	}
	if !strings.HasSuffix(spec, " {audio}") {
		t.Fatalf("spec missing audio placeholder: %s", spec)
	}
	if _, err := asr.ParseEngineSpec(spec); err != nil {
		t.Fatalf("spec does not parse: %v", err)
	}
}

// A stored spec matches the catalogue engine by command family, so an engine
// picked from disk (which discovery names "moonshine") and one installed from
// the network both read as the same engine — this is what lights the "in use"
// row.
func TestEngineMatchesStoredSpecByCommand(t *testing.T) {
	moonshine := Moonshine()

	for _, spec := range []string{
		`moonshine=/data/engines/bin/sherpa-onnx-offline --tokens=/x/tokens.txt {audio}`,
		`moonshine="C:\Program Files\Zowe\engines\bin\sherpa-onnx-offline.exe" -f {audio}`,
	} {
		if !moonshine.Matches(spec) {
			t.Fatalf("moonshine should match stored spec: %s", spec)
		}
	}

	if moonshine.Matches(`whisper=C:\tools\whisper-cli.exe -m model.bin {audio}`) {
		t.Fatal("moonshine must not match a whisper spec")
	}
	if moonshine.Matches("") {
		t.Fatal("moonshine must not match an empty spec")
	}
}
