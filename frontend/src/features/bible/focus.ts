// A passage another screen wants the Bible workspace to open. It lives apart
// from BibleWorkspace so AppShell and the detection panel can name the type
// without importing the workspace component.
export type BiblePassage = {
  bookId: number;
  chapter: number;
  // Zero means the whole chapter, matching ScriptureRequest.
  verseStart: number;
  verseEnd: number;
};

// nonce makes a repeated request for the same passage open the pane again; a
// prop that only ever holds the same numbers would not re-run the jump effect.
export type BibleFocus = BiblePassage & { nonce: number };

// focusReference is the reference text the workspace jumps with, built from a
// passage rather than typed. It uses the book's own spelling so numbered books
// keep their number ("1 John 5:7", not "John 5:7"), and it is the exact shape
// referenceGuide.parseReference accepts: chapter only, one verse, or a range.
export function focusReference(passage: BiblePassage, bookName: string): string {
  if (passage.verseStart <= 0) return `${bookName} ${passage.chapter}`;
  if (passage.verseEnd > passage.verseStart) {
    return `${bookName} ${passage.chapter}:${passage.verseStart}-${passage.verseEnd}`;
  }
  return `${bookName} ${passage.chapter}:${passage.verseStart}`;
}
