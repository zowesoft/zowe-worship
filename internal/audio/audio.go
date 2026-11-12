// Package audio is the PRD's AudioService: device discovery, capture, levels
// and stream health.
//
// Capture happens in Go over WASAPI (wasapi_windows.go), not in the webview:
// the engine wants 16 kHz mono Int16 PCM and the PRD keeps audio processing
// out of the UI, so the webview no longer opens the microphone itself. It asks
// for devices, starts and stops a stream and reads PCM slices over the bridge,
// and listens for the PRD's audio.input.started and audio.input.health_changed
// events, plus audio.input.level, which carries the readings that would
// otherwise have to be polled.
package audio

import (
	"math"
	"sync"
	"time"
)

const (
	// SampleRate is the one rate everything downstream sees: whatever the
	// device runs at is resampled to it before a sample leaves this package.
	SampleRate = 16000

	// BufferSeconds bounds the ring at a slow reference plus the sentence
	// around it. It is a bound, not a queue that grows: once full, the oldest
	// second is the one that goes.
	BufferSeconds = 60
)

// Health states, as audio.input.health_changed reports them.
const (
	HealthOK      = "ok"      // audio is flowing
	HealthStalled = "stalled" // the device went quiet while the stream was open
	HealthError   = "error"   // the stream ended badly, with a detail
	HealthStopped = "stopped" // the operator or the app ended it
)

const (
	// defaultStall is how long the stream may deliver nothing before it is
	// reported as stalled: a removed device stops producing packets while
	// its handle stays open.
	defaultStall = 2 * time.Second

	// defaultInterval is how often levels are published while running.
	defaultInterval = 100 * time.Millisecond

	stalledDetail = "the input device stopped delivering audio"
)

// Device is one capture input WASAPI can open. The JSON shape is what the
// studio's device menu binds, so the field names are the frontend's.
type Device struct {
	DeviceID string `json:"deviceId"`
	Label    string `json:"label"`
	Default  bool   `json:"default"`
}

// Features are one published reading: the two measurements the segmenter
// lives on, plus how much audio the ring holds right now.
type Features struct {
	Level            float64 `json:"level"`
	ZeroCrossingRate float64 `json:"zeroCrossingRate"`
	BufferedSeconds  float64 `json:"bufferedSeconds"`
	CapturedSamples  int64   `json:"capturedSamples"`
}

// Health is one published state of the stream. Detail carries the reason for
// an error, and Level repeats the meter so a UI can draw the health and the
// level from one event.
type Health struct {
	State  string  `json:"state"`
	Detail string  `json:"detail,omitempty"`
	Level  float64 `json:"level"`
}

// Started describes the stream that just opened - the device actually chosen,
// not the one that was asked for, since an empty id means the default.
type Started struct {
	DeviceID   string `json:"deviceId"`
	Label      string `json:"label"`
	SampleRate int    `json:"sampleRate"`
}

// opener opens one device and delivers what it captures. The stream runs its
// own loop and must push only 16 kHz mono Int16 blocks.
type opener func(deviceID string, sink sink) (stream, Device, error)

// lister reports the inputs that can be opened right now.
type lister func() ([]Device, error)

// stream is a running capture. close ends it and waits for the loop to let
// go, so nothing can be pushing afterwards.
type stream interface {
	close() error
}

// sink is what a capture loop reports into. The methods are unexported on
// purpose: only this package builds capture loops, and Service is the one
// implementation they may talk to.
type sink interface {
	push(chunk []int16)
	fail(err error)
}

// Service owns the running stream: one device at a time behind a bounded ring,
// with levels published as they move and health published when it changes.
// The zero value is not usable; New builds one.
type Service struct {
	open opener
	list lister

	onStarted func(Started)
	onHealth  func(Health)
	onLevel   func(Features)

	// interval and stall are configuration, set before the first Start.
	interval time.Duration
	stall    time.Duration

	mu       sync.Mutex
	ring     *ring
	stream   stream
	stop     chan struct{}
	running  bool
	device   Device
	level    float64
	zcr      float64
	lastFeed time.Time
	fatal    error
	// healthState/healthDetail make every publisher agree on what was already
	// announced, so a transition is emitted once.
	healthState  string
	healthDetail string
}

// New builds a service over the platform capture. Nothing touches hardware
// until Start or Inputs is called.
func New() *Service {
	return &Service{
		open:        openDevice,
		list:        enumerateInputs,
		ring:        newRing(SampleRate * BufferSeconds),
		interval:    defaultInterval,
		stall:       defaultStall,
		healthState: HealthStopped,
	}
}

// OnStarted registers the audio.input.started publisher. Register before the
// first Start.
func (s *Service) OnStarted(fn func(Started)) {
	s.mu.Lock()
	s.onStarted = fn
	s.mu.Unlock()
}

// OnHealth registers the audio.input.health_changed publisher. Register before
// the first Start.
func (s *Service) OnHealth(fn func(Health)) {
	s.mu.Lock()
	s.onHealth = fn
	s.mu.Unlock()
}

// OnLevel registers the audio.input.level publisher. Register before the first
// Start.
func (s *Service) OnLevel(fn func(Features)) {
	s.mu.Lock()
	s.onLevel = fn
	s.mu.Unlock()
}

// Inputs reports the capture devices, with the default marked.
func (s *Service) Inputs() ([]Device, error) {
	return s.list()
}

// Start opens the device (the default when deviceID is empty), clears the ring
// for a fresh session and begins publishing. A stream that is already running
// is ended first, so switching devices is just another Start.
func (s *Service) Start(deviceID string) error {
	s.shutdown()

	st, device, err := s.open(deviceID, s)
	if err != nil {
		return err
	}

	stop := make(chan struct{})
	s.mu.Lock()
	s.stream = st
	s.stop = stop
	s.running = true
	s.device = device
	s.fatal = nil
	s.lastFeed = time.Now()
	// A new session starts with an empty ring, so Analyze cannot pick up the
	// last session's audio and the segmenter's absolute offsets restart with
	// it. Health stays as it was, so the first tick announces "ok".
	s.ring = newRing(SampleRate * BufferSeconds)
	s.level, s.zcr = 0, 0
	s.mu.Unlock()

	go s.monitor(stop)

	s.emitStarted(Started{DeviceID: device.DeviceID, Label: device.Label, SampleRate: SampleRate})
	return nil
}

// Stop ends the stream, keeps the buffered audio readable and drops the meter
// to zero. A stream that died with an error keeps that error on screen rather
// than being overwritten by a cheerful "stopped".
func (s *Service) Stop() {
	s.shutdown()

	s.mu.Lock()
	buffered, captured := s.ring.bufferedSeconds(), s.ring.captured()
	s.level, s.zcr = 0, 0
	fatal := s.fatal
	s.mu.Unlock()

	if fatal == nil {
		s.setHealth(HealthStopped, "")
		s.emitHealth(Health{State: HealthStopped})
	}
	s.emitLevel(Features{BufferedSeconds: buffered, CapturedSamples: captured})
}

// Running reports whether a capture loop is live.
func (s *Service) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// TakeLast reads the most recent seconds of audio, oldest first. Asking for
// more than the ring holds returns what it holds.
func (s *Service) TakeLast(seconds float64) []int16 {
	count := int(math.Round(seconds * SampleRate))
	if count < 0 {
		count = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ring.takeLast(count)
}

// TakeRange reads an absolute sample span, clamped to what the ring still
// retains - a segment that wrapped out comes back as the part that did not.
func (s *Service) TakeRange(from, to int64) []int16 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ring.rangeOf(from, to)
}

// shutdown ends the running stream and its monitor. Waiting for the stream
// blocks until the capture loop has let go of its sink, so no lock may be held
// across it: the loop pushes under this same mutex.
func (s *Service) shutdown() {
	s.mu.Lock()
	st, stop := s.stream, s.stop
	s.stream, s.stop, s.running = nil, nil, false
	s.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if st != nil {
		_ = st.close()
	}
}

// push is the capture loop's only way in: it records the block, the readings
// taken from it and how fresh the stream is. It never blocks.
func (s *Service) push(chunk []int16) {
	if len(chunk) == 0 {
		return
	}
	s.mu.Lock()
	s.ring.append(chunk)
	s.level, s.zcr = features(chunk)
	s.lastFeed = time.Now()
	s.mu.Unlock()
}

// fail is the capture loop reporting that it ended badly. The stream is torn
// down after the report so the error stays the last thing published.
func (s *Service) fail(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.fatal = err
	s.mu.Unlock()

	s.setHealth(HealthError, err.Error())
	s.emitHealth(Health{State: HealthError, Detail: err.Error()})
	go s.Stop()
}

// monitor publishes levels on a steady tick and health only when it changes.
// It exits when the stream it was started for ends.
func (s *Service) monitor(stop <-chan struct{}) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.mu.Lock()
			level, zcr := s.level, s.zcr
			buffered, captured := s.ring.bufferedSeconds(), s.ring.captured()
			fatal := s.fatal
			quiet := time.Since(s.lastFeed)
			stall := s.stall

			state, detail := HealthOK, ""
			switch {
			case fatal != nil:
				state, detail = HealthError, fatal.Error()
			case quiet > stall:
				state, detail = HealthStalled, stalledDetail
			}
			changed := s.healthState != state || s.healthDetail != detail
			s.healthState, s.healthDetail = state, detail
			s.mu.Unlock()

			s.emitLevel(Features{
				Level:            level,
				ZeroCrossingRate: zcr,
				BufferedSeconds:  buffered,
				CapturedSamples:  captured,
			})
			if changed {
				s.emitHealth(Health{State: state, Detail: detail, Level: level})
			}
		}
	}
}

// setHealth records a state and reports whether it was a change.
func (s *Service) setHealth(state, detail string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := s.healthState != state || s.healthDetail != detail
	s.healthState, s.healthDetail = state, detail
	return changed
}

func (s *Service) emitStarted(v Started) {
	s.mu.Lock()
	fn := s.onStarted
	s.mu.Unlock()
	if fn != nil {
		fn(v)
	}
}

func (s *Service) emitHealth(v Health) {
	s.mu.Lock()
	fn := s.onHealth
	s.mu.Unlock()
	if fn != nil {
		fn(v)
	}
}

func (s *Service) emitLevel(v Features) {
	s.mu.Lock()
	fn := s.onLevel
	s.mu.Unlock()
	if fn != nil {
		fn(v)
	}
}

// ring is the bounded capture buffer, addressed in absolute samples so a
// segment named while the audio keeps arriving still reads back that audio.
// It mirrors what the webview used to keep, so the segmenter's offsets mean
// the same thing as before.
type ring struct {
	samples  []int16
	writePos int
	filled   int
	// total counts every sample ever appended, which is what makes a span
	// nameable after the ring has wrapped.
	total int64
}

func newRing(capacity int) *ring {
	return &ring{samples: make([]int16, capacity)}
}

func (r *ring) append(chunk []int16) {
	capacity := len(r.samples)
	for _, sample := range chunk {
		r.samples[r.writePos] = sample
		r.writePos++
		if r.writePos == capacity {
			r.writePos = 0
		}
	}
	r.filled = min(capacity, r.filled+len(chunk))
	r.total += int64(len(chunk))
}

func (r *ring) captured() int64 { return r.total }

func (r *ring) bufferedSeconds() float64 {
	return float64(r.filled) / SampleRate
}

// rangeOf reads an absolute span that is still inside the buffer. A span that
// has already wrapped out is clamped to what is retained; an empty span comes
// back nil rather than a zero-length block the caller has to test twice.
func (r *ring) rangeOf(from, to int64) []int16 {
	capacity := int64(len(r.samples))
	start := max(from, r.total-int64(r.filled))
	end := min(to, r.total)
	if end <= start {
		return nil
	}
	out := make([]int16, int(end-start))
	pos := start % capacity
	for i := range out {
		out[i] = r.samples[pos]
		pos++
		if pos == capacity {
			pos = 0
		}
	}
	return out
}

// takeLast reads the newest count samples, oldest first.
func (r *ring) takeLast(count int) []int16 {
	available := min(count, r.filled)
	if available <= 0 {
		return nil
	}
	out := make([]int16, available)
	capacity := len(r.samples)
	pos := (r.writePos - available + capacity) % capacity
	for i := range out {
		out[i] = r.samples[pos]
		pos++
		if pos == capacity {
			pos = 0
		}
	}
	return out
}

// features measures one block the way the segmenter expects: level is RMS
// scaled to roughly 0..1 (the segmenter's floors are tuned against this
// scaling), zeroCrossingRate is sign changes per sample, which separates
// voiced speech from fricatives and hiss.
func features(chunk []int16) (level, zcr float64) {
	if len(chunk) == 0 {
		return 0, 0
	}
	var total float64
	crossings := 0
	previous := int16(0)
	for i, sample := range chunk {
		total += float64(sample) * float64(sample)
		if i > 0 && (sample >= 0) != (previous >= 0) {
			crossings++
		}
		previous = sample
	}
	level = math.Sqrt(total/float64(len(chunk))) / 32768 * 4
	if level > 1 {
		level = 1
	}
	return level, float64(crossings) / float64(len(chunk))
}
