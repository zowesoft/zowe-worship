package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"zowe-worship/internal/asr"
	"zowe-worship/internal/detection"
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
	BookID     int    `json:"bookId"`
	Chapter    int    `json:"chapter"`
	VerseStart int    `json:"verseStart"`
	VerseEnd   int    `json:"verseEnd"`
	CreatedAt  string `json:"createdAt"`
}

// DetectionResult returns the transcript even when nothing was detected, so
// the panel can show why the queue stayed empty.
type DetectionResult struct {
	Transcript string          `json:"transcript"`
	Candidates []DetectionView `json:"candidates"`
}

const detectionProgressEvent = "detection:progress"

// detectionConfigEvent exists because engine and switch settings live in a
// different webview; without it the studio panel stays stale until restart.
const detectionConfigEvent = "detection:config"

type DetectionProgressView struct {
	Transcript string          `json:"transcript"`
	Candidates []DetectionView `json:"candidates"`
}

type DetectionConfigView struct {
	EngineSpec string `json:"engineSpec"`
	Disabled   bool   `json:"disabled"`
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

	transcript, err := spec.TranscribeStream(wavPath, a.streamDetectionProgress)
	if err != nil {
		return DetectionResult{}, err
	}

	// The final transcript can still widen a hearing that streamed in narrow,
	// so the queue settles through the same grow-in-place merge.
	if _, err := a.detection.MergeLive(transcript, asr.DetectReferences(transcript)); err != nil {
		return DetectionResult{}, err
	}
	return DetectionResult{
		Transcript: transcript,
		Candidates: toDetectionViews(a.detection.List()),
	}, nil
}

// streamDetectionProgress merges each segment so a growing hearing replaces its
// pending candidate, and forwards the partial transcript to the studio.
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
					Candidates: toDetectionViews(a.detection.List()),
				},
			})
		}
	}
}

func (a *App) ListDetections() ([]DetectionView, error) {
	if a.detection == nil {
		return nil, errStorageUnavailable
	}
	return toDetectionViews(a.detection.List()), nil
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
	return toDetectionViews(a.detection.List()), nil
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
		EngineSpec: a.settings.GetDefault(ctx, settings.KeyAsrEngine, ""),
		Disabled:   a.detectionDisabled(),
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

func toDetectionViews(candidates []detection.Candidate) []DetectionView {
	views := make([]DetectionView, 0, len(candidates))
	for _, candidate := range candidates {
		reference := candidate.Reference
		views = append(views, DetectionView{
			ID:         candidate.ID,
			Transcript: candidate.Transcript,
			Reference:  reference.String(),
			BookID:     reference.Book.ID,
			Chapter:    reference.Chapter,
			VerseStart: reference.VerseStart,
			VerseEnd:   reference.VerseEnd,
			CreatedAt:  candidate.CreatedAt.Format("15:04:05"),
		})
	}
	return views
}
