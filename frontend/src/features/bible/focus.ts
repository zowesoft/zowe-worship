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

// A slide on the wall, in the shape the workspace needs to follow it.
export type WallSlide = {
  kind: string;
  reference?: string;
  label?: string;
  bookId?: number;
  chapter?: number;
  verseStart?: number;
  verseEnd?: number;
};

// wallReference is the label of the passage on the wall, if any. Live wins
// over preview, exactly as the workspace follows it.
export function wallReference(
  state: { live?: WallSlide | null; preview?: WallSlide | null } | null | undefined,
): { reference: string; label: string; live: boolean } | null {
  const live = state?.live;
  const preview = state?.preview;
  const slide = live?.kind === 'scripture'
    ? live
    : preview?.kind === 'scripture'
      ? preview
      : null;
  if (!slide) return null;
  const reference = slide.reference ?? '';
  return { reference, label: slide.label ?? reference, live: slide === live };
}

// wallScripture is the passage the room is being shown, so the Bible workspace
// can follow it. Live wins over preview: what is on the wall is what the
// operator is serving, and it may have come from anywhere - a spoken
// detection, a schedule item, a search result, a restored slide - so the
// passage travels with the slide instead of being reconstructed per source.
export function wallScripture(
  state: { live?: WallSlide | null; preview?: WallSlide | null } | null | undefined,
): BiblePassage | null {
  const live = state?.live;
  const preview = state?.preview;
  const slide = live?.kind === 'scripture'
    ? live
    : preview?.kind === 'scripture'
      ? preview
      : null;
  if (!slide || !slide.bookId || !slide.chapter) return null;
  return {
    bookId: slide.bookId,
    chapter: slide.chapter,
    verseStart: slide.verseStart ?? 0,
    verseEnd: slide.verseEnd ?? 0,
  };
}
