// Automatic segmentation for continuous listening. The capture layer already
// measures a level per audio block; this state machine watches that level and
// closes a segment when the speaker pauses, so the engine transcribes one
// reference plus its sentence instead of a fixed 20-second slice.
//
// It deals in absolute sample offsets rather than "the last N seconds", so a
// segment is exactly the audio between two moments in the capture buffer even
// while more audio keeps arriving.

export const SAMPLE_RATE = 16000;

// mic.ts scales a 16-bit RMS to roughly 0..1; room noise sits well below this
// and ordinary speech clears it.
export const VOICE_LEVEL = 0.06;

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
  private speaking = false;
  private start = 0;
  private lastVoice = 0;
  private firstVoice = 0;

  constructor(rate: number = SAMPLE_RATE) {
    this.rate = rate;
  }

  // hearing reports whether speech is inside a segment right now, which the
  // panel shows as the live state instead of a spinner.
  get hearing(): boolean {
    return this.speaking;
  }

  reset(): void {
    this.speaking = false;
    this.start = 0;
    this.lastVoice = 0;
    this.firstVoice = 0;
  }

  // feed takes one level reading and the capture's absolute sample count. It
  // returns a closed segment, or null while the speaker is still going.
  feed(level: number, capturedSamples: number): SegmentRange | null {
    const voice = level >= VOICE_LEVEL;

    if (voice) {
      if (!this.speaking) {
        this.speaking = true;
        this.start = Math.max(0, capturedSamples - this.samples(PRE_ROLL_MS));
        this.firstVoice = capturedSamples;
      }
      this.lastVoice = capturedSamples;
    }

    if (!this.speaking) {
      return null;
    }

    // No pause for a long stretch: cut at the cap and let the next loud reading
    // open a fresh segment, so a continuous sermon still produces segments.
    if (capturedSamples - this.start >= this.samples(MAX_SEGMENT_MS)) {
      const segment = { from: this.start, to: capturedSamples };
      this.reset();
      return segment;
    }

    if (!voice && capturedSamples - this.lastVoice >= this.samples(SILENCE_MS)) {
      // Time actually spent speaking, not the width of the window: a cough
      // followed by silence is too short to be a reference.
      const speech = this.lastVoice - this.firstVoice;
      const from = this.start;
      const to = Math.min(capturedSamples, this.lastVoice + this.samples(TAIL_MS));
      this.reset();
      if (speech < this.samples(MIN_SPEECH_MS)) {
        return null;
      }
      return { from, to };
    }

    return null;
  }

  private samples(ms: number): number {
    return Math.round((ms * this.rate) / 1000);
  }
}
