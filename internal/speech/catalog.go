// Package speech owns the engine catalogue: what to fetch, from where, and the
// hash every byte must match before the detector may use it.
package speech

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// Engine is a model shared by every platform plus the per-OS runtime that runs
// it.
type Engine struct {
	// ID is the settings key and the on-disk folder name.
	ID string
	// Command is the family name before "=" in a spec, matching what discovery
	// emits, so a bundled, downloaded or hand-picked engine all read the same.
	Command     string
	Name        string
	Description string
	// ModelDir is where the model files land, under engines/models/.
	ModelDir string
	Model    []File
	Runtimes map[string]Runtime
}

// Runtime is the per-platform half: one release archive plus the hashes that
// prove it.
type Runtime struct {
	// Archive is the release asset file name the two files come from.
	Archive string
	// ArchiveURL is optional; empty means derive it from Archive.
	ArchiveURL string
	// ArchiveSHA256 pins the whole download; empty means unverified.
	ArchiveSHA256 string
	Binary        string
	// BinarySHA256 pins the transcriber after extraction.
	BinarySHA256 string
	// Library is the runtime library inside the archive, e.g. onnxruntime.dll;
	// empty when the engine needs none.
	Library string
	// LibrarySHA256 pins the library after extraction.
	LibrarySHA256 string
}

func (r Runtime) URL() string {
	if r.ArchiveURL != "" {
		return r.ArchiveURL
	}
	return RuntimeArchiveURL(r.Archive)
}

type File struct {
	Name   string
	SHA256 string
	URL    string
}

func Key(goos, goarch string) string {
	return goos + "/" + goarch
}

func HostKey() string {
	return Key(runtime.GOOS, runtime.GOARCH)
}

func (e Engine) RuntimeFor(goos, goarch string) (Runtime, bool) {
	r, ok := e.Runtimes[Key(goos, goarch)]
	return r, ok
}

func (e Engine) HostRuntime() (Runtime, bool) {
	return e.RuntimeFor(runtime.GOOS, runtime.GOARCH)
}

// releaseBase is where every runtime archive is published.
const releaseBase = "https://github.com/k2-fsa/sherpa-onnx/releases/download"

// moonshineModelBase is the HuggingFace repo holding the platform-independent
// Moonshine tiny int8 model.
const moonshineModelBase = "https://huggingface.co/csukuangfj/sherpa-onnx-moonshine-tiny-en-int8/resolve/main"

// Catalog's first entry is the engine the installer bundles; the rest are
// download-only.
//
// Hashes are pinned from the v2.0.1 release. The model is portable, so one hash
// covers every platform; each runtime pins its own files.
func Catalog() []Engine {
	return []Engine{Moonshine()}
}

// Moonshine is the engine the installer bundles: small, fast, and free of any
// model-level license gate.
func Moonshine() Engine {
	return Engine{
		ID:          "moonshine-tiny-en",
		Command:     "moonshine",
		Name:        "Moonshine tiny (English)",
		Description: "Bundled with the app. Fast, offline, English only.",
		ModelDir:    "moonshine-tiny-en-int8",
		Model: []File{
			{Name: "preprocess.onnx", SHA256: "f33addce61a143460fe753b5ee5b7db255e5140b5b779c065b94f6c83ff0bf4e", URL: moonshineModelBase + "/preprocess.onnx"},
			{Name: "encode.int8.onnx", SHA256: "8774dfba578de027ec6595c2c654a0836434489bc963a0db124a7f181f571acb", URL: moonshineModelBase + "/encode.int8.onnx"},
			{Name: "uncached_decode.int8.onnx", SHA256: "216737000dd5881a17aa043f6bbd286add33e4c3b0ae257153e2ec15438bdc41", URL: moonshineModelBase + "/uncached_decode.int8.onnx"},
			{Name: "cached_decode.int8.onnx", SHA256: "2aff28bba6a03d8dcf5c9feac45462629bae37317442299f28115ad09da773f6", URL: moonshineModelBase + "/cached_decode.int8.onnx"},
			{Name: "tokens.txt", SHA256: "1165c2aeb9f72f457a83be2d459a09054f27490acd9b41bd43794dfd25e296ea", URL: moonshineModelBase + "/tokens.txt"},
		},
		Runtimes: map[string]Runtime{
			Key("windows", "amd64"): {
				Archive:       "sherpa-onnx-v2.0.1-win-x64-shared-MT-Release-no-tts.tar.bz2",
				ArchiveSHA256: "a644dfe2f7944afec9a356d2557c0452380f20ffcf2f8c704653cfce37558596",
				Binary:        "sherpa-onnx-offline.exe",
				BinarySHA256:  "047bff7c67bedbf99d1a0e55773ed6770f67d161466f6e4977af3e217de99e27",
				Library:       "onnxruntime.dll",
				LibrarySHA256: "7f66f939a881baf4f46a2216496798edf4a1429878b646d12674aa62f27d8a25",
			},
			Key("windows", "arm64"): {
				Archive:       "sherpa-onnx-v2.0.1-win-arm64-shared-MT-Release-no-tts.tar.bz2",
				ArchiveSHA256: "f9959b93bff00f4d68620b59817846a5955a1b90fdf9441ba56e1780e4a10368",
				Binary:        "sherpa-onnx-offline.exe",
				BinarySHA256:  "6ee641c5c5e46be66f8aa48ea393d69320263db864871dbe5d41d59f70554920",
				Library:       "onnxruntime.dll",
				LibrarySHA256: "968d081836cad9af537105f59e34d4997bca2ce6d78ff39d3e47660620e8a642",
			},
			Key("linux", "amd64"): {
				Archive:       "sherpa-onnx-v2.0.1-linux-x64-shared-no-tts.tar.bz2",
				ArchiveSHA256: "b3b095f619059d54914027ff38f8a0a132ea3bbc5df32a212a9743a8bd1467b2",
				Binary:        "sherpa-onnx-offline",
				BinarySHA256:  "1dc6b2842f521538a2d73d0bb1002f7c6b1acd1bc0fbf10fbb6a823c8c1a1c60",
				Library:       "libonnxruntime.so",
				LibrarySHA256: "4b3607aebd1784b26b6f9b20e4bd974c7ab8287043e4d095cb7d2cb40b5e566e",
			},
			Key("linux", "arm64"): {
				Archive:       "sherpa-onnx-v2.0.1-linux-aarch64-shared-cpu.tar.bz2",
				ArchiveSHA256: "a8fc9fb3940d1aa3f83136767742b05068f00253f560d53c4f5aeb75ab4cb350",
				Binary:        "sherpa-onnx-offline",
				BinarySHA256:  "d193811d7bb7f92deb45fbb6643d4df32a3116a5b8ab53f3fe87c3200d172364",
				Library:       "libonnxruntime.so",
				LibrarySHA256: "cfc800d4438f15e8d9017521326776ca7117d8783cb847ccccc2fc61ac4d540f",
			},
			Key("darwin", "arm64"): {
				Archive:       "sherpa-onnx-v2.0.1-osx-arm64-shared-no-tts.tar.bz2",
				ArchiveSHA256: "4fcc6c77c8d8d2e2f04cfdc17c6fd224c01469b78529370eb9d8e0e5ec97ac79",
				Binary:        "sherpa-onnx-offline",
				BinarySHA256:  "2bda8e350cce4d77e1ffda40c71289e3db14bfeaa026ab6ba286c7b829673dfe",
				Library:       "libonnxruntime.dylib",
				LibrarySHA256: "3567d114f7299d559993e536d605a6f46d7bc9d2542004accc80ee9bf5457f0b",
			},
			Key("darwin", "amd64"): {
				Archive:       "sherpa-onnx-v2.0.1-osx-x64-shared-no-tts.tar.bz2",
				ArchiveSHA256: "029b5ea0a4bd2f0c034a337c0043019323fd06491728d7fc712592e3b558be52",
				Binary:        "sherpa-onnx-offline",
				BinarySHA256:  "220367793f1021fe2e7f54537fcf9869564725ad9bc0dfc1396027e7356b018b",
				Library:       "libonnxruntime.dylib",
				LibrarySHA256: "2bf27180c09e6fd3f407c9ab25205ebf1ff95f2d53bfa35e35409b5849e0ac70",
			},
		},
	}
}

// Spec builds the engine command line in the "name=bin args {audio}" form
// discovery emits.
func (e Engine) Spec(root string) (string, error) {
	r, ok := e.HostRuntime()
	if !ok {
		return "", fmt.Errorf("engine %q has no runtime for this platform (%s)", e.ID, HostKey())
	}
	if e.ID != Moonshine().ID {
		return "", fmt.Errorf("engine %q has no spec builder", e.ID)
	}

	modelDir := ModelPath(root, e)
	flag := func(name string) string {
		return "--moonshine-" + name + "=" + quoteArg(filepath.Join(modelDir, modelFiles[name]))
	}

	args := []string{
		e.Command + "=" + quoteArg(BinaryPath(root, r)),
		flag("preprocessor"),
		flag("encoder"),
		flag("uncached-decoder"),
		flag("cached-decoder"),
		"--tokens=" + quoteArg(filepath.Join(modelDir, "tokens.txt")),
		"{audio}",
	}
	return strings.Join(args, " "), nil
}

var modelFiles = map[string]string{
	"preprocessor":     "preprocess.onnx",
	"encoder":          "encode.int8.onnx",
	"uncached-decoder": "uncached_decode.int8.onnx",
	"cached-decoder":   "cached_decode.int8.onnx",
}

// quoteArg wraps a path in double quotes when it contains whitespace, the only
// quoting form asr.ParseEngineSpec understands.
func quoteArg(path string) string {
	if strings.ContainsAny(path, " \t") {
		return `"` + path + `"`
	}
	return path
}

func Find(id string) (Engine, error) {
	for _, engine := range Catalog() {
		if engine.ID == id {
			return engine, nil
		}
	}
	return Engine{}, fmt.Errorf("unknown speech engine %q", id)
}

func RuntimeArchiveURL(asset string) string {
	return releaseBase + "/v2.0.1/" + asset
}

// SpecCommand matches stored specs to catalogue engines by family name, not by
// binary path.
func SpecCommand(spec string) string {
	name, _, found := strings.Cut(strings.TrimSpace(spec), "=")
	if !found {
		return ""
	}
	return strings.TrimSpace(name)
}

func (e Engine) Matches(spec string) bool {
	return e.Command != "" && SpecCommand(spec) == e.Command
}
