# Zowe Worship — Feature Implementation Tracker

Living status of the work described in `Zowe_Worship_Full_PRD_and_Design_Guide.txt`
(§17 Feature Workstreams). The PRD describes intent; this file records what is
actually built, where it lives, and what is still open, so any agent can pick up
the next slice without re-deriving the state of the repo.

Update this file whenever a slice lands. Keep the claim verifiable: name the
files that prove it, and re-run the verification commands below before marking a
row **Done**.

## Verification baseline

Last verified: **2026-11-03** on branch `feat/f04-songs-presentation`.

| Command | Result |
| --- | --- |
| `CGO_ENABLED=0 go build ./...` | pass |
| `CGO_ENABLED=0 go test ./...` | pass (all packages) |
| `cd frontend && npm run typecheck` | pass (`tsc --noEmit`) |

Real Bible data is exercised by `internal/bible/integration_test.go`, which is
skipped unless `ZOWE_BIBLE_DATASETS` points at a directory holding `KJV/` (and
optionally `AMP/`). Treat reference/phrase/prefix search as *unverified against
real text* on a machine without that env var.

## Status legend

- **Done** — implemented, wired to the UI, and covered by tests or a documented
  manual check.
- **Partial** — working slice exists but a named part of the PRD scope is
  missing (listed).
- **Planned** — not started. Empty packages (`.gitkeep` only) do not count as
  implementation.

## Workstream summary

| ID | Feature group | Status | Where |
| --- | --- | --- | --- |
| F01 | Desktop foundation and reliability | **Partial** | `main.go`, `app.go`, `internal/database`, `internal/settings`, `app_settings.go` |
| F02 | Bible library and translation search | **Partial** | `internal/bible`, `internal/scripture`, `app.go`, `frontend/src/features/bible` |
| F03 | Service order and multi-display presentation | **Partial** | `internal/presentation`, `internal/serviceorder`, `display.go`, `frontend/src/features/service-order`, `frontend/src/features/display` |
| F04 | Songs and lyric presentation | **Partial** | `internal/songs`, `frontend/src/features/songs` |
| F05 | Media library and playback | **Partial** | `internal/media`, `app_media.go`, `frontend/src/features/media` |
| F06 | Custom theme editor | **Done** | `internal/themes`, `app_themes.go`, `frontend/src/features/themes`, `frontend/src/components/ThemedSlide.tsx` |
| F07 | Offline AI scripture detection | **Planned** | `internal/asr` detector + `cmd/asr-bench` benchmark groundwork; `internal/audio`, `internal/speech`, `internal/confidence`, `frontend/src/features/live-detection` (empty) |
| F08 | Semantic scripture and song search | **Planned** | `internal/search` (empty); no model/index work |
| F09 | Online lyrics discovery (Genius) | **Planned** | no package, no UI |
| F10 | EasyWorship migration | **Planned** | no package, no UI |

## F01 — Desktop foundation and reliability (Partial)

Done:
- [x] Wails v3 shell, typed bindings under `frontend/bindings/`, Taskfile build
      (`Taskfile.yml`); commit `b8f1c2d`.
- [x] SQLite via `modernc.org/sqlite` with FTS5; versioned migrations
      `internal/database/migrations/001..014`, checksum-tracked (`migrate.go`).
- [x] App lifecycle wiring, storage paths, backups (`internal/database/backup.go`,
      `app_settings.go` `BackupNow` / `GetStorage`).
- [x] Settings store with typed keys and fallbacks (`internal/settings/store.go`,
      `KeyDefaultTranslation`, `KeyVersesPerSlide`).
- [x] Utility-window helper for editors (`windows.go` `openUtilityWindow`).

Open:
- [ ] Packaging / recovery round-trip beyond a manual backup (`BackupNow` writes a
      copy; restore is not implemented).
- [ ] Migration rollback / downgrade handling (only forward migrations exist).

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

Open:
- [ ] `NIV`, `ESV`, `NLT`, `NASB` packages — only `KJV` (+ `AMP` in fixtures) are
      targeted; add per-translation license/distribution metadata + integrity
      checks before bundling.
- [ ] Relaxed multi-term / stem-prefix ranking ("God love world" →
      "For God so loved the world") — PRD §5 calls this a planned follow-up to
      the initial all-terms FTS5 search.
- [ ] Performance targets measured (reference p95 < 50 ms, full-text p95 < 150 ms)
      — no benchmark recorded.
- [ ] Attribution / license surface for installed translations in the UI.

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
- [x] Keyboard shortcuts F8/F9/F12, Ctrl+Shift+S, Alt+Enter, ArrowLeft/Right,
      Esc (`AppShell.tsx` handler) — see caveat under F07.

Open:
- [ ] Presentation history writing — table exists (`009_presentation_history.sql`)
      but no code inserts rows yet.
- [ ] Reopen/restore past output from history.
- [ ] Mixed-DPI, disconnect/reconnect recovery tests for the audience display.
- [ ] Configured operator shortcuts (PRD §10: "must be configurable") — shortcuts
      are hard-coded, no settings UI.
- [ ] Additional audience outputs (intentionally deferred past single-output).
- [ ] §14 acceptance evidence for navigation-not-interrupting-live, display error
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
- [ ] §14 acceptance evidence: "preview lyric slides, advance the intended section
      without changing live output during editing".
- [ ] Confirm song FTS is exercised by tests against real lyrics (currently
      fixture-level).

## F05 — Media library and playback (Partial)

Done:
- [x] Import images/video/audio, store metadata and availability, list, delete
      (`internal/media`, `app_media.go`; migrations `005..007`).
- [x] Image framing (aspect ratio / crop) and editor window
      (`frontend/src/features/media/MediaEditorWindow.tsx`).
- [x] Project images, video and audio as slides; transport play/pause/restart
      (`app_media.go` `ProjectMedia`, `media_control.go`).
- [x] Relink a missing asset (`App.RelinkMedia`).
- [x] Preview playback kept separate from audience playback.

Open:
- [ ] Stop / seek / volume controls beyond play-pause-restart (PRD §8: "where
      supported").
- [ ] Explicit declared-format/codec list surfaced to the user.
- [ ] §14 evidence: missing file, unsupported codec, relink, failure without
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
      audience window render identically (`frontend/src/components/ThemedSlide.tsx`,
      `themed-slide.css`; container-query sizing).
- [x] Per-item theme override with operator picker (`App.SetItemTheme`,
      `AppShell.tsx` `PreviewTheme`).
- [x] Editing a theme refreshes the studio preview through the `presentation`
      event (`app_themes.go` `themeChange`, `display.go` `publishToDisplay`,
      `StudioProvider.tsx` `Events.On('presentation')`).
- [x] Overflow handled via `container-type: size` + `cqh`/`cqw` units.

Open:
- [ ] Arbitrary-script rejection is by construction (themes are data, no code
      fields) — no explicit test asserting a malicious theme payload is refused.
- [ ] §14 evidence: theme overflow across output aspect ratios captured.

## F07 — Offline AI scripture detection (Planned)

Nothing is wired to the UI. `internal/audio`, `internal/speech`,
`internal/confidence` and `frontend/src/features/live-detection` contain only
`.gitkeep`. Groundwork that does exist: `internal/asr` (sidecar engine adapter,
WAV prep, conservative `DetectReferences` spoken-reference detector with tests)
plus `cmd/asr-bench` and `benchmarks/` to score engines before any is chosen
(`task asr:bench`).

Important caveat on F03's shortcuts: `F8` (listening), `F9` (pause
auto-projection) and `F12` (emergency disable) are **wired to placeholder
notices** in `AppShell.tsx` — they do not start or stop real work yet, because
the detection subsystem does not exist.

To build (PRD §7): audio capture + device levels; Whisper model lifecycle;
finalized transcripts; explicit-reference parsing; candidate queue; calibrated
confidence bands; gated auto-projection with the safety checklist; detection
panel; bounded in-memory transcript state.

## F08 — Semantic scripture and song search (Planned)

`internal/search` is empty. No embedding model, no versioned index, no
`search_embeddings` migration. Prerequisite: benchmark a local model on the
Windows baseline before selecting it (PRD §5).

## F09 — Online lyrics discovery (Planned)

No package, no UI, no credential-store integration. Discovery only — must never
become a dependency of the offline core (PRD §8, §17).

## F10 — EasyWorship migration (Planned)

No read-only adapters, no preflight, no `import_runs` migration. Preserve source
DB and existing library on failure (PRD §8, §14).

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
| `presentation_history` | table exists; **no writer** |
| `detection_history` | table exists; **no writer** (F07) |
| `bible_fts` / `bible_book_aliases` | inside per-translation DBs; alias parsing via `books.go` |
| `search_embeddings` | not created (F08) |
| `import_runs` | not created (F10) |

Note: the PRD §11 also lists `song_fts` in the app DB; song search currently uses
the FTS index created in `003_songs.sql`.

## Application events (PRD §11)

Implemented: a single `presentation` event republishes the slide pair to the
studio and audience windows (`display.go` `publishToDisplay`,
`StudioProvider.tsx`), plus `media-control` / `media-ended`.

Not implemented: the `audio.*`, `transcript.*` and `scripture.*` events, which
belong to F07.

## Cross-cutting deviations to keep in mind

- **F07/F08 placeholders vs. roadmap.** The PRD §17 baseline said only the
  scaffold existed; that is now stale. F01–F06 have working slices (F06 fully),
  while F07–F10 remain genuinely unstarted.
- **Keyboard shortcuts look done but aren't.** Only navigation/projection
  shortcuts act; the detection shortcuts are stubs (see F07).
- **`presentation_history` and `detection_history`** are created but unwritten.

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
   `benchmarks/` (models/tools gitignored) groundwork for F07.

The bindings under `frontend/bindings/**` were regenerated with `-i`
(plain interfaces, no factories); the exported surface is unchanged.

## Recommended next workstreams

In dependency order:

1. **Presentation history writing** (F03) — finish a cheap table already in
   place; unblocks §14 evidence for projection recording.
3. **Relaxed Bible search ranking** (F02) — the remaining meaningful search gap.
4. **Configured shortcuts + settings UI** (F01/F03).
5. **F07 offline detection** — the largest unstarted block; start with the audio
   adapter and Whisper lifecycle behind `internal/speech`.

Recommending a workstream is not authorization to build it — confirm scope first.
