package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"zowe-worship/internal/asr"
	"zowe-worship/internal/confidence"
	"zowe-worship/internal/detection"
	"zowe-worship/internal/policy"
	"zowe-worship/internal/settings"
)

// detectionAudioRate matches what the webview captures: 16 kHz mono 16-bit
// PCM, the format every supported engine wants.
const detectionAudioRate = 16000

// maxDetectionAudioBytes caps one analysis request at 30 s of 16 kHz mono
// 16-bit PCM. Longer inputs can push autoregressive engines like Moonshine
// into a decode loop that pins the CPU and starves small machines of memory.
const maxDetectionAudioBytes = 30 * detectionAudioRate * 2

// DetectionView is one candidate in the approval queue.
type DetectionView struct {
	ID         string `json:"id"`
	Transcript string `json:"transcript"`
	Reference  string `json:"reference"`
	// Method, Confidence and Explanation are what the panel shows beneath the
	// reference. The band names the policy outcome so the panel never has to
	// re-derive a threshold.
	Method         string  `json:"method"`
	Confidence     float64 `json:"confidence"`
	ConfidenceBand string  `json:"confidenceBand"`
	Explanation    string  `json:"explanation"`
	Segment        int     `json:"segment"`
	Version        int     `json:"version"`
	Finalized      bool    `json:"finalized"`
	Repeats        int     `json:"repeats"`
	BookID         int     `json:"bookId"`
	Chapter        int     `json:"chapter"`
	VerseStart     int     `json:"verseStart"`
	VerseEnd       int     `json:"verseEnd"`
	CreatedAt      string  `json:"createdAt"`
	// AutoProjection explains a card the gates held back; nil when the switch is
	// off, the card would have gone out, or it is not settled yet.
	AutoProjection *AutoProjectionChecklist `json:"autoProjection,omitempty"`
}

// GateView is one line of the safety checklist the panel shows for a card the
// gates held back.
type GateView struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Passed bool   `json:"passed"`
}

// AutoProjectionChecklist explains why a settled card stayed in the queue while
// auto-projection was armed: the first failing gate, plus every gate's result.
type AutoProjectionChecklist struct {
	Reason string     `json:"reason"`
	Gates  []GateView `json:"gates"`
}

// DetectionResult returns the transcript even when nothing was detected, so
// the panel can show why the queue stayed empty.
type DetectionResult struct {
	Transcript string          `json:"transcript"`
	Candidates []DetectionView `json:"candidates"`
	// Segment numbers the analysis that just closed; Repeats counts hearings
	// that folded into an existing pending card.
	Segment int `json:"segment"`
	Repeats int `json:"repeats"`
	// AutoProjected lists what the safety gates sent to the room without the
	// operator, so the panel can say so instead of the card vanishing.
	AutoProjected []string `json:"autoProjected"`
}

const detectionProgressEvent = "detection:progress"

// detectionConfigEvent exists because engine and switch settings live in a
// different webview; without it the studio panel stays stale until restart.
const detectionConfigEvent = "detection:config"

type DetectionProgressView struct {
	Transcript string          `json:"transcript"`
	Candidates []DetectionView `json:"candidates"`
	Segment    int             `json:"segment"`
}

type DetectionConfigView struct {
	EngineSpec string `json:"engineSpec"`
	Disabled   bool   `json:"disabled"`
	// AutoProjection is the operator's standing choice; AutoProjectionPaused is
	// the F9 pause. They are separate so the panel can say "Hybrid" or "Paused".
	AutoProjection       bool `json:"autoProjection"`
	AutoProjectionPaused bool `json:"autoProjectionPaused"`
}

// DetectScripture queues what the detector finds; nothing is projected until
// the operator approves a candidate.
func (a *App) DetectScripture(pcm []byte) (DetectionResult, error) {
	if a.detection == nil || a.settings == nil {
		return DetectionResult{}, errStorageUnavailable
	}
	if len(pcm) == 0 {
		return DetectionResult{}, errors.New("no audio was captured")
	}
	if len(pcm) > maxDetectionAudioBytes {
		return DetectionResult{}, errors.New("audio chunk is too long — analyze at most 30 seconds at a time")
	}
	if a.detectionDisabled() {
		return DetectionResult{}, errors.New("detection is disabled (F12)")
	}

	spec, err := a.detectionEngine()
	if err != nil {
		return DetectionResult{}, err
	}

	// The engine reads files, not buffers.
	dir, err := os.MkdirTemp("", "zowe-detect-")
	if err != nil {
		return DetectionResult{}, fmt.Errorf("prepare audio temp dir: %w", err)
	}
	defer os.RemoveAll(dir)
	wavPath := filepath.Join(dir, "chunk.wav")
	if err := asr.WriteWAV(wavPath, asr.AudioSpec{
		SampleRate:    detectionAudioRate,
		Channels:      1,
		BitsPerSample: 16,
		FormatTag:     1,
		Data:          pcm,
	}); err != nil {
		return DetectionResult{}, fmt.Errorf("write audio chunk: %w", err)
	}

	// Partials may only refine this segment's cards; the final transcript closes
	// the segment and settles them under their existing IDs.
	a.detection.BeginSegment()
	transcript, err := spec.TranscribeStream(wavPath, a.streamDetectionProgress)
	if err != nil {
		a.detection.CloseSegment()
		return DetectionResult{}, err
	}
	return a.settleSegment(transcript)
}

// streamDetectionProgress merges each segment so a growing hearing replaces its
// pending candidate, and forwards the partial transcript to the studio.
// settleSegment closes the open segment on its final transcript: the queue
// settles the cards under their existing IDs and the policy layer weighs them.
// DetectScripture calls it once per analysis; tests drive it directly so the
// segment lifecycle needs no speech engine on disk.
func (a *App) settleSegment(transcript string) (DetectionResult, error) {
	if a.detection == nil {
		return DetectionResult{}, errStorageUnavailable
	}
	settled, err := a.detection.Finalize(transcript, asr.DetectReferences(transcript))
	if err != nil {
		return DetectionResult{}, err
	}
	return DetectionResult{
		Transcript:    transcript,
		Candidates:    a.toDetectionViews(a.detection.List()),
		Segment:       settled.Segment,
		Repeats:       settled.Repeats,
		AutoProjected: a.autoProject(settled.Settled),
	}, nil
}

func (a *App) streamDetectionProgress(partial string) {
	if a.detection == nil {
		return
	}
	if _, err := a.detection.MergeLive(partial, asr.DetectReferences(partial)); err != nil {
		return
	}
	if desktop := application.Get(); desktop != nil {
		if studio, ok := desktop.Window.GetByName(studioWindow); ok {
			studio.DispatchWailsEvent(&application.CustomEvent{
				Name: detectionProgressEvent,
				Data: DetectionProgressView{
					Transcript: partial,
					Candidates: a.toDetectionViews(a.detection.List()),
					Segment:    a.detection.CurrentSegment(),
				},
			})
		}
	}
}

func (a *App) ListDetections() ([]DetectionView, error) {
	if a.detection == nil {
		return nil, errStorageUnavailable
	}
	return a.toDetectionViews(a.detection.List()), nil
}

// DetectionHistoryView is one recorded decision. Pending candidates live only in
// memory, so this is how the panel recounts what was approved, dismissed or
// cleared.
type DetectionHistoryView struct {
	ID             string  `json:"id"`
	Transcript     string  `json:"transcript"`
	Reference      string  `json:"reference"`
	Confidence     float64 `json:"confidence"`
	ConfidenceBand string  `json:"confidenceBand"`
	Decision       string  `json:"decision"`
	CreatedAt      string  `json:"createdAt"`
}

// The panel shows a short recent list, not an export: the cap keeps a mistaken
// large limit from scanning the whole table.
const maxDetectionHistoryRows = 50

// ListDetectionHistory returns past decisions newest first. A limit of zero
// means the panel default.
func (a *App) ListDetectionHistory(limit int) ([]DetectionHistoryView, error) {
	if a.core == nil {
		return nil, errStorageUnavailable
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > maxDetectionHistoryRows {
		limit = maxDetectionHistoryRows
	}

	rows, err := a.core.QueryContext(a.context(),
		`SELECT id, transcript, reference, confidence, decision, created_at
		 FROM detection_history
		 ORDER BY created_at DESC, rowid DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read detection history: %w", err)
	}
	defer rows.Close()

	history := make([]DetectionHistoryView, 0, limit)
	for rows.Next() {
		var view DetectionHistoryView
		if err := rows.Scan(&view.ID, &view.Transcript, &view.Reference,
			&view.Confidence, &view.Decision, &view.CreatedAt); err != nil {
			return nil, fmt.Errorf("read detection history row: %w", err)
		}
		view.ConfidenceBand = confidence.BandOf(view.Confidence)
		history = append(history, view)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read detection history: %w", err)
	}
	return history, nil
}

// ApproveDetection previews only; it never changes what the audience sees.
func (a *App) ApproveDetection(id string) (PresentationView, error) {
	if a.detection == nil || a.settings == nil {
		return PresentationView{}, errStorageUnavailable
	}

	ctx := a.context()
	candidate, err := a.detection.Approve(ctx, strings.TrimSpace(id))
	if err != nil {
		return PresentationView{}, err
	}

	reference := candidate.Reference
	translation := a.settings.GetDefault(ctx, settings.KeyDefaultTranslation, "kjv")
	return a.applied(a.presentation.PreviewScripture(ctx, translation,
		reference.Book.ID, reference.Chapter, reference.VerseStart, reference.VerseEnd,
		a.oneVersePerSlide(ctx)))
}

func (a *App) DismissDetection(id string) ([]DetectionView, error) {
	if a.detection == nil {
		return nil, errStorageUnavailable
	}
	if _, err := a.detection.Dismiss(a.context(), strings.TrimSpace(id)); err != nil {
		return nil, err
	}
	return a.toDetectionViews(a.detection.List()), nil
}

func (a *App) ClearDetections() ([]DetectionView, error) {
	if a.detection == nil {
		return nil, errStorageUnavailable
	}
	if err := a.detection.Clear(a.context()); err != nil {
		return nil, err
	}
	return []DetectionView{}, nil
}

// EngineOption is one engine found on this computer, ready to select without
// typing a path.
type EngineOption struct {
	Label string `json:"label"`
	Spec  string `json:"spec"`
}

func (a *App) ListDetectionEngines() ([]EngineOption, error) {
	cwd, _ := os.Getwd()
	exe, _ := os.Executable()
	dataRoot := ""
	if paths, err := a.paths(); err == nil {
		dataRoot = paths.Root
	}

	discovered := asr.DiscoverEngines(detectionEngineRoots(cwd, exe, dataRoot)...)
	options := make([]EngineOption, 0, len(discovered))
	for _, engine := range discovered {
		options = append(options, EngineOption{Label: engine.Name, Spec: engine.Spec})
	}
	return options, nil
}

// detectionEngineRoots is in scan order. It lives outside the binding so the
// installer, downloader and detector can be tested against one layout.
//
//   - <cwd>/benchmarks: the repository's fetched toolchain during development.
//   - <exe dir>/benchmarks: a dev build run from the repo root.
//   - <exe dir>/engines: the bundled engine installed read-only beside the app.
//   - <data root>/engines: engines the app downloaded for this user.
func detectionEngineRoots(cwd, exe, dataRoot string) []string {
	var roots []string
	if cwd != "" {
		roots = append(roots, filepath.Join(cwd, "benchmarks"))
	}
	if exe != "" {
		exeDir := filepath.Dir(exe)
		roots = append(roots,
			filepath.Join(exeDir, "benchmarks"),
			filepath.Join(exeDir, "engines"),
		)
	}
	if dataRoot != "" {
		roots = append(roots, filepath.Join(dataRoot, "engines"))
	}
	return roots
}

func (a *App) GetDetectionConfig() (DetectionConfigView, error) {
	if a.settings == nil {
		return DetectionConfigView{}, errStorageUnavailable
	}
	ctx := a.context()
	return DetectionConfigView{
		EngineSpec:           a.settings.GetDefault(ctx, settings.KeyAsrEngine, ""),
		Disabled:             a.detectionDisabled(),
		AutoProjection:       a.autoProjectionEnabled(ctx),
		AutoProjectionPaused: a.autoProjectionPaused(ctx),
	}, nil
}

// SetDetectionEngine parses before storing, so a typo fails at save rather than
// mid-service.
func (a *App) SetDetectionEngine(spec string) error {
	if a.settings == nil {
		return errStorageUnavailable
	}
	if _, err := asr.ParseEngineSpec(strings.TrimSpace(spec)); err != nil {
		return err
	}
	if err := a.settings.Set(a.context(), settings.KeyAsrEngine, strings.TrimSpace(spec)); err != nil {
		return err
	}
	a.notifyDetectionConfig()
	return nil
}

// SetDetectionDisabled stops new detections only; the display is left as it is.
func (a *App) SetDetectionDisabled(disabled bool) error {
	if a.settings == nil {
		return errStorageUnavailable
	}
	value := "false"
	if disabled {
		value = "true"
	}
	if err := a.settings.Set(a.context(), settings.KeyDetectionDisabled, value); err != nil {
		return err
	}
	a.notifyDetectionConfig()
	return nil
}

// notifyDetectionConfig is best-effort: a missing window means nobody is
// listening.
func (a *App) notifyDetectionConfig() {
	desktop := application.Get()
	if desktop == nil {
		return
	}
	if studio, ok := desktop.Window.GetByName(studioWindow); ok {
		studio.DispatchWailsEvent(&application.CustomEvent{Name: detectionConfigEvent})
	}
}

// SetAutoProjection is the operator's standing choice. Turning it on never
// clears a pause: F9 is the resume, so the switch alone cannot start projecting.
func (a *App) SetAutoProjection(enabled bool) error {
	if a.settings == nil {
		return errStorageUnavailable
	}
	if err := a.settings.Set(a.context(), settings.KeyAutoProjectionEnabled, boolSetting(enabled)); err != nil {
		return err
	}
	a.notifyDetectionConfig()
	return nil
}

// SetAutoProjectionPaused is the F9 pause. It stops automatic projections only;
// detection keeps running and every candidate keeps queuing for the operator.
func (a *App) SetAutoProjectionPaused(paused bool) error {
	if a.settings == nil {
		return errStorageUnavailable
	}
	if err := a.settings.Set(a.context(), settings.KeyAutoProjectionPaused, boolSetting(paused)); err != nil {
		return err
	}
	a.notifyDetectionConfig()
	return nil
}

func boolSetting(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func (a *App) autoProjectionEnabled(ctx context.Context) bool {
	if a.settings == nil {
		return false
	}
	return a.settings.GetDefault(ctx, settings.KeyAutoProjectionEnabled, "false") == "true"
}

// A missing pause reads as paused: the safe answer if the setting was never
// written on this machine.
func (a *App) autoProjectionPaused(ctx context.Context) bool {
	if a.settings == nil {
		return true
	}
	return a.settings.GetDefault(ctx, settings.KeyAutoProjectionPaused, "true") == "true"
}

// autoProject weighs each card one segment settled against the PRD safety
// checklist. It is the only path from a detection to the display: a blocked
// card stays queued for the operator, and a controller failure leaves the live
// slide exactly as it was.
func (a *App) autoProject(settled []detection.Candidate) []string {
	if a.presentation == nil || a.bible == nil || a.settings == nil || len(settled) == 0 {
		return nil
	}

	ctx := a.context()
	translation := a.settings.GetDefault(ctx, settings.KeyDefaultTranslation, "kjv")
	projected := make([]string, 0, len(settled))

	for _, candidate := range settled {
		reference := candidate.Reference
		if verdict := a.autoProjectionVerdict(ctx, candidate, len(settled)); !verdict.Project {
			continue
		}

		state, projectErr := a.presentation.ProjectScripture(ctx, translation,
			reference.Book.ID, reference.Chapter, reference.VerseStart, reference.VerseEnd,
			a.oneVersePerSlide(ctx))
		if _, projectErr = a.projected(state, projectErr); projectErr != nil {
			log.Printf("zowe: auto-project %s: %v", reference.String(), projectErr)
			continue
		}
		if _, err := a.detection.AutoProject(ctx, candidate.ID); err != nil {
			log.Printf("zowe: record auto-projected %s: %v", reference.String(), err)
			continue
		}
		projected = append(projected, reference.String())
	}
	return projected
}

// autoProjectionVerdict runs the PRD checklist for one candidate. Projection and
// the panel's explanation both call it, so the operator is told exactly what the
// gates acted on.
func (a *App) autoProjectionVerdict(ctx context.Context, candidate detection.Candidate, settledInSegment int) policy.Verdict {
	reference := candidate.Reference
	translation := a.settings.GetDefault(ctx, settings.KeyDefaultTranslation, "kjv")
	passage, passageErr := a.bible.Passage(ctx, translation,
		reference.Book.ID, reference.Chapter, reference.VerseStart, reference.VerseEnd)
	return policy.Evaluate(policy.Request{
		Enabled:          a.autoProjectionEnabled(ctx),
		Paused:           a.autoProjectionPaused(ctx),
		Disabled:         a.detectionDisabled(),
		Finalized:        candidate.Finalized,
		Explicit:         candidate.Method == detection.MethodExplicitReference,
		BookID:           reference.Book.ID,
		Chapter:          reference.Chapter,
		VerseScoped:      reference.VerseStart > 0,
		PassageAvailable: passageErr == nil && len(passage) > 0,
		SettledInSegment: settledInSegment,
		Repeats:          candidate.Repeats,
		Band:             confidence.BandOf(candidate.Confidence),
	})
}

// autoProjectionChecklist explains a held-back card to the operator. It stays nil
// when auto-projection is off or paused — nothing is being withheld by a gate
// then — and when the card would have been projected.
func (a *App) autoProjectionChecklist(candidate detection.Candidate, settledInSegment int) *AutoProjectionChecklist {
	if a.settings == nil || a.bible == nil || !candidate.Finalized {
		return nil
	}
	ctx := a.context()
	if !a.autoProjectionEnabled(ctx) || a.autoProjectionPaused(ctx) || a.detectionDisabled() {
		return nil
	}
	verdict := a.autoProjectionVerdict(ctx, candidate, settledInSegment)
	if verdict.Project {
		return nil
	}
	checklist := &AutoProjectionChecklist{Reason: verdict.Reason, Gates: make([]GateView, 0, len(verdict.Gates))}
	for _, gate := range verdict.Gates {
		checklist.Gates = append(checklist.Gates, GateView{ID: gate.ID, Label: gate.Label, Passed: gate.Passed})
	}
	return checklist
}

func (a *App) detectionDisabled() bool {
	if a.settings == nil {
		return false
	}
	return a.settings.GetDefault(a.context(), settings.KeyDetectionDisabled, "false") == "true"
}

func (a *App) detectionEngine() (*asr.ExecutableEngine, error) {
	spec := strings.TrimSpace(a.settings.GetDefault(a.context(), settings.KeyAsrEngine, ""))
	if spec == "" {
		return nil, errors.New("no speech engine is configured — set one in Settings")
	}
	return asr.ParseEngineSpec(spec)
}

func (a *App) toDetectionViews(candidates []detection.Candidate) []DetectionView {
	// One card per settled passage is the norm; two from the same segment means
	// the conflict gate refused both, and the checklist has to say so.
	settledBySegment := make(map[int]int, len(candidates))
	for _, candidate := range candidates {
		if candidate.Finalized {
			settledBySegment[candidate.Segment]++
		}
	}

	views := make([]DetectionView, 0, len(candidates))
	for _, candidate := range candidates {
		reference := candidate.Reference
		views = append(views, DetectionView{
			ID:             candidate.ID,
			Transcript:     candidate.Transcript,
			Reference:      reference.String(),
			Method:         candidate.Method,
			Confidence:     candidate.Confidence,
			ConfidenceBand: confidence.BandOf(candidate.Confidence),
			Explanation:    candidate.Explanation,
			Segment:        candidate.Segment,
			Version:        candidate.Version,
			Finalized:      candidate.Finalized,
			Repeats:        candidate.Repeats,
			BookID:         reference.Book.ID,
			Chapter:        reference.Chapter,
			VerseStart:     reference.VerseStart,
			VerseEnd:       reference.VerseEnd,
			CreatedAt:      candidate.CreatedAt.Format("15:04:05"),
			AutoProjection: a.autoProjectionChecklist(candidate, settledBySegment[candidate.Segment]),
		})
	}
	return views
}
