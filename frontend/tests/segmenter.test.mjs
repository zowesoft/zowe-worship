// Runs the segmenter state machine under node's test runner:
//   node --experimental-strip-types --test tests/segmenter.test.mjs
// Kept outside src/ so tsc never sees the .ts import extension.
import assert from 'node:assert/strict';
import test from 'node:test';

import {
  MAX_SEGMENT_MS,
  MIN_SPEECH_MS,
  PRE_ROLL_MS,
  SILENCE_MS,
  SpeechSegmenter,
  VOICE_LEVEL,
} from '../src/features/detection/segmenter.ts';

const RATE = 16000;
const samples = (ms) => Math.round((ms * RATE) / 1000);
const LOUD = VOICE_LEVEL + 0.2;
const QUIET = 0;

test('closes a segment after the speaker pauses', () => {
  const segmenter = new SpeechSegmenter(RATE);
  assert.equal(segmenter.feed(LOUD, 5 * RATE), null);
  assert.equal(segmenter.hearing, true);

  const lastVoice = 5 * RATE + samples(1200);
  assert.equal(segmenter.feed(LOUD, lastVoice), null);
  assert.equal(segmenter.feed(QUIET, lastVoice + samples(SILENCE_MS - 100)), null);

  const segment = segmenter.feed(QUIET, lastVoice + samples(SILENCE_MS));
  assert.ok(segment, 'a pause past the gap must close the segment');
  assert.equal(segmenter.hearing, false);
  // The segment starts before the first loud block and keeps a little tail.
  assert.equal(segment.from, 5 * RATE - samples(PRE_ROLL_MS));
  assert.equal(segment.to, lastVoice + samples(250));
});

test('drops a blip shorter than the minimum speech', () => {
  const segmenter = new SpeechSegmenter(RATE);
  segmenter.feed(LOUD, RATE);
  // One loud block is not enough speech, even once the pause closes the window.
  const segment = segmenter.feed(QUIET, RATE + samples(SILENCE_MS));
  assert.equal(segment, null);
  assert.equal(segmenter.hearing, false);
});

test('cuts a stretch with no pause at the cap', () => {
  const segmenter = new SpeechSegmenter(RATE);
  assert.equal(segmenter.feed(LOUD, 0), null);

  const segment = segmenter.feed(LOUD, samples(MAX_SEGMENT_MS));
  assert.ok(segment, 'a long stretch must be cut before the backend limit');
  assert.equal(segment.from, 0);
  assert.equal(segment.to, samples(MAX_SEGMENT_MS));

  // Listening continues: the next loud reading opens a new segment.
  assert.equal(segmenter.feed(LOUD, samples(MAX_SEGMENT_MS) + 1), null);
  assert.equal(segmenter.hearing, true);
});

test('stays quiet until the level crosses the voice gate', () => {
  const segmenter = new SpeechSegmenter(RATE);
  assert.equal(segmenter.feed(VOICE_LEVEL - 0.01, RATE), null);
  assert.equal(segmenter.hearing, false);
});

test('reset abandons an in-flight segment', () => {
  const segmenter = new SpeechSegmenter(RATE);
  segmenter.feed(LOUD, RATE);
  segmenter.reset();
  assert.equal(segmenter.hearing, false);
  assert.equal(segmenter.feed(QUIET, RATE + samples(SILENCE_MS * 2)), null);
});
