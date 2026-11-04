package asr

// Engine discovery: the operator should pick a speech engine from a list of
// what is actually installed, not type a filesystem path by hand. Discovery
// scans a set of root directories for known engine binaries and pairs them
// with the models sitting beside them, producing ready-to-run engine specs in
// the same "name=bin args {audio}" form ParseEngineSpec accepts.

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DiscoveredEngine is one selectable engine: a human label and the spec that
// runs it. Spec is complete — no further editing required.
type DiscoveredEngine struct {
	Name string `json:"name"`
	Spec string `json:"spec"`
}

// maxDiscoveryDepth bounds the walk so a stray large tree cannot stall the
// settings window.
const maxDiscoveryDepth = 5

// DiscoverEngines scans the given roots for whisper.cpp and sherpa-onnx
// installations and returns every engine/model pair it can wire up. Roots are
// typically the repository's benchmarks folder in development and an engines
// folder under the data root for installed builds. Missing roots are simply
// skipped; the result may be empty.
func DiscoverEngines(roots ...string) []DiscoveredEngine {
	var (
		whisperBins   []string
		whisperModels []string
		sherpaBins    []string
		sherpaModels  []string // directories holding tokens.txt
	)

	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil // a missing or unreadable root contributes nothing
			}
			depth := strings.Count(strings.TrimPrefix(path, root), string(filepath.Separator))
			if entry.IsDir() {
				if depth >= maxDiscoveryDepth {
					return fs.SkipDir
				}
				return nil
			}

			name := strings.ToLower(entry.Name())
			switch {
			case name == "whisper-cli.exe" || name == "whisper-cli":
				whisperBins = append(whisperBins, path)
			// Exact names only: the sherpa-onnx bin folder also holds
			// sherpa-onnx-offline-audio-tagging, -tts, -punctuation and
			// friends, which accept entirely different flags.
			case name == "sherpa-onnx-offline.exe" || name == "sherpa-onnx-offline":
				sherpaBins = append(sherpaBins, path)
			case strings.HasPrefix(name, "ggml-") && strings.HasSuffix(name, ".bin"),
				strings.HasSuffix(name, ".ggml"):
				whisperModels = append(whisperModels, path)
			}
			return nil
		})

		// Model directories are found with a shallow walk: a model is a
		// directory that holds tokens.txt.
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || !entry.IsDir() || path == root {
				return nil
			}
			depth := strings.Count(strings.TrimPrefix(path, root), string(filepath.Separator))
			if depth > maxDiscoveryDepth {
				return fs.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "tokens.txt")); err == nil {
				sherpaModels = append(sherpaModels, path)
				return fs.SkipDir // no nested model dirs inside a model
			}
			return nil
		})
	}

	sort.Strings(whisperBins)
	sort.Strings(whisperModels)
	sort.Strings(sherpaBins)
	sort.Strings(sherpaModels)

	var engines []DiscoveredEngine
	for _, bin := range whisperBins {
		for _, model := range whisperModels {
			engines = append(engines, DiscoveredEngine{
				Name: "Whisper " + whisperModelLabel(model),
				Spec: "whisper=" + quotePath(bin) + " -m " + quotePath(model) + " -np -f {audio}",
			})
		}
	}
	for _, bin := range sherpaBins {
		for _, model := range sherpaModels {
			if engine, ok := sherpaEngine(bin, model); ok {
				engines = append(engines, engine)
			}
		}
	}
	return engines
}

// sherpaEngine builds a spec for one model directory, or reports false when
// the directory does not match a known sherpa-onnx model family.
func sherpaEngine(bin, dir string) (DiscoveredEngine, bool) {
	label := prettifyModelName(filepath.Base(dir))

	// Parakeet: encoder/decoder/joiner + tokens.
	encoder, ok := firstMatch(dir, "encoder*.onnx")
	if ok {
		decoder, _ := firstMatch(dir, "decoder*.onnx")
		joiner, _ := firstMatch(dir, "joiner*.onnx")
		tokens, _ := firstMatch(dir, "tokens.txt")
		return DiscoveredEngine{
			Name: label,
			Spec: "parakeet=" + quotePath(bin) +
				" --encoder=" + quotePath(encoder) +
				" --decoder=" + quotePath(decoder) +
				" --joiner=" + quotePath(joiner) +
				" --tokens=" + quotePath(tokens) +
				" {audio}",
		}, true
	}

	// Moonshine: preprocess/encoder/decoders + tokens.
	preprocess, ok := firstMatch(dir, "preprocess*.onnx")
	if ok {
		encoder, _ := firstMatch(dir, "encode*.onnx")
		uncached, _ := firstMatch(dir, "uncached_decode*.onnx")
		cached, _ := firstMatch(dir, "cached_decode*.onnx")
		tokens, _ := firstMatch(dir, "tokens.txt")
		return DiscoveredEngine{
			Name: label,
			Spec: "moonshine=" + quotePath(bin) +
				" --moonshine-preprocessor=" + quotePath(preprocess) +
				" --moonshine-encoder=" + quotePath(encoder) +
				" --moonshine-uncached-decoder=" + quotePath(uncached) +
				" --moonshine-cached-decoder=" + quotePath(cached) +
				" --tokens=" + quotePath(tokens) +
				" {audio}",
		}, true
	}
	return DiscoveredEngine{}, false
}

func firstMatch(dir, pattern string) (string, bool) {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil || len(matches) == 0 {
		return "", false
	}
	return matches[0], true
}

// whisperModelLabel turns "ggml-tiny.en.bin" into "tiny.en".
func whisperModelLabel(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.TrimPrefix(base, "ggml-")
	base = strings.TrimSuffix(base, ".ggml")
	return base
}

// prettifyModelName turns "parakeet-tdt-0.6b-v3-int8" into "Tdt 0.6b v3 int8".
func prettifyModelName(name string) string {
	name = strings.ReplaceAll(name, "-", " ")
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// quotePath wraps a path in double quotes when it contains whitespace, the
// only quoting form ParseEngineSpec understands. Backslashes stay literal.
func quotePath(path string) string {
	if strings.ContainsAny(path, " \t") {
		return `"` + path + `"`
	}
	return path
}
