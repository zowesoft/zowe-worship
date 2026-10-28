package database

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const appDirName = "Zowe Worship"

// Paths describes the per-user, local-only storage layout. Translation packages
// are large and user-downloadable, so they never live under roaming AppData.
type Paths struct {
	Root string
}

func DefaultPaths() (Paths, error) {
	root, err := defaultRoot()
	if err != nil {
		return Paths{}, err
	}
	return Paths{Root: root}, nil
}

func defaultRoot() (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve local app data: %w", err)
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "Zowesoft", appDirName), nil

	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "Zowesoft", appDirName), nil

	default:
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve home directory: %w", err)
			}
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "zowesoft", "zowe-worship"), nil
	}
}

func (p Paths) CoreDatabase() string {
	return filepath.Join(p.Root, "zowe.db")
}

func (p Paths) Translations() string {
	return filepath.Join(p.Root, "translations")
}

func (p Paths) Backups() string {
	return filepath.Join(p.Root, "backups")
}

func (p Paths) Ensure() error {
	for _, dir := range []string{p.Root, p.Translations(), p.Backups()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create data directory %s: %w", dir, err)
		}
	}
	return nil
}
