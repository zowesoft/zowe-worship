import type * as models from '../../../bindings/zowe-worship/models';

type Book = models.BibleBookView;

// The part of the input that still names a book: an optional leading number plus
// the letters after it, stopping at the chapter number or colon. "John 3:16" ->
// "John ", "1 John 5" -> "1 John "; a word query keeps all of it.
export function bookPortion(text: string): string {
  const match = text.match(/^\s*[123]?\s*[^0-9:]*/);
  return match ? match[0] : '';
}

function normalize(value: string): string {
  return value.toLowerCase().replace(/\./g, '').trim();
}

// "1 John" -> "John", so a letter prefix can reach a numbered book by its name.
function withoutLeadingNumber(name: string): string {
  return name.replace(/^\s*[123]\s+/, '');
}

function namesFor(book: Book): string[] {
  return [book.name, withoutLeadingNumber(book.name), book.abbreviation].map(normalize);
}

// The first book whose name starts with what was typed, plus the remainder to
// ghost after the caret: "J" ghosts "oshua", "1 jo" ghosts "hn". Undefined when
// the name is complete or nothing matches.
export function inlineBookCompletion(
  text: string,
  books: Book[],
): { book: Book; remainder: string } | undefined {
  const typed = bookPortion(text);
  const needle = normalize(typed);
  if (needle === '') return undefined;
  const book = books.find((candidate) => normalize(candidate.name).startsWith(needle));
  if (!book) return undefined;
  const name = normalize(book.name);
  if (name === needle) return undefined;
  return { book, remainder: book.name.slice(typed.length) };
}

// The book the input names in full, which is what lets Enter jump to a chapter
// instead of searching for the text.
export function resolveBook(text: string, books: Book[]): Book | undefined {
  const needle = normalize(bookPortion(text));
  if (needle === '') return undefined;
  return books.find((book) => namesFor(book).includes(needle));
}

// A book followed by a bare chapter number and no colon, e.g. "John 3"; Enter
// appends the colon and moves the caret to the verse.
export function pendingChapter(
  text: string,
  books: Book[],
): { book: Book; chapter: string } | undefined {
  const book = resolveBook(text, books);
  if (!book) return undefined;
  const rest = text.slice(bookPortion(text).length);
  const match = rest.match(/^\s*(\d+)\s*$/);
  if (!match) return undefined;
  return { book, chapter: match[1] };
}

// A typed reference for the browse pane to jump to. Chapter is null while only
// the book is typed, so the caller decides whether that alone moves the pane.
// Undefined when the text names no book or trails anything besides a reference:
// "living water" and "John 3 somewhere" are not jumps.
export function parseReference(
  text: string,
  books: Book[],
): { book: Book; chapter: number | null; verse: number | null; verseEnd: number | null } | undefined {
  const book = resolveBook(text, books);
  if (!book) return undefined;
  const rest = text.slice(bookPortion(text).length).trim();
  if (rest === '') return { book, chapter: null, verse: null, verseEnd: null };
  const match = rest.match(/^(\d+)(?:\s*[:.]\s*(\d+))?(?:\s*-\s*(\d+))?$/);
  if (!match) return undefined;
  return {
    book,
    chapter: Number(match[1]),
    verse: match[2] ? Number(match[2]) : null,
    verseEnd: match[3] ? Number(match[3]) : null,
  };
}

// Whether text may be entered in browse mode: the book portion must prefix an
// installed book name, and the rest may only add chapter/verse syntax. "for god
// so" is refused, so word queries stay in search mode. With no books loaded
// there is nothing to validate against, so the input is accepted.
export function acceptsBrowseInput(text: string, books: Book[]): boolean {
  if (text === '' || books.length === 0) return true;
  const typedBook = bookPortion(text);
  const needle = normalize(typedBook);
  if (needle === '') return false;
  const names = books.some((book) =>
    [book.name, withoutLeadingNumber(book.name), book.abbreviation]
      .map(normalize)
      .some((name) => name.startsWith(needle)),
  );
  if (!names) return false;
  return /^[\d\s:.\-]*$/.test(text.slice(typedBook.length));
}
