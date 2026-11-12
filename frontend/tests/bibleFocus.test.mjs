// Runs the Bible focus path under node's test runner:
//   node --experimental-strip-types --test tests/bibleFocus.test.mjs
//
// The detection card's Search action hands over numbers, and the workspace jumps
// by typing an equivalent reference. These tests read that text back through the
// real parser, so "lands on the right verse" is checked end to end rather than
// assumed from the string format.
import assert from 'node:assert/strict';
import test from 'node:test';

import { focusReference, wallScripture, wallReference } from '../src/features/bible/focus.ts';
import { parseReference } from '../src/features/bible/referenceGuide.ts';

const books = [
  { id: 40, name: 'Matthew', abbreviation: 'Matt', testament: 'new', chapters: 28 },
  { id: 43, name: 'John', abbreviation: 'John', testament: 'new', chapters: 21 },
  { id: 62, name: '1 John', abbreviation: '1 John', testament: 'new', chapters: 5 },
];

function resolve(passage, bookName) {
  const text = focusReference(passage, bookName);
  const parsed = parseReference(text, books);
  assert.ok(parsed, `the workspace could not parse ${JSON.stringify(text)}`);
  return { text, parsed };
}

test('a single verse resolves to that verse', () => {
  const { text, parsed } = resolve(
    { bookId: 43, chapter: 3, verseStart: 16, verseEnd: 16 },
    'John',
  );
  assert.equal(text, 'John 3:16');
  assert.equal(parsed.book.id, 43);
  assert.equal(parsed.chapter, 3);
  assert.equal(parsed.verse, 16);
  assert.equal(parsed.verseEnd, null);
});

test('a verse range keeps both ends', () => {
  const { text, parsed } = resolve(
    { bookId: 40, chapter: 11, verseStart: 28, verseEnd: 30 },
    'Matthew',
  );
  assert.equal(text, 'Matthew 11:28-30');
  assert.equal(parsed.chapter, 11);
  assert.equal(parsed.verse, 28);
  assert.equal(parsed.verseEnd, 30);
});

// A chapter-only candidate (verseStart 0) must open the chapter, not verse 0,
// which does not exist and would select nothing.
test('a chapter-only passage opens the chapter with no verse selected', () => {
  const { text, parsed } = resolve(
    { bookId: 43, chapter: 3, verseStart: 0, verseEnd: 0 },
    'John',
  );
  assert.equal(text, 'John 3');
  assert.equal(parsed.chapter, 3);
  assert.equal(parsed.verse, null);
});

// The book name comes from the installed list, and numbered books must keep
// their number or the jump would land in the wrong book entirely.
test('a numbered book keeps its number', () => {
  const { text, parsed } = resolve(
    { bookId: 62, chapter: 5, verseStart: 7, verseEnd: 7 },
    '1 John',
  );
  assert.equal(text, '1 John 5:7');
  assert.equal(parsed.book.id, 62);
  assert.equal(parsed.chapter, 5);
  assert.equal(parsed.verse, 7);
});

// The focus effect clears the request once it has jumped, so a repeated Search
// must produce the same request again rather than depend on a stale one.
test('the same passage produces the same request every time', () => {
  const passage = { bookId: 43, chapter: 3, verseStart: 16, verseEnd: 16 };
  assert.equal(focusReference(passage, 'John'), focusReference({ ...passage }, 'John'));
});

// The wall's scripture is the passage the Bible workspace follows. It may have
// come from anywhere - a spoken detection, a schedule item, a restored slide -
// so the passage travels with the slide rather than being rebuilt per source.
test('the live scripture is the passage the workspace follows', () => {
  const passage = wallScripture({
    live: { kind: 'scripture', bookId: 43, chapter: 3, verseStart: 16, verseEnd: 16 },
    preview: null,
  });
  assert.deepEqual(passage, { bookId: 43, chapter: 3, verseStart: 16, verseEnd: 16 });
});

test('a previewed scripture is followed when nothing is live yet', () => {
  const passage = wallScripture({
    live: null,
    preview: { kind: 'scripture', bookId: 62, chapter: 5, verseStart: 7, verseEnd: 7 },
  });
  assert.deepEqual(passage, { bookId: 62, chapter: 5, verseStart: 7, verseEnd: 7 });
});

test('live wins over the preview', () => {
  const passage = wallScripture({
    live: { kind: 'scripture', bookId: 43, chapter: 3, verseStart: 16, verseEnd: 16 },
    preview: { kind: 'scripture', bookId: 19, chapter: 2, verseStart: 1, verseEnd: 1 },
  });
  assert.equal(passage.bookId, 43);
});

// A slide that is not a scripture has no passage to follow, and a wall that is
// showing nothing at all is nothing to follow.
test('a slide that is not scripture is not followed', () => {
  assert.equal(wallScripture({ live: { kind: 'song' }, preview: null }), null);
  assert.equal(wallScripture({ live: { kind: 'media' }, preview: { kind: 'text' } }), null);
  assert.equal(wallScripture({ live: null, preview: null }), null);
  assert.equal(wallScripture(null), null);
});

// The browse jump field names the passage on the wall instead of the static
// hint, so what is showing is visible right where the operator jumps from.
test('the jump field can name the passage on the wall', () => {
  const wall = wallReference({
    live: null,
    preview: { kind: 'scripture', reference: 'Mark 4:35', label: 'Mark 4:35 (KJV)' },
  });
  assert.deepEqual(wall, { reference: 'Mark 4:35', label: 'Mark 4:35 (KJV)', live: false });
});

test('a live passage is reported as live', () => {
  const wall = wallReference({
    live: { kind: 'scripture', reference: 'John 3:16', label: 'John 3:16 (KJV)' },
    preview: { kind: 'scripture', reference: 'Mark 4:35', label: 'Mark 4:35 (KJV)' },
  });
  assert.equal(wall.live, true);
  assert.equal(wall.label, 'John 3:16 (KJV)');
});

test('a wall showing nothing names no passage', () => {
  assert.equal(wallReference({ live: null, preview: null }), null);
  assert.equal(wallReference({ live: { kind: 'song' }, preview: null }), null);
  assert.equal(wallReference(null), null);
});

// A chapter without verses is followed as the chapter, not as verse zero,
// which does not exist.
test('a chapter-only passage follows as the chapter', () => {
  const passage = wallScripture({
    live: { kind: 'scripture', bookId: 43, chapter: 3 },
  });
  assert.deepEqual(passage, { bookId: 43, chapter: 3, verseStart: 0, verseEnd: 0 });
});
