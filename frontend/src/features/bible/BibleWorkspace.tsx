import { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import type { FormEvent, KeyboardEvent, MouseEvent as ReactMouseEvent } from 'react';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import { App } from '../../../bindings/zowe-worship';
import * as models from '../../../bindings/zowe-worship/models';
import { bookPortion, inlineBookCompletion, resolveBook, pendingChapter, parseReference, acceptsBrowseInput } from './referenceGuide';
import { focusReference } from './focus';
import type { BibleFocus } from './focus';


// The Songs screen reuses this row/action look; everything else stays inline.
const ROW_ACTION =
  'inline-flex items-center gap-[5px] px-[7px] py-1.5 border border-transparent ' +
  'rounded-[5px] bg-transparent text-secondary text-[10px] whitespace-nowrap ' +
  'disabled:text-[#6F809B] max-[1279px]:[&_span]:hidden';

const STATUS_PANEL =
  'min-h-[210px] flex flex-col items-center justify-center gap-2 p-6 border ' +
  'border-dashed border-border rounded-lg bg-app text-secondary text-center ' +
  '[&>svg]:opacity-70 [&_strong]:text-text [&_strong]:text-[12px] ' +
  '[&_strong]:font-medium [&_p]:max-w-[440px] [&_p]:text-[11px] [&_p]:leading-[1.7]';

const SEARCH_FIELD =
  'flex-1 min-w-0 flex items-center gap-2 min-h-[38px] px-3 border border-[#35445C] ' +
  'rounded-[7px] bg-app [&>svg]:text-secondary ' +
  'focus-within:border-blue focus-within:shadow-[0_0_0_2px_rgba(71,120,255,0.18)]';

const SEARCH_INPUT =
  'flex-1 min-w-0 h-9 border-0 outline-none bg-transparent text-text text-[12px] ' +
  'placeholder:text-[#6F809B] disabled:text-secondary';

const FILTER =
  'min-h-[30px] px-2 border border-[#2A3950] rounded-md bg-elevated text-secondary ' +
  'text-[10px] disabled:opacity-[0.65]';

const BADGE =
  'inline-flex justify-center px-1 py-px border border-border rounded-[4px] ' +
  'text-secondary text-[9px] tracking-[0.4px]';


const modes = [
  { value: 'auto', label: 'References + text' },
  { value: 'words', label: 'Full text' },
  { value: 'phrase', label: 'Exact phrase' },
  { value: 'prefix', label: 'Word prefix' },
] as const;

type Mode = (typeof modes)[number]['value'];
type ActiveTab = 'search' | 'browse';

const SEARCH_DEBOUNCE_MS = 300;
const MIN_QUERY_LENGTH = 2;
const PAGE_SIZE = 50;

// Shown when a keystroke is refused in browse mode: the field only builds
// references out of book names, and words belong to search mode.
const BROWSE_INPUT_HINT =
  'Browse takes references from book names only, like John 3:16. Use search mode for words.';


function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

// The row is a click target for preview/send-live; a click that lands on the
// action buttons must not also fire the row handler.
function fromActionArea(event: ReactMouseEvent<HTMLElement>): boolean {
  return (event.target as HTMLElement).closest('[data-row-actions]') !== null;
}

/** Render a snippet that may contain <mark> tags safely. */
function SnippetText({ snippet, text, selected }: { snippet?: string; text: string; selected?: boolean }) {
  // A selected row reads its snippet in full-strength text instead of the muted
  // secondary colour used for the rest of the list.
  const cls =
    'min-w-0 truncate text-[11px] [&_mark]:text-blue [&_mark]:font-semibold [&_mark]:rounded-[2px] [&_mark]:px-px '
    + (selected ? 'text-text' : 'text-secondary');
  if (snippet) {
    // snippet uses <mark> tags from FTS5; we trust the backend to only emit those.
    const safe = snippet.replace(/<(?!\/?mark>)[^>]+>/gi, '');
    return <span className={cls} dangerouslySetInnerHTML={{ __html: safe }} />;
  }
  return <span className={cls}>{text}</span>;
}


type SearchState =
  | { kind: 'idle' }
  | { kind: 'error'; message: string }
  | { kind: 'results'; results: models.BibleVerse[]; hasMore: boolean };

type ChapterBlock = { bookId: number; chapter: number; results: models.BibleVerse[] };

type BrowseState =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'error'; message: string }
  | { kind: 'results'; chapters: ChapterBlock[]; bookId: number; chapter: number };

type Selection = { bookId: number; chapter: number; start: number; end: number };

type ActionState = {
  busy: string | null;
  status: { kind: 'message' | 'error'; text: string } | null;
};


export function BibleWorkspace({
  focus = null,
  onFocusHandled,
}: {
  // A passage the detection panel asked this workspace to open. Cleared by the
  // parent once consumed, so it only ever moves the pane once.
  focus?: BibleFocus | null;
  onFocusHandled?: () => void;
} = {}) {
  const studio = useStudio();
  const bridgeAvailable = studio.bridgeAvailable;

  const [translations, setTranslations] = useState<models.TranslationView[]>([]);
  const [translationId, setTranslationId] = useState('');
  const [translationError, setTranslationError] = useState<string | null>(
    bridgeAvailable ? null : 'Open the Zowe Worship desktop app to search installed translations.',
  );
  const [loadingTranslations, setLoadingTranslations] = useState(bridgeAvailable);

  const [books, setBooks] = useState<models.BibleBookView[]>([]);

  const [activeTab, setActiveTab] = useState<ActiveTab>('browse');

  const [query, setQuery] = useState('');
  const [mode, setMode] = useState<Mode>('auto');
  const [testament, setTestament] = useState('');   // '' | 'old' | 'new'
  const [bookFilter, setBookFilter] = useState(0);  // 0 = all
  const [search, setSearch] = useState<SearchState>({ kind: 'idle' });
  const [pending, setPending] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const [selection, setSelection] = useState<Selection | null>(null);
  const [action, setAction] = useState<ActionState>({ busy: null, status: null });

  // The inline book-name ghost only shows while focused and still typing the
  // book portion in "auto".
  const [fieldFocused, setFieldFocused] = useState(false);

  const requestId = useRef(0);
  const debounceRef = useRef<number | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  // The reading pane under the pinned search area. Scrolls go through this ref,
  // not scrollIntoView, which would walk every ancestor and drag the header too.
  const paneRef = useRef<HTMLDivElement>(null);
  // The row the next render should pin to the top of the pane (the searched
  // verse, not its chapter's first). Consumed once the row is in the DOM.
  const pinnedScrollKey = useRef<string | null>(null);
  // Arrow-stepping writes the highlighted reference into the field for display;
  // that write must not re-trigger the search debounce and reset the pane.
  const skipQuerySync = useRef(false);
  // Where the reading pane was before a search moved it, so clearing the search
  // can put it back.
  const browseReturn = useRef<{ bookId: number; chapter: number } | null>(null);
  // Monotonic token for chapter loads: an overlapping jump and clear must
  // never let a stale response overwrite the pane the operator moved to next.
  const loadSeq = useRef(0);

  const [browseBookId, setBrowseBookId] = useState(1);
  const [browseChapter, setBrowseChapter] = useState(1);
  const [browseState, setBrowseState] = useState<BrowseState>({ kind: 'idle' });
  const [browseMore, setBrowseMore] = useState(false);


  useEffect(() => {
    if (!bridgeAvailable) return;

    let active = true;
    Promise.all([
      App.ListTranslations(),
      App.GetSetting('default_translation_id').catch(() => ''),
      App.ListBooks(),
    ])
      .then(([items, defaultId, bookList]) => {
        if (!active) return;
        const installed = items ?? [];
        setTranslations(installed);
        const preferred =
          (defaultId ? installed.find((item) => item.id === defaultId) : undefined) ??
          installed.find((item) => item.id === 'kjv') ??
          installed[0];
        setTranslationId(preferred?.id ?? '');
        setBooks(bookList ?? []);
      })
      .catch((error: unknown) => {
        if (active) setTranslationError(errorMessage(error));
      })
      .finally(() => {
        if (active) setLoadingTranslations(false);
      });

    return () => {
      active = false;
    };
  }, [bridgeAvailable]);

  const activeTranslation = translations.find((t) => t.id === translationId);
  const trimmedQuery = query.trim();
  const canSearch =
    bridgeAvailable && translationId !== '' && trimmedQuery.length >= MIN_QUERY_LENGTH;
  const canAct = bridgeAvailable && translationId !== '' && action.busy === null;
  // Going live must not wait on an in-flight action: the first click of a
  // double-click sets busy, and Enter can arrive mid-send. Only the bridge gates.
  const canGoLive = bridgeAvailable && translationId !== '';

  const filteredBooks =
    testament === '' ? books : books.filter((b) => b.testament === testament);

  // The operator is still naming a book while the text after it holds no
  // chapter digit or colon. Keying off the chapter position, not "any digit",
  // keeps the "1" in "1 John" in the book stage.
  const bookText = bookPortion(query);
  const bookCommitted = Boolean(resolveBook(query, books)) && /\s$/.test(query);
  const inBookStage =
    mode === 'auto' &&
    query !== '' &&
    !bookCommitted &&
    !/[0-9:]/.test(query.slice(bookText.length));

  // The book the typed prefix completes to, and the remainder to ghost after the
  // caret while in the book stage.
  const completion = useMemo(
    () => (inBookStage ? inlineBookCompletion(query, books) : undefined),
    [inBookStage, query, books],
  );
  const ghost = fieldFocused && completion && completion.remainder !== '' ? completion.remainder : '';

  // Put the caret at the end of the field on the next frame, after React has
  // committed the new value — setting it synchronously would be overwritten.
  function caretToEnd(value: string) {
    requestAnimationFrame(() => {
      const el = inputRef.current;
      if (!el) return;
      el.focus();
      el.setSelectionRange(value.length, value.length);
    });
  }

  function acceptBook(book: models.BibleBookView) {
    const next = `${book.name} `;
    setQuery(next);
    caretToEnd(next);
  }

  function handleSearchKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    const el = event.currentTarget;
    const caretAtEnd = el.selectionStart === el.value.length && el.selectionEnd === el.value.length;

    // Accept the ghosted book name. Tab and Enter always accept; Right and End
    // only when the caret is at the end, so caret movement stays normal. Enter
    // accepting here also stops a half-typed book sending its partial query live.
    if (completion && completion.remainder !== '') {
      const accepts =
        event.key === 'Tab' ||
        event.key === 'Enter' ||
        ((event.key === 'ArrowRight' || event.key === 'End') && caretAtEnd);
      if (accepts) {
        event.preventDefault();
        acceptBook(completion.book);
        return;
      }
    }

    // Escape dismisses the ghost by dropping focus from the field.
    if (event.key === 'Escape') {
      setFieldFocused(false);
      el.blur();
      return;
    }

    // Space right after a chapter number becomes the colon, so the operator can
    // keep typing the verse without reaching for another key.
    if (event.key === ' ') {
      const pendingChapterRef = pendingChapter(query, books);
      if (pendingChapterRef) {
        event.preventDefault();
        const next = `${pendingChapterRef.book.name} ${pendingChapterRef.chapter}:`;
        setQuery(next);
        caretToEnd(next);
        return;
      }
    }

    if (event.key !== 'Enter') return;

    // Browse takes references: Enter jumps the reading pane there (a whole
    // book name lands on its first chapter) and sends the reference live,
    // never running a keyword search.
    if (activeTab === 'browse') {
      event.preventDefault();
      void browseEnter();
      return;
    }

    // Enter sends the highlighted verse (or the top hit) live. !pending avoids
    // sending the previous query's stale top hit while a search is in flight.
    if (canGoLive && !pending && search.kind === 'results' && search.results.length > 0) {
      event.preventDefault();
      const highlighted =
        selection === null
          ? undefined
          : search.results.find(
              (v) => v.bookId === selection.bookId && v.chapter === selection.chapter && v.verse === selection.start,
            );
      void perform('live', highlighted ?? search.results[0]);
      return;
    }
  }


  function cancelDebounce() {
    if (debounceRef.current !== null) {
      window.clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
  }

  const executeSearch = useCallback(
    async (text: string, tid: string, off: number, replace: boolean) => {
      const currentRequest = ++requestId.current;
      if (replace) setPending(true);
      else setLoadingMore(true);

      try {
        const response = await App.SearchBible({
          translationId: tid,
          query: text,
          mode,
          limit: PAGE_SIZE,
          offset: off,
          testament,
          bookIds: bookFilter > 0 ? [bookFilter] : [],
        });
        if (currentRequest !== requestId.current) return;

        const incoming = response.results ?? [];
        // A reference search returns the whole reading, so there is never a
        // next page to load for it.
        const hasMore = response.kind !== 'reference' && incoming.length === PAGE_SIZE;

        if (replace) {
          setSearch({ kind: 'results', results: incoming, hasMore });
          // The selection is what the query named: the verse range of a
          // reference search (highlighted and acted on as one block), or the
          // top result's own row for text and whole-chapter searches.
          const first = incoming[0];
          // Pin the searched verse to the top. A reference renders its whole
          // chapter, so the first result row would bury the verse asked for; a
          // text search ranks its best hit first, so its top row is the target.
          const pinned = first === undefined
            ? null
            : `${first.bookId}:${first.chapter}:${response.verseStart > 0 ? response.verseStart : first.verse}`;
          pinnedScrollKey.current = pinned;
          setSelection(
            first === undefined
              ? null
              : response.verseStart > 0
                ? {
                    bookId: first.bookId,
                    chapter: first.chapter,
                    start: response.verseStart,
                    end: response.verseEnd || response.verseStart,
                  }
                : { bookId: first.bookId, chapter: first.chapter, start: first.verse, end: first.verse },
          );
          setOffset(off + incoming.length);
        } else {
          setSearch((prev) => {
            const existing = prev.kind === 'results' ? prev.results : [];
            return { kind: 'results', results: [...existing, ...incoming], hasMore };
          });
          setOffset((o) => o + incoming.length);
        }
      } catch (error: unknown) {
        if (currentRequest !== requestId.current) return;
        if (replace) setSearch({ kind: 'error', message: errorMessage(error) });
      } finally {
        if (currentRequest === requestId.current) {
          setPending(false);
          setLoadingMore(false);
        }
      }
    },
    [mode, testament, bookFilter],
  );

  useEffect(() => {
    if (!bridgeAvailable || translationId === '') return;
    if (skipQuerySync.current) {
      skipQuerySync.current = false;
      cancelDebounce();
      return;
    }
    cancelDebounce();

    if (trimmedQuery === '') {
      requestId.current += 1;
      setPending(false);
      setSearch({ kind: 'idle' });
      setSelection(null);
      setOffset(0);
      // An empty field means reading again: put the pane back where it was
      // before a search or a reference jump moved it (Genesis on a fresh
      // workspace). Covers both the clear button and backspacing to empty.
      const home = browseReturn.current;
      if (home) {
        browseReturn.current = null;
        const alreadyHome =
          home.bookId === browseBookId &&
          home.chapter === browseChapter &&
          browseState.kind === 'results';
        if (!alreadyHome) {
          setBrowseBookId(home.bookId);
          setBrowseChapter(home.chapter);
          void loadChapter(home.bookId, home.chapter);
        }
      }
      return;
    }

    // Browse is reference entry, not search: typing narrows the book ghost
    // and moves the reading pane as soon as the text names a chapter. Words
    // are search mode's job - nothing is queried from here.
    if (activeTab === 'browse') {
      setAction({ busy: null, status: null });
      debounceRef.current = window.setTimeout(() => {
        debounceRef.current = null;
        void jumpToReference(trimmedQuery, { bookOnly: false });
      }, SEARCH_DEBOUNCE_MS);
      return;
    }

    if (trimmedQuery.length < MIN_QUERY_LENGTH) return;

    setAction({ busy: null, status: null });
    debounceRef.current = window.setTimeout(() => {
      debounceRef.current = null;
      setOffset(0);
      void executeSearch(trimmedQuery, translationId, 0, true);
    }, SEARCH_DEBOUNCE_MS);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [trimmedQuery, mode, testament, bookFilter, translationId, bridgeAvailable, activeTab, books]);

  useEffect(() => cancelDebounce, []);

  function searchNow() {
    cancelDebounce();
    if (!canSearch) return;
    setAction({ busy: null, status: null });
    setOffset(0);
    void executeSearch(trimmedQuery, translationId, 0, true);
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    // In browse the field takes a reference: Enter jumps the pane and sends
    // the reference live instead of searching for the text.
    if (activeTab === 'browse') {
      void browseEnter();
      return;
    }
    searchNow();
  }

  function loadMore() {
    if (!canSearch || loadingMore) return;
    void executeSearch(trimmedQuery, translationId, offset, false);
  }

  function clearSearch() {
    cancelDebounce();
    requestId.current += 1;
    setPending(false);
    setQuery('');
    setSearch({ kind: 'idle' });
    setSelection(null);
    setOffset(0);
    // The empty-query branch of the search effect puts the pane back where the
    // operator had been browsing.
    setActiveTab('browse');
    inputRef.current?.focus();
  }


  // The address an action on this row operates on: inside the selection the
  // whole range (Job 29:15-20 sends all six verses to the schedule), outside
  // it just the row's own verse. The label names what will be sent.
  function actionTarget(result: models.BibleVerse): {
    start: number;
    end: number;
    label: string;
    inRange: boolean;
  } {
    const inRange =
      selection !== null &&
      selection.bookId === result.bookId &&
      selection.chapter === result.chapter &&
      result.verse >= selection.start &&
      result.verse <= selection.end;
    const start = inRange && selection !== null ? selection.start : result.verse;
    const end = inRange && selection !== null ? selection.end : result.verse;
    const label =
      end > start ? `${result.book} ${result.chapter}:${start}-${end}` : result.reference;
    return { start, end, label, inRange };
  }

  async function perform(kind: 'preview' | 'live' | 'add', result: models.BibleVerse) {
    const target = actionTarget(result);
    const key = `${kind}:${target.label}`;
    setAction({ busy: key, status: null });

    // Acting on a verse outside the selected range collapses the selection to
    // that verse - the range was what the search named, the click is the
    // operator's decision. Acting inside keeps the whole range selected.
    if (!target.inRange && kind !== 'add') {
      setSelection({ bookId: result.bookId, chapter: result.chapter, start: result.verse, end: result.verse });
    }

    const input = {
      translationId,
      bookId: result.bookId,
      chapter: result.chapter,
      verseStart: target.start,
      verseEnd: target.end,
    };

    try {
      if (kind === 'preview') {
        await studio.previewScripture(input);
        setAction({ busy: null, status: { kind: 'message', text: `Previewing ${target.label}.` } });
        return;
      }
      if (kind === 'live') {
        await studio.projectScripture(input);
        setAction({ busy: null, status: { kind: 'message', text: `${target.label} is live.` } });
        return;
      }
      const order = await studio.addScripture(input);
      setAction({
        busy: null,
        status: {
          kind: 'message',
          text: `Added ${target.label} to ${order.service?.name ?? 'the service'}.`,
        },
      });
    } catch (error: unknown) {
      setAction({ busy: null, status: { kind: 'error', text: errorMessage(error) } });
    }
  }


  // The text resolves to a book and chapter, and the pane goes there. Text
  // naming no reference returns null, so keywords never run from here.
  async function jumpToReference(
    text: string,
    options: { bookOnly: boolean },
  ): Promise<{ book: models.BibleBookView; chapter: number; verse: number | null; verseEnd: number | null } | null> {
    const target = parseReference(text, books);
    if (!target) return null;
    if (target.chapter === null && !options.bookOnly) return null;
    const chapter = target.chapter ?? 1;
    if (chapter < 1 || chapter > target.book.chapters) return null;
    const resolved = { book: target.book, chapter, verse: target.verse, verseEnd: target.verseEnd };

    const alreadyThere =
      target.book.id === browseBookId && chapter === browseChapter && browseState.kind === 'results';
    if (!alreadyThere) {
      // Remember where reading was before the pane moves. Only the first move
      // counts: clearing after several jumps returns all the way home.
      if (browseReturn.current === null) {
        browseReturn.current = { bookId: browseBookId, chapter: browseChapter };
      }
      setBrowseBookId(target.book.id);
      setBrowseChapter(chapter);
      await loadChapter(target.book.id, chapter);
    }

    if (target.verse !== null) {
      // A named verse (or range, "Joshua 1:8-10") is the selection: its rows
      // highlight together and act together.
      setSelection({
        bookId: target.book.id,
        chapter,
        start: target.verse,
        end: target.verseEnd ?? target.verse,
      });
      const key = `${target.book.id}:${chapter}:${target.verse}`;
      // Pin the searched verse to the top so it is the first thing seen; the
      // chapter renders whole, so the scroll waits for that render.
      pinnedScrollKey.current = key;
      // When the pane is already on this chapter nothing reloads, so the pin
      // effect's dependencies never change and the row would be selected without
      // being brought into view. The rows are already in the DOM, so scroll now.
      if (alreadyThere) requestAnimationFrame(() => scrollVerseToTop(key));
    } else {
      setSelection(null);
    }
    return resolved;
  }

  // Send whatever the pane is highlighting — a stepper or search result — not
  // the stale text still sitting in the field.
  async function browseEnter(): Promise<void> {
    const highlighted =
      selection === null
        ? undefined
        : visibleVerses().find(
            (v) => v.bookId === selection.bookId && v.chapter === selection.chapter && v.verse === selection.start,
          );
    if (highlighted !== undefined && canGoLive) {
      void perform('live', highlighted);
      return;
    }
    const target = await jumpToReference(query, { bookOnly: true });
    if (!target) {
      setAction({ busy: null, status: { kind: 'message', text: browseJumpHint() } });
      return;
    }
    const verseEnd = target.verse !== null ? (target.verseEnd ?? target.verse) : null;
    const reference =
      target.verse === null
        ? `${target.book.name} ${target.chapter}`
        : verseEnd !== null && verseEnd > target.verse
          ? `${target.book.name} ${target.chapter}:${target.verse}-${verseEnd}`
          : `${target.book.name} ${target.chapter}:${target.verse}`;
    if (!canGoLive) return; // the pane moved, but there is no audience to send to

    setAction({ busy: `live:${reference}`, status: null });
    try {
      await studio.projectScripture({
        translationId,
        bookId: target.book.id,
        chapter: target.chapter,
        verseStart: target.verse ?? 0,
        verseEnd: verseEnd ?? 0,
      });
      setAction({ busy: null, status: { kind: 'message', text: `${reference} is live.` } });
    } catch (error: unknown) {
      setAction({ busy: null, status: { kind: 'error', text: errorMessage(error) } });
    }
  }

  function browseJumpHint() {
    return 'Browse moves by reference, like John 3:16. Use search mode for words.';
  }

  async function loadChapter(bookId: number, chapter: number, append = false) {
    if (!bridgeAvailable || translationId === '') return;
    const seq = ++loadSeq.current;
    if (append) setBrowseMore(true);
    else setBrowseState({ kind: 'loading' });

    try {
      const response = await App.BrowsePassage({
        translationId,
        bookId,
        chapter,
        verseStart: 0,
        verseEnd: 0,
      });
      if (seq !== loadSeq.current) return; // a newer load owns the pane
      const block: ChapterBlock = { bookId, chapter, results: response.results ?? [] };
      setBrowseState((current) => (
        append && current.kind === 'results'
          ? { ...current, chapters: [...current.chapters, block] }
          : { kind: 'results', chapters: [block], bookId, chapter }
      ));
    } catch (error: unknown) {
      if (seq !== loadSeq.current) return;
      // A failed top-up leaves the chapters already on screen alone; only a
      // first load can leave the pane with nothing to show.
      if (!append) setBrowseState({ kind: 'error', message: errorMessage(error) });
    } finally {
      if (seq === loadSeq.current) setBrowseMore(false);
    }
  }

  // The chapter after this one, rolling into the next book; null at the end of
  // the translation, which is not the same as a page that has not loaded yet.
  function nextChapter(bookId: number, chapter: number): { bookId: number; chapter: number } | null {
    const book = books.find((candidate) => candidate.id === bookId);
    const chapters = book?.chapters ?? 1;
    if (chapter < chapters) return { bookId, chapter: chapter + 1 };

    const index = books.findIndex((candidate) => candidate.id === bookId);
    const following = books[index + 1];
    if (!following) return null;
    return { bookId: following.id, chapter: 1 };
  }

  // Scroll one verse row to the top of the pane by itself. The offset is
  // measured against the pane's own box, so no ancestor (the header) moves.
  function scrollVerseToTop(key: string) {
    const pane = paneRef.current;
    const row = pane?.querySelector(`[data-verse="${key}"]`);
    if (pane && row) {
      const delta = row.getBoundingClientRect().top - pane.getBoundingClientRect().top;
      pane.scrollTo({ top: pane.scrollTop + delta });
    }
  }

  // The verses on screen in reading order - ranked hits during a search, else
  // the chapter blocks. Arrow keys step this list so the highlight stays visible.
  function visibleVerses(): models.BibleVerse[] {
    if (activeTab === 'search' && search.kind === 'results') return search.results;
    if (browseState.kind === 'results') return browseState.chapters.flatMap((c) => c.results);
    return [];
  }

  // Nudge the pane just enough to keep the row on screen; unlike
  // scrollVerseToTop this preserves the operator's place while stepping.
  function keepRowVisible(key: string) {
    const pane = paneRef.current;
    const row = pane?.querySelector(`[data-verse="${key}"]`);
    if (!pane || !row) return;
    const box = pane.getBoundingClientRect();
    const r = row.getBoundingClientRect();
    if (r.top < box.top) pane.scrollTo({ top: pane.scrollTop + (r.top - box.top) });
    else if (r.bottom > box.bottom) pane.scrollTo({ top: pane.scrollTop + (r.bottom - box.bottom) });
  }

  // Up/Down step the highlight one verse and preview it; Enter sends it live.
  // Scoped here so the shell's Left/Right schedule shortcuts are untouched.
  function handleWorkspaceKeyDown(event: React.KeyboardEvent<HTMLElement>) {
    const target = event.target as HTMLElement;
    // Dropdowns keep their own arrow/Enter behaviour (choosing an option), and
    // the row action buttons keep Enter for their own action.
    const ownsKeys = target.closest('select, [data-row-actions]') !== null;

    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      if (ownsKeys || selection === null || !canGoLive) return;
      const list = visibleVerses();
      const anchor = event.key === 'ArrowDown' ? selection.end : selection.start;
      const index = list.findIndex(
        (v) => v.bookId === selection.bookId && v.chapter === selection.chapter && v.verse === anchor,
      );
      const next = list[index + (event.key === 'ArrowDown' ? 1 : -1)];
      if (index === -1 || next === undefined) return;
      event.preventDefault();
      setSelection({ bookId: next.bookId, chapter: next.chapter, start: next.verse, end: next.verse });
      // Show the highlighted reference in the field so it matches what Enter
      // sends; the flag keeps this write from re-running the search debounce.
      skipQuerySync.current = true;
      setQuery(next.reference);
      void perform('preview', next);
      keepRowVisible(`${next.bookId}:${next.chapter}:${next.verse}`);
      return;
    }

    if (event.key === 'Enter') {
      // The search field owns Enter (submit / send top hit); a row button's
      // default Enter would only preview, so send the highlight live instead.
      if (ownsKeys || event.defaultPrevented || event.target === inputRef.current) return;
      if (selection === null || !canGoLive) return;
      const verse = visibleVerses().find(
        (v) => v.bookId === selection.bookId && v.chapter === selection.chapter && v.verse === selection.start,
      );
      if (verse === undefined) return;
      event.preventDefault();
      void perform('live', verse);
    }
  }

  function handleBrowseScroll(event: React.UIEvent<HTMLDivElement>) {
    const box = event.currentTarget;
    // The reading pane is on screen in browse mode and, in search mode, while
    // no query is active; scrolling forward loads the next chapter only then.
    const readingVisible = activeTab === 'browse' || (!pending && search.kind === 'idle');
    if (!readingVisible || browseMore || browseState.kind !== 'results') return;
    if (box.scrollHeight - box.scrollTop - box.clientHeight > 260) return;

    const last = browseState.chapters[browseState.chapters.length - 1];
    const next = nextChapter(last.bookId, last.chapter);
    if (!next) return;

    setBrowseBookId(next.bookId);
    setBrowseChapter(next.chapter);
    void loadChapter(next.bookId, next.chapter, true);
  }

  // The tab never opens empty: Genesis 1 is where reading starts.
  useEffect(() => {
    if (!bridgeAvailable || translationId === '') return;
    setBrowseBookId(1);
    setBrowseChapter(1);
    void loadChapter(1, 1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bridgeAvailable, translationId]);

  // Another screen (the detection panel's Search action) can ask for one
  // passage. The jump goes through the same reference path as typing it, so the
  // chapter loads and the verse or range is selected and pinned exactly as a
  // manual browse would leave it.
  useEffect(() => {
    if (!focus || !bridgeAvailable || translationId === '' || books.length === 0) return;
    const book = books.find((candidate) => candidate.id === focus.bookId);
    if (!book) return;
    setActiveTab('browse');
    setQuery('');
    void jumpToReference(focusReference(focus, book.name), { bookOnly: false });
    onFocusHandled?.();
    // The nonce is what makes a second request for the same passage run again.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [focus, bridgeAvailable, translationId, books]);

  // Searching moves that same reading pane to the book that was searched, and
  // clearing the search puts it back where the operator had been reading.
  useEffect(() => {
    if (search.kind !== 'results' || search.results.length === 0) return;
    const first = search.results[0];
    if (browseReturn.current === null) {
      browseReturn.current = { bookId: browseBookId, chapter: browseChapter };
    }
    if (first.bookId === browseBookId && first.chapter === browseChapter) return;

    setBrowseBookId(first.bookId);
    setBrowseChapter(first.chapter);
    void loadChapter(first.bookId, first.chapter);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search]);

  // Pin a requested verse to the top once its row renders. A reference search
  // waits for its chapter, so the key is held across renders until then.
  useEffect(() => {
    const key = pinnedScrollKey.current;
    if (key === null) return;
    if (paneRef.current?.querySelector(`[data-verse="${key}"]`)) {
      scrollVerseToTop(key);
      pinnedScrollKey.current = null;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [search, browseState]);


  function renderVerseRow(result: models.BibleVerse, showRef = true) {
    const target = actionTarget(result);
    const selected = target.inRange;
    return (
      <li
        key={`${result.bookId}-${result.chapter}-${result.verse}`}
        data-verse={`${result.bookId}:${result.chapter}:${result.verse}`}
        className={
          'flex items-center gap-2 px-2 py-px border rounded-[7px] cursor-pointer select-none '
          + (selected
            ? 'bg-selected border-[#304C80] shadow-[inset_3px_0_var(--color-blue)]'
            : 'bg-app border-transparent hover:bg-[#0C1523]')
        }
        title={`Click to preview ${target.label} · double-click to send live`}
        onClick={(event) => {
          if (fromActionArea(event) || !canAct) return;
          void perform('preview', result);
        }}
        onDoubleClick={(event) => {
          // The first click of the double-click already started a preview, so
          // action.busy is set by now; gate on the bridge only, not on busy.
          if (fromActionArea(event) || !canGoLive) return;
          void perform('live', result);
        }}
      >
        <button
          type="button"
          className={
            // The selected-row highlight is the focus indicator here; a browser
            // focus ring on top of it reads as a second, unwanted border.
            'flex-1 min-w-0 grid items-center gap-2 py-[9px] px-1 border-0 bg-transparent text-left text-inherit focus:outline-none focus-visible:outline-none '
            + (showRef ? 'grid-cols-[84px_36px_minmax(0,1fr)] max-[1279px]:grid-cols-[78px_32px_minmax(0,1fr)] max-[959px]:grid-cols-[72px_30px_minmax(0,1fr)] max-[959px]:pl-0.5'
                       : 'grid-cols-[28px_36px_minmax(0,1fr)]')
          }
          aria-pressed={selected}
        >
          {showRef && <span className="text-[11px] font-semibold whitespace-nowrap text-text">{result.reference}</span>}
          {!showRef && (
            <span className="text-[11px] font-semibold whitespace-nowrap text-text">
              <span className="text-[11px] font-bold text-secondary">{result.verse}</span>
            </span>
          )}
          <span className="inline-flex justify-center px-1 py-px border border-border rounded-[4px] text-secondary text-[9px] tracking-[0.4px]">
            {activeTranslation?.abbreviation ?? ''}
          </span>
          <SnippetText snippet={result.snippet} text={result.text} selected={selected} />
        </button>
        <div data-row-actions className="flex items-center gap-0.5">
          <button
            type="button"
            className={ROW_ACTION}
            disabled={!canAct}
            onClick={(e) => { e.stopPropagation(); void perform('preview', result); }}
            aria-label={`Preview ${target.label}`}
            title={`Preview ${target.label}`}
          >
            <Icon name="preview" size={13} /><span>Preview</span>
          </button>
          <button
            type="button"
            className={ROW_ACTION}
            disabled={!canAct}
            onClick={(e) => { e.stopPropagation(); void perform('live', result); }}
            aria-label={`Send ${target.label} live`}
            title={`Send ${target.label} live`}
          >
            <Icon name="display" size={13} /><span>Send Live</span>
          </button>
          <button
            type="button"
            className={ROW_ACTION}
            disabled={!canAct}
            onClick={(e) => { e.stopPropagation(); void perform('add', result); }}
            aria-label={`Add ${target.label} to the service`}
            title={`Add ${target.label} to the service`}
          >
            <Icon name="plus" size={13} /><span>Add to Service</span>
          </button>
        </div>
      </li>
    );
  }

  function renderSearchResults() {
    if (translationError) {
      return (
        <div className={STATUS_PANEL} role="status">
          <Icon name="bible" size={30} />
          <strong>Bible search unavailable</strong>
          <p>{translationError}</p>
        </div>
      );
    }
    if (loadingTranslations) {
      return (
        <div className={STATUS_PANEL} role="status">
          <strong>Loading translations…</strong>
          <p>Reading installed scripture packages.</p>
        </div>
      );
    }
    if (translations.length === 0) {
      return (
        <div className={STATUS_PANEL} role="status">
          <Icon name="bible" size={30} />
          <strong>No translations installed</strong>
          <p>Install a Bible translation package to search scripture.</p>
        </div>
      );
    }
    if (search.kind === 'error') {
      return (
        <div className={`${STATUS_PANEL} border-[#5A2B35]! [&_strong]:text-error!`} role="alert">
          <strong>Search failed</strong>
          <p>{search.message}</p>
        </div>
      );
    }
    if (pending) {
      return (
        <div className={STATUS_PANEL} role="status">
          <strong>Searching {activeTranslation?.abbreviation ?? ''}…</strong>
          <p>Reading the local translation package.</p>
        </div>
      );
    }
    if (search.kind === 'results') {
      const { results, hasMore } = search;
      if (results.length === 0) {
        return (
          <div className={STATUS_PANEL} role="status">
            <strong>No verses found</strong>
            <p>
              Nothing in {activeTranslation?.abbreviation ?? 'this translation'} matches "
              {trimmedQuery}". Try another reference or fewer words.
            </p>
          </div>
        );
      }
      return (
        <>
          <ul className="list-none grid gap-1 m-0 p-0" aria-label="Search results">
            {results.map((r) => renderVerseRow(r))}
          </ul>
          {hasMore && (
            <div className="flex justify-center pt-2.5 pb-1">
              <button
                type="button"
                className="min-h-8 inline-flex items-center justify-center gap-1.5 px-3 py-1.5 border border-[#35445C] rounded-md bg-elevated text-[11px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950] min-w-[160px]"
                disabled={loadingMore}
                onClick={loadMore}
              >
                {loadingMore ? 'Loading…' : 'Load more results'}
              </button>
            </div>
          )}
        </>
      );
    }
    // No query is active: show the reading pane instead of a poster, so
    // scripture (Genesis 1 from the top) is on screen before anyone types.
    return renderReadingPane();
  }

  function renderReadingPane() {
    if (browseState.kind === 'idle') {
      return (
        <div className={STATUS_PANEL} role="status">
          <Icon name="bible" size={28} />
          <strong>Opening the Bible…</strong>
          <p>Reading Genesis 1 while the translation loads.</p>
        </div>
      );
    }
    if (browseState.kind === 'loading') {
      return (
        <div className={STATUS_PANEL} role="status">
          <strong>Loading chapter…</strong>
        </div>
      );
    }
    if (browseState.kind === 'error') {
      return (
        <div className={`${STATUS_PANEL} border-[#5A2B35]! [&_strong]:text-error!`} role="alert">
          <strong>Could not load chapter</strong>
          <p>{browseState.message}</p>
        </div>
      );
    }
    return (
      <>
        {browseState.chapters.map((block) => (
          <section key={`${block.bookId}-${block.chapter}`} className="grid gap-1.5">
            <div className="flex items-center gap-2 px-1.5 pt-1 pb-2 text-[12px] font-semibold text-text">
              <strong>
                {books.find((b) => b.id === block.bookId)?.name} {block.chapter}
              </strong>
              <span className={BADGE}>{activeTranslation?.abbreviation ?? ''}</span>
            </div>
            <ul className="list-none grid gap-1 m-0 p-0" aria-label="Chapter verses">
              {block.results.map((r) => renderVerseRow(r, false))}
            </ul>
          </section>
        ))}
        {browseMore && (
          <p className={STATUS_PANEL} role="status">Loading the next chapter…</p>
        )}
      </>
    );
  }

  function renderBrowse() {
    if (!bridgeAvailable || translations.length === 0) {
      return (
        <div className={STATUS_PANEL} role="status">
          <Icon name="bible" size={30} />
          <strong>No translations installed</strong>
          <p>Install a Bible translation package to browse chapters.</p>
        </div>
      );
    }
    return renderReadingPane();
  }


  const resultCount = search.kind === 'results' ? search.results.length : null;


  return (
    <section className="bg-panel border border-border rounded-[10px] grid grid-rows-[auto_minmax(0,1fr)] overflow-hidden flex-1 min-h-0" aria-label="Bible" onKeyDown={handleWorkspaceKeyDown}>
      {/* The search area is pinned: results and chapters scroll in the pane
          beneath it, so nothing pushes the field out of reach. */}
      <div className="grid gap-2.5 px-3 pt-3 pb-2.5 content-start max-[959px]:px-2.5 max-[959px]:pt-2.5 max-[959px]:pb-2">
        {/* Always visible in both modes: the leading icon names the current
            mode and toggles to the other; the translation selector sits where a
            submit button would have been, since typing is enough. */}
        <form className="flex items-stretch gap-2.5 max-[959px]:gap-2" onSubmit={handleSubmit} role="search">
          <div className={SEARCH_FIELD}>
            <button
              type="button"
              className="grid place-items-center w-6 h-6 shrink-0 border-0 rounded-[4px] bg-transparent text-secondary hover:bg-elevated hover:text-text"
              onClick={() => {
                if (activeTab === 'search') {
                  // Entering browse drops a word query, which is not browse
                  // input, and keeps a typed reference.
                  if (!acceptsBrowseInput(query, books)) setQuery('');
                  setActiveTab('browse');
                } else {
                  setActiveTab('search');
                }
                // The operator stays in the field while switching panes.
                inputRef.current?.focus();
              }}
              aria-label={
                activeTab === 'search'
                  ? 'Search mode — switch to browse'
                  : 'Browse mode — switch to search'
              }
              title={
                activeTab === 'search'
                  ? 'Search mode — click to browse chapters'
                  : 'Browse mode — click to search the Bible'
              }
            >
              {/* The icon names the mode you are in, matching the placeholder
                  and what typing does. */}
              <Icon name={activeTab === 'search' ? 'search' : 'bible'} size={15} />
            </button>
            {/* The input and its ghost share this box: an invisible copy of
                the typed text pushes the muted completion to start exactly
                where the caret sits, so the book name completes inline. */}
            <div className="relative flex-1 min-w-0 flex items-center">
              <input
                id="bible-search-input"
                ref={inputRef}
                type="text"
                value={query}
                onChange={(event) => {
                  const next = event.target.value;
                  // Browse only accepts text building toward a book name; cutting
                  // back is safe because every prefix of acceptable input is too.
                  if (activeTab === 'browse' && !acceptsBrowseInput(next, books)) {
                    setAction({ busy: null, status: { kind: 'message', text: BROWSE_INPUT_HINT } });
                    return;
                  }
                  setQuery(next);
                }}
                onKeyDown={handleSearchKeyDown}
                onFocus={() => setFieldFocused(true)}
                onBlur={() => setFieldFocused(false)}
                aria-label="Search the Bible"
                placeholder={
                  activeTab === 'browse'
                    ? 'Jump to a reference, e.g. John 3:16'
                    : 'Search references or words, e.g. John 3:16 or living water'
                }
                autoComplete="off"
                spellCheck={false}
                disabled={Boolean(translationError) || loadingTranslations}
                className={SEARCH_INPUT + ' relative z-10'}
                role="combobox"
                aria-autocomplete="inline"
                aria-expanded={false}
                aria-controls="bible-search-ghost"
              />
              {ghost !== '' && (
                <span
                  id="bible-search-ghost"
                  aria-hidden="true"
                  className="absolute inset-0 z-0 flex items-center pointer-events-none whitespace-pre text-[12px] text-[#6F809B]"
                >
                  <span className="invisible">{query}</span>
                  {ghost}
                </span>
              )}
            </div>
            {query !== '' && (
              <button
                type="button"
                className="grid place-items-center w-6 h-6 shrink-0 border-0 rounded-[4px] bg-transparent text-secondary hover:bg-elevated hover:text-text"
                onClick={clearSearch}
                aria-label="Clear search"
                title="Clear search"
              >
                <Icon name="close" size={14} />
              </button>
            )}
          </div>
          <select
            className="min-h-8 shrink-0 px-2 border border-[#35445C] rounded-md bg-elevated text-text text-[11px] disabled:text-secondary"
            aria-label="Bible translation"
            value={translationId}
            onChange={(event) => setTranslationId(event.target.value)}
            disabled={translations.length === 0}
          >
            {translations.length === 0 && <option value="">No translations</option>}
            {translations.map((t) => (
              <option key={t.id} value={t.id}>
                {t.abbreviation}
              </option>
            ))}
          </select>
        </form>
        {activeTab === 'search' && (
          <div className="flex items-center gap-2 flex-wrap">
              <select
                className={FILTER}
                aria-label="Filter by testament"
                value={testament}
                onChange={(e) => {
                  setTestament(e.target.value);
                  setBookFilter(0); // reset book filter when testament changes
                }}
                disabled={books.length === 0}
              >
                <option value="">All Testaments</option>
                <option value="old">Old Testament</option>
                <option value="new">New Testament</option>
              </select>

              <select
                className={FILTER}
                aria-label="Filter by book"
                value={bookFilter}
                onChange={(e) => setBookFilter(Number(e.target.value))}
                disabled={books.length === 0}
              >
                <option value={0}>All Books</option>
                {filteredBooks.map((b) => (
                  <option key={b.id} value={b.id}>
                    {b.name}
                  </option>
                ))}
              </select>

              <select
                className={FILTER}
                aria-label="Search mode"
                value={mode}
                onChange={(event) => setMode(event.target.value as Mode)}
              >
                {modes.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>

              <span className="text-secondary text-[10px] ml-0.5" aria-live="polite">
                {pending
                  ? 'Searching…'
                  : resultCount !== null
                    ? `${resultCount} result${resultCount === 1 ? '' : 's'}`
                    : ''}
              </span>
          </div>
        )}
        {/* Errors surface in the pinned header so a failed projection stays
            visible without covering the verses below. */}
        {action.status?.kind === 'error' && (
          <p className="text-error text-[10px] leading-snug" role="alert">
            {action.status.text}
          </p>
        )}
      </div>

      {/* Results and chapters scroll in their own pane, so a long reading
          never moves the field. */}
      <div ref={paneRef} className="grid px-3 pb-3 min-h-0 overflow-y-auto content-start max-[959px]:px-2.5 max-[959px]:pb-2.5" onScroll={handleBrowseScroll}>
        {activeTab === 'search' && (
          <div aria-busy={pending}>
            {renderSearchResults()}
          </div>
        )}

        {activeTab === 'browse' && renderBrowse()}
      </div>
    </section>
  );
}
