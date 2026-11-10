# Records a real-speech benchmark set: one clip per line of a spoken script,
# plus manifest.json with gold references so asr-bench can score the set and
# print the confidence threshold sweep.
#
# The tts-set proves the pipeline but is clean synthetic audio; only recordings
# of an actual speaker, through the room's mic, say anything about the shipped
# confidence bands.
#
# Usage:
#   powershell -File benchmarks/record-set.ps1 -ListDevices
#   powershell -File benchmarks/record-set.ps1 -Device "Microphone (USB Audio)"
#   powershell -File benchmarks/record-set.ps1 -Lines benchmarks/calibration-lines.txt
#
# Output: benchmarks/sermon-set/ (gitignored: real recordings stay on the
# machine that made them) with clip WAVs and an appended manifest.json.

param(
    [string]$SetDir = "benchmarks\sermon-set",
    [string]$Device = "",
    [string]$Lines = "benchmarks\calibration-lines.txt",
    [double]$Seconds = 8,
    [int]$Countdown = 3,
    [switch]$ListDevices
)

$ErrorActionPreference = "Stop"

function Find-Ffmpeg {
    $cmd = Get-Command ffmpeg -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $local = "benchmarks\tools\ffmpeg.exe"
    if (Test-Path $local) { return (Resolve-Path $local).Path }
    throw "ffmpeg not found: install it (choco install ffmpeg) or put ffmpeg.exe in benchmarks\tools\"
}

# ffmpeg prints the dshow device names on stderr while failing to open the dummy
# input, so the exit code is expected to be non-zero here.
function Get-AudioDevices([string]$Ffmpeg) {
    # Under $ErrorActionPreference = "Stop", a native command writing to stderr
    # (which ffmpeg always does) raises NativeCommandError and kills the script.
    $previous = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $out = & $Ffmpeg -hide_banner -list_devices true -f dshow -i dummy 2>&1 | Out-String
    $ErrorActionPreference = $previous
    $devices = @()
    foreach ($line in $out -split "`r?`n") {
        if ($line -match '"([^"]+)"\s+\(audio\)') {
            $devices += $Matches[1]
        }
    }
    return $devices
}

$ffmpeg = Find-Ffmpeg
$devices = Get-AudioDevices $ffmpeg

if ($ListDevices) {
    if ($devices.Count -eq 0) {
        Write-Host "No DirectShow audio capture devices found."
    } else {
        Write-Host "Audio capture devices:"
        $devices | ForEach-Object { Write-Host "  $_" }
    }
    exit 0
}

if ([string]::IsNullOrWhiteSpace($Device)) {
    if ($devices.Count -eq 1) {
        $Device = $devices[0]
        Write-Host "Using the only capture device: $Device"
    } else {
        Write-Host "Pass -Device with one of:"
        $devices | ForEach-Object { Write-Host "  $_" }
        throw "no capture device selected (use -ListDevices to see them)"
    }
}

if (-not (Test-Path $Lines)) { throw "spoken script not found: $Lines" }

# Script format, one clip per line:
#   <references> :: <sentence to read aloud>
# references is one or more references separated by ';' (written form, the gold
# label), or empty for a clip that must yield nothing (announcements, music).
$planned = @()
$number = 0
foreach ($raw in Get-Content $Lines) {
    $line = $raw.Trim()
    if ($line -eq "" -or $line.StartsWith("#")) { continue }
    $split = $line.IndexOf("::")
    if ($split -lt 0) { throw "line without '::': $line" }
    $refsText = $line.Substring(0, $split).Trim()
    $spoken = $line.Substring($split + 2).Trim()
    if ($spoken -eq "") { throw "line with nothing to read: $line" }

    $number++
    $refs = @()
    if ($refsText -ne "") {
        $refs = @($refsText.Split(";") | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" })
    }
    # A file name the operator and the report can both read at a glance.
    $slug = if ($refs.Count -gt 0) { $refs[0] } else { "no-reference" }
    $slug = ($slug.ToLower() -replace "[^a-z0-9]+", "-").Trim("-")
    $planned += [pscustomobject]@{
        ID       = "{0:d2}-{1}" -f $number, $slug
        File     = "{0:d2}-{1}.wav" -f $number, $slug
        Refs     = $refs
        Spoken   = $spoken
    }
}

if ($planned.Count -eq 0) { throw "$Lines declares no clips" }

New-Item -ItemType Directory -Force $SetDir | Out-Null
$manifestPath = Join-Path $SetDir "manifest.json"
$setName = Split-Path (Resolve-Path $SetDir) -Leaf

$clips = @()
if (Test-Path $manifestPath) {
    # Re-running a recording session appends; keep what is already there.
    $existing = Get-Content $manifestPath -Raw | ConvertFrom-Json
    if ($existing.clips) { $clips = @($existing.clips) }
    if ($existing.name) { $setName = $existing.name }
}

$failed = @()
foreach ($clip in $planned) {
    $wav = Join-Path $SetDir $clip.File
    Write-Host ""
    Write-Host "--------------------------------------------------------------"
    Write-Host ("{0}  ({1})" -f $clip.ID, ($(if ($clip.Refs.Count) { $clip.Refs -join ", " } else { "no reference expected" })))
    Write-Host $clip.Spoken
    Write-Host "--------------------------------------------------------------"
    for ($i = $Countdown; $i -gt 0; $i--) {
        Write-Host ("  recording in {0}..." -f $i)
        Start-Sleep -Seconds 1
    }
    Write-Host ("  recording {0}s" -f $Seconds)

    $ffargs = @(
        "-hide_banner", "-loglevel", "error", "-y",
        "-f", "dshow", "-i", ("audio={0}" -f $Device),
        "-t", $Seconds,
        "-ac", 1, "-ar", 16000, "-c:a", "pcm_s16le",
        $wav
    )
    $previous = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $err = & $ffmpeg @ffargs 2>&1 | Out-String
    $code = $LASTEXITCODE
    $ErrorActionPreference = $previous
    if ($code -ne 0 -or -not (Test-Path $wav) -or (Get-Item $wav).Length -lt 44) {
        Write-Host ("  FAILED: {0}" -f ($err.Trim() -split "`n" | Select-Object -Last 2)) -ForegroundColor Red
        $failed += $clip.ID
        continue
    }

    # Re-recording an existing id replaces its entry instead of duplicating it.
    $clips = @($clips | Where-Object { $_.id -ne $clip.ID })
    $clips += [pscustomobject]@{
        id         = $clip.ID
        file       = $clip.File
        references = $clip.Refs
        transcript = $clip.Spoken
    }
    Write-Host ("  saved {0}" -f $wav)
}

$manifest = [pscustomobject]@{ name = $setName; clips = $clips }
$json = $manifest | ConvertTo-Json -Depth 6
# Windows PowerShell writes UTF-8 with BOM; asr-bench strips it, and so does
# every other editor here, so leave the encoding default.
Set-Content -Path $manifestPath -Value $json -Encoding utf8

Write-Host ""
Write-Host ("manifest: {0} ({1} clip(s))" -f $manifestPath, $clips.Count)
if ($failed.Count -gt 0) {
    Write-Host ("failed clips: {0}" -f ($failed -join ", ")) -ForegroundColor Red
    exit 1
}
Write-Host "Next: go run ./cmd/asr-bench -set $SetDir -engine `"whisper.tiny=... -f {audio}`""
