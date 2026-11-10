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

// speech is a block that clears the level gate with a voiced zero-crossing rate.
const speech = (level = LOUD) => ({ level, zeroCrossingRate: 0.08 });
const silence = () => ({ level: QUIET, zeroCrossingRate: 0.02 });
const music = (level = 0.25) => ({ level, zeroCrossingRate: 0.03 });
const cymbal = (level = 0.3) => ({ level, zeroCrossingRate: 0.6 });

test('closes a segment after the speaker pauses', () => {
  const segmenter = new SpeechSegmenter(RATE);
  assert.equal(segmenter.feed(speech(), 5 * RATE), null);
  assert.equal(segmenter.hearing, true);

  const lastVoice = 5 * RATE + samples(1200);
  assert.equal(segmenter.feed(speech(), lastVoice), null);
  assert.equal(segmenter.feed(silence(), lastVoice + samples(SILENCE_MS - 100)), null);

  const segment = segmenter.feed(silence(), lastVoice + samples(SILENCE_MS));
  assert.ok(segment, 'a pause past the gap must close the segment');
  assert.equal(segmenter.hearing, false);
  assert.equal(segment.from, 5 * RATE - samples(PRE_ROLL_MS));
  assert.equal(segment.to, lastVoice + samples(250));
});

test('drops a blip shorter than the minimum speech', () => {
  const segmenter = new SpeechSegmenter(RATE);
  segmenter.feed(speech(), RATE);
  const segment = segmenter.feed(silence(), RATE + samples(SILENCE_MS));
  assert.equal(segment, null);
  assert.equal(segmenter.hearing, false);
});

test('cuts a stretch with no pause at the cap', () => {
  const segmenter = new SpeechSegmenter(RATE);
  assert.equal(segmenter.feed(speech(0.2), 0), null);
  // A long stretch whose level keeps moving is still speech.
  let block = 0;
  for (let at = samples(500); at < samples(MAX_SEGMENT_MS); at += samples(250)) {
    segmenter.feed(speech(block++ % 2 === 0 ? 0.2 : 0.45), at);
  }
  const segment = segmenter.feed(speech(), samples(MAX_SEGMENT_MS));
  assert.ok(segment, 'a long stretch must be cut before the backend limit');
  assert.equal(segment.to, samples(MAX_SEGMENT_MS));
  assert.equal(segmenter.hearing, false);
});

test('stays quiet until the level crosses the voice gate', () => {
  const segmenter = new SpeechSegmenter(RATE);
  assert.equal(segmenter.feed({ level: VOICE_LEVEL - 0.01, zeroCrossingRate: 0.08 }, RATE), null);
  assert.equal(segmenter.hearing, false);
});

test('ignores a quiet passage under the hard floor', () => {
  const segmenter = new SpeechSegmenter(RATE);
  for (let at = 0; at < samples(4000); at += samples(250)) {
    assert.equal(segmenter.feed({ level: 0.03, zeroCrossingRate: 0.08 }, at), null);
  }
  assert.equal(segmenter.hearing, false);
});

test('raises the gate above a noisy room', () => {
  const segmenter = new SpeechSegmenter(RATE);
  // Let the room teach it its floor: a fan at 0.05.
  for (let i = 0; i < 200; i++) {
    segmenter.feed({ level: 0.05, zeroCrossingRate: 0.2 }, i * samples(50));
  }
  assert.ok(segmenter.floor > 0.04 && segmenter.floor < 0.06, `floor = ${segmenter.floor}`);

  // Just above the old fixed gate, but not above the room: not speech.
  assert.equal(segmenter.feed({ level: 0.07, zeroCrossingRate: 0.08 }, samples(11000)), null);
  assert.equal(segmenter.hearing, false);

  // Well above the room, it is speech again.
  assert.equal(segmenter.feed(speech(0.3), samples(11500)), null);
  assert.equal(segmenter.hearing, true);
});

test('ignores a steady backing track', () => {
  const segmenter = new SpeechSegmenter(RATE);
  // Four seconds of nearly constant level: a sustained pad, not syllables.
  let block = 0;
  for (let at = 0; at <= samples(4000); at += samples(250)) {
    const segment = segmenter.feed(music(0.2 + (block++ % 2 === 0 ? 0.005 : 0)), at);
    assert.equal(segment, null, 'a steady stretch must not be transcribed');
  }
  assert.equal(segmenter.hearing, true, 'the guard decides when the stretch closes');
  const final = segmenter.feed(silence(), samples(4000) + samples(SILENCE_MS));
  assert.equal(final, null, 'music must be discarded, not sent to the engine');
  assert.equal(segmenter.hearing, false);
});

test('keeps modulated speech over a backing track', () => {
  const segmenter = new SpeechSegmenter(RATE);
  let block = 0;
  for (let at = 0; at <= samples(4000); at += samples(250)) {
    // Syllables swing the level hard within the same stretch.
    segmenter.feed(music(block++ % 2 === 0 ? 0.15 : 0.45), at);
  }
  const segment = segmenter.feed(silence(), samples(4000) + samples(SILENCE_MS));
  assert.ok(segment, 'speech-like modulation must survive the music guard');
});

test('ignores cymbals and hiss above the speech zero-crossing band', () => {
  const segmenter = new SpeechSegmenter(RATE);
  for (let at = 0; at < samples(3000); at += samples(250)) {
    assert.equal(segmenter.feed(cymbal(), at), null);
  }
  assert.equal(segmenter.hearing, false);
});

test('reset abandons an in-flight segment', () => {
  const segmenter = new SpeechSegmenter(RATE);
  segmenter.feed(speech(), RATE);
  segmenter.reset();
  assert.equal(segmenter.hearing, false);
  assert.equal(segmenter.feed(silence(), RATE + samples(SILENCE_MS * 2)), null);
});
