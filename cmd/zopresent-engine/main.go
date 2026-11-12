// Command zopresent-engine stages a speech engine for packaging: it builds the tree
// the packager copies beside the app, using the pinned hashes in internal/speech
// so the shipped bytes are the verified ones, not whatever is on the build
// machine. The model is platform-independent; -goos/-goarch pick the runtime, so
// a Windows installer can be staged from any host.
//
// Usage:
//
//	go run ./cmd/zopresent-engine -out build/engines -goos windows -goarch amd64
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"zopresent/internal/speech"
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
	// A live progress bar only reads well on a terminal; piped to a log it would
	// emit thousands of lines, so fall back to coarse per-file marks there.
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

	// InstallFor writes the notices into the tree; fail if they are missing so
	// the redistributed binaries always carry their terms.
	noticesPath := filepath.Join(out, speech.NoticesFile)
	if _, err := os.Stat(noticesPath); err != nil {
		return fmt.Errorf("expected third-party notices at %s: %w", noticesPath, err)
	}

	fmt.Printf("staged %s for %s/%s into %s\n", engine.Name, goos, goarch, out)
	return nil
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
