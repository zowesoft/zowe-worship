# Zowe Worship — Feature Implementation Tracker

Living status of the work described in `Zowe_Worship_Full_PRD_and_Design_Guide.txt`.
The PRD describes intent; this file records what is
actually built, where it lives, and what is still open, so any agent can pick up
the next slice without re-deriving the state of the repo.

Update this file whenever a slice lands. Keep the claim verifiable: name the
files that prove it, and re-run the verification commands below before marking a
row **Done**.

## Verification baseline

Last verified: **2026-11-09** on branch `main`.

| Command | Result |
| --- | --- |
| `CGO_ENABLED=0 go build ./...` | pass |
| `CGO_ENABLED=0 go test ./...` | pass (all packages; root package 40.0 s) |
| `cd frontend && npm run typecheck` | pass (`tsc --noEmit`) |
| `cd frontend && npm run build` | pass (typecheck + production bundle) |

The generated bindings were refreshed from the current services on 2026-11-09
(commit `1d4dd85`); before that the committed TypeScript lagged `media_control.go`
and failed typecheck. Keep the bindings in the same change as a Go API edit.

Real Bible data is exercised by `internal/bible/integration_test.go`, which is
skipped unless `ZOWE_BIBLE_DATASETS` points at a directory holding `KJV/` (and
optionally `AMP/`). Treat reference/phrase/prefix search as *unverified against
real text* on a machine without that env var.

## Status legend

- **Done** — implemented, wired to the UI, and covered by tests or a documented
  manual check.
- **Partial** — working slice exists but a named part of the scope is
  missing (listed).
- **Planned** — not started. Empty packages (`.gitkeep` only) do not count as
  implementation.

## Workstream summary

| ID | Feature group | Status | Where |
| --- | --- | --- | --- |
| F01 | Desktop foundation and reliability | **Partial** | `main.go`, `app.go`, `internal/database`, `internal/settings`, `app_settings.go` |
| F02 | Bible library and translation search | **Partial** | `internal/bible`, `internal/scripture`, `app.go`, `frontend/src/features/bible` |
| F03 | Service order and multi-display presentation | **Partial** | `internal/presentation`, `internal/serviceorder`, `internal/history`, `display.go`, `frontend/src/features/service-order`, `frontend/src/features/display`, `frontend/src/features/presentation` |
| F04 | Songs and lyric presentation | **Partial** | `internal/songs`, `frontend/src/features/songs` |
| F05 | Media library and playback | **Partial** | `internal/media`, `app_media.go`, `frontend/src/features/media` |
| F06 | Custom theme editor | **Done** | `internal/themes`, `app_themes.go`, `frontend/src/features/themes`, `frontend/src/components/ThemedSlide.tsx` |
| F07 | Offline AI scripture detection | **Partial** | `internal/asr`, `internal/detection`, `app_detection.go`, `frontend/src/features/detection`; `cmd/asr-bench` + `benchmarks/` groundwork |
| F08 | Semantic scripture and song search | **Planned** | `internal/search` (empty); no model/index work |
| F09 | Online lyrics discovery (Genius) | **Planned** | no package, no UI |
| F10 | EasyWorship migration | **Planned** | no package, no UI |

## F01 — Desktop foundation and reliability (Partial)

Done:
- [x] Wails v3 shell, typed bindings under `frontend/bindings/`, Taskfile build
      (`Taskfile.yml`); commit `b8f1c2d`.
- [x] SQLite via `modernc.org/sqlite` with FTS5; versioned migrations
      `internal/database/migrations/001..017`, checksum-tracked (`migrate.go`).
- [x] App lifecycle wiring, storage paths, backups (`internal/database/backup.go`,
      `app_settings.go` `BackupNow` / `GetStorage`).
- [x] Restore a backup into the running app: the candidate is integrity-checked,
      the current data is snapshotted first, and the swap rolls back if the
      restored copy will not open or migrate (`app_settings.go` `RestoreBackup`,
      `internal/database/restore.go`, Settings → Backups).
- [x] Settings store with typed keys and fallbacks (`internal/settings/store.go`,
      `KeyDefaultTranslation`, `KeyVersesPerSlide`).
- [x] Utility-window helper for editors (`windows.go` `openUtilityWindow`).
- [x] Single-instance guard (`main.go` `SingleInstance`, id
      `com.zowesoft.zowe-worship`). A second launch is refused and the running
      studio is restored and focused instead. Keyed on the identifier, not the
      executable name, so a renamed copy is refused too. Two studios would each
      hold their own preview/live state while driving one projector and one
      database.
- [x] Downgrade refusal: `Migrate` refuses a database whose applied migrations
      this build does not know (`SchemaAheadError`), and the reason shows in
      Settings rather than storage failing silently (`app.go`
      `storageProblem`, `StorageView.Problem`).
- [x] Cross-platform packaging wired: per-platform Taskfiles for macOS
      (`build/darwin` — .app bundle, universal binary, DMG task) and Linux
      (`build/linux` — deb/rpm/AppImage via nfpm, .desktop entry), with the
      app's identity (com.zowesoft.zowe-worship) in Info.plist, nfpm.yaml and
      the desktop file. `.github/workflows/release.yml` builds all three OSes
      on native GitHub runners on every `v*` tag and attaches the packages to
      the release — no local Mac or Linux machine needed.

Open:
- [ ] First tagged release must prove the workflow end to end: packages built
      on each runner, attached to the GitHub release, and at least smoke-run
      on a real macOS/Linux machine. The repo is not on GitHub yet.
- [ ] Package signing (Windows Authenticode, macOS Developer ID + notarization)
      — the workflow ships unsigned packages until certificates exist.
- [ ] Bundling the offline speech engine into the Linux/macOS packages (the
      Windows installer already stages it; the darwin/linux Taskfiles do not
      yet copy `common:engines:stage` output into their layouts).

## F02 — Bible library and translation search (Partial)

Done:
- [x] Per-translation SQLite packages installed into `internal/bible` with FTS5
      (`internal/bible/sqlite.go`, `repository.go`, `package.go`).
- [x] Reference parser and canonical resolution (`internal/scripture/reference.go`,
      `internal/bible/books.go`).
- [x] Search modes: words, phrase, prefix, with testament/book filters, snippets
      and pagination (`internal/bible/model.go` `SearchRequest`, `SearchResult`
      `Snippet`; exposed via `App.SearchBible`, `App.BrowsePassage`).
- [x] Workspace UI with search, highlight, passage browsing as stacked chapter
      blocks, testament filter, pagination (`frontend/src/features/bible/`).
- [x] Bible tab renders Genesis 1 from app load and continues scrolled reading.
- [x] Relaxed multi-term ranking for the default full-text ("words") search
      (`internal/bible/sqlite.go` `searchRelaxed`). Two stages: prefix-AND first
      (every word required, matched as a prefix so "love" reaches "loved"), then
      — only when nothing carries every word — prefix-OR ranked by bm25. The
      installed packages tokenize with `unicode61` (no stemmer), so prefixing is
      how we bridge inflections without rebuilding every translation database.
      Measured against the real `kjv-1.0.0.db` (31,102 verses):
      - "God love world": exact-AND found 4 verses and missed John 3:16;
        prefix-AND finds 5 with John 3:16 at #2 (behind 1 John 4:9, which also
        carries all three words).
      - "believe saved": exact-AND 5 → prefix-AND 8, reaching "believeth", with
        Mark 16:16 on top.
      - "God loved shepherd" (no verse has all three): stage 1 empty, stage 2
        widens and every top hit carries two of the three words.
- [x] Guided reference entry as inline autocomplete
      (`frontend/src/features/bible/referenceGuide.ts` + `BibleWorkspace.tsx`).
      While the operator is still naming a book in "auto" mode, the field ghosts
      the rest of the matching book name in muted text right after the caret
      (no dropdown). Tab, Enter, or Right-at-end accepts the ghost into the
      canonical name plus a trailing space; a space typed right after a chapter
      number immediately becomes the colon ("John 3" + space → "John 3:"), so
      the operator types book → chapter → verse without reaching for a key.
      Numbered books are reached through their number ("1 jo" ghosts "1 John").
- [x] One keypress to air scripture. With results on screen and nothing still
      in flight, Enter sends the top result straight to the live output; double
      clicking a row sends that specific row live. Single click still previews.
      The live path ignores the in-flight "busy" gate, because the first click
      of a double-click has already started a preview by the time the dblclick
      lands (that gate is what previously swallowed the double-click).
- [x] License and attribution metadata per translation: a registry of known
      translations' copyright facts (`internal/bible/licenses.go`), columns on
      `bible_translations` (migration `017`), and the line shown under each
      translation in Settings (`SettingsWindow.tsx`).
- [x] Bundling gate and integrity checks in `cmd/bible-import`: a complete
      dataset must carry all 66 canonical books (`bible.MissingBooks`), the
      license/attribution can be stated with flags (a known translation falls
      back to its registry entry), and importing a copyrighted translation
      prints its copyright holder as a notice.
- [x] Performance targets measured and enforced by `cmd/bible-bench` (reference
      p95 < 50 ms, full-text p95 < 150 ms; exits non-zero on a miss). Measured
      over 40 runs per workload after 3 warmups, on the real installed packages:
      - NIV (31,087 verses): references p95 4.1–6.0 ms; searches p95 11.8–31.9 ms
        ("shepherd still waters" worst at 31.9 ms).
      - KJV (31,102 verses): references p95 6.0–9.0 ms; searches p95 10.8–30.9 ms.
      The widened-stage bm25 ordering from the earlier note remains the
      production behaviour; nothing regressed at 40 runs per workload.

Open:
- [ ] `NIV`, `ESV`, `NLT`, `NASB` packages — datasets import through
      `cmd/bible-import` from the operator's own copies; nothing is bundled
      with the installer until the copyright holders' permission is on file.
## F03 — Service order and multi-display presentation (Partial)

Done:
- [x] Preview/live controller with a single source of truth; preview never writes
      live directly (`internal/presentation/controller.go`; `sendLive` promotes
      preview → live only on success).
- [x] Service order: services CRUD, items, reorder, duplicate, notes, duration,
      per-item status, custom kinds (`internal/serviceorder`, migrations
      `002_services.sql`, `013_service_order_items_ext.sql`).
- [x] Item kinds: scripture, song, media, text, announcement, blank
      (`internal/presentation/controller.go` `contentForItem`).
- [x] Schedule a passage, split into one row per verse when the verses-per-slide
      setting is "one" (`app.go` `AddScriptureToService`).
- [x] Audience display on a second window with its own surface
      (`display.go`, `frontend/src/features/display/AudienceDisplay.tsx`).
- [x] Persistent production bar: preview/live, previous/next, clear, send live
      (`frontend/src/layouts/AppShell.tsx`).
- [x] Configurable operator shortcuts, stored per machine and rebound in
      Settings → Shortcuts (`app_shortcuts.go`, `frontend/src/lib/shortcuts.ts`,
      `SettingsWindow.tsx`; commit `6f3f5a4`). Defaults: Ctrl+L send live,
      Ctrl+F Bible search, F8 listening, F9 auto-projection, F12 emergency,
      ←/→ previous/next schedule item. F9 pauses/resumes auto-projection.
- [x] Presentation history writing — every command that changes live output
      records a row (`app.go` `projected`/`recordProjection`, `internal/history`),
      exposed through `App.RecentProjections` and shown in
      `frontend/src/features/presentation/HistoryPanel.tsx`. Preview commands
      deliberately do not write. Migration
      `015_presentation_history_kinds.sql` drops the stale `kind` CHECK that
      predated text/announcement/blank and the split media kinds.
- [x] Reopen a past projection from history: a row carrying a schedule item puts
      that item back on the wall live and writes a fresh history row
      (`app.go` `RestoreProjection`, `HistoryPanel.tsx`; commit `2308a8a`).

Open:
- [ ] Mixed-DPI, disconnect/reconnect recovery tests for the audience display.
- [ ] Additional audience outputs (intentionally deferred past single-output).
- [ ] Acceptance evidence for navigation-not-interrupting-live, display error
      preserving last live slide, etc.

## F04 — Songs and lyric presentation (Partial)

Done:
- [x] Song library: create/edit/duplicate/delete, sections, arrangements,
      CCLI/copyright metadata (`internal/songs`, migration
      `014_song_arrangements_and_meta.sql`; commit `194562f`).
- [x] Song FTS lookup over titles and lyrics (`song_fts`).
- [x] Lyric slides projected through the presentation controller; add sections or
      arrangements to a service order (`app.go` `AddSongSectionToService`,
      `AddSongArrangementToService`, `PreviewSongSection`, `ProjectSongSection`).
- [x] Songs workspace + editor window (`frontend/src/features/songs/`).

Open:
- [ ] Acceptance evidence: preview lyric slides and advance the intended section
      without changing live output during editing.
- [ ] Confirm song FTS is exercised by tests against real lyrics (currently
      fixture-level).

## F05 — Media library and playback (Partial)

Done:
- [x] Import images/video/audio, store metadata and availability, list, delete
      (`internal/media`, `app_media.go`; migrations `005..007`).
- [x] Image framing (aspect ratio / crop) and editor window
      (`frontend/src/features/media/MediaEditorWindow.tsx`).
- [x] Project images, video and audio as slides; transport play/pause/restart
      in the studio (`app_media.go` `ProjectMedia`, `media_control.go`).
- [x] Full transport end to end: clock, seek, stop and room volume in the studio
      (`AppShell.tsx`), all six commands validated in `media_control.go` and
      acted on by the player (`useTransportPlayer`, `AudienceDisplay.tsx`).
- [x] Playback without a display: the studio's live pane is the player when no
      audience display is connected (`AppShell.tsx`, broadcast `MediaControl`).
- [x] Local preview playback with play/pause and seek, separate from the room
      (`AppShell.tsx` preview transport).
- [x] Background pictures for audio/video clips, picked in the media editor
      (`internal/media/slides.go` `SetBackground`, `MediaEditorWindow.tsx`).
- [x] Relink a missing asset (`App.RelinkMedia`).

Open:
- [ ] Explicit declared-format/codec list surfaced to the user.
- [ ] Acceptance evidence: missing file, unsupported codec, relink, failure without
      blanking a valid live slide.

## F06 — Custom theme editor (Done)

Done:
- [x] Create/duplicate/edit/save/delete scripture, lyrics and text themes
      (`internal/themes`, `app_themes.go`; migrations `008_slide_templates.sql`,
      `011_slide_theme_reference_style.sql`).
- [x] Typography, size, line spacing, alignment, colour, background,
      reference/copyright placement, per-item reference style.
- [x] Live theme preview; resolves and **freezes** a theme into each projected
      slide so an edited theme cannot restyle live content
      (`internal/presentation/controller.go` `themeFor` / `Content.Theme`).
- [x] Shared `ThemedSlide` component so editor preview, studio monitor and
      audience window render identically (`frontend/src/components/ThemedSlide.tsx`;
      container-query sizing lives inline as Tailwind arbitrary classes).
- [x] Per-item theme override with operator picker (`App.SetItemTheme`,
      `AppShell.tsx` `PreviewTheme`).
- [x] Editing a theme refreshes the studio preview through the `presentation`
      event (`app_themes.go` `themeChange`, `display.go` `publishToDisplay`,
      `StudioProvider.tsx` `Events.On('presentation')`).
- [x] Overflow handled via `container-type: size` + `cqh`/`cqw` units.

Open:
- [ ] Arbitrary-script rejection is by construction (themes are data, no code
      fields) — no explicit test asserting a malicious theme payload is refused.
- [ ] Acceptance evidence: theme overflow across output aspect ratios captured.

## F07 — Offline AI scripture detection (Partial)

First slice landed: mic → transcript → candidate → approval → preview. The
webview captures the input device (`getUserMedia` + AudioWorklet) into a
16 kHz mono Int16 ring buffer; **Analyze** (panel button) sends the last ~20 s
to `DetectScripture`, which wraps the PCM in a temp WAV, runs the engine spec
from settings, transcribes, and queues every reference `DetectReferences`
finds. `internal/detection` holds the candidate queue in memory and records
every approve/dismiss/clear in `detection_history` (previously unwritten).
Approving previews the passage in the default translation — the operator still
sends it live by hand. `F8` toggles listening, `F12` persists the
emergency disable (`detection_disabled`), and the engine command is set in
Settings → Live detection.

Candidates carry the detection method, an explanation sentence and a
confidence score, and `App.ListDetectionHistory` reads `detection_history` back
newest first for the panel's recent-decisions list (`app_detection.go`,
`internal/detection/queue.go`, and the detection panel in `AppShell.tsx`).
`internal/confidence` scores the phrasing the detector matched — a written
`3:16` (0.96) and spoken "chapter/verse" words (0.93) rank above bare numbers
(0.86), and a chapter-only hearing (0.72) ranks lowest because it still needs
narrowing. `confidence.BandOf` splits those at the PRD's starting edges (0.92
high, 0.65 medium), and the panel shows the percentage plus the spelled-out band
so color never carries the reading alone. These are evidence-strength scores
rather than measured accuracy, and auto-projection stays off; `cmd/asr-bench`
now scores them on labeled audio — band accuracy plus a threshold sweep from
`confidence.Sweep` — so the edges can be moved once real recordings exist.

`DetectScripture` now opens a numbered analysis segment, lets partials refine
only that segment's cards, and settles them on the final transcript with stable
IDs and versions. An identical hearing that arrives while a card is still
pending and within `RepeatWindow` (two minutes) folds into it (`repeats` on the
card) instead of queuing a duplicate the operator must dismiss twice; folding
is not an operator decision, so `detection_history` keeps only what the
operator actually decided.

Auto-projection runs through `internal/policy`, the only code allowed to send a
detection to the display (`app_detection.go` `autoProject`). The PRD's eight
safety gates are named checks — finalized segment, unambiguous reference,
passage exists in the selected translation, explicit intent, no competing
candidate, not a repeat, switched on and not paused, high confidence — and a
failing gate leaves the card in the operator's queue with the live slide
untouched. The switch is off by default and every start boots paused, so
nothing projects before the operator arms it; F9 pauses and resumes without
stopping audio capture, a successful automatic projection is recorded as
`auto_projected` in `detection_history`, and the panel names what went to the
room instead of the card silently vanishing. A card the gates hold back carries
the checklist itself — the gate that refused it and every gate's pass/fail — in
a collapsible block on the card, shown while auto-projection is armed so the
operator can see what to change instead of guessing.

Listening is continuous now: `segmenter.ts` feeds on the level the capture
already reports and closes a segment after 700 ms of quiet (or cuts at 25 s, so
a request never crosses the backend's 30 s cap). The microphone keeps running
through the service, a segment that arrives mid-transcription waits in one slot
rather than piling up requests, and the panel shows hearing / transcribing
instead of a manual button. Continuous mode is on by default and can be switched
off, in which case the "Analyze last 20 s" button works exactly as before.

The segmenter also decides what is *not* speech, so a quiet room or a backing
track does not reach the engine: the capture reports level plus a zero-crossing
rate per block, the level gate rises above a slowly learned room floor (so a
quiet passage under the fan noise is ignored), and blocks outside the speech
zero-crossing band are treated as cymbals or hiss. A stretch of 2.5 s or more
whose level barely moves is discarded as sustained music when it closes, which
keeps a worship band from producing candidates. These are heuristics tuned
offline, not a trained speech/music classifier: they need a listen on real
sanctuary audio before the thresholds are trusted.

Still open (later slices): moving the confidence edges onto real recorded
services (the sweep is in `cmd/asr-bench`, which needs those recordings), and
the empty `internal/audio` package. The shipped engine is
Moonshine (`internal/speech`, below); Whisper works only from a hand-typed spec
and has no model lifecycle of its own.

`F9` pauses and resumes auto-projection without stopping audio capture; the
Production card carries the standing switch, its Hybrid/Paused status, and the
plain statement of what the gates will and will not project.

### Engine provisioning (F07 slice — landed)

The engine is only *discovered* if it already sits on disk. Discovery now scans
four roots: `<cwd>/benchmarks`, `<exe dir>/benchmarks`, `<exe dir>/engines`,
and `<data root>/engines` (the last two are the bundled and downloaded homes).
Verified against the on-disk engines: the smoke clip transcribes and detects
Romans 8:28 on both whisper (`ggml-tiny.en`) and Moonshine
(`moonshine-tiny-en-int8`), and the app's own pipeline handles both specs end
to end (`app_detection_e2e_test.go`).

**Decision: bundle Moonshine in the installer; other engines are downloaded.**
Moonshine via sherpa-onnx is the one to ship — no per-model license gate,
small, ~16× realtime, and it is the only engine a first run needs.

#### Built: `internal/speech`

- `catalog.go` — the engine catalogue and the platform matrix. `Moonshine()`
  carries the five model files (one hash each, identical on every OS) and a
  runtime entry per GOOS/GOARCH with pinned archive + binary + library hashes.
  All six platform runtimes are pinned from the real v2.0.1 assets.
- `installer.go` — `Verify(root, engine)` hashes every file before an engine is
  trusted; `Install(root, engine)` downloads only what is missing or fails its
  hash, extracts the runtime archive, and re-verifies end to end. Progress is
  reported at most once per whole percent so a 53 MB model cannot flood a
  listener.
- `archive.go` — tar.bz2 extraction with a zip-slip guard: entry names are
  cleaned and resolved under the destination, and anything escaping it (via
  `..` or an absolute path) is refused. Symlinks/devices are skipped.

Proven end to end from the real releases: a fresh install fetched all seven
files from GitHub + HuggingFace, verified every hash, extracted the win-x64
runtime, and the installed engine transcribed the smoke clip and detected
**Romans 8:28**.

#### Settings UI

Settings → Live detection lists the catalogue through `ListSpeechEngines`
(installed / bundled / active / supported per row). **Get engine** calls
`InstallSpeechEngine`, which downloads in the background and streams
`speech:progress` for a percent bar; the install then auto-selects what it
fetched by calling `SelectSpeechEngine`, so a first run needs no second step.
A stored spec matches a catalogue engine by **command family** ("moonshine"),
not by exact path, so an engine picked from disk, one bundled, and one just
downloaded all light the same "In use" row. Verified live: fresh download →
progress bar → "In use", with the engine landing at `<data root>/engines`
(138 MB, correct layout).

#### Cross-platform shape

The model is **platform-independent** (plain ONNX) — one 119 MB set works on
every OS/arch. The **runtime is per-OS/arch**. So the on-disk layout splits:

```
engines/
  bin/<the OS-native runtime>            # per-platform
  models/moonshine-tiny-en-int8/         # shared by every platform
    preprocess.onnx encode.int8.onnx
    uncached_decode.int8.onnx cached_decode.int8.onnx tokens.txt
```

Runtime per platform, from the sherpa-onnx `v2.0.1` release. Use the
`shared-no-tts` asset (we never synthesise speech) and extract only the offline
binary plus its library:

| Platform        | Release asset (`…-v2.0.1-…`)            | Extract                                  | Compressed |
|-----------------|------------------------------------------|------------------------------------------|-----------|
| Windows x64     | `win-x64-shared-MT-Release-no-tts`       | `sherpa-onnx-offline.exe`, `onnxruntime.dll` | 25.6 MB |
| Windows arm64   | `win-arm64-shared-MT-Release-no-tts`     | same two names                           | 24.6 MB |
| Linux x64       | `linux-x64-shared-no-tts`                | `sherpa-onnx-offline`, `libonnxruntime.so` | 25.3 MB |
| Linux arm64     | `linux-aarch64-shared-cpu`               | same                                     | 27.9 MB |
| macOS arm64     | `osx-arm64-shared-no-tts`                | `sherpa-onnx-offline`, `libonnxruntime.dylib` | 18.7 MB |
| macOS x64       | `osx-x64-shared-no-tts`                  | same                                     | 21.1 MB |

Why not the `static` builds: they are self-contained (no lib to ship) but huge
— win-x64 241 MB, osx-universal2 506 MB compressed — because they bundle every
model family. Shared + two files is far smaller for one engine.

#### Two roots — who may write them

| Root                 | Written by                        | Holds        |
|----------------------|-----------------------------------|--------------|
| `<exe dir>/engines`  | Installer (machine scope, admin)  | **Bundled Moonshine** — read-only to a standard user |
| `<data root>/engines`| The app (the user)                | **Downloaded** engines |

A per-user data folder is, by construction, readable and writable by the
person running the app; nothing under `<data root>` can be hidden from them.
The closest a desktop app gets to a protected engine is the machine-scope
install, where `<exe dir>` lives in Program Files and a standard user cannot
replace it without admin. That resists casual tampering, not a local
administrator or admin-level malware — no desktop app can. Hash-verifying
before use turns silent replacement into a clear error. Downloaded engines go
to `<data root>/engines` because that is the writable, per-user location.

#### Installer

`cmd/zowe-engine` stages the engine tree for packaging (`task engines:stage`),
drawing from the pinned releases so the shipped bytes are the verified ones.
Because `InstallFor` takes an explicit GOOS/GOARCH, the staging is
platform-parameterized: the model is shared and only the runtime differs, so
any host can assemble a package for another OS — a Windows tree and a Linux
tree were both staged from a Windows host.

The Windows installer ships it: `create:nsis:installer` depends on
`engines:stage`, and `project.nsi` copies `build/engines/*` to
`$INSTDIR\engines` (guarded by `!if /FileExists`, so an unstaged build still
packages and the app falls back to "Get engine"). Installing to Program Files
keeps the bundled engine read-only to a standard user.

Verified end to end: the installer compiled (121 MB install data), a silent
install laid down `zowe-worship.exe` plus a 138 MB `engines/` tree whose every
file matched its pinned hash, and that installed engine transcribed the smoke
clip and detected **Romans 8:28**.

Also fixed along the way: `build/windows/nsis/wails_tools.nsh` is committed with
the Wails template defaults ("probe" / "My Company") and the Taskfile flow never
regenerates it, so the installer would have named the app `probe.exe`. The real
`INFO_*` values are now defined in `project.nsi` ahead of the include, which
overrides those defaults.

#### Attribution

The redistributed pieces are sherpa-onnx (Apache-2.0), ONNX Runtime (MIT), and
the Moonshine models (MIT). Their full texts are vendored under
`internal/speech/notices/` and embedded into the binary, so the app carries the
licenses whether or not the engine is installed. `InstallFor` writes
`THIRD-PARTY-NOTICES.txt` into the engine root beside the binaries — the file
the installer's `File /r` picks up and ships in `engines/`.

Settings → Live detection surfaces this: the engine card lists each component
with its license, and **Open the full notices file** opens the written copy
through `GetSpeechAttribution` + `OpenPath`. When nothing is installed yet the
list still renders from the embedded texts, with the file link absent.

#### Still open

- macOS and Linux packaging do not exist yet (no `build/darwin` bundle, no
  `build/linux`). The code and the staging task are already cross-platform;
  each OS needs its own package layout that places the `engines/` tree beside
  the app, then a one-line `engines:stage` dependency like Windows has.
- Whether the app itself carries a top-level LICENSE is an owner call; the
  third-party attributions above are independent of it.

## F08 — Semantic scripture and song search (Planned)

`internal/search` is empty. No embedding model, no versioned index, no
`search_embeddings` migration. Prerequisite: benchmark a local model on the
Windows baseline before selecting it.

## F09 — Online lyrics discovery (Planned)

No package, no UI, no credential-store integration. Discovery only — must never
become a dependency of the offline core.

## F10 — EasyWorship migration (Planned)

No read-only adapters, no preflight, no `import_runs` migration. Preserve source
DB and existing library on failure.

## Data model status

Tables in `internal/database/migrations/`:

| Table | State |
| --- | --- |
| `bible_translations` / `bible_books` / `bible_chapters` / `bible_verses` | per-translation package DBs (not in the app DB) |
| `services` / `service_items` | Done (F03) |
| `songs` / `song_sections` / `song_arrangements` / `song_arrangement_items` | Done (F04) |
| `song_fts` | Done (F04) |
| `media_assets` | Done (F05) |
| `slide_templates` | Done (F06) — themes |
| `settings` | Done (F01) |
| `presentation_history` | Done (F03) — written by `internal/history` on every live change |
| `detection_history` | Done (F07) — decisions written by `internal/detection` |
| `bible_fts` / `bible_book_aliases` | inside per-translation DBs; alias parsing via `books.go` |
| `search_embeddings` | not created (F08) |
| `import_runs` | not created (F10) |

Note: song search uses the FTS index created in `003_songs.sql`; there is no
separate `song_fts` table in the app DB.

## Application events

Implemented: `presentation` republishes the slide pair to the studio and
audience windows (`display.go` `publishToDisplay`, `StudioProvider.tsx`);
`media-control`, `media-progress` and `media-ended` (`media_control.go`);
`detection:progress` and `detection:config` (`app_detection.go`); and
`speech:progress` / `speech:config` (`app_speech.go`).

Not implemented: the `audio.*`, `transcript.*` and `scripture.*` events, which
belong to F07.

## Cross-cutting deviations to keep in mind

- **F07/F08 placeholders vs. roadmap.** The original baseline claimed only the
  scaffold existed; that is now stale. F01–F07 have working slices (F06 fully),
  while F08–F10 remain genuinely unstarted.
- **Shortcuts are configurable; `F9` is still a stub.** The map is stored per
  machine and edited in Settings → Shortcuts; `F8` and `F12` drive real
  detection state, `F9` has nothing to pause (see F07).
- **`presentation_history`** is written on every live change and read back by
  the History panel (F03); `detection_history` is written on every
  approve/dismiss/clear but never read back (F07). Neither is exported.

## Landed on `feat/f04-songs-presentation`

Operator-reported fixes and supporting work, all committed and verified by
build + tests + typecheck:

1. **Verses per slide setting** — default one verse per slide, with a
   "Whole passage" option. `internal/settings/store.go` `KeyVersesPerSlide`,
   `app_settings.go` `oneVersePerSlide`, `SettingsWindow.tsx` "Scripture slides"
   card, `app.go` passage splitting, controller one-verse walking
   (`previewSource.oneVerse`, `showVerse`, `stepVerse`).
2. **Reference colour not refreshing** — preview rebuilt from a tracked source
   and republished on theme edits (`controller.go` `RefreshPreview`/`previewSource`,
   `app_themes.go` `themeChange`, `StudioProvider.tsx`).
3. **Bible tab loads from Genesis 1 and scrolls on** — `BibleWorkspace.tsx`
   loads Genesis 1 on mount, stacks chapter blocks, search moves the reading pane
   and clearing restores the previous position.
4. **Tailwind v4 redesign** — every feature stylesheet inlined as utilities;
   only `tokens.css` and a thin `global.css` base layer remain.
5. **ASR benchmark harness** — `internal/asr` detector, `cmd/asr-bench` and
   `benchmarks/` (models/tools gitignored) groundwork for F07. It now also
   scores every detection with the production confidence function and prints a
   threshold sweep, so the bands are calibrated on the same audio.
   `benchmarks/record-set.ps1` records the real-speech set that sweep needs
   (one labeled clip per line, manifest written as it goes, output gitignored);
   no set of real recordings is committed yet, so the bands are still
   uncalibrated against a live room.

The bindings under `frontend/bindings/**` were regenerated with `-i`
(plain interfaces, no factories); the exported surface is unchanged.

## Fix order for the open gaps

Derived from a PRD-to-code audit on 2026-11-09. It replaces the earlier
recommended-workstreams list, whose top three entries (configured shortcuts,
restoring a past projection, engine provisioning) had already landed. Sizes are
S/M/L, not dates. The order follows the dependencies and the P0/P1 split in the
PRD, not the workstream numbering.

The one critical path is **F07**: finalized segments → confidence → gates →
auto-projection. Auto-projection cannot be built honestly on the current manual
20-second chunk, because the PRD's safety checklist requires a finalized, stable
transcript plus a duplicate/repeat gate.

### Phase 0 — Correct the record (S) — closed 2026-11-09

The tracker understated the app, which is why this audit was needed:

- [x] F01 "restore is not implemented" was stale: `RestoreBackup`
      (`app_settings.go`), `internal/database/restore.go` and the Settings →
      Backups restore action all exist.
- [x] F03 "reopen/restore past output" was stale: `App.RestoreProjection`
      (`app.go`) and the History panel restore action (commit `2308a8a`).
- [x] F03 "configurable shortcuts" was stale: `app_shortcuts.go` and Settings →
      Shortcuts (commit `6f3f5a4`).
- [x] F05 stop/seek/volume was half-done: the Go side (`media_control.go`) and
      the audience player handle all six commands; only the studio UI is missing.
- [x] Baseline refreshed to 2026-11-09 on `main` — all four commands pass.

### Phase 1 — P0 detection safety chain

1. **Read `detection_history` + richer candidate metadata (S–M) — landed
   2026-11-09.** A binding that
   reads decisions back, and method/confidence/explanation on `DetectionView`.
   Everything after this needs the data surface, and it closes three
   detection-panel requirements at once.
2. **Finalized-segment lifecycle + repeat protection (M–L) — landed
   2026-11-09.** `DetectScripture`
   transcribes one 20 s blob today. Introduce finalized segments with stable
   candidate IDs/versions and the duplicate/rapid-repeat gate on top of the
   existing `MergeLive` merge. **Critical path.**
3. **Confidence scoring (M) — landed 2026-11-10.** `internal/confidence`
   scores the phrasing behind a match and bands it at the PRD's starting edges;
   the queue, `DetectionView` and `DetectionHistoryView` carry the score and
   band, and the panel draws `96% High` beside the method. Scoring is
   evidence-based, not yet measured against labeled audio, so auto-projection
   stays off and the bands remain the PRD's tuning values.
4. **Policy, safety gates, auto-projection (M–L) — landed 2026-11-10.**
   `internal/policy` is the only path from a detection to `internal/presentation`
   and each PRD §7 gate has its own test, plus app-level tests that a blocked
   card stays queued and a refused passage leaves the live slide alone. UI: the
   Production card carries the switch, Hybrid/Paused status and the gate
   explainer; F9 pauses and resumes; every start boots paused.
5. **Continuous listening and segmentation (M–L) — landed 2026-11-10.**
   `frontend/src/features/detection/segmenter.ts` watches the level the capture
   already measures and closes a segment on a spoken pause (or at the 25 s cap,
   under the backend's 30 s limit), so listening no longer waits for the
   operator to press Analyze. Segments are drained by absolute sample offset
   (`MicCapture.takeRange`), transcribed one at a time with a single pending
   slot, and the manual Analyze button remains as a fallback when the toggle is
   off.
6. **Panel parity + candidate keyboard (S–M).** Project, Search and Preview per
   candidate, plus Enter / Esc / Alt+Enter / Ctrl+Up-Down in `lib/shortcuts.ts`
   and the panel. After slice 4, so Project goes through the gates instead of
   bypassing them.
7. **Further detection types (M each, sequential).** Contextual → quotation →
   correction → multi-reference. Each type stays disabled until it clears the
   precision target on labeled audio.

### Phase 2 — Reliability and evidence (parallel to Phase 1)

Different files from detection, so these run alongside it:

- [ ] Media transport UI: stop, seek, volume (S). Backend done.
- [ ] Display reconnect and mixed-DPI recovery (M); `clearAudienceDisplay`
      currently forgets the output when the window closes.
- [ ] `internal/audio` decision (S–M): build the Go capture service, or record
      the webview capture as the accepted adapter and emit `audio.input.*`.
- [ ] Bible reference/full-text p95 benchmarks, plus the acceptance sweep
      (navigation not interrupting live, media failure preserving the slide).
- [ ] Packaging: macOS/Linux bundles and migration rollback (M–L), owner call.

### Phase 3 — P1 content

- [ ] Translation packages plus license/attribution surface (S for the surface,
      L for licensed content). KJV attribution can ship before NIV/ESV/NLT/NASB
      rights exist.
- [ ] Speech-assisted scripture search (S–M), cheap once slices 2–3 exist.
- [ ] Media playlists (M).
- [ ] Additional audience outputs (M–L).

### Phase 4 — P1 discovery and migration

All three are gated on external input, which is why they come last:

- [ ] F08 semantic search (L): benchmark a local model on the Windows baseline
      before choosing it; no `search_embeddings` until then.
- [ ] F09 Genius (M–L): provider terms and an OS credential store, not just code.
- [ ] F10 EasyWorship (L): permitted fixtures, a supported-format list,
      `import_runs`, and transactional import with a result summary.

### Decisions needed before their phase

- Per-type enablement for auto-projection (the switch defaults off and boots paused).
- Whisper vs Moonshine as the V1 engine — it changes the model-lifecycle work
  (before 1.5 and the `internal/audio` decision).
- Whether Live Detection stays a persistent panel or becomes a page (before 1.6).
- Translation rights and whether the app carries a top-level LICENSE (3.x).
- Supported EasyWorship formats and the semantic model choice (4.x).

### Exit criteria for every phase

`CGO_ENABLED=0 go build ./...`, `CGO_ENABLED=0 go test ./...` and
`cd frontend && npm run typecheck` pass; scoped Conventional Commits
(`feat(detection)`, `feat(media)`, …); this file updated in the same change.

Recommending a phase is not authorization to build it — confirm scope first.
