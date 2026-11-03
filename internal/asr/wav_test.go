package asr

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestWAVRoundTripAndConversion(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.wav")

	// 48 kHz stereo, one cycle of a square-ish wave, 16-bit PCM.
	sampleRate, frames := 48000, 96
	data := make([]byte, 2*2*frames)
	for i := 0; i < frames; i++ {
		sample := uint16(30000)
		if i%2 == 1 {
			sample = 65536 - 30000
		}
		for channel := 0; channel < 2; channel++ {
			offset := (i*2 + channel) * 2
			writeUint16LE(data[offset:], sample)
		}
	}
	spec := AudioSpec{SampleRate: sampleRate, Channels: 2, BitsPerSample: 16, FormatTag: 1, Data: data}
	if err := WriteWAV(source, spec); err != nil {
		t.Fatalf("write: %v", err)
	}

	read, err := ReadWAV(source)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if read.SampleRate != 48000 || read.Channels != 2 || read.BitsPerSample != 16 {
		t.Fatalf("read spec mismatch: %+v", read)
	}
	if !read.NeedsConversion() {
		t.Fatal("stereo 48k should need conversion")
	}

	converted := ToMono16k16bit(read)
	if converted.SampleRate != 16000 || converted.Channels != 1 {
		t.Fatalf("converted spec: %+v", converted)
	}
	// 96 frames at 48 kHz -> 32 frames at 16 kHz, with rounding slack.
	frameCount := len(converted.Data) / 2
	if frameCount < 30 || frameCount > 34 {
		t.Fatalf("converted frame count = %d", frameCount)
	}

	target := filepath.Join(dir, "converted.wav")
	if err := WriteWAV(target, converted); err != nil {
		t.Fatalf("write converted: %v", err)
	}
	check, err := ReadWAV(target)
	if err != nil {
		t.Fatalf("read converted: %v", err)
	}
	if check.NeedsConversion() {
		t.Fatalf("converted wav still flagged: %+v", check)
	}

	// Float32 source decodes too.
	floatData := make([]byte, 8) // two mono float32 samples
	writeFloat32(floatData[0:], 0.5)
	writeFloat32(floatData[4:], -0.5)
	floatSpec := AudioSpec{SampleRate: 16000, Channels: 1, BitsPerSample: 32, FormatTag: 3, Data: floatData}
	decoded := decodeSamples(floatSpec)
	if len(decoded) != 2 || decoded[0] < 0.49 || decoded[0] > 0.51 || decoded[1] > -0.49 {
		t.Fatalf("float decode: %v", decoded)
	}
}

func TestPrepare16kMonoPassthrough(t *testing.T) {
	dir := t.TempDir()
	mono := filepath.Join(dir, "mono.wav")
	spec := AudioSpec{SampleRate: 16000, Channels: 1, BitsPerSample: 16, FormatTag: 1, Data: make([]byte, 32)}
	if err := WriteWAV(mono, spec); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := Prepare16kMono(mono, dir)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if got != mono {
		t.Fatalf("16k mono passthrough changed path: %q", got)
	}
}

func TestPrepare16kMonoRejectsNonWav(t *testing.T) {
	dir := t.TempDir()
	junk := filepath.Join(dir, "junk.wav")
	if err := os.WriteFile(junk, []byte("definitely not a wav"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Prepare16kMono(junk, dir); err == nil {
		t.Fatal("expected error for non-wav file")
	}
}

// TestEngineOnSyntheticClip exercises the spec form that quotes a binary
// containing spaces; real engine execution is covered by the benchmark smoke
// run in benchmarks/ (see README).
func TestEngineSpecQuotedPaths(t *testing.T) {
	if _, err := ParseEngineSpec(`name="C:\Program Files\tool.exe" -f {audio}`); err != nil {
		t.Fatalf("quoted path spec failed: %v", err)
	}
}

func writeUint16LE(data []byte, value uint16) {
	data[0] = byte(value)
	data[1] = byte(value >> 8)
}

func writeFloat32(data []byte, value float32) {
	bits := math.Float32bits(value)
	writeUint16LE(data[0:], uint16(bits))
	writeUint16LE(data[2:], uint16(bits>>16))
}
