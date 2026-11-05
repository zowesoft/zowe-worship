// Command zowe-engine stages a speech engine for packaging.
//
// The installer ships one engine (Moonshine) read-only beside the app. This
// command produces the tree the packager copies into place, using the pinned
// hashes in internal/speech so the shipped bytes are the verified ones — not
// whatever happens to be on the build machine.
//
// The engine's model is the same on every platform; only the runtime differs,
// so -goos/-goarch choose which runtime to fetch. That lets a Windows installer
// be staged from any host, and leaves the same command ready for the macOS and
// Linux packages when they exist.
//
// Usage:
//
//	go run ./cmd/zowe-engine -out build/engines -goos windows -goarch amd64
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"zowe-worship/internal/speech"
)

func main() {
	var (
		engineID = flag.String("engine", "moonshine-tiny-en", "catalogue engine id to stage")
		out      = flag.String("out", "build/engines", "directory to stage engines/ into")
		goos     = flag.String("goos", runtime.GOOS, "target OS for the runtime (windows, darwin, linux)")
		goarch   = flag.String("goarch", runtime.GOARCH, "target architecture (amd64, arm64)")
		verify   = flag.Bool("verify", true, "re-verify every staged file against its pinned hash")
	)
	flag.Parse()

	if err := run(*engineID, *out, *goos, *goarch, *verify); err != nil {
		fmt.Fprintln(os.Stderr, "stage engine:", err)
		os.Exit(1)
	}
}

func run(engineID, out, goos, goarch string, verify bool) error {
	engine, err := speech.Find(engineID)
	if err != nil {
		return err
	}
	if _, ok := engine.RuntimeFor(goos, goarch); !ok {
		return fmt.Errorf("no runtime for %s on %s/%s", engineID, goos, goarch)
	}

	installer := speech.NewInstaller()
	// A live bar rewrites one line, which only reads well on a terminal; when
	// output is piped to a log it would emit thousands of lines, so progress is
	// limited to coarse per-file marks in that case.
	live := isTerminal(os.Stdout)
	lastFile := ""
	installer.Progress = func(p speech.Progress) {
		if live {
			pct := 0
			if p.TotalBytes > 0 {
				pct = int(p.Bytes * 100 / p.TotalBytes)
			}
			fmt.Printf("\r  %-40s %d/%d %3d%%", p.File, p.Done+1, p.Total, pct)
			return
		}
		if p.File != lastFile {
			lastFile = p.File
			fmt.Printf("  fetching %s (%d/%d)\n", p.File, p.Done+1, p.Total)
		}
	}

	// Stage with the target platform's runtime, so the tree matches what the
	// packaged app will look for at <exe dir>/engines.
	if err := installer.InstallFor(out, engine, goos, goarch); err != nil {
		return err
	}
	if live {
		fmt.Println()
	}

	if verify {
		if err := speech.VerifyFor(out, engine, goos, goarch); err != nil {
			return fmt.Errorf("staged tree failed verification: %w", err)
		}
	}

	// The engines directory sits beside the app in the package; the notices are
	// written by InstallFor, so they travel with the redistributed binaries
	// automatically, whichever packager copies the tree in.
	noticesPath := filepath.Join(out, speech.NoticesFile)
	if _, err := os.Stat(noticesPath); err != nil {
		return fmt.Errorf("expected third-party notices at %s: %w", noticesPath, err)
	}

	fmt.Printf("staged %s for %s/%s into %s\n", engine.Name, goos, goarch, out)
	return nil
}

// isTerminal reports whether f is a character device, i.e. a real console
// rather than a pipe or a redirected file.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
