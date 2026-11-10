// Runs the Bible focus path under node's test runner:
//   node --experimental-strip-types --test tests/bibleFocus.test.mjs
//
// The detection card's Search action hands over numbers, and the workspace jumps
// by typing an equivalent reference. These tests read that text back through the
// real parser, so "lands on the right verse" is checked end to end rather than
// assumed from the string format.
import assert from 'node:assert/strict';
import test from 'node:test';

import { focusReference } from '../src/features/bible/focus.ts';
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
