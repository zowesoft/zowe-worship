package asr

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

type Engine interface {
	Name() string
	Transcribe(path string) (string, error)
}

// DefaultTranscribeTimeout is sized for a full live church-service recording.
const DefaultTranscribeTimeout = 30 * time.Minute

// ExecutableEngine runs a sidecar transcriber; "{audio}" in Args is replaced
// per clip with the prepared 16 kHz mono WAV path.
type ExecutableEngine struct {
	name    string
	bin     string
	args    []string
	timeout time.Duration
}

// ParseEngineSpec parses a spec such as
//
//	whisper-cli="C:\tools\whisper-cli.exe" -m "%MODELS%\ggml-small.en.bin" -np -f {audio}
//
// The name before "=" is optional; without it the binary file name is used.
// Quoted arguments may contain spaces.
func ParseEngineSpec(spec string) (*ExecutableEngine, error) {
	tokens, err := splitSpec(spec)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, errors.New("empty engine spec")
	}

	name := ""
	bin := tokens[0]
	if base, tail, found := strings.Cut(bin, "="); found {
		name, bin = base, tail
		if bin == "" {
			if len(tokens) < 2 {
				return nil, fmt.Errorf("engine %q: binary path is missing", name)
			}
			bin = tokens[1]
			tokens = tokens[1:]
		}
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(bin), filepath.Ext(bin))
	}

	// The binary path is deliberately not validated here: specs may reference
	// PATH-resolved or not-yet-fetched executables; exec.Command reports absence.

	return &ExecutableEngine{
		name:    name,
		bin:     bin,
		args:    tokens[1:],
		timeout: DefaultTranscribeTimeout,
	}, nil
}

func splitSpec(spec string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	quoted := false
	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}
	for _, r := range spec {
		switch {
		case r == '"':
			quoted = !quoted
		case unicode.IsSpace(r) && !quoted:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	if quoted {
		return nil, errors.New("unterminated quote in engine spec")
	}
	flush()
	return tokens, nil
}

func (e *ExecutableEngine) Name() string { return e.name }

func (e *ExecutableEngine) Transcribe(path string) (string, error) {
	return e.TranscribeStream(path, nil)
}

// TranscribeStream fires onPartial as stamped segments arrive (whisper.cpp);
// engines that only speak at the end (sherpa-onnx offline) never call it mid-run.
func (e *ExecutableEngine) TranscribeStream(path string, onPartial func(partial string)) (string, error) {
	args := make([]string, 0, len(e.args))
	for _, arg := range e.args {
		arg = strings.ReplaceAll(arg, "{audio}", path)
		args = append(args, arg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), e.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.bin, args...)
	hideConsole(cmd)
	var stderr strings.Builder
	cmd.Stderr = &stderr

	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("engine %q stdout: %w", e.name, err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("engine %q failed to start: %w", e.name, err)
	}

	var stdout strings.Builder
	var spoken strings.Builder
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		stdout.WriteString(line)
		stdout.WriteString("\n")
		if onPartial == nil {
			continue
		}
		text, ok := segmentText(line)
		if !ok {
			continue
		}
		spoken.WriteString(text)
		spoken.WriteString(" ")
		onPartial(strings.TrimSpace(spoken.String()))
	}
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("engine %q failed: %w\nstderr: %s", e.name, err, stderrTail(stderr.String()))
	}
	return CleanTranscript(stdout.String()), nil
}

// segmentText accepts only stamped whisper.cpp segments; log noise and
// end-of-run output are unreliable to stream mid-run.
func segmentText(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	loc := segmentStamp.FindStringIndex(trimmed)
	if loc == nil || loc[0] != 0 {
		return "", false
	}
	text := strings.TrimSpace(trimmed[loc[1]:])
	if text == "" {
		return "", false
	}
	return text, true
}

func stderrTail(text string) string {
	if len(text) > 2000 {
		return text[:2000]
	}
	return text
}

var (
	// sherpa-onnx-offline prints result: {"text": "...", ...}; extracting text
	// keeps every sherpa family (Moonshine, Parakeet TDT, Whisper) on one path.
	sherpaResultText = regexp.MustCompile(`"text"\s*:\s*"((?:\\.|[^"\\])*)"`)

	// whisper.cpp stamps each segment: [00:00:00.000 --> 00:00:07.000] text.
	segmentStamp = regexp.MustCompile(`\[\d{1,2}:\d{2}:\d{2}\.\d{3}\s*-->\s*\d{1,2}:\d{2}:\d{2}\.\d{3}\]\s*`)
	// Engine log prefixes that leak onto stdout (older builds, verbose flags).
	noiseLine = regexp.MustCompile(`(?m)^\s*(whisper_|ggml_|main\s*:|system_info|load_|print_|wav |sherpa|#)\S*.*$`)
)

func CleanTranscript(raw string) string {
	// sherpa-onnx JSON result line(s) beat plain text extraction.
	if matches := sherpaResultText.FindAllStringSubmatch(raw, -1); len(matches) > 0 {
		var texts []string
		for _, match := range matches {
			texts = append(texts, unescapeJSONString(match[1]))
		}
		return strings.Join(texts, " ")
	}

	raw = segmentStamp.ReplaceAllString(raw, " ")
	raw = noiseLine.ReplaceAllString(raw, "")
	lines := strings.Split(raw, "\n")
	var kept []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, " ")
}

func unescapeJSONString(value string) string {
	replacements := [][2]string{
		{`\n`, "\n"}, {`\t`, "\t"},
		{`\\`, `\`}, {`\"`, `"`},
	}
	for _, pair := range replacements {
		value = strings.ReplaceAll(value, pair[0], pair[1])
	}
	return value
}
