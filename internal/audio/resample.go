package audio

// sampleKind is how the device encodes its mix format; only these two leave
// this package.
type sampleKind int

const (
	kindFloat32 sampleKind = iota
	kindInt16
)

// resampler turns interleaved device frames into the one shape everything
// downstream speaks: mono, 16 kHz, Int16.
//
// The fractional read cursor carries across blocks, exactly as the webview's
// worklet had to: a cursor measured within each block but restarted per block
// would freeze after the first block and leave the rest of the capture as
// silence.
type resampler struct {
	// ratio is input samples per output sample: 3 at 48 kHz, 1 when the
	// engine already runs at 16 kHz.
	ratio float64
	// carry is the fractional position the last block ended on, in [0, ratio).
	carry float64
}

func newResampler(deviceRate int) *resampler {
	ratio := 1.0
	if deviceRate > 0 {
		ratio = float64(deviceRate) / SampleRate
	}
	return &resampler{ratio: ratio}
}

// float32Frames downmixes interleaved float frames (the format WASAPI's mix
// format normally uses) and resamples them.
func (r *resampler) float32Frames(src []float32, channels int) []int16 {
	if channels <= 0 || len(src) < channels {
		return nil
	}
	frames := len(src) / channels
	at := func(frame int) float64 {
		base := frame * channels
		var sum float64
		for c := 0; c < channels; c++ {
			sum += float64(src[base+c])
		}
		return sum / float64(channels)
	}
	return r.resample(frames, at)
}

// int16Frames does the same for a 16-bit device. The mix is taken in floats,
// so a PCM device can land one LSB away from its input - inaudible, and the
// live mix formats are float anyway.
func (r *resampler) int16Frames(src []int16, channels int) []int16 {
	if channels <= 0 || len(src) < channels {
		return nil
	}
	frames := len(src) / channels
	at := func(frame int) float64 {
		base := frame * channels
		var sum int64
		for c := 0; c < channels; c++ {
			sum += int64(src[base+c])
		}
		return float64(sum) / float64(channels) / 32768
	}
	return r.resample(frames, at)
}

// resample walks positions of frames with linear interpolation between the two
// frames a read lands between. The last frame of a block is held (there is no
// next frame to read yet); the carry keeps the walk continuous regardless.
func (r *resampler) resample(frames int, at func(frame int) float64) []int16 {
	if frames <= 0 {
		return nil
	}
	out := make([]int16, 0, int(float64(frames)/r.ratio)+2)
	pos := r.carry
	for pos < float64(frames) {
		first := int(pos)
		second := first + 1
		if second >= frames {
			second = first
		}
		fraction := pos - float64(first)
		a, b := at(first), at(second)
		out = append(out, toInt16(float32(a+(b-a)*fraction)))
		pos += r.ratio
	}
	r.carry = pos - float64(frames)
	return out
}

// toInt16 scales a clipped [-1, 1] sample the same way the worklet did:
// full-scale negative goes to -32768, positive to 32767.
func toInt16(value float32) int16 {
	switch {
	case value > 1:
		value = 1
	case value < -1:
		value = -1
	}
	if value < 0 {
		return int16(value * 32768)
	}
	return int16(value * 32767)
}
