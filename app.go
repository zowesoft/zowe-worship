package main

import "context"

type App struct {
	ctx context.Context
}

type AppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetAppInfo() AppInfo {
	return AppInfo{
		Name:    "Zowe Worship",
		Version: "0.1.0",
	}
}
