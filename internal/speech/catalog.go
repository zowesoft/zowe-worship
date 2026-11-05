// Package speech provisions the offline speech engine Live Detection runs on.
// Two things place an engine on disk: the installer ships one (Moonshine)
// beside the executable, read-only, and the app downloads any other into the
// user's data root. This package owns the catalogue — what to fetch, from
// where, and the hash each byte must match — plus verifying that a bundled
// engine is intact before the detector is allowed to use it.
package speech

import (
	"fmt"
	"runtime"
	"strings"
)

// Engine is one downloadable speech engine: a model shared by every platform
// plus the transcribe binary and its runtime library, which differ per OS and
// architecture.
type Engine struct {
	// ID is the stable key stored in settings and used for the on-disk folder.
	ID string
	// Name is what the operator sees.
	Name string
	// Description explains the trade-off in one line.
	Description string
	// ModelDir is the folder the model files land in, under engines/models/.
	ModelDir string
	// Model is the platform-independent set of files that define the engine.
	Model []File
	// Runtimes maps GOOS/GOARCH to the transcribe binary and its library.
	Runtimes map[string]Runtime
}

// Runtime is the per-platform half of an engine. The two files are extracted
// from one release archive; keeping their hashes here lets a download be
// verified byte for byte before it is trusted.
type Runtime struct {
	// Archive is the release asset file name the two files come from.
	Archive string
	// ArchiveURL is the full download address for the archive. Empty means it
	// is built from the release base, which is what the real catalogue does.
	ArchiveURL string
	// ArchiveSHA256 pins the whole download; empty means unverified.
	ArchiveSHA256 string
	// Binary is the transcriber's file name inside the archive's bin/ folder.
	Binary string
	// BinarySHA256 pins the transcriber after extraction.
	BinarySHA256 string
	// Library is the runtime library's file name inside the archive's bin/ (or
	// lib/) folder, e.g. onnxruntime.dll. Empty when the engine needs none.
	Library string
	// LibrarySHA256 pins the library after extraction.
	LibrarySHA256 string
}

// URL is where the archive is fetched from.
func (r Runtime) URL() string {
	if r.ArchiveURL != "" {
		return r.ArchiveURL
	}
	return RuntimeArchiveURL(r.Archive)
}

// File is one downloaded file and the hash it must match.
type File struct {
	Name   string
	SHA256 string
	URL    string
}

// Key is the GOOS/GOARCH key used in Engine.Runtimes.
func Key(goos, goarch string) string {
	return goos + "/" + goarch
}

// HostKey is Key for the machine this build runs on.
func HostKey() string {
	return Key(runtime.GOOS, runtime.GOARCH)
}

// RuntimeFor returns the runtime half for a platform, if the engine ships one.
func (e Engine) RuntimeFor(goos, goarch string) (Runtime, bool) {
	r, ok := e.Runtimes[Key(goos, goarch)]
	return r, ok
}

// HostRuntime returns the runtime half for the machine this build runs on.
func (e Engine) HostRuntime() (Runtime, bool) {
	return e.RuntimeFor(runtime.GOOS, runtime.GOARCH)
}

// releaseBase is where every runtime archive is published.
const releaseBase = "https://github.com/k2-fsa/sherpa-onnx/releases/download"

// moonshineModelBase is the HuggingFace repo holding the platform-independent
// Moonshine tiny int8 model.
const moonshineModelBase = "https://huggingface.co/csukuangfj/sherpa-onnx-moonshine-tiny-en-int8/resolve/main"

// Catalog lists the engines the app knows how to fetch. The first entry is the
// one the installer bundles; the rest are download-only.
//
// Hashes are pinned from the v2.0.1 release. A model file's hash is the same
// on every platform (the model is portable), so one entry pins all OSes; only
// the runtime differs and carries its own hashes.
func Catalog() []Engine {
	return []Engine{Moonshine()}
}

// Moonshine is the engine the installer bundles: small, fast, and free of any
// model-level license gate.
func Moonshine() Engine {
	return Engine{
		ID:          "moonshine-tiny-en",
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

// Find returns the catalogue entry with the given id.
func Find(id string) (Engine, error) {
	for _, engine := range Catalog() {
		if engine.ID == id {
			return engine, nil
		}
	}
	return Engine{}, fmt.Errorf("unknown speech engine %q", id)
}

// RuntimeArchiveURL is the download address for a runtime archive.
func RuntimeArchiveURL(asset string) string {
	return releaseBase + "/v2.0.1/" + asset
}

// EngineIDFromSpec recovers an engine id from an engine spec. The spec names
// the engine before "=", matching the ids the catalogue uses.
func EngineIDFromSpec(spec string) string {
	name, _, found := strings.Cut(strings.TrimSpace(spec), "=")
	if !found {
		return ""
	}
	return strings.TrimSpace(name)
}
