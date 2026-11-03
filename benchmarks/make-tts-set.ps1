# Generates a synthetic sermon benchmark set: text -> WAV via Windows SAPI,
# plus manifest.json with gold references so asr-bench can score the set.
#
# This produces CLEAN synthetic audio (no room reverb, no PA distortion), so
# treat results as optimistic upper bounds. It exists to vary the spoken
# REFERENCE PHRASINGS, which LibriVox-style Bible reading audio never
# contains. Benchmark real sanctuary recordings before picking the engine.
#
# Usage:  powershell -File benchmarks/make-tts-set.ps1
# Output: benchmarks/tts-set/ (gitignored audio; manifest is committed)

$ErrorActionPreference = "Stop"
$setDir = "benchmarks\tts-set"
New-Item -ItemType Directory -Force $setDir | Out-Null

Add-Type -AssemblyName System.Speech
$synth = [System.Speech.Synthesis.SpeechSynthesizer]::new()

# Vary rate/voice between segments so no two clips sound like test data.
$voiceNames = @($synth.GetInstalledVoices() | ForEach-Object { $_.VoiceInfo.Name })

#嫁给 rate/voice variety into segments so no two clips sound like test data.
$segments = @(
    @{
        id    = "opening-romans"
        rate  = -2
        refs  = @("Romans 8:28")
        text  = "Good morning, everyone. Let us open our Bibles this morning to Romans, chapter eight, verse twenty eight. And we know that all things work together for good to them that love God. Let us pray."
    },
    @{
        id    = "second-corinthians"
        rate  = 1
        refs  = @("2 Corinthians 4")
        text  = "This morning we continue in second Corinthians, chapter four. In the middle of trouble we do not lose heart. Our light affliction works a far more eternal weight of glory."
    },
    @{
        id    = "gospel-psalm-combo"
        rate  = 0
        refs  = @("John 3:16", "Psalms 51:10")
        text  = "Please turn to John, three sixteen. And then hold your place and go over to Psalm fifty one, verse ten, and we will come back to the gospel in a moment."
    },
    @{
        id    = "matthew-range"
        rate  = 2
        refs  = @("Matthew 11:28-30")
        text  = "The passage for tonight is Matthew, chapter eleven, verses twenty eight through thirty. Come unto me, all ye that labour and are heavy laden, and I will give you rest."
    },
    @{
        id    = "announcements-trap"
        rate  = -1
        refs  = @()
        text  = "Before we open the word, a few announcements. The youth group meets on the twenty eighth at six thirty. The choir still needs fifteen volunteers for the Christmas program, and the food pantry served one hundred nineteen families last month."
    },
    @{
        id    = "revelation-bookends"
        rate  = 0
        refs  = @("Genesis 1:1", "Revelation 22:21")
        text  = "Genesis one one opens the whole story: in the beginning God created the heaven and the earth. And Revelation, twenty two twenty one, closes it: the grace of our Lord Jesus Christ be with you all."
    },
    @{
        id    = "peter-psalm-hundred"
        rate  = 1
        refs  = @("1 Peter 5:7", "Psalms 119:105")
        text  = "Two texts finish our series tonight. First Peter, five seven: casting all your care upon him, for he careth for you. And Psalm one hundred nineteen, verse one hundred five: thy word is a lamp unto my feet."
    },
    @{
        id    = "the-book-of-james"
        rate  = -3
        refs  = @("James 1:5")
        text  = "Turn with me, if you would, to the book of James. Chapter one. Verse five. If any of you lack wisdom, let him ask of God, that giveth to all men liberally."
    },
    @{
        id    = "samuel-anniversary"
        rate  = 2
        refs  = @("1 Samuel 1")
        text  = "For our tenth anniversary we return to where it all started, first Samuel chapter one, and Hannah's prayer. Also remember today is May twenty eighth at one hundred nineteen years our oldest member turns one hundred twenty."
    }
)

$jsonClips = @()
$index = 0
foreach ($seg in $segments) {
    $wavPath = Join-Path $setDir "$($seg.id).wav"
    $voice = $voiceNames[$index % $voiceNames.Count]
    $synth.SelectVoice($voice)
    $synth.Rate = [int]$seg.rate
    $synth.SetOutputToWaveFile((Join-Path (Get-Location) $wavPath))
    $synth.Speak([string]$seg.text)
    $synth.SetOutputToDefaultAudioDevice()

    $jsonClips += [ordered]@{
        id         = $seg.id
        file       = "$($seg.id).wav"
        references = $seg.refs
        transcript = $seg.text
    }
    "SYNTH $wavPath (rate $($seg.rate), voice $voice)"
    $index++
}
$synth.Dispose()

# Gold references give canonical book IDs + verse numbers for scoring; the
# transcript doubles as WER gold text (SAPI reading is nearly literal).
$manifest = [ordered]@{
    name  = "tts-synthetic"
    clips = $jsonClips
}
$json = $manifest | ConvertTo-Json -Depth 5
# UTF-8 WITHOUT BOM — Go's JSON decoder rejects BOM-prefixed files.
[System.IO.File]::WriteAllText((Join-Path (Get-Location) "$setDir\manifest.json"), $json, [System.Text.UTF8Encoding]::new($false))
"MANIFEST $setDir\manifest.json"
$refs = ($jsonClips | ForEach-Object { $_.references.Count } | Measure-Object -Sum).Sum
"TOTAL references in set: $refs"
