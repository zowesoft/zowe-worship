package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// DetectionView is one candidate in the approval queue. The reference fields
// mirror ScriptureRequest so the frontend can preview the passage without a
// second lookup.
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

// DetectionResult answers one analyze request: what was heard and what the
// detector made of it. The transcript is returned even when nothing was
// detected, so the panel can show why the queue stayed empty.
type DetectionResult struct {
	Transcript string          `json:"transcript"`
	Candidates []DetectionView `json:"candidates"`
}

// DetectionConfigView is the Live Detection settings card: which engine
// transcribes and whether the emergency switch is on.
type DetectionConfigView struct {
	EngineSpec string `json:"engineSpec"`
	Disabled   bool   `json:"disabled"`
}

// DetectScripture transcribes one chunk of captured audio and queues every
// reference the detector finds. Nothing is projected here: the operator
// approves a candidate first, per the PRD.
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

	// The engine works file-based, so the chunk becomes a temporary WAV. The
	// frontend already delivers 16 kHz mono 16-bit PCM, which is exactly what
	// the WAV wrapper stores.
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

	transcript, err := spec.Transcribe(wavPath)
	if err != nil {
		return DetectionResult{}, err
	}

	added, err := a.detection.Add(transcript, asr.DetectReferences(transcript))
	if err != nil {
		return DetectionResult{}, err
	}
	return DetectionResult{
		Transcript: transcript,
		Candidates: toDetectionViews(added),
	}, nil
}

// ListDetections returns the candidates waiting for a decision.
func (a *App) ListDetections() ([]DetectionView, error) {
	if a.detection == nil {
		return nil, errStorageUnavailable
	}
	return toDetectionViews(a.detection.List()), nil
}

// ApproveDetection previews the candidate's passage in the default
// translation. Per the PRD the operator still sends it live by hand, so
// approval alone never changes what the audience sees.
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

// DismissDetection drops one candidate and returns the queue that remains.
func (a *App) DismissDetection(id string) ([]DetectionView, error) {
	if a.detection == nil {
		return nil, errStorageUnavailable
	}
	if _, err := a.detection.Dismiss(a.context(), strings.TrimSpace(id)); err != nil {
		return nil, err
	}
	return toDetectionViews(a.detection.List()), nil
}

// ClearDetections empties the queue and returns the empty list, so the
// frontend can adopt the result directly.
func (a *App) ClearDetections() ([]DetectionView, error) {
	if a.detection == nil {
		return nil, errStorageUnavailable
	}
	if err := a.detection.Clear(a.context()); err != nil {
		return nil, err
	}
	return []DetectionView{}, nil
}

// EngineOption is one speech engine discovered on this computer, ready to be
// selected in Settings without typing a path.
type EngineOption struct {
	Label string `json:"label"`
	Spec  string `json:"spec"`
}

// ListDetectionEngines scans the usual engine locations — the benchmarks
// folder beside the app in development and an engines folder under the data
// root — and returns every engine/model pair found there.
func (a *App) ListDetectionEngines() ([]EngineOption, error) {
	var roots []string
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, filepath.Join(cwd, "benchmarks"))
	}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Join(filepath.Dir(exe), "benchmarks"))
	}
	if paths, err := a.paths(); err == nil {
		roots = append(roots, filepath.Join(paths.Root, "engines"))
	}

	discovered := asr.DiscoverEngines(roots...)
	options := make([]EngineOption, 0, len(discovered))
	for _, engine := range discovered {
		options = append(options, EngineOption{Label: engine.Name, Spec: engine.Spec})
	}
	return options, nil
}

// GetDetectionConfig reads the Live Detection settings card.
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

// SetDetectionEngine stores the engine spec after parsing it, so a typo is
// rejected here instead of failing mid-service.
func (a *App) SetDetectionEngine(spec string) error {
	if a.settings == nil {
		return errStorageUnavailable
	}
	if _, err := asr.ParseEngineSpec(strings.TrimSpace(spec)); err != nil {
		return err
	}
	return a.settings.Set(a.context(), settings.KeyAsrEngine, strings.TrimSpace(spec))
}

// SetDetectionDisabled writes the F12 emergency switch. It only stops new
// detections; the audience display is left exactly as it is.
func (a *App) SetDetectionDisabled(disabled bool) error {
	if a.settings == nil {
		return errStorageUnavailable
	}
	value := "false"
	if disabled {
		value = "true"
	}
	return a.settings.Set(a.context(), settings.KeyDetectionDisabled, value)
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
