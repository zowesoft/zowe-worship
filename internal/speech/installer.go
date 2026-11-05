package speech

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrChecksum is returned when a downloaded or bundled file does not match its
// pinned hash. The caller must treat the file as unusable rather than run it.
var ErrChecksum = errors.New("file checksum does not match")

// Progress reports how far a download has come: Done of Total files finished,
// and Bytes of TotalBytes within the file in flight, so a caller can show both
// a count and a bar.
type Progress struct {
	File       string
	Done       int
	Total      int
	Bytes      int64
	TotalBytes int64
}

// Layout is where engines live. Two roots, deliberately distinct: the bundled
// engine sits read-only beside the installed app, the downloadable ones under
// the writable per-user data root.
type Layout struct {
	// BundledRoot holds the engine the installer shipped (<exe dir>/engines).
	BundledRoot string
	// DownloadRoot holds engines fetched for this user (<data root>/engines).
	DownloadRoot string
}

// ModelPath is where an engine's model files land under a root.
func ModelPath(root string, engine Engine) string {
	return filepath.Join(root, "models", engine.ModelDir)
}

// BinaryPath is the transcribe binary's path under a root.
func BinaryPath(root string, r Runtime) string {
	return filepath.Join(root, "bin", r.Binary)
}

// LibraryPath is the runtime library's path under a root, or "" if the engine
// needs none.
func LibraryPath(root string, r Runtime) string {
	if r.Library == "" {
		return ""
	}
	return filepath.Join(root, "bin", r.Library)
}

// Present reports whether the engine's files for a platform all sit under
// root. It checks existence only; use VerifyFor to also check integrity.
func Present(root string, engine Engine) bool {
	return PresentFor(root, engine, runtime.GOOS, runtime.GOARCH)
}

// PresentFor is Present for a specific target platform, so a packaging step can
// check an engine staged for another OS.
func PresentFor(root string, engine Engine, goos, goarch string) bool {
	r, ok := engine.RuntimeFor(goos, goarch)
	if !ok {
		return false
	}
	if _, err := os.Stat(BinaryPath(root, r)); err != nil {
		return false
	}
	if lib := LibraryPath(root, r); lib != "" {
		if _, err := os.Stat(lib); err != nil {
			return false
		}
	}
	modelDir := ModelPath(root, engine)
	for _, f := range engine.Model {
		if _, err := os.Stat(filepath.Join(modelDir, f.Name)); err != nil {
			return false
		}
	}
	return true
}

// Verify checks that the engine for this machine is present AND intact.
func Verify(root string, engine Engine) error {
	return VerifyFor(root, engine, runtime.GOOS, runtime.GOARCH)
}

// VerifyFor checks that the engine's files for a target platform are present
// AND match their pinned hashes. A bundled engine that fails this is corrupt or
// was tampered with, and must not be run.
func VerifyFor(root string, engine Engine, goos, goarch string) error {
	r, ok := engine.RuntimeFor(goos, goarch)
	if !ok {
		return fmt.Errorf("engine %q has no runtime for %s", engine.ID, Key(goos, goarch))
	}

	if err := verifyHash(BinaryPath(root, r), r.BinarySHA256); err != nil {
		return err
	}
	if lib := LibraryPath(root, r); lib != "" {
		if err := verifyHash(lib, r.LibrarySHA256); err != nil {
			return err
		}
	}
	modelDir := ModelPath(root, engine)
	for _, f := range engine.Model {
		if err := verifyHash(filepath.Join(modelDir, f.Name), f.SHA256); err != nil {
			return err
		}
	}
	return nil
}

// verifyHash hashes one file and compares it to want. An empty want means the
// file is not pinned and is accepted as-is.
func verifyHash(path, want string) error {
	if want == "" {
		return nil
	}
	got, err := hashFile(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("%w: %s", ErrChecksum, filepath.Base(path))
	}
	return nil
}

// hashFile returns the SHA-256 of a file as lowercase hex.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// Installer fetches engine files over HTTP and verifies each before accepting
// it. It never runs an engine; it only places files the detector later uses.
type Installer struct {
	client *http.Client
	// Progress, when set, is called as a download advances.
	Progress func(Progress)
	// extract unpacks a runtime archive. It defaults to the .tar.bz2 reader for
	// the real releases; tests substitute a plain-tar reader so the download
	// and placement path can be exercised without a bzip2 encoder.
	extract func(archive, dest string) error
}

// NewInstaller returns an installer with a client suited to large model files:
// no overall timeout, since a 53 MB decode can outlast a default one.
func NewInstaller() *Installer {
	return &Installer{
		client:  &http.Client{Timeout: 0},
		extract: extractTarBz2,
	}
}

// Install places the engine for this machine under root, downloading only what
// is missing or fails its hash. It is safe to re-run: an intact engine is left
// untouched.
func (i *Installer) Install(root string, engine Engine) error {
	return i.InstallFor(root, engine, runtime.GOOS, runtime.GOARCH)
}

// InstallFor places the engine for a target platform under root. The runtime
// archive is fetched, its members extracted, and everything hash-verified;
// nothing is registered until every byte checks out. A packaging step calls
// this with an explicit platform so a Windows installer can be assembled from a
// non-Windows host — the model is shared, only the runtime differs.
func (i *Installer) InstallFor(root string, engine Engine, goos, goarch string) error {
	r, ok := engine.RuntimeFor(goos, goarch)
	if !ok {
		return fmt.Errorf("engine %q has no runtime for %s", engine.ID, Key(goos, goarch))
	}

	total := len(engine.Model) + 2
	done := 0

	// Model files: platform-independent, one download each.
	modelDir := ModelPath(root, engine)
	for _, f := range engine.Model {
		dest := filepath.Join(modelDir, f.Name)
		if verifyHash(dest, f.SHA256) == nil {
			done++
			continue
		}
		if err := i.download(f.URL, dest, f.SHA256, f.Name, done, total); err != nil {
			return err
		}
		done++
	}

	// Runtime: one archive whose members become the binary and the library.
	if err := i.installRuntime(root, engine, r, done, total); err != nil {
		return err
	}

	// Final gate: the engine is only usable if it verifies end to end.
	return VerifyFor(root, engine, goos, goarch)
}

// installRuntime fetches the platform archive and extracts the transcribe
// binary and its library into root/bin, verifying each against its hash.
func (i *Installer) installRuntime(root string, engine Engine, r Runtime, done, total int) error {
	binOK := verifyHash(BinaryPath(root, r), r.BinarySHA256) == nil
	libOK := r.Library == "" || verifyHash(LibraryPath(root, r), r.LibrarySHA256) == nil
	if binOK && libOK {
		return nil
	}

	tmp, err := os.MkdirTemp("", "zowe-runtime-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	archive := filepath.Join(tmp, r.Archive)
	url := r.URL()
	// The archive itself is verified against its pinned hash when we have one,
	// so a truncated download cannot pass as complete.
	if err := i.downloadFile(url, archive, r.ArchiveSHA256, r.Archive, done, total); err != nil {
		return err
	}

	extractDir := filepath.Join(tmp, "x")
	if err := i.extract(archive, extractDir); err != nil {
		return err
	}

	binSrc, err := findFile(extractDir, r.Binary)
	if err != nil {
		return fmt.Errorf("locate %s in %s: %w", r.Binary, r.Archive, err)
	}
	if err := installFile(binSrc, BinaryPath(root, r), r.BinarySHA256); err != nil {
		return err
	}

	if r.Library != "" {
		libSrc, err := findFile(extractDir, r.Library)
		if err != nil {
			return fmt.Errorf("locate %s in %s: %w", r.Library, r.Archive, err)
		}
		if err := installFile(libSrc, LibraryPath(root, r), r.LibrarySHA256); err != nil {
			return err
		}
	}
	return nil
}

// installFile moves one extracted file into place, checking its hash first and
// marking the binary executable.
func installFile(src, dest, want string) error {
	if err := verifyHash(src, want); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	_ = os.Remove(dest)
	return os.Rename(src, dest)
}

// download fetches a model file to dest, verifying the hash.
func (i *Installer) download(url, dest, want, name string, done, total int) error {
	return i.downloadFile(url, dest, want, name, done, total)
}

// downloadFile streams url into dest, hashing as it goes, and only publishes
// the result when it matches want. It writes to a temporary file first so a
// partial download never looks like a finished one.
func (i *Installer) downloadFile(url, dest, want, name string, done, total int) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	resp, err := i.client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: status %s", url, resp.Status)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".download-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	sum := sha256.New()
	wrapped := &progressWriter{
		w:      io.MultiWriter(tmp, sum),
		name:   name,
		report: i.Progress,
		done:   done,
		total:  total,
		size:   resp.ContentLength,
	}
	if _, err := io.Copy(wrapped, resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", dest, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if want != "" && !strings.EqualFold(hex.EncodeToString(sum.Sum(nil)), want) {
		return fmt.Errorf("%w: %s", ErrChecksum, filepath.Base(dest))
	}
	_ = os.Remove(dest)
	return os.Rename(tmpName, dest)
}

// progressWriter reports bytes written so a download can show a bar. It emits
// at most once per whole percent (and once at the end), because a naive
// per-write callback fires thousands of times for a 53 MB model and would
// flood any listener that forwards it to a display.
type progressWriter struct {
	w             io.Writer
	name          string
	report        func(Progress)
	done          int
	total         int
	size          int64
	seen          int64
	lastPct       int
	lastEmitBytes int64
	reported      bool
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.seen += int64(n)
	if p.report == nil {
		return n, err
	}

	if p.size <= 0 {
		// Unknown length: emit roughly every megabyte rather than staying
		// silent, so a caller still sees movement on a server that omits it.
		if p.seen-p.lastEmitBytes >= 1<<20 {
			p.lastEmitBytes = p.seen
			p.report(Progress{File: p.name, Done: p.done, Total: p.total, Bytes: p.seen})
		}
		return n, err
	}

	pct := int(p.seen * 100 / p.size)
	if p.reported && pct == p.lastPct {
		return n, err
	}
	p.reported = true
	p.lastPct = pct
	p.report(Progress{
		File:       p.name,
		Done:       p.done,
		Total:      p.total,
		Bytes:      p.seen,
		TotalBytes: p.size,
	})
	return n, err
}
