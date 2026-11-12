package main

import "embed"

// assets is the built frontend, baked into the binary so the app needs nothing
// else on the machine it runs on.
//
//go:embed all:frontend/dist
var assets embed.FS
