# Fetches the benchmark toolchain for Zopresent. The harness itself is pure
# Go and runs on Windows, macOS and Linux; only the downloads differ per OS.
# Usage: powershell -File benchmarks/fetch.ps1
$ErrorActionPreference = "Stop"

New-Item -ItemType Directory -Force benchmarks\tools, benchmarks\models | Out-Null

# whisper.cpp CPU build for the host architecture. Release asset names follow
# the nightly build tag scheme (bNNNN); adjust if the tag moved on.
$tag = "b5454"
$base = "https://github.com/ggml-org/whisper.cpp/releases/download/$tag"

if (-not (Test-Path benchmarks\tools\Release\whisper-cli.exe)) {
    Invoke-WebRequest -Uri "$base/whisper-bin-x64.zip" -OutFile benchmarks\tools\whisper-bin-x64.zip
    cmd /c "cd /d benchmarks\tools && tar -xf whisper-bin-x64.zip"
    Remove-Item benchmarks\tools\whisper-bin-x64.zip
}

if (-not (Test-Path benchmarks\models\ggml-tiny.en.bin)) {
    # 75 MB smoke-test model. Drop-in equivalents for the benchmark:
    # small.en (~466 MB), distil-small.en, base.en.
    Invoke-WebRequest -Uri "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.en.bin" `
        -OutFile benchmarks\models\ggml-tiny.en.bin
}

# --- sherpa-onnx runtime (runs Moonshine AND Parakeet from ONNX) -----------
# For other platforms pick the matching asset from:
#   https://github.com/k2-fsa/sherpa-onnx/releases/tag/v2.0.1
#   macOS arm64: sherpa-onnx-v2.0.1-osx-arm64-shared.tar.bz2
#   linux x64:   sherpa-onnx-v2.0.1-linux-x64-shared.tar.bz2
$SHERPA_TAG = "v2.0.1"
$SHERPA_DIR = "benchmarks\tools\sherpa-onnx-v2.0.1-win-x64-shared-MT-Release"
if (-not (Test-Path "$SHERPA_DIR\bin\sherpa-onnx-offline.exe")) {
    Invoke-WebRequest -Uri "https://github.com/k2-fsa/sherpa-onnx/releases/download/$SHERPA_TAG/sherpa-onnx-$SHERPA_TAG-win-x64-shared-MT-Release.tar.bz2" `
        -OutFile benchmarks\tools\sherpa-win-x64.tar.bz2
    cmd /c "cd /d benchmarks\tools && tar -xjf sherpa-win-x64.tar.bz2"
    Remove-Item benchmarks\tools\sherpa-win-x64.tar.bz2
}

# --- Moonshine tiny int8 (~124 MB total) -----------------------------------
$moonshineBase = "https://huggingface.co/csukuangfj/sherpa-onnx-moonshine-tiny-en-int8/resolve/main"
foreach ($file in @("preprocess.onnx", "encode.int8.onnx", "uncached_decode.int8.onnx", "cached_decode.int8.onnx", "tokens.txt")) {
    $dest = "benchmarks\models\moonshine-tiny-en-int8\$file"
    New-Item -ItemType Directory -Force benchmarks\models\moonshine-tiny-en-int8 | Out-Null
    if (-not (Test-Path $dest)) {
        Invoke-WebRequest -Uri "$moonshineBase/$file" -OutFile $dest
    }
}

# --- Parakeet TDT 0.6B v3 int8 (~671 MB total) -----------------------------
# Downloaded with curl.exe (ships with Windows 10+/most systems): large
# HuggingFace files die with "unexpected EOF" in Invoke-WebRequest, while
# curl supports --retry and -C - resume.
function Fetch-Large {
    param([string]$Url, [string]$Dest)
    if (Test-Path $Dest) { return }
    New-Item -ItemType Directory -Force (Split-Path $Dest) | Out-Null
    curl.exe -L --fail --retry 10 --retry-delay 3 --retry-all-errors -C - -o $Dest $Url
    if ($LASTEXITCODE -ne 0) {
        throw "download failed: $Url (curl exit $LASTEXITCODE)"
    }
}

Fetch-Large "https://huggingface.co/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/resolve/main/encoder.int8.onnx" "benchmarks\models\parakeet-tdt-0.6b-v3-int8\encoder.int8.onnx"
Fetch-Large "https://huggingface.co/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/resolve/main/decoder.int8.onnx" "benchmarks\models\parakeet-tdt-0.6b-v3-int8\decoder.int8.onnx"
Fetch-Large "https://huggingface.co/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/resolve/main/joiner.int8.onnx" "benchmarks\models\parakeet-tdt-0.6b-v3-int8\joiner.int8.onnx"
Fetch-Large "https://huggingface.co/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/resolve/main/tokens.txt" "benchmarks\models\parakeet-tdt-0.6b-v3-int8\tokens.txt"

Write-Host "Tools ready:"
Get-ChildItem benchmarks\tools\Release\*.exe | Select-Object Name
Get-ChildItem benchmarks\models | Select-Object Name
