package main

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"zopresent/internal/database"
	"zopresent/internal/settings"
)

// settingsWindow doubles as the reuse key: opening Settings again moves the
// existing window instead of stacking another.
const settingsWindow = "settings"

// maxBackupRows caps the list; BackupCount still reports the whole folder.
const maxBackupRows = 8

type BackupView struct {
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
	CreatedAt string `json:"createdAt"`
}

type StorageView struct {
	Version            string            `json:"version"`
	DataRoot           string            `json:"dataRoot"`
	DatabaseBytes      int64             `json:"databaseBytes"`
	MediaBytes         int64             `json:"mediaBytes"`
	MediaItems         int               `json:"mediaItems"`
	Translations       []TranslationView `json:"translations"`
	DefaultTranslation string            `json:"defaultTranslation"`
	Backups            []BackupView      `json:"backups"`
	BackupCount        int               `json:"backupCount"`
	// Problem says why storage could not be opened at startup, if it could not.
	Problem string `json:"problem,omitempty"`
}

// OpenSettings opens even when storage failed, which is exactly when the
// operator needs to see where the app looked.
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
	if a.storageProblem != nil {
		view.Problem = a.storageProblem.Error()
	}

	coreSize, err := fileSize(paths.CoreDatabase())
	if err == nil {
		view.DatabaseBytes = coreSize
	}

	// The folder holds thumbnails too, so bytes come from disk and the item
	// count from the library.
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

	if a.settings != nil {
		view.DefaultTranslation = a.settings.GetDefault(a.context(), settings.KeyDefaultTranslation, "kjv")
	}

	return view, nil
}

// An unset key returns an empty string.
func (a *App) GetSetting(key string) (string, error) {
	if a.settings == nil {
		return "", errStorageUnavailable
	}
	return a.settings.GetDefault(a.context(), key, ""), nil
}

func (a *App) SetSetting(key, value string) error {
	if a.settings == nil {
		return errStorageUnavailable
	}
	return a.settings.Set(a.context(), key, value)
}

// oneVersePerSlide defaults to true for anything but "all", so a missing or
// malformed setting still gives the readable wall-sized verse.
func (a *App) oneVersePerSlide(ctx context.Context) bool {
	if a.settings == nil {
		return true
	}
	return a.settings.GetDefault(ctx, settings.KeyVersesPerSlide, "one") != "all"
}

func (a *App) SetDefaultTranslation(id string) (StorageView, error) {
	if a.settings == nil {
		return StorageView{}, errStorageUnavailable
	}
	if err := a.settings.Set(a.context(), settings.KeyDefaultTranslation, id); err != nil {
		return StorageView{}, err
	}
	return a.GetStorage()
}

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

// restoreMu serialises restores; two windows can call bindings at once.
var restoreMu sync.Mutex

// RestoreBackup swaps the live database for a named backup and reopens storage
// on it; media and translation packages live outside the core database and are
// untouched.
//
// The name must resolve inside the backups folder and pass an integrity check;
// the current data is backed up first; and the old files are kept until the
// restored copy opens, migrates and checks out — any failure rolls straight
// back.
func (a *App) RestoreBackup(name string) (StorageView, error) {
	if a.core == nil {
		return StorageView{}, errStorageUnavailable
	}
	restoreMu.Lock()
	defer restoreMu.Unlock()

	paths, err := a.paths()
	if err != nil {
		return StorageView{}, err
	}
	if base := filepath.Base(strings.TrimSpace(name)); base != strings.TrimSpace(name) || base == "." {
		return StorageView{}, fmt.Errorf("restore a copy from the backups folder by name")
	}
	backup := filepath.Join(paths.Backups(), strings.TrimSpace(name))
	info, err := os.Stat(backup)
	if err != nil || info.IsDir() {
		return StorageView{}, fmt.Errorf("backup %q was not found in the backups folder", name)
	}

	if err := database.CheckRestoreCandidate(a.context(), backup); err != nil {
		return StorageView{}, err
	}
	if _, err := database.BeforeRestore(a.context(), a.core, paths.Backups()); err != nil {
		return StorageView{}, fmt.Errorf("back up the current data first: %w", err)
	}

	live := paths.CoreDatabase()
	keep := live + ".prior"
	// Clear keep-files a previous interrupted restore may have left; the
	// current data is snapshotted already.
	_ = os.Remove(keep)
	database.DiscardDatabase(keep)

	if err := a.core.Close(); err != nil {
		return StorageView{}, fmt.Errorf("close the live database: %w", err)
	}
	a.core = nil

	err = database.ReplaceDatabase(live, backup, keep)
	if err == nil {
		var restored *sql.DB
		restored, err = database.Open(a.context(), live)
		if err == nil {
			if err = database.Migrate(a.context(), restored, paths.Backups()); err == nil {
				if err = database.CheckIntegrity(a.context(), restored); err == nil {
					a.rewireStores(restored, paths)
					database.DiscardDatabase(keep)
					return a.GetStorage()
				}
			}
			_ = restored.Close()
		}
	}

	// The copy-in, the open or the migrate failed: undo the swap.
	rollErr := database.RollbackDatabase(live, keep)
	if rollErr != nil {
		return StorageView{}, fmt.Errorf("restore failed (%v) and the previous database could not be returned (%v); the files are kept, see the backups folder", err, rollErr)
	}
	reopened, reopenErr := database.Open(a.context(), live)
	if reopenErr != nil {
		return StorageView{}, fmt.Errorf("restore failed (%v) and the previous database would not reopen (%v)", err, reopenErr)
	}
	a.rewireStores(reopened, paths)
	return StorageView{}, fmt.Errorf("restore failed: %w", err)
}

func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// folderBytes counts a missing folder as empty: nothing imported yet is normal,
// not a fault.
func folderBytes(dir string) (int64, error) {
	var total int64

	err := filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
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
