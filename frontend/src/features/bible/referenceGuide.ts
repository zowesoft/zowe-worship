import type * as models from '../../../bindings/zowe-worship/models';

type Book = models.BibleBookView;

// The part of the input that still names a book: an optional leading book
// number ("1 John") plus the letters after it, stopping at the chapter number
// or colon. "John 3:16" -> "John ", "1 John 5" -> "1 John ", and a bare "1"
// stays "1" so numbered books can be reached by their number. A plain text
// query like "living water" keeps all of it (and simply matches no book).
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

// The inline autocomplete pick: the first book whose canonical name starts with
// what the operator has typed, plus the not-yet-typed remainder to ghost after
// the caret. Typing "J" ghosts "oshua" onto "Joshua"; typing "1 jo" ghosts "hn"
// onto "1 John". Returns undefined once the name is typed in full (nothing left
// to ghost) or when no book name matches, so a plain word query never ghosts.
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

// The one book the input already names in full, if any. This is what lets Enter
// jump to the chapter once a whole book name is typed, instead of running a
// search for it.
export function resolveBook(text: string, books: Book[]): Book | undefined {
  const needle = normalize(bookPortion(text));
  if (needle === '') return undefined;
  return books.find((book) => namesFor(book).includes(needle));
}

// A resolved book followed by a bare chapter number and no colon yet, e.g.
// "John 3". Enter on this appends the colon and moves the caret to the verse.
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

// A typed reference resolved against the installed books, for the browse pane
// to jump to: the book, the chapter it names (null while only the book is
// typed so the caller decides whether a book alone should move the pane), the
// verse when one was given ("John 3:16" -> chapter 3, verse 16) and the end of
// a named range ("John 3:16-18" -> verseEnd 18, null when no range was typed).
// Returns undefined when the text names no book or carries anything besides a
// reference after it - "living water" and "John 3 somewhere" are not jumps.
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

// Whether text may be entered in browse mode. The book portion has to stay a
// prefix of an installed book name - that is the only vocabulary the inline
// suggestion knows - and everything after it may add chapter and verse syntax
// (digits, colon, dot, dash, spaces) and nothing else. "for god so" is
// refused at the first letter, so a word query can never be typed here at
// all; words belong to search mode. With no books loaded there is nothing to
// validate against, so input is accepted and the jump fails until the list
// arrives.
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
