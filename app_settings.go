package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"zowe-worship/internal/database"
)

// settingsWindow names the settings window so opening it again moves the same
// window instead of stacking a second one.
const settingsWindow = "settings"

// maxBackupRows keeps the backup list to the copies an operator can scan at a
// glance; the folder may hold more, and the count says so.
const maxBackupRows = 8

type BackupView struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt"`
}

// StorageView answers the two questions Settings exists for: where the
// church's data sits on this computer, and whether it is safe.
type StorageView struct {
	Version       string            `json:"version"`
	DataRoot      string            `json:"dataRoot"`
	DatabaseBytes int64             `json:"databaseBytes"`
	MediaBytes    int64             `json:"mediaBytes"`
	MediaItems    int               `json:"mediaItems"`
	Translations  []TranslationView `json:"translations"`
	Backups       []BackupView      `json:"backups"`
	BackupCount   int               `json:"backupCount"`
}

// OpenSettings shows the storage and library summary in its own window. It
// opens even when storage failed, because that is exactly when the operator
// needs to see where the app looked.
func (a *App) OpenSettings() (EditorView, error) {
	return openUtilityWindow(settingsWindow, "Settings", "/?view=settings", 680, 620, 520, 460)
}

func (a *App) GetStorage() (StorageView, error) {
	paths, err := a.paths()
	if err != nil {
		return StorageView{}, err
	}

	view := StorageView{
		Version:  appVersion,
		DataRoot: paths.Root,
	}

	coreSize, err := fileSize(paths.CoreDatabase())
	if err == nil {
		view.DatabaseBytes = coreSize
	}

	// The folder holds thumbnails beside the imported files, so the bytes are
	// read from disk and the item count from the library.
	mediaBytes, err := folderBytes(paths.Media())
	if err == nil {
		view.MediaBytes = mediaBytes
	}
	if a.media != nil {
		assets, err := a.media.List(a.context())
		if err == nil {
			view.MediaItems = len(assets)
		}
	}

	if a.bible != nil {
		translations, err := a.ListTranslations()
		if err == nil {
			view.Translations = translations
		}
	}

	backups, count, err := listBackups(paths.Backups())
	if err == nil {
		view.Backups = backups
		view.BackupCount = count
	}

	return view, nil
}

// BackupNow writes a snapshot the operator asked for and returns the refreshed
// view, so the new file appears in the list without a second call.
func (a *App) BackupNow() (StorageView, error) {
	if a.core == nil {
		return StorageView{}, errStorageUnavailable
	}

	paths, err := a.paths()
	if err != nil {
		return StorageView{}, err
	}
	if _, err := database.ManualBackup(a.context(), a.core, paths.Backups()); err != nil {
		return StorageView{}, err
	}
	return a.GetStorage()
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// folderBytes totals the files under a directory. A missing folder is reported
// as empty rather than as an error: nothing has been imported yet is a normal
// state, not a fault.
func folderBytes(dir string) (int64, error) {
	var total int64

	err := filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			// A folder that is not there yet simply holds nothing.
			if os.IsNotExist(err) {
				return fs.SkipAll
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func listBackups(dir string) ([]BackupView, int, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []BackupView{}, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}

	var backups []BackupView
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, BackupView{
			Name:      entry.Name(),
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}

	sort.Slice(backups, func(i, j int) bool { return backups[i].CreatedAt > backups[j].CreatedAt })

	count := len(backups)
	if len(backups) > maxBackupRows {
		backups = backups[:maxBackupRows]
	}
	if backups == nil {
		backups = []BackupView{}
	}
	return backups, count, nil
}
