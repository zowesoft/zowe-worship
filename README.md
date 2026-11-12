# Zopresent

**Scripture-first worship presentation for a single operator and one audience display.**

Zopresent is a desktop application for church worship services: one operator
runs the service — scripture, songs, media and themes — from a studio window,
while the audience display shows exactly what is on air. It works fully
offline, including its AI scripture detection, and it never ships copyrighted
Bible translations: you install the texts you are licensed to use.

---

## Features

**Service order and live presentation** — build a service from scripture,
songs, media and custom items; send any item preview or live with a keystroke
(Ctrl+L preview, F8 listen, F9/F12 transport). A separate audience window
mirrors the live output, and a decision history lets the operator restore
anything that was already projected.

**Bible library** — install the translations you hold licenses for through the
command-line importer (nothing copyrighted ships with the app). Reference
entry (`Jn 3:16`), full-text word/phrase/prefix search, verse ranges, and one
keypress to air the top result. License and attribution lines are recorded per
translation and shown in Settings. Reference lookups answer in single-digit
milliseconds; the search targets are enforced by a benchmark
(`cmd/bible-bench`).

**Songs** — song library with multiple arrangements and sections; add a whole
arrangement or a single section to the service; preview and project sections.

**Media** — image, audio and video library with a transport (play, pause, seek,
volume, stop), audio/video assets that can carry a background picture, and an
editor that doubles as the player when no second display is connected.

**Themes** — a full theme editor (typography, size, spacing, alignment,
colours, backgrounds, reference placement) with live preview. A theme is
*resolved and frozen* into each projected slide, so editing a theme can never
restyle content that is already on air. Slides auto-fit text through
container-query sizing.

**Offline AI scripture detection** — while listening, on-device speech
recognition watches the preacher's words, detects scripture references as they
are spoken, and queues them for one-click approval with an explicit confidence
statement. Everything runs locally; no audio, transcript or text ever leaves
the machine. Auto-projection is gated behind operator-configurable safety
checks and is off by default.

**Desktop foundation** — checksum-tracked SQLite migrations, automatic
backups, restore-into-the-running-app with rollback, refusal to open a
database from a newer build (rather than corrupting it), a single-instance
guard that focuses the running studio instead of starting a second one, and
per-user settings with typed keys.

---

## Installation

Download the installer for your platform from the
[releases page](https://github.com/zowesoft/zopresent/releases):

| Platform | Package |
|---|---|
| Windows | NSIS installer (`.exe`) |
| Linux | `.deb`, `.rpm` or `.AppImage` (GTK 4 + WebKitGTK 6.0) |
| macOS | `.app` bundle (universal, ad-hoc signed) |

Packages are currently **unsigned** — SmartScreen (Windows) or Gatekeeper
(macOS) will ask for confirmation on first run.

Per-user data (database, backups, installed translations, media library) lives
under the OS convention: `%LOCALAPPDATA%\Zowesoft\Zopresent` on Windows.

### Installing Bible translations

The app deliberately bundles **no** Bible text — translations are copyrighted
works and redistribution requires the publisher's permission. Install your own
copies with the importer:

```sh
go run ./cmd/bible-import \
  -source /path/to/NIV_bible.json \
  -id niv -name "New International Version" -abbr NIV \
  -license "Copyrighted" \
  -attribution "Holy Bible, New International Version, NIV. Copyright 1973, 1978, 1984, 2011 by Biblica, Inc."
```

The importer verifies the dataset is complete (all 66 books), prints the
copyright holder as a notice for restricted translations, and records the
license and attribution line that Settings will display. Public-domain
translations (KJV, for example) are pre-registered in
`internal/bible/licenses.go`.

---

## Building from source

Prerequisites:

- Go 1.26+
- Node.js 20.19+ or 22.12+ (npm)
- The Wails CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`
- Platform extras: NSIS (Windows installer), `libgtk-4-dev libwebkitgtk-6.0-dev pkg-config` (Linux)

```sh
# production binary (frontend is embedded)
wails3 task build

# installer for the host OS (NSIS / deb+rpm+AppImage / .app)
wails3 task package

# development mode with hot reload
wails3 task dev
```

On Windows, `scripts/rebuild.sh` does the full loop — build, clean side
effects, relaunch, verify the window came up — in one command:

```sh
scripts/rebuild.sh
```

### Tests and benchmarks

```sh
go test ./...                 # Go tests
cd frontend && npm run typecheck   # TypeScript

go run ./cmd/bible-bench      # reference/search latency targets (p95)
go run ./cmd/asr-bench -set benchmarks/sample-set   # scripture-detection benchmark
```

The bible benchmark fails (non-zero exit) if any workload misses its target:
reference lookups p95 < 50 ms, full-text search p95 < 150 ms.

---

## Releases

Tagging a commit (`git tag v0.1.0 && git push origin v0.1.0`) starts the
release workflow: each desktop OS is packaged on its native GitHub runner and
the artifacts are attached to the GitHub release. The offline speech engine is
bundled into every package — the Windows installer, the Linux deb/rpm/AppImage
and the macOS .app.

---

## Project layout

```
app*.go, main.go, display.go   application layer exposed to the frontend
internal/                      domain packages (bible, songs, media, themes,
                               presentation, serviceorder, detection, asr,
                               speech, database, settings, confidence, policy)
cmd/bible-import               translation dataset importer
cmd/bible-bench                bible latency benchmark
cmd/asr-bench                  scripture-detection benchmark
cmd/zopresent-engine                offline speech-engine installer/fetcher
frontend/                      React + TypeScript UI (Vite)
build/                         per-platform packaging (NSIS, nfpm, darwin)
docs/                          feature implementation tracker
```

---

## License

Copyright © 2026 Zowesoft. All rights reserved. The source-available licence in
`LICENSE` — free to use, source visible, no redistribution, no reverse
engineering — governs the software; it is not an OSI open-source licence and
has not had legal review. Bible translations installed through the importer remain subject to
their own copyright holders — note that several popular translations (NIV,
ESV, NLT, NASB) may **not** be redistributed without their publishers'
permission, which is why the app ships none of them.
