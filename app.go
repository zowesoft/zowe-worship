package main

import (
	"context"
	"database/sql"
	"log"

	"zowe-worship/internal/bible"
	"zowe-worship/internal/database"
)

type App struct {
	ctx      context.Context
	dataRoot string
	core     *sql.DB
	bible    *bible.SQLiteRepository
}

type AppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func NewApp() *App {
	return &App{}
}

// newAppAt lets tests point storage at a temporary directory.
func newAppAt(dataRoot string) *App {
	return &App{dataRoot: dataRoot}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	paths, err := a.paths()
	if err != nil {
		log.Printf("zowe: storage unavailable: %v", err)
		return
	}
	if err := paths.Ensure(); err != nil {
		log.Printf("zowe: storage unavailable: %v", err)
		return
	}

	core, err := database.Open(ctx, paths.CoreDatabase())
	if err != nil {
		log.Printf("zowe: open database: %v", err)
		return
	}
	if err := database.Migrate(ctx, core, paths.Backups()); err != nil {
		_ = core.Close()
		log.Printf("zowe: migrate database: %v", err)
		return
	}

	a.core = core
	a.bible = bible.NewSQLiteRepository(core, paths.Translations())
}

func (a *App) shutdown(_ context.Context) {
	if a.core == nil {
		return
	}
	if err := a.core.Close(); err != nil {
		log.Printf("zowe: close database: %v", err)
	}
	a.core = nil
}

func (a *App) paths() (database.Paths, error) {
	if a.dataRoot != "" {
		return database.Paths{Root: a.dataRoot}, nil
	}
	return database.DefaultPaths()
}

func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:    "Zowe Worship",
		Version: "0.1.0",
	}
}
