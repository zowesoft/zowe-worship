package main

import (
	"path/filepath"
	"testing"
)

// The engine roots are the contract between the installer, the downloader and
// the scanner: a bundled engine must sit under <exe dir>/engines (read-only)
// and a downloaded one under <data root>/engines (writable). This pins the
// order and the exact folders.
func TestDetectionEngineRootsLayout(t *testing.T) {
	exe := filepath.Join("P:", string(filepath.Separator), "Apps", "Zopresent", "zopresent.exe")
	dataRoot := filepath.Join("P:", string(filepath.Separator), "User", "Zowesoft", "Zopresent")
	cwd := filepath.Join("P:", string(filepath.Separator), "src", "zopresent")

	roots := detectionEngineRoots(cwd, exe, dataRoot)

	want := []string{
		filepath.Join(cwd, "benchmarks"),
		filepath.Join("P:", string(filepath.Separator), "Apps", "Zopresent", "benchmarks"),
		filepath.Join("P:", string(filepath.Separator), "Apps", "Zopresent", "engines"),
		filepath.Join(dataRoot, "engines"),
	}
	if len(roots) != len(want) {
		t.Fatalf("roots = %v, want %v", roots, want)
	}
	for i := range want {
		if roots[i] != want[i] {
			t.Fatalf("root %d = %q, want %q", i, roots[i], want[i])
		}
	}

	// A bundled engine lives beside the exe, read-only to the user; the
	// downloaded tree lives under the data root. Keep them distinct.
	if roots[2] == roots[3] {
		t.Fatalf("bundled and downloaded engine roots must differ, both = %q", roots[2])
	}
}

// Missing inputs must simply drop the roots they would have contributed, so a
// failure to read cwd or locate the exe does not blank the whole list.
func TestDetectionEngineRootsTolerantOfMissingInputs(t *testing.T) {
	roots := detectionEngineRoots("", "", "")
	if len(roots) != 0 {
		t.Fatalf("roots from no inputs = %v, want none", roots)
	}

	onlyData := detectionEngineRoots("", "", filepath.Join("P:", string(filepath.Separator), "data"))
	if len(onlyData) != 1 || filepath.Base(onlyData[0]) != "engines" {
		t.Fatalf("data-root-only roots = %v", onlyData)
	}
}
