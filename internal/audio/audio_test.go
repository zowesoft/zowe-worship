package audio

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeStream is a capture the test drives by hand: it records that it was
// closed and nothing else.
type fakeStream struct {
	closed chan struct{}
	once   sync.Once
}

func (s *fakeStream) close() error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func (s *fakeStream) isClosed() bool {
	select {
	case <-s.closed:
		return true
	default:
		return false
	}
}

// fakeCapture stands in for the platform: it records what was opened and hands
// back a stream the test can close.
type fakeCapture struct {
	device  Device
	err     error
	opened  []string
	streams []*fakeStream
}

func (f *fakeCapture) open(deviceID string, sink sink) (stream, Device, error) {
	f.opened = append(f.opened, deviceID)
	if f.err != nil {
		return nil, Device{}, f.err
	}
	st := &fakeStream{closed: make(chan struct{})}
	f.streams = append(f.streams, st)
	return st, f.device, nil
}

func (f *fakeCapture) list() ([]Device, error) {
	return []Device{f.device}, nil
}

// newTestService builds a service over the fake capture, with a short publish
// interval so the tests do not wait on the real one.
func newTestService(t *testing.T) (*Service, *fakeCapture) {
	t.Helper()

	capture := &fakeCapture{device: Device{DeviceID: "test-device", Label: "Test Microphone"}}
	service := &Service{
		open:        capture.open,
		list:        capture.list,
		ring:        newRing(SampleRate * BufferSeconds),
		interval:    5 * time.Millisecond,
		stall:       40 * time.Millisecond,
		healthState: HealthStopped,
	}
	return service, capture
}

func waitForHealth(t *testing.T, events <-chan Health, want string) Health {
	t.Helper()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case got := <-events:
			if got.State == want {
				return got
			}
		case <-deadline:
			t.Fatalf("health never reported %q", want)
		}
	}
}

func waitForLevel(t *testing.T, events <-chan Features, wantSamples int64) Features {
	t.Helper()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case got := <-events:
			if got.CapturedSamples >= wantSamples {
				return got
			}
		case <-deadline:
			t.Fatalf("no level event reported %d captured samples", wantSamples)
		}
	}
}

func TestServiceStartsPublishesAndStops(t *testing.T) {
	service, capture := newTestService(t)

	started := make(chan Started, 4)
	levels := make(chan Features, 64)
	health := make(chan Health, 32)
	service.OnStarted(func(v Started) { started <- v })
	service.OnLevel(func(v Features) { levels <- v })
	service.OnHealth(func(v Health) { health <- v })

	if err := service.Start("test-device"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !service.Running() {
		t.Fatal("the service is not running after Start")
	}
	if len(capture.opened) != 1 || capture.opened[0] != "test-device" {
		t.Fatalf("opened = %v, want the chosen device", capture.opened)
	}

	select {
	case got := <-started:
		if got.DeviceID != "test-device" || got.Label != "Test Microphone" {
			t.Fatalf("started = %+v, want the device that was opened", got)
		}
		if got.SampleRate != SampleRate {
			t.Fatalf("started sample rate = %d, want %d", got.SampleRate, SampleRate)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no audio.input.started event")
	}

	service.push([]int16{1000, -1000, 2000, -2000})
	reading := waitForLevel(t, levels, 4)
	if reading.Level <= 0 {
		t.Fatalf("level = %v, want a reading of the block that was pushed", reading.Level)
	}
	if reading.BufferedSeconds <= 0 {
		t.Fatalf("buffered seconds = %v, want the ring to report what it holds", reading.BufferedSeconds)
	}

	waitForHealth(t, health, HealthOK)

	service.Stop()
	if service.Running() {
		t.Fatal("the service is still running after Stop")
	}
	waitForHealth(t, health, HealthStopped)
}

func TestServiceReportsAStalledDevice(t *testing.T) {
	service, _ := newTestService(t)

	health := make(chan Health, 32)
	service.OnHealth(func(v Health) { health <- v })

	if err := service.Start(""); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer service.Stop()

	service.push([]int16{1, 2, 3})
	waitForHealth(t, health, HealthOK)

	// Nothing arrives for longer than the stall window, which is what a
	// removed or muted device looks like from here.
	stalled := waitForHealth(t, health, HealthStalled)
	if stalled.Detail == "" {
		t.Fatal("a stalled stream must say why")
	}
}

func TestServiceReportsAFailedStream(t *testing.T) {
	service, _ := newTestService(t)

	health := make(chan Health, 32)
	service.OnHealth(func(v Health) { health <- v })

	if err := service.Start(""); err != nil {
		t.Fatalf("start: %v", err)
	}
	service.fail(errors.New("the device was removed"))

	failed := waitForHealth(t, health, HealthError)
	if failed.Detail != "the device was removed" {
		t.Fatalf("detail = %q, want the reason the stream ended", failed.Detail)
	}

	// The stream tears itself down, and the error stays the last word: a
	// cheerful "stopped" must not cover why the capture died.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if !service.Running() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if service.Running() {
		t.Fatal("the service is still running after the stream failed")
	}
	select {
	case later := <-health:
		if later.State != HealthError {
			t.Fatalf("health = %q, want the error to stay on screen", later.State)
		}
	default:
	}
}

func TestStartReplacesTheRunningStream(t *testing.T) {
	service, capture := newTestService(t)

	if err := service.Start("first"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := service.Start("second"); err != nil {
		t.Fatalf("restart: %v", err)
	}
	defer service.Stop()

	if len(capture.opened) != 2 {
		t.Fatalf("opened = %v, want the second start to open a stream of its own", capture.opened)
	}
	if !capture.streams[0].isClosed() {
		t.Fatal("the first stream was left running")
	}
	if !service.Running() {
		t.Fatal("the service is not running after the restart")
	}
}

func TestStartReportsADeviceThatCannotBeOpened(t *testing.T) {
	service, capture := newTestService(t)
	capture.err = errors.New("no such device")

	if err := service.Start("ghost"); err == nil {
		t.Fatal("starting a device that cannot be opened succeeded")
	}
	if service.Running() {
		t.Fatal("the service is running after a failed start")
	}
}

func TestTakeLastAndRangeServeTheRing(t *testing.T) {
	service, _ := newTestService(t)

	service.push([]int16{1, 2, 3, 4, 5})

	last := service.TakeLast(5.0 / SampleRate)
	if len(last) != 5 || last[0] != 1 || last[4] != 5 {
		t.Fatalf("take last = %v, want the five samples that were pushed", last)
	}

	spanned := service.TakeRange(2, 5)
	if len(spanned) != 3 || spanned[0] != 3 || spanned[2] != 5 {
		t.Fatalf("take range = %v, want samples three to five", spanned)
	}

	if empty := service.TakeRange(50, 60); len(empty) != 0 {
		t.Fatalf("take range = %v, want nothing for a span that never happened", empty)
	}
}

func TestRingWrapsAndClampsOldSpans(t *testing.T) {
	ring := newRing(4)
	ring.append([]int16{1, 2, 3, 4, 5, 6})

	if ring.captured() != 6 {
		t.Fatalf("captured = %d, want every sample ever appended", ring.captured())
	}
	if ring.filled != 4 {
		t.Fatalf("filled = %d, want the ring to be full", ring.filled)
	}

	last := ring.takeLast(3)
	if len(last) != 3 || last[0] != 4 || last[2] != 6 {
		t.Fatalf("take last = %v, want the three newest samples", last)
	}

	// The first two samples have wrapped out, so the span comes back clamped
	// to what is still retained rather than as a shorter, wrong window.
	spanned := ring.rangeOf(0, 6)
	if len(spanned) != 4 || spanned[0] != 3 || spanned[3] != 6 {
		t.Fatalf("range = %v, want the four samples the ring still holds", spanned)
	}
}

func TestFeaturesMeasureLevelAndZeroCrossings(t *testing.T) {
	level, zcr := features([]int16{0, 0, 0, 0})
	if level != 0 || zcr != 0 {
		t.Fatalf("silence measured %v level and %v crossings, want both zero", level, zcr)
	}

	// A square wave changes sign every sample, which is the cymbal case the
	// segmenter is meant to hear as "not speech".
	square := []int16{1000, -1000, 1000, -1000, 1000, -1000, 1000, -1000}
	level, zcr = features(square)
	if level <= 0 {
		t.Fatalf("level = %v, want a reading of a loud block", level)
	}
	if zcr < 0.8 {
		t.Fatalf("zero crossing rate = %v, want nearly every sample to flip", zcr)
	}
}

func TestResamplerCarriesTheCursorAcrossBlocks(t *testing.T) {
	resampler := newResampler(48000)
	block := make([]float32, 128)
	for i := range block {
		block[i] = 0.5
	}

	first := resampler.float32Frames(block, 1)
	second := resampler.float32Frames(block, 1)
	if len(first) == 0 {
		t.Fatal("the first block produced nothing")
	}
	if len(second) == 0 {
		t.Fatal("the second block produced nothing: the read cursor did not carry over")
	}
	total := len(first) + len(second)
	if total < 80 || total > 90 {
		t.Fatalf("produced %d samples from 256 frames at 48 kHz, want about 85", total)
	}
	if first[0] != 16383 {
		t.Fatalf("first sample = %d, want 0.5 scaled to 16 bit", first[0])
	}
}

func TestResamplerDownmixesAndPassesThrough(t *testing.T) {
	resampler := newResampler(16000)
	out := resampler.float32Frames([]float32{1, 0, -1, 0.5}, 1)
	if len(out) != 4 {
		t.Fatalf("produced %d samples, want one per frame", len(out))
	}
	if out[0] != 32767 || out[1] != 0 || out[2] != -32768 || out[3] != 16383 {
		t.Fatalf("samples = %v, want the input scaled to 16 bit", out)
	}

	stereo := newResampler(16000)
	mixed := stereo.float32Frames([]float32{1, 0, 0, 1}, 2)
	if len(mixed) != 2 || mixed[0] != 16383 {
		t.Fatalf("stereo mix = %v, want the channels averaged", mixed)
	}
}

// TestLiveCaptureDeliversSamplesFromTheDefaultDevice opens the machine's own
// microphone. This is the one part of the package that cannot be faked, so it
// is checked against real hardware when there is any.
func TestLiveCaptureDeliversSamplesFromTheDefaultDevice(t *testing.T) {
	service := New()

	devices, err := service.Inputs()
	if err != nil {
		t.Skipf("this machine reports no audio devices: %v", err)
	}
	if len(devices) == 0 {
		t.Skip("this machine has no capture devices")
	}
	if err := service.Start(""); err != nil {
		t.Fatalf("the default input could not be opened: %v", err)
	}
	defer service.Stop()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(service.TakeLast(1)) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the default input delivered no audio within three seconds")
}
