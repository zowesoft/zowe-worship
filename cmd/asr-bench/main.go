// Command asr-bench runs the spoken-reference benchmark: it transcribes a
// set of sermon clips through one or more pluggable ASR engine sidecars,
// feeds each transcript through the detector (internal/asr), and scores how
// many spoken references survived — plus word error rate when a gold
// transcript is provided for the clip.
//
// Usage:
//
//	go run ./cmd/asr-bench -set benchmarks/sermon-set ^
//	  -engine whisper-cli=bin\whisper-cli.exe -m models\ggml-small.en.bin -np -f {audio}
//
// With no -engine flags the manifest's gold transcripts (or .txt files beside
// the clips) run through the detector alone, isolating detector behavior from
// engine behavior. See benchmarks/README.md.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zowe-worship/internal/asr"
	"zowe-worship/internal/scripture"
)

type manifest struct {
	Name  string      `json:"name"`
	Clips []benchClip `json:"clips"`
}

type benchClip struct {
	// ID is the display name for the clip (defaults to the file name or
	// "clip-NN").
	ID string `json:"id"`
	// File is the WAV path relative to the set directory. Optional in
	// detector-only mode.
	File string `json:"file"`
	// References are the gold references spoken in the clip.
	References []string `json:"references"`
	// Transcript optionally provides gold text used for WER scoring and for
	// detector-only runs.
	Transcript string `json:"transcript"`
}

type clipResult struct {
	Clip         string   `json:"clip"`
	Expected     []string `json:"expected"`
	Detected     []string `json:"detected"`
	Hits         []string `json:"hits"`
	Missed       []string `json:"missed"`
	Extra        []string `json:"extra"`
	VerseExact   int      `json:"verse_hits"`
	Transcript   string   `json:"transcript,omitempty"`
	WER          float64  `json:"wer,omitempty"`
	HasGoldWords bool     `json:"has_gold_words"`
}

type engineSummary struct {
	Engine       string       `json:"engine"`
	Clips        []clipResult `json:"clips"`
	Expected     int          `json:"expected"`
	Hits         int          `json:"hits"`
	Extra        int          `json:"extra"`
	VerseExact   int          `json:"verse_hits"`
	TranscribeMs int64        `json:"transcribe_ms"`
	MeanWER      float64      `json:"mean_wer,omitempty"`
	WERClips     int          `json:"wer_clips"`
}

type engineSummaries struct {
	Engines []engineSummary `json:"engines"`
}

func main() {
	setDir := flag.String("set", "benchmarks/sermon-set", "directory containing manifest.json and clip audio")
	var engineSpecs multiFlag
	flag.Var(&engineSpecs, "engine", "engine spec name=bin args... {audio}; repeatable")
	dumpTranscripts := flag.String("dump", "", "optional directory to write raw transcripts per engine")
	outJSON := flag.String("out", "", "optional path for a JSON report")
	flag.Parse()

	setPath = strings.TrimSuffix(*setDir, "/") // normalise windows slashes naturally
	manifestPath := filepath.Join(*setDir, "manifest.json")
	set, err := readManifest(manifestPath)
	if err != nil {
		exit(err.Error())
	}
	if len(set.Clips) == 0 {
		exit(fmt.Sprintf("%s declares no clips", manifestPath))
	}

	// Audio conversion happens once into a scratch directory, so conversion
	// cost is never attributed to a particular engine.
	workDir, cleanup := scratchDir()
	defer cleanup()

	gold := parseGold(set.Clips)

	var runs []benchmarkRun

	if len(engineSpecs.values) > 0 {
		for _, spec := range engineSpecs.values {
			engine, err := asr.ParseEngineSpec(spec)
			if err != nil {
				exit(err.Error())
			}
			texts, errs, elapsed := transcribeClips(engine, set, workDir, *dumpTranscripts)
			runs = append(runs, benchmarkRun{name: engine.Name(), texts: texts, errs: errs, elapsed: elapsed})
		}
	} else {
		texts, errs := detectorOnlyTranscripts(set)
		if *dumpTranscripts != "" {
			for i, text := range texts {
				if errs[i] == nil {
					writeDump(*dumpTranscripts, "detector-only", clipName(set.Clips[i], i), text)
				}
			}
		}
		runs = append(runs, benchmarkRun{name: "detector-only", texts: texts, errs: errs})
	}

	var report engineSummaries
	for _, run := range runs {
		report.Engines = append(report.Engines, scoreRun(set, gold, run))
	}

	printReport(report.Engines)

	if *outJSON != "" {
		if err := writeJSON(*outJSON, report); err != nil {
			exit(fmt.Sprintf("write %s: %v", *outJSON, err))
		}
		fmt.Printf("report written to %s\n", *outJSON)
	}
}

// setPath is the root directory of the active benchmark set; a single
// process run examines one set at a time.
var setPath string

func readManifest(path string) (manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, err
	}
	// Windows tools (PowerShell < 7) emit UTF-8 with BOM; strip it.
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return manifest{}, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// transcribeClips drives the engine over every clip, converting audio to
// 16 kHz mono first where the recording needs it.
func transcribeClips(engine asr.Engine, set manifest, workDir, dump string) ([]string, []error, int64) {
	texts := make([]string, len(set.Clips))
	errs := make([]error, len(set.Clips))
	var elapsed int64

	for i, clip := range set.Clips {
		if clip.File == "" {
			// Detector-only clip passing through an engine run.
			texts[i], errs[i] = clipTranscript(clip)
			continue
		}

		audioPath := filepath.Join(setPath, clip.File)
		ready, err := asr.Prepare16kMono(audioPath, workDir)
		if err != nil {
			errs[i] = err
			continue
		}

		start := time.Now()
		text, err := engine.Transcribe(ready)
		elapsed += int64(time.Since(start) / time.Millisecond)
		if err != nil {
			errs[i] = err
			continue
		}
		texts[i] = text
		if dump != "" {
			writeDump(dump, engine.Name(), clipName(clip, i), text)
		}
	}
	return texts, errs, elapsed
}

// detectorOnlyTranscripts resolves transcript text for clips when no engine
// is configured: gold manifest text wins, else a sibling .txt file.
func detectorOnlyTranscripts(set manifest) ([]string, []error) {
	texts := make([]string, len(set.Clips))
	errs := make([]error, len(set.Clips))
	for i, clip := range set.Clips {
		texts[i], errs[i] = clipTranscript(clip)
	}
	return texts, errs
}

func clipTranscript(clip benchClip) (string, error) {
	if clip.Transcript != "" {
		return clip.Transcript, nil
	}
	if clip.File == "" {
		return "", fmt.Errorf("clip has neither file nor gold transcript")
	}
	base := strings.TrimSuffix(clip.File, filepath.Ext(clip.File))
	txt := filepath.Join(setPath, base+".txt")
	if raw, err := os.ReadFile(txt); err == nil {
		return string(raw), nil
	}
	return "", fmt.Errorf("no transcript source for %q", clip.File)
}

func clipName(clip benchClip, index int) string {
	if clip.ID != "" {
		return clip.ID
	}
	if clip.File != "" {
		return strings.TrimSuffix(clip.File, filepath.Ext(clip.File))
	}
	return fmt.Sprintf("clip-%02d", index)
}

// goldRefs stores parsed gold references per clip index.
type goldRefs = [][]scripture.Reference

func parseGold(clips []benchClip) goldRefs {
	gold := make(goldRefs, len(clips))
	for i, clip := range clips {
		for _, ref := range clip.References {
			parsed, err := scripture.Parse(ref)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: manifest gold %q: %v\n", ref, err)
				continue
			}
			gold[i] = append(gold[i], parsed)
		}
	}
	return gold
}

// benchmarkRun is one engine's output over the manifest.
type benchmarkRun struct {
	name    string
	texts   []string
	errs    []error
	elapsed int64
}

func scoreRun(set manifest, gold goldRefs, run benchmarkRun) engineSummary {
	summary := engineSummary{Engine: run.name, TranscribeMs: run.elapsed}

	for i := range set.Clips {
		clip := set.Clips[i]
		result := clipResult{Clip: clipName(clip, i), Expected: clip.References}

		if run.errs[i] != nil {
			result.Detected = []string{fmt.Sprintf("ERROR: %v", run.errs[i])}
			result.Missed = clip.References
			summary.Clips = append(summary.Clips, result)
			summary.Expected += len(clip.References)
			continue
		}

		detections := asr.DetectReferences(run.texts[i])
		result.Transcript = run.texts[i]

		// Greedy bipartite match on book+chapter; verse precision is
		// reported separately because ranges legitimately widen.
		matchedDetection := make([]bool, len(detections))
		matchedGold := make([]bool, len(gold[i]))
		for d := range detections {
			for g := range gold[i] {
				if matchedGold[g] {
					continue
				}
				if detections[d].Reference.Book.ID == gold[i][g].Book.ID &&
					detections[d].Reference.Chapter == gold[i][g].Chapter {
					matchedDetection[d] = true
					matchedGold[g] = true
					result.Hits = append(result.Hits, detections[d].Reference.String())
					if detections[d].Reference.VerseStart == gold[i][g].VerseStart {
						result.VerseExact++
					}
					break
				}
			}
		}
		for d := range detections {
			result.Detected = append(result.Detected, detections[d].Reference.String())
			if !matchedDetection[d] {
				result.Extra = append(result.Extra, detections[d].Reference.String())
			}
		}
		for g := range gold[i] {
			if !matchedGold[g] {
				result.Missed = append(result.Missed, gold[i][g].String())
			}
		}

		// WER is meaningful only when the transcript came out of an engine;
		// detector-only mode echoes the gold text, so scoring it would be 0.
		if clip.Transcript != "" && run.name != "detector-only" {
			result.HasGoldWords = true
			result.WER = wordErrorRate(run.texts[i], clip.Transcript)
			summary.WERClips++
		}

		summary.Expected += len(clip.References)
		summary.Hits += len(result.Hits)
		summary.Extra += len(result.Extra)
		summary.VerseExact += result.VerseExact
		if result.HasGoldWords {
			summary.MeanWER += result.WER
		}
		summary.Clips = append(summary.Clips, result)
	}

	if summary.WERClips > 0 {
		summary.MeanWER /= float64(summary.WERClips)
	}
	return summary
}

func wordErrorRate(hypothesis, reference string) float64 {
	h := words(hypothesis)
	g := words(reference)
	if len(g) == 0 {
		return 0
	}
	if len(h) == 0 {
		return 1
	}
	prev := make([]int, len(h)+1)
	row := make([]int, len(h)+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= len(g); i++ {
		prev, row = row, prev
		row[0] = i
		for j := 1; j <= len(h); j++ {
			substitution := prev[j-1]
			if g[i-1] != h[j-1] {
				substitution++
			}
			row[j] = minInt(prev[j]+1, row[j-1]+1, substitution)
		}
	}
	return float64(row[len(h)]) / float64(len(g))
}

func words(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !isWordRune(r)
	})
}

func isWordRune(r rune) bool {
	isAlnum := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
	return isAlnum
}

func minInt(values ...int) int {
	smallest := values[0]
	for _, v := range values[1:] {
		if v < smallest {
			smallest = v
		}
	}
	return smallest
}

func printReport(summaries []engineSummary) {
	for _, s := range summaries {
		fmt.Printf("== %s ==\n", s.Engine)
		recall := 0.0
		if s.Expected > 0 {
			recall = float64(s.Hits) / float64(s.Expected) * 100
		}
		precision := 0.0
		if s.Hits+s.Extra > 0 {
			precision = float64(s.Hits) / float64(s.Hits+s.Extra) * 100
		}
		fmt.Printf("   recall %d/%d (%.1f%%)  precision %.1f%%  verse-exact %d/%d\n",
			s.Hits, s.Expected, recall, precision, s.VerseExact, s.Expected)
		if s.MeanWER > 0 {
			fmt.Printf("   mean WER %.1f%% over %d clips\n", s.MeanWER*100, s.WERClips)
		}
		for _, clip := range s.Clips {
			state := "OK"
			switch {
			case strings.Contains(strings.Join(clip.Detected, ""), "ERROR:"):
				state = "ERROR"
			case len(clip.Missed) > 0:
				state = "MISS"
			case len(clip.Extra) > 0:
				state = "EXTRA"
			}
			fmt.Printf("   %-24s %s", clip.Clip, state)
			if len(clip.Extra) > 0 {
				fmt.Printf("  extra=%v", clip.Extra)
			}
			if clip.HasGoldWords {
				fmt.Printf("  wer=%.1f%%", clip.WER*100)
			}
			fmt.Println()
			if len(clip.Missed) > 0 {
				fmt.Printf("        missed %v\n", clip.Missed)
			}
			if state != "OK" && strings.TrimSpace(clip.Transcript) != "" {
				shown := clip.Transcript
				if len(shown) > 140 {
					shown = shown[:140] + " [...]"
				}
				fmt.Printf("        | %s\n", shown)
			}
		}
		fmt.Println()
	}
}

func writeJSON(path string, report engineSummaries) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeDump(dir, engine, clip, text string) {
	if err := os.MkdirAll(filepath.Join(dir, engine), 0o755); err != nil {
		return
	}
	safe := strings.NewReplacer(
		":", "", "/", "-", "\\", "-", "?", "-", "*", "-", "\"", "'",
	).Replace(clip)
	_ = os.WriteFile(filepath.Join(dir, engine, safe+".txt"), []byte(text), 0o644)
}

func scratchDir() (string, func()) {
	dir, err := os.MkdirTemp("", "asr-bench")
	if err != nil {
		exit(err.Error())
	}
	return dir, func() { _ = os.RemoveAll(dir) }
}

func exit(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

type multiFlag struct{ values []string }

func (m *multiFlag) String() string { return strings.Join(m.values, "\n") }

func (m *multiFlag) Set(value string) error {
	m.values = append(m.values, value)
	return nil
}
