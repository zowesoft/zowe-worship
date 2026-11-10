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
