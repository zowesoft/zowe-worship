package speech

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testEngine is a catalogue-shaped engine whose runtime points at a test
// server, so the download path can be exercised without the network.
func testEngine(t *testing.T, server *httptest.Server, modelHash, binHash, libHash string) Engine {
	t.Helper()
	return Engine{
		ID:       "test-engine",
		Name:     "Test",
		ModelDir: "test-model",
		Model: []File{
			{Name: "model.onnx", SHA256: modelHash, URL: server.URL + "/model.onnx"},
		},
		Runtimes: map[string]Runtime{
			HostKey(): {
				Archive:       "runtime.tar.bz2",
				ArchiveURL:    server.URL + "/runtime.tar.bz2",
				ArchiveSHA256: "",
				Binary:        "sherpa-onnx-offline" + exeSuffix(),
				BinarySHA256:  binHash,
				Library:       "runtime-lib" + libSuffix(),
				LibrarySHA256: libHash,
			},
		},
	}
}

func exeSuffix() string {
	if os.PathSeparator == '\\' {
		return ".exe"
	}
	return ""
}

func libSuffix() string {
	return ".so"
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestVerifyAcceptsIntactEngineAndRejectsTampered(t *testing.T) {
	model := []byte("model-bytes")
	bin := []byte("bin-bytes")
	lib := []byte("lib-bytes")
	engine := Engine{
		ID:       "e",
		ModelDir: "m",
		Model:    []File{{Name: "model.onnx", SHA256: sha(model)}},
		Runtimes: map[string]Runtime{
			HostKey(): {
				Binary: "sherpa-onnx-offline", BinarySHA256: sha(bin),
				Library: "libonnxruntime.so", LibrarySHA256: sha(lib),
			},
		},
	}

	root := t.TempDir()
	write(t, ModelPath(root, engine), "model.onnx", model)
	write(t, filepath.Join(root, "bin"), "sherpa-onnx-offline", bin)
	write(t, filepath.Join(root, "bin"), "libonnxruntime.so", lib)

	if !Present(root, engine) {
		t.Fatal("intact engine reported absent")
	}
	if err := Verify(root, engine); err != nil {
		t.Fatalf("intact engine failed verify: %v", err)
	}

	// Corrupt the binary: verify must refuse it so a tampered engine cannot run.
	write(t, filepath.Join(root, "bin"), "sherpa-onnx-offline", []byte("tampered"))
	if err := Verify(root, engine); !errors.Is(err, ErrChecksum) {
		t.Fatalf("tampered engine verify err = %v, want ErrChecksum", err)
	}
}

func TestInstallDownloadsVerifiesAndPlaces(t *testing.T) {
	model := []byte("the-model")
	bin := []byte("the-binary")
	lib := []byte("the-library")

	mux := http.NewServeMux()
	mux.HandleFunc("/model.onnx", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(model) })
	mux.HandleFunc("/runtime.tar.bz2", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(tarBytes(t, map[string][]byte{
			"sherpa-onnx-v-test/bin/" + "sherpa-onnx-offline" + exeSuffix(): bin,
			"sherpa-onnx-v-test/bin/" + "runtime-lib" + libSuffix():         lib,
		}))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	engine := testEngine(t, server, sha(model), sha(bin), sha(lib))
	root := t.TempDir()

	var seen []string
	inst := NewInstaller()
	inst.extract = extractPlainTar // fixtures are uncompressed tar, not bzip2
	inst.Progress = func(p Progress) { seen = append(seen, p.File) }

	if err := inst.Install(root, engine); err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := Verify(root, engine); err != nil {
		t.Fatalf("verify after install: %v", err)
	}
	if len(seen) == 0 {
		t.Fatal("no progress was reported")
	}

	// The license terms must travel with the installed bytes, so the notices
	// file has to be written for a download just as for a packaged install.
	notices, err := os.ReadFile(filepath.Join(root, NoticesFile))
	if err != nil {
		t.Fatalf("install did not write %s: %v", NoticesFile, err)
	}
	if !strings.Contains(string(notices), "sherpa-onnx") || !strings.Contains(string(notices), "Apache-2.0") {
		t.Fatal("notices file does not mention the engine's license")
	}

	// A second install must be a no-op that still verifies (idempotent).
	if err := inst.Install(root, engine); err != nil {
		t.Fatalf("re-install: %v", err)
	}
}

func TestInstallRejectsWrongHashAndLeavesNoFile(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/model.onnx", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("real")) })
	server := httptest.NewServer(mux)
	defer server.Close()

	// Pin a hash the served bytes cannot match.
	engine := testEngine(t, server, "deadbeef", "x", "y")
	root := t.TempDir()

	inst := NewInstaller()
	inst.extract = extractPlainTar
	err := inst.Install(root, engine)
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("install err = %v, want ErrChecksum", err)
	}
	if _, statErr := os.Stat(filepath.Join(ModelPath(root, engine), "model.onnx")); statErr == nil {
		t.Fatal("a file that failed its hash was left in place")
	}
}

func TestExtractRefusesArchiveEntryEscapingDestination(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out")

	err := extractTar(bytes.NewReader(tarBytes(t, map[string][]byte{
		"../escaped.txt": []byte("should not be written"),
	})), dest)
	if err == nil {
		t.Fatal("archive with an escaping entry was extracted")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "escaped.txt")); statErr == nil {
		t.Fatal("escaping entry was written outside the destination")
	}
}

func TestSafeJoinKeepsPathsInsideRoot(t *testing.T) {
	root := t.TempDir()
	ok, err := safeJoin(root, "bin/tool")
	if err != nil {
		t.Fatalf("legitimate entry rejected: %v", err)
	}
	if !strings.HasPrefix(ok, root) {
		t.Fatalf("legitimate entry left the root: %s", ok)
	}
	for _, bad := range []string{"../up", "a/../../up", "/abs", ".."} {
		if _, err := safeJoin(root, bad); err == nil {
			t.Fatalf("unsafe entry %q was accepted", bad)
		}
	}
}

// extractPlainTar is the extract signature Installer expects, backed by a tar
// reader over an uncompressed archive. Test fixtures use it so the download and
// placement path runs without a bzip2 encoder (Go's stdlib has none).
func extractPlainTar(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTar(f, dest)
}

// write creates a file with contents under dir.
func write(t *testing.T, dir, name string, contents []byte) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), contents, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// tarBytes builds an uncompressed tar in memory from name->content pairs. The
// extractor's safety and placement logic is what matters here; the bzip2 layer
// is a thin wrapper over the same tar reader.
func tarBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatalf("tar header: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("tar write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return buf.Bytes()
}
