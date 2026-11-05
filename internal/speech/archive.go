package speech

import (
	"archive/tar"
	"compress/bzip2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractTarBz2 unpacks a .tar.bz2 release archive into dest. The bzip2 layer
// is just decompression; the tar walk below is where the safety lives.
func extractTarBz2(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTar(bzip2.NewReader(f), dest)
}

// extractTar writes a tar stream into dest. Entries are constrained to dest: an
// archive is untrusted input, so a member whose path escapes the destination
// (via ".." or an absolute path) is refused rather than written.
func extractTar(r io.Reader, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	root, err := filepath.Abs(dest)
	if err != nil {
		return err
	}

	tr := tar.NewReader(r)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}

		target, err := safeJoin(root, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode)&0o777)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		default:
			// Symlinks, devices and the like are not part of a release we
			// consume; skipping them keeps a hostile archive inert.
		}
	}
}

// safeJoin resolves name under root and refuses anything that lands outside it.
// Archive entry names are POSIX-style regardless of host OS, so both "/" and
// the host separator are treated as absolute-path markers.
func safeJoin(root, name string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(name))
	if strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "\\") ||
		filepath.IsAbs(cleaned) || filepath.VolumeName(cleaned) != "" {
		return "", fmt.Errorf("archive entry escapes destination: %q", name)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("archive entry escapes destination: %q", name)
	}
	target := filepath.Join(root, cleaned)
	// Belt and braces: the joined path must still sit under root.
	if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
		return "", fmt.Errorf("archive entry escapes destination: %q", name)
	}
	return target, nil
}

// findFile locates the first regular file named name anywhere under root. The
// release archives nest the members under a versioned folder, so the search is
// by file name rather than a fixed path.
func findFile(root, name string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == name {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s not found in archive", name)
	}
	return found, nil
}
