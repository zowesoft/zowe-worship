package asr

// Audio preparation: church recordings arrive in whatever format the mixer or
// capture device produced. Engines that work file-based (whisper.cpp CLI)
// need 16 kHz mono 16-bit PCM WAV. Everything here is pure Go so the harness
// runs identically on Windows, macOS and Linux.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

type AudioSpec struct {
	SampleRate    int
	Channels      int
	BitsPerSample int
	FormatTag     int // 1 = PCM, 3 = IEEE float
	Data          []byte
}

// ReadWAV parses the fmt and data chunks of a RIFF/WAVE file. It supports the
// integer and float encodings ASR pipelines encounter in practice.
func ReadWAV(path string) (AudioSpec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return AudioSpec{}, err
	}
	if len(raw) < 12 || string(raw[:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return AudioSpec{}, errors.New("not a RIFF/WAVE file")
	}

	var spec AudioSpec
	offset := 12
	for offset+8 <= len(raw) {
		id := string(raw[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(raw[offset+4 : offset+8]))
		body := raw[offset+8:]
		if size < 0 || offset+8+size > len(raw) {
			size = len(raw) - offset - 8
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return AudioSpec{}, errors.New("wav fmt chunk too small")
			}
			spec.FormatTag = int(binary.LittleEndian.Uint16(body[0:2]))
			spec.Channels = int(binary.LittleEndian.Uint16(body[2:4]))
			spec.SampleRate = int(binary.LittleEndian.Uint32(body[4:8]))
			spec.BitsPerSample = int(binary.LittleEndian.Uint16(body[14:16]))
		case "data":
			spec.Data = body[:size]
		}
		offset += 8 + size
		if size%2 == 1 {
			offset++ // RIFF pads odd chunks
		}
	}
	if spec.SampleRate == 0 || spec.Channels == 0 {
		return AudioSpec{}, errors.New("wav file missing fmt or data chunk")
	}
	return spec, nil
}

// ToMono16k16bit converts any supported encoding to 16 kHz mono signed PCM,
// averaging channels and linear-interpolating the sample count.
func ToMono16k16bit(spec AudioSpec) AudioSpec {
	samples := decodeSamples(spec)
	const target = 16000

	var resampled []float32
	if spec.SampleRate == target {
		resampled = samples
	} else {
		outLength := len(samples) * target / maxInt(spec.SampleRate, 1)
		resampled = make([]float32, outLength)
		scale := float64(len(samples)) / float64(maxInt(outLength, 1))
		for i := range resampled {
			pos := float64(i) * scale
			low := int(pos)
			if low >= len(samples)-1 {
				resampled[i] = samples[len(samples)-1]
				continue
			}
			fraction := float32(pos - float64(low))
			resampled[i] = samples[low] + (samples[low+1]-samples[low])*fraction
		}
	}

	data := make([]byte, 2*len(resampled))
	for i, sample := range resampled {
		if sample > 1 {
			sample = 1
		} else if sample < -1 {
			sample = -1
		}
		binary.LittleEndian.PutUint16(data[2*i:], uint16(int16(sample*32767)))
	}
	return AudioSpec{
		SampleRate:    target,
		Channels:      1,
		BitsPerSample: 16,
		FormatTag:     1,
		Data:          data,
	}
}

// decodeSamples converts the wav byte payload into mono float32 samples in
// [-1, 1], averaging across channels.
func decodeSamples(spec AudioSpec) []float32 {
	bytesPerFrame := spec.BitsPerSample / 8 * spec.Channels
	if bytesPerFrame == 0 {
		return nil
	}
	frameCount := len(spec.Data) / bytesPerFrame
	samples := make([]float32, frameCount)
	sampleValue := func(data []byte, bits int, tag int) float32 {
		switch {
		case tag == 3 && bits == 32:
			return math.Float32frombits(binary.LittleEndian.Uint32(data))
		}
		return 0
	}
	_ = sampleValue

	for frame := 0; frame < frameCount; frame++ {
		start := frame * bytesPerFrame
		var sum float32
		for channel := 0; channel < spec.Channels; channel++ {
			offset := start + channel*(spec.BitsPerSample/8)
			sum += readSampleFloat(spec.Data[offset:], spec.BitsPerSample, spec.FormatTag)
		}
		samples[frame] = sum / float32(spec.Channels)
	}
	return samples
}

func readSampleFloat(data []byte, bits int, formatTag int) float32 {
	switch {
	case formatTag == 3 && bits == 32 && len(data) >= 4:
		return math.Float32frombits(binary.LittleEndian.Uint32(data))
	case formatTag == 3 && bits == 64 && len(data) >= 8:
		return float32(math.Float64frombits(binary.LittleEndian.Uint64(data)))
	}
	switch bits {
	case 8:
		return (float32(data[0]) - 128) / 128
	case 16:
		return float32(int16(binary.LittleEndian.Uint16(data[:2]))) / 32768
	case 24:
		if len(data) < 3 {
			return 0
		}
		value := int32(data[0]) | int32(data[1])<<8 | int32(data[2])<<16
		if value&0x800000 != 0 {
			value -= 1 << 24
		}
		return float32(value) / 8388608
	case 32:
		if len(data) < 4 {
			return 0
		}
		return float32(int32(binary.LittleEndian.Uint32(data[:4]))) / 2147483648
	}
	return 0
}

// WriteWAV writes a 16 kHz mono 16-bit WAV plus canonical RIFF header.
func WriteWAV(path string, spec AudioSpec) error {
	size := len(spec.Data)
	header := make([]byte, 44)
	copy(header[0:], "RIFF")
	binary.LittleEndian.PutUint32(header[4:], uint32(36+size))
	copy(header[8:], "WAVE")
	copy(header[12:], "fmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], uint16(spec.FormatTag))
	binary.LittleEndian.PutUint16(header[22:], uint16(spec.Channels))
	binary.LittleEndian.PutUint32(header[24:], uint32(spec.SampleRate))
	binary.LittleEndian.PutUint32(header[28:], uint32(spec.SampleRate*spec.Channels*spec.BitsPerSample/8))
	binary.LittleEndian.PutUint16(header[32:], uint16(spec.Channels*spec.BitsPerSample/8))
	binary.LittleEndian.PutUint16(header[34:], uint16(spec.BitsPerSample))
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], uint32(size))

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := out.Write(header); err != nil {
		return err
	}
	_, err = out.Write(spec.Data)
	return err
}

// NeedsConversion reports whether the wav already matches engine input
// requirements.
func (spec AudioSpec) NeedsConversion() bool {
	return spec.SampleRate != 16000 || spec.Channels != 1 ||
		spec.BitsPerSample != 16 || spec.FormatTag != 1
}

// Prepare16kMono returns a path to a 16 kHz mono PCM wav the engine can read.
// Files that already comply map to themselves; anything else is converted
// into a temporary file below dir.
func Prepare16kMono(path string, dir string) (string, error) {
	spec, err := ReadWAV(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	if !spec.NeedsConversion() {
		return path, nil
	}
	out := filepath.Join(dir, filepath.Base(path)+"-16k.wav")
	if err := WriteWAV(out, ToMono16k16bit(spec)); err != nil {
		return "", err
	}
	return out, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
