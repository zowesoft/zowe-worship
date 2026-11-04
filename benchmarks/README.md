# ASR Spoken-Reference Benchmark

Measures how many **spoken scripture references survive transcription and
detection** — the accuracy metric that decides which ASR engine backs the
future Live Detection workspace (PRD section 9: AI detection is a suggestion
only; the operator always confirms).

The pipeline: recording → 16 kHz mono conversion → engine sidecar → transcript
→ `asr.DetectReferences` → matched against gold references in `manifest.json`.

## Layout

```
benchmarks/
  fetch.ps1        downloads whisper.cpp CPU binaries + tiny.en model
  make-tts-set.ps1 generates a synthetic sermon set (Windows SAPI voices)
  sample-set/      detector-only demo set (no audio required)
  smoke-set/       engine smoke test (SAPI-synthesized speech + tone)
  tts-set/         generated sermon set with varied reference phrasings
  models/          downloaded .ggml/.onnx models          [gitignored]
  tools/           downloaded engine binaries            [gitignored]
  dumps/           -dump transcripts for debugging       [gitignored]
  sermon-set/      create this with YOUR real recordings (see below)
```

`tools/` and `models/` are machine-local. Everything else stays in git.

## Quick start

```powershell
task asr:bench:fetch     # whisper.cpp (CPU x64) + ggml-tiny.en.bin (~75 MB)
task asr:bench           # detector-only demo: no models, no audio
task asr:bench:whisper   # full engine smoke test
powershell -File benchmarks/make-tts-set.ps1   # regenerate tts-set
```

Everything is pure Go except the sidecar binaries, so the same commands work
on Windows, macOS and Linux — only `fetch.ps1` (and its per-OS asset choice)
is Windows-specific today. Release asset names on the whisper.cpp nightly
builds follow `whisper-bin-<os>-<arch>` conventions. `make-tts-set.ps1` is
also Windows-bound (SAPI); on other platforms use edge-tts or piper to
synthesize the same scripts.

## Building a real set

Record (or collect) sermon clips where references are actually spoken, save
them as WAV into e.g. `benchmarks/sermon-set/`, and write `manifest.json`:

```json
{
  "name": "sunday-2026-10",
  "clips": [
    {
      "id": "opening-call",
      "file": "call-to-worship.wav",
      "references": ["Romans 8:28"],
      "transcript": "optional gold text for WER scoring"
    }
  ]
}
```

- `references` — gold truth; chapter-level hits count, verse-exact is
  reported separately (ranges legitimately widen).
- `transcript` — optional; makes WER appear in the report.
- No `file` + a `transcript` = detector-only clip (no audio needed).

Any WAV encoding the engine ecosystem produces (8/16/24/32-bit PCM, 32/64-bit
float, any channel layout) converts automatically to 16 kHz mono first.

## Engine specs

```
-engine name=binary args... {audio}
```

`{audio}` is replaced by the prepared clip path. Quote arguments containing
spaces. Examples:

```powershell
# whisper.cpp, Windows (CPU)
-engine "whisper.tiny=benchmarks/tools/Release/whisper-cli.exe -m benchmarks/models/ggml-tiny.en.bin -np -f {audio}"

# same engine, macOS/Linux
-engine "whisper.tiny=whisper-cli -m models/ggml-tiny.en.bin -np -f {audio}"

# Moonshine tiny (int8) via sherpa-onnx — one runtime also runs Parakeet
$SHERPA = "benchmarks/tools/sherpa-onnx-v2.0.1-win-x64-shared-MT-Release/bin"   # Windows
# $SHERPA = "benchmarks/tools/sherpa-onnx-osx-arm64-shared/bin"                # macOS (adjust tarball)
-engine "moonshine.tiny=$SHERPA/sherpa-onnx-offline.exe --moonshine-preprocessor=benchmarks/models/moonshine-tiny-en-int8/preprocess.onnx --moonshine-encoder=benchmarks/models/moonshine-tiny-en-int8/encode.int8.onnx --moonshine-uncached-decoder=benchmarks/models/moonshine-tiny-en-int8/uncached_decode.int8.onnx --moonshine-cached-decoder=benchmarks/models/moonshine-tiny-en-int8/cached_decode.int8.onnx --tokens=benchmarks/models/moonshine-tiny-en-int8/tokens.txt {audio}"

# Parakeet TDT 0.6B v3 (int8) via the same sherpa-onnx runtime
-engine "parakeet.v3=$SHERPA/sherpa-onnx-offline.exe --encoder=benchmarks/models/parakeet-tdt-0.6b-v3-int8/encoder.int8.onnx --decoder=benchmarks/models/parakeet-tdt-0.6b-v3-int8/decoder.int8.onnx --joiner=benchmarks/models/parakeet-tdt-0.6b-v3-int8/joiner.int8.onnx --tokens=benchmarks/models/parakeet-tdt-0.6b-v3-int8/tokens.txt {audio}"
```

Multiple `-engine` flags run in one pass for a comparison table. Whisper
segment stamps and sherpa-onnx JSON result lines are stripped to plain text
before scoring; engine log noise on stdout is removed automatically.

## Metrics

- **recall** — spoken references detected / gold references (the number that
  matters most: a missed reference is a missed opportunity to serve the
  speaker).
- **precision** — of detected references, how many were actually spoken
  (false positives mean the operator sees junk candidates).
- **verse-exact** — detected reference resolved the exact verseStart.
- **WER** — word error rate when gold text exists (context: transcriber
  quality; WER alone does not predict detection success — `Romans 8` from a
  mangled `Romans 8:28` still projects the right chapter).

## Current model tiers

| Tier | Engine | Models | Disk (models only) |
|------|--------|--------|--------------------|
| low | moonshine.tiny (sherpa-onnx, int8) | `moonshine-tiny-en-int8/` | ~124 MB |
| default | whisper.tiny (whisper.cpp) | `ggml-tiny.en.bin` | ~75 MB |
| high | parakeet.v3 (sherpa-onnx, int8) | `parakeet-tdt-0.6b-v3-int8/` | ~671 MB |

Notes:
- Any single bundle stays below an 800 MB disk budget; the three tiers
  together cost ~870 MB, so models can be deleted after benchmarking.
- **Moonshine degenerates on long inputs**: beyond roughly 20–30 s its decoder
  can fall into a repetition loop ("I'm a little bit more than a," ×400),
  pinning the CPU and growing memory until a small machine thrashes. Keep it
  to short chunks (the live pipeline analyzes ~20 s); use whisper.cpp or
  Parakeet for whole-clip transcription.
- `whisper.cpp` also has `small.en` (~466 MB) as a middle option via the same
  CLI — add the model file if you want it scored.
- Parakeet v3 is multilingual (25 European languages) and reports ~6.3% WER
  on the HF leaderboard; Moonshine targets latency over accuracy; whisper
  `tiny.en` is the accuracy floor for smoke-testing the pipeline.
- The sherpa-onnx runtime (~90 MB unpacked, per OS) sits in `tools/`, not the
  model budget, but counts toward your disk: total toolchain ≈ 1 GB with all
  tiers installed.

The whole point of this harness: **benchmark those on real sanctuary audio
through the actual detector before committing.**

