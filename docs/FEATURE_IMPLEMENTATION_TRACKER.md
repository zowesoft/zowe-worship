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
      `internal/database/migrations/001..014`, checksum-tracked (`migrate.go`).
- [x] App lifecycle wiring, storage paths, backups (`internal/database/backup.go`,
      `app_settings.go` `BackupNow` / `GetStorage`).
- [x] Settings store with typed keys and fallbacks (`internal/settings/store.go`,
      `KeyDefaultTranslation`, `KeyVersesPerSlide`).
- [x] Utility-window helper for editors (`windows.go` `openUtilityWindow`).
- [x] Single-instance guard (`main.go` `SingleInstance`, id
      `com.zowesoft.zowe-worship`). A second launch is refused and the running
      studio is restored and focused instead. Keyed on the identifier, not the
      executable name, so a renamed copy is refused too. Not a PRD requirement;
      added because two studios would each hold their own preview/live state
      while driving one projector and one database.

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
- [x] Relaxed multi-term ranking for the default full-text ("words") search
      (`internal/bible/sqlite.go` `searchRelaxed`). Two stages: prefix-AND first
      (every word required, matched as a prefix so "love" reaches "loved"), then
      — only when nothing carries every word — prefix-OR ranked by bm25. The
      installed packages tokenize with `unicode61` (no stemmer), so prefixing is
      how we bridge inflections without rebuilding every translation database.
      Measured against the real `kjv-1.0.0.db` (31,102 verses):
      - "God love world": exact-AND found 4 verses and missed John 3:16;
        prefix-AND finds 5 with John 3:16 at #2 (behind 1 John 4:9, which also
        carries all three words). This is the PRD §5 example.
      - "believe saved": exact-AND 5 → prefix-AND 8, reaching "believeth", with
        Mark 16:16 on top.
      - "God loved shepherd" (no verse has all three): stage 1 empty, stage 2
        widens and every top hit carries two of the three words.

Open:
- [ ] `NIV`, `ESV`, `NLT`, `NASB` packages — only `KJV` (+ `AMP` in fixtures) are
      targeted; add per-translation license/distribution metadata + integrity
      checks before bundling.
- [ ] Performance targets measured (reference p95 < 50 ms, full-text p95 < 150 ms)
      — no benchmark recorded. Note: a per-row coverage recount for the widened
      stage was rejected after measuring 29–58 s over 4,000+ OR candidates;
      plain bm25 ordering answers the same queries in 20–430 ms and still floats
      the highest-coverage verses to the top.
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
- [x] Presentation history writing — every command that changes live output
      records a row (`app.go` `projected`/`recordProjection`, `internal/history`),
      exposed through `App.RecentProjections` and shown in
      `frontend/src/features/presentation/HistoryPanel.tsx`. Preview commands
      deliberately do not write. Migration
      `015_presentation_history_kinds.sql` drops the stale `kind` CHECK that
      predated text/announcement/blank and the split media kinds.

Open:
- [ ] Reopen/restore past output from history — rows carry the schedule item id
      in `payload`, so a restore slice has what it needs.
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

## F07 — Offline AI scripture detection (Partial)

First slice landed: mic → transcript → candidate → approval → preview. The
webview captures the input device (`getUserMedia` + AudioWorklet) into a
16 kHz mono Int16 ring buffer; **Analyze** (panel button) sends the last ~20 s
to `DetectScripture`, which wraps the PCM in a temp WAV, runs the engine spec
from settings, transcribes, and queues every reference `DetectReferences`
finds. `internal/detection` holds the candidate queue in memory and records
every approve/dismiss/clear in `detection_history` (previously unwritten).
Approving previews the passage in the default translation — the operator still
sends it live, per PRD §9. `F8` toggles listening, `F12` persists the
emergency disable (`detection_disabled`), and the engine command is set in
Settings → Live detection.

Still open (later slices): continuous listening with automatic segmentation
(currently manual chunks only), Whisper model lifecycle management, finalized
transcripts, calibrated confidence bands, gated auto-projection with the
safety checklist, and the remaining `internal/audio` / `internal/speech` /
`internal/confidence` packages from the PRD §7 plan.

`F9` (pause auto-projection) remains a placeholder notice because nothing
auto-projects yet.

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
| `presentation_history` | Done (F03) — written by `internal/history` on every live change |
| `detection_history` | Done (F07) — decisions written by `internal/detection` |
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
  scaffold existed; that is now stale. F01–F07 have working slices (F06 fully),
  while F08–F10 remain genuinely unstarted.
- **Keyboard shortcuts look done but aren't.** Navigation/projection shortcuts
  act; `F8` and `F12` now drive real detection state, `F9` is still a stub
  (see F07).
- **`presentation_history`** is written on every live change (F03);
  `detection_history` is written on every approve/dismiss/clear (F07). Both read
  and write; neither is surfaced as an export yet.

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

1. **Guided reference entry** (F02) — as the operator types, suggest matching
   books (including numbered books via their number, e.g. "J" → "1 John"), then
   step the cursor book → chapter → verse with Tab/Enter, inserting the colon.
2. **Configured shortcuts + settings UI** (F01/F03).
3. **Reopen/restore past output** (F03) — `presentation_history` now has rows and
   carries the item id in `payload`, so this is the natural follow-on.
4. **F07 offline detection, next slice** — continuous listening with automatic
   segmentation (silence-based), Whisper model lifecycle behind
   `internal/speech`, confidence bands.

Recommending a workstream is not authorization to build it — confirm scope first.
