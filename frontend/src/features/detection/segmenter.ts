// Automatic segmentation for continuous listening. The capture layer reports
// two cheap features per audio block — level and zero-crossing rate — and this
// state machine decides when a block sounds like speech, closes a segment when
// the speaker pauses, and discards stretches that are really a quiet room or a
// backing track.
//
// It deals in absolute sample offsets rather than "the last N seconds", so a
// segment is exactly the audio between two moments in the capture buffer even
// while more audio keeps arriving.

export const SAMPLE_RATE = 16000;

// One block's readings, as mic.ts measures them.
export type VoiceReading = {
  // 16 kHz mono RMS, scaled to roughly 0..1.
  level: number;
  // Sign changes per sample: voiced speech is low, fricatives are high,
  // cymbals and hiss are higher still.
  zeroCrossingRate: number;
};

// A hard floor for anything worth transcribing, regardless of the room.
export const VOICE_LEVEL = 0.06;

// Speech must also clear the measured room floor by this factor. A quiet
// passage that never rises above the fan noise is not speech the engine can
// resolve, and transcribing it only invites hallucinated references.
export const VOICE_MARGIN = 1.8;

// How quickly the quiet baseline follows the room. Slow enough that a phrase
// does not drag the floor up with it.
export const NOISE_FLOOR_RATE = 0.05;

// Speech zero-crossing rate band. Below it sits a sustained bass drone; above
// it sits cymbals, hiss and room noise.
export const MIN_SPEECH_ZCR = 0.004;
export const MAX_SPEECH_ZCR = 0.35;

// A long stretch whose level barely moves is a sustained note or a backing
// track, not syllables: speech swings far wider within one phrase.
export const STEADY_MS = 2500;
export const STEADY_RATIO = 1.3;

// A preacher pauses between clauses; this is the gap that ends a segment.
export const SILENCE_MS = 700;

// Shorter bursts are a chair scraping or a cough, not a reference.
export const MIN_SPEECH_MS = 400;

// The Go pipeline refuses a request longer than 30 s, so a stretch with no pause
// is cut before that.
export const MAX_SEGMENT_MS = 25_000;

// Audio kept before the first loud block, so the first word survives.
export const PRE_ROLL_MS = 350;

// Silence kept after the last loud block, so the last word survives.
export const TAIL_MS = 250;

export type SegmentRange = {
  from: number;
  to: number;
};

export class SpeechSegmenter {
  private readonly rate: number;
  private noiseFloor = 0;
  private speaking = false;
  private start = 0;
  private firstVoice = 0;
  private lastVoice = 0;
  private loudMin = Number.POSITIVE_INFINITY;
  private loudMax = 0;

  constructor(rate: number = SAMPLE_RATE) {
    this.rate = rate;
  }

  // hearing reports whether speech is inside a segment right now, which the
  // panel shows as the live state instead of a spinner.
  get hearing(): boolean {
    return this.speaking;
  }

  // floor is the quiet baseline the segmenter has learned, exposed so tests and
  // the panel can explain why a quiet passage was ignored.
  get floor(): number {
    return this.noiseFloor;
  }

  reset(): void {
    this.speaking = false;
    this.start = 0;
    this.firstVoice = 0;
    this.lastVoice = 0;
    this.loudMin = Number.POSITIVE_INFINITY;
    this.loudMax = 0;
  }

  // feed takes one block's readings and the capture's absolute sample count. It
  // returns a closed segment, or null while the speaker is still going or the
  // stretch turned out not to be speech.
  feed(reading: VoiceReading, capturedSamples: number): SegmentRange | null {
    const { level, zeroCrossingRate } = reading;
    this.trackNoise(level);
    const speechLike = this.isSpeech(level, zeroCrossingRate);

    if (speechLike) {
      if (!this.speaking) {
        this.speaking = true;
        this.start = Math.max(0, capturedSamples - this.samples(PRE_ROLL_MS));
        this.firstVoice = capturedSamples;
        this.loudMin = level;
        this.loudMax = level;
      }
      this.lastVoice = capturedSamples;
      this.loudMin = Math.min(this.loudMin, level);
      this.loudMax = Math.max(this.loudMax, level);
    }

    if (!this.speaking) {
      return null;
    }

    // No pause for a long stretch: cut at the cap. A steady level over that
    // long was music, so it is dropped rather than transcribed.
    if (capturedSamples - this.start >= this.samples(MAX_SEGMENT_MS)) {
      const segment = this.settle(capturedSamples, capturedSamples);
      this.reset();
      return segment;
    }

    if (!speechLike && capturedSamples - this.lastVoice >= this.samples(SILENCE_MS)) {
      const to = Math.min(capturedSamples, this.lastVoice + this.samples(TAIL_MS));
      const segment = this.settle(this.start, to);
      this.reset();
      return segment;
    }

    return null;
  }

  // settle applies the duration and music guards to the closed window.
  private settle(from: number, to: number): SegmentRange | null {
    const speech = this.lastVoice - this.firstVoice;
    if (speech < this.samples(MIN_SPEECH_MS)) {
      return null;
    }
    if (speech >= this.samples(STEADY_MS) && this.loudMax < this.loudMin * STEADY_RATIO) {
      return null;
    }
    return { from, to };
  }

  private isSpeech(level: number, zeroCrossingRate: number): boolean {
    const floor = Math.max(VOICE_LEVEL, this.noiseFloor * VOICE_MARGIN);
    return level >= floor &&
      zeroCrossingRate >= MIN_SPEECH_ZCR &&
      zeroCrossingRate <= MAX_SPEECH_ZCR;
  }

  // Quiet readings tell the segmenter what the room sounds like; loud ones must
  // not drag that baseline up with them.
  private trackNoise(level: number): void {
    if (level >= VOICE_LEVEL) {
      return;
    }
    this.noiseFloor += (level - this.noiseFloor) * NOISE_FLOOR_RATE;
  }

  private samples(ms: number): number {
    return Math.round((ms * this.rate) / 1000);
  }
}
