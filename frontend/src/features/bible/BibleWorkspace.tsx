import { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import type { FormEvent, KeyboardEvent, MouseEvent as ReactMouseEvent } from 'react';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import { App } from '../../../bindings/zowe-worship';
import * as models from '../../../bindings/zowe-worship/models';
import { bookPortion, inlineBookCompletion, resolveBook, pendingChapter } from './referenceGuide';

// ─── styles ──────────────────────────────────────────────────────────────────

// Shared with the Songs screen, which reuses the same row/action look. Kept as
// constants because the string repeats on every row; everything else is inline.
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

// ─── constants ───────────────────────────────────────────────────────────────

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

// ─── helpers ─────────────────────────────────────────────────────────────────

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

// ─── types ───────────────────────────────────────────────────────────────────

type SearchState =
  | { kind: 'idle' }
  | { kind: 'error'; message: string }
  | { kind: 'results'; results: models.BibleVerse[]; hasMore: boolean };

// A chapter block is one chapter's verses. Reading straight through the Bible
// means several blocks stacked in order, so the list grows downward instead of
// swapping the chapter out from under the operator's scroll wheel.
type ChapterBlock = { bookId: number; chapter: number; results: models.BibleVerse[] };

type BrowseState =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'error'; message: string }
  | { kind: 'results'; chapters: ChapterBlock[]; bookId: number; chapter: number };

type ActionState = {
  busy: string | null;
  status: { kind: 'message' | 'error'; text: string } | null;
};

// ─── component ───────────────────────────────────────────────────────────────

export function BibleWorkspace() {
  const studio = useStudio();
  const bridgeAvailable = studio.bridgeAvailable;

  // — translations ——————————————————————————————————————————————
  const [translations, setTranslations] = useState<models.TranslationView[]>([]);
  const [translationId, setTranslationId] = useState('');
  const [translationError, setTranslationError] = useState<string | null>(
    bridgeAvailable ? null : 'Open the Zowe Worship desktop app to search installed translations.',
  );
  const [loadingTranslations, setLoadingTranslations] = useState(bridgeAvailable);

  // — books (for filter dropdowns and browse) ———————————————————
  const [books, setBooks] = useState<models.BibleBookView[]>([]);

  // — tabs ——————————————————————————————————————————————————————
  const [activeTab, setActiveTab] = useState<ActiveTab>('browse');

  // — search state ——————————————————————————————————————————————
  const [query, setQuery] = useState('');
  const [mode, setMode] = useState<Mode>('auto');
  const [testament, setTestament] = useState('');   // '' | 'old' | 'new'
  const [bookFilter, setBookFilter] = useState(0);  // 0 = all
  const [search, setSearch] = useState<SearchState>({ kind: 'idle' });
  const [pending, setPending] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const [selectedReference, setSelectedReference] = useState<string | null>(null);
  const [action, setAction] = useState<ActionState>({ busy: null, status: null });

  // — guided reference entry ————————————————————————————————————
  // Whether the field has focus; the inline book-name ghost only shows while
  // focused and while the operator is still typing the book portion in "auto".
  const [fieldFocused, setFieldFocused] = useState(false);

  const requestId = useRef(0);
  const debounceRef = useRef<number | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  // Where the reading pane was before a search moved it, so clearing the search
  // can put it back.
  const browseReturn = useRef<{ bookId: number; chapter: number } | null>(null);

  // — browse state ——————————————————————————————————————————————
  const [browseBookId, setBrowseBookId] = useState(1);
  const [browseChapter, setBrowseChapter] = useState(1);
  const [browseState, setBrowseState] = useState<BrowseState>({ kind: 'idle' });
  const [browseMore, setBrowseMore] = useState(false);

  // ─── load translations + books on mount ──────────────────────────────────

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

  // ─── derived ─────────────────────────────────────────────────────────────

  const activeTranslation = translations.find((t) => t.id === translationId);
  const trimmedQuery = query.trim();
  const canSearch =
    bridgeAvailable && translationId !== '' && trimmedQuery.length >= MIN_QUERY_LENGTH;
  const canAct = bridgeAvailable && translationId !== '' && action.busy === null;
  // Going live must not wait for an in-flight action to settle: the first click
  // of a double-click starts a preview (which sets busy), and Enter can arrive
  // while a previous send is still resolving. Only the bridge gate applies.
  const canGoLive = bridgeAvailable && translationId !== '';

  // Books filtered by current testament selection (for "All Books" dropdown)
  const filteredBooks =
    testament === '' ? books : books.filter((b) => b.testament === testament);

  // — guided reference entry ————————————————————————————————————————————————
  // The operator is still naming a book while the text after the book portion
  // holds no chapter digit or colon and the mode resolves references ("auto").
  // Keying off the chapter position (not "any digit") lets a leading book number
  // like the "1" in "1 John" stay part of the book stage. Once a whole book name
  // is committed (it resolves and the field ends in the space we add on accept),
  // there is nothing left to complete.
  const bookText = bookPortion(query);
  const bookCommitted = Boolean(resolveBook(query, books)) && /\s$/.test(query);
  const inBookStage =
    mode === 'auto' &&
    query !== '' &&
    !bookCommitted &&
    !/[0-9:]/.test(query.slice(bookText.length));

  // The one book the typed prefix completes to, and the remainder to ghost after
  // the caret. Only while focused and still in the book stage, and only when a
  // remainder is actually left to show.
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
    // accept only when the caret already sits at the end, so ordinary caret
    // movement through the typed text still behaves normally. Enter accepting
    // here also keeps a half-typed book from sending its partial query's top
    // hit live before the operator has finished the reference.
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

    // Once a search has landed for what is actually typed, Enter sends the top
    // result straight to the audience; double-clicking a row sends that row
    // instead. Guarding on !pending avoids sending the previous query's stale
    // top hit while the current search is still in flight.
    if (canGoLive && !pending && search.kind === 'results' && search.results.length > 0) {
      event.preventDefault();
      void perform('live', search.results[0]);
      return;
    }
    // Otherwise let the form submit and run the search.
  }

  // ─── search helpers ───────────────────────────────────────────────────────

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
        const hasMore = incoming.length === PAGE_SIZE;

        if (replace) {
          setSearch({ kind: 'results', results: incoming, hasMore });
          setSelectedReference(incoming[0]?.reference ?? null);
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

  // Debounced search trigger
  useEffect(() => {
    if (!bridgeAvailable || translationId === '') return;
    cancelDebounce();

    if (trimmedQuery === '') {
      requestId.current += 1;
      setPending(false);
      setSearch({ kind: 'idle' });
      setSelectedReference(null);
      setOffset(0);
      return;
    }
    if (trimmedQuery.length < MIN_QUERY_LENGTH) return;

    setAction({ busy: null, status: null });
    debounceRef.current = window.setTimeout(() => {
      debounceRef.current = null;
      setOffset(0);
      // Typing is a search: leave the reading pane for the results as soon as
      // the query is long enough to run.
      setActiveTab('search');
      void executeSearch(trimmedQuery, translationId, 0, true);
    }, SEARCH_DEBOUNCE_MS);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [trimmedQuery, mode, testament, bookFilter, translationId, bridgeAvailable]);

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

    const back = browseReturn.current;
    if (back) {
      browseReturn.current = null;
      setBrowseBookId(back.bookId);
      setBrowseChapter(back.chapter);
      void loadChapter(back.bookId, back.chapter);
    }
    setSelectedReference(null);
    setOffset(0);
    inputRef.current?.focus();
  }

  // ─── action handler ───────────────────────────────────────────────────────

  async function perform(kind: 'preview' | 'live' | 'add', result: models.BibleVerse) {
    const key = `${kind}:${result.reference}`;
    setAction({ busy: key, status: null });

    const input = {
      translationId,
      bookId: result.bookId,
      chapter: result.chapter,
      verseStart: result.verse,
      verseEnd: result.verse,
    };

    try {
      if (kind === 'preview') {
        await studio.previewScripture(input);
        setSelectedReference(result.reference);
        setAction({ busy: null, status: { kind: 'message', text: `Previewing ${result.reference}.` } });
        return;
      }
      if (kind === 'live') {
        await studio.projectScripture(input);
        setSelectedReference(result.reference);
        setAction({ busy: null, status: { kind: 'message', text: `${result.reference} is live.` } });
        return;
      }
      const order = await studio.addScripture(input);
      setAction({
        busy: null,
        status: {
          kind: 'message',
          text: `Added ${result.reference} to ${order.service?.name ?? 'the service'}.`,
        },
      });
    } catch (error: unknown) {
      setAction({ busy: null, status: { kind: 'error', text: errorMessage(error) } });
    }
  }

  // ─── browse ───────────────────────────────────────────────────────────────

  async function loadChapter(bookId: number, chapter: number, append = false) {
    if (!bridgeAvailable || translationId === '') return;
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
      const block: ChapterBlock = { bookId, chapter, results: response.results ?? [] };
      setBrowseState((current) => (
        append && current.kind === 'results'
          ? { ...current, chapters: [...current.chapters, block] }
          : { kind: 'results', chapters: [block], bookId, chapter }
      ));
    } catch (error: unknown) {
      // A failed top-up leaves the chapters already on screen alone; only a
      // first load can leave the pane with nothing to show.
      if (!append) setBrowseState({ kind: 'error', message: errorMessage(error) });
    } finally {
      setBrowseMore(false);
    }
  }

  // nextChapter is the chapter after this one, rolling into the next book at the
  // end. It stops at the last chapter of the last book, which is the end of the
  // translation rather than a page that has not loaded yet.
  function nextChapter(bookId: number, chapter: number): { bookId: number; chapter: number } | null {
    const book = books.find((candidate) => candidate.id === bookId);
    const chapters = book?.chapters ?? 1;
    if (chapter < chapters) return { bookId, chapter: chapter + 1 };

    const index = books.findIndex((candidate) => candidate.id === bookId);
    const following = books[index + 1];
    if (!following) return null;
    return { bookId: following.id, chapter: 1 };
  }

  function handleBrowseScroll(event: React.UIEvent<HTMLDivElement>) {
    const box = event.currentTarget;
    if (activeTab !== 'browse' || browseMore || browseState.kind !== 'results') return;
    if (box.scrollHeight - box.scrollTop - box.clientHeight > 260) return;

    const last = browseState.chapters[browseState.chapters.length - 1];
    const next = nextChapter(last.bookId, last.chapter);
    if (!next) return;

    setBrowseBookId(next.bookId);
    setBrowseChapter(next.chapter);
    void loadChapter(next.bookId, next.chapter, true);
  }

  // The Bible tab never opens on an empty pane. Genesis 1 is what a person
  // expects to find there, and they can scroll forward from it.
  useEffect(() => {
    if (!bridgeAvailable || translationId === '') return;
    setBrowseBookId(1);
    setBrowseChapter(1);
    void loadChapter(1, 1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bridgeAvailable, translationId]);

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

  // ─── renderers ────────────────────────────────────────────────────────────

  function renderVerseRow(result: models.BibleVerse, showRef = true) {
    const selected = selectedReference === result.reference;
    return (
      <li
        key={`${result.bookId}-${result.chapter}-${result.verse}`}
        className={
          'flex items-center gap-2 px-2 py-px border rounded-[7px] cursor-pointer select-none '
          + (selected
            ? 'bg-selected border-[#304C80] shadow-[inset_3px_0_var(--color-blue)]'
            : 'bg-app border-transparent hover:bg-[#0C1523]')
        }
        title={`Click to preview ${result.reference} · double-click to send live`}
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
            'flex-1 min-w-0 grid items-center gap-2 py-[9px] px-1 border-0 bg-transparent text-left text-inherit '
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
            aria-label={`Preview ${result.reference}`}
            title={`Preview ${result.reference}`}
          >
            <Icon name="preview" size={13} /><span>Preview</span>
          </button>
          <button
            type="button"
            className={ROW_ACTION}
            disabled={!canAct}
            onClick={(e) => { e.stopPropagation(); void perform('live', result); }}
            aria-label={`Send ${result.reference} live`}
            title={`Send ${result.reference} live`}
          >
            <Icon name="display" size={13} /><span>Send Live</span>
          </button>
          <button
            type="button"
            className={ROW_ACTION}
            disabled={!canAct}
            onClick={(e) => { e.stopPropagation(); void perform('add', result); }}
            aria-label={`Add ${result.reference} to the service`}
            title={`Add ${result.reference} to the service`}
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
    return (
      <div className={STATUS_PANEL} role="status">
        <Icon name="search" size={30} />
        <strong>Search the Bible</strong>
        <p>
          Start typing a reference like John 3:16, a chapter like Psalm 23, or words like living
          water.
        </p>
      </div>
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
    return (
      <div className="grid gap-3">
        {/* Browse results */}
        <div>
          {browseState.kind === 'idle' && (
            <div className={STATUS_PANEL} role="status">
              <Icon name="bible" size={28} />
              <strong>Start typing to search</strong>
              <p>Search a reference or scroll to read through the Bible.</p>
            </div>
          )}
          {browseState.kind === 'loading' && (
            <div className={STATUS_PANEL} role="status">
              <strong>Loading chapter…</strong>
            </div>
          )}
          {browseState.kind === 'error' && (
            <div className={`${STATUS_PANEL} border-[#5A2B35]! [&_strong]:text-error!`} role="alert">
              <strong>Could not load chapter</strong>
              <p>{browseState.message}</p>
            </div>
          )}
          {browseState.kind === 'results' && (
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
          )}
        </div>
      </div>
    );
  }

  // ─── computed counts ──────────────────────────────────────────────────────

  const resultCount = search.kind === 'results' ? search.results.length : null;

  // ─── JSX ─────────────────────────────────────────────────────────────────

  return (
    <section className="bg-panel border border-border rounded-[10px] grid grid-rows-[minmax(0,1fr)] overflow-hidden" aria-label="Bible">
      <div className="grid gap-2.5 p-3 min-h-0 overflow-y-auto content-start max-[959px]:p-2.5" onScroll={handleBrowseScroll}>
        {/* ── SEARCH FIELD ROW ─────────────────────────────────────────
            Always visible in both modes: the leading icon toggles between the
            results pane and the chapter reading pane, and the translation
            selector sits where a submit button would have been — typing is
            enough, there is no button to hit. */}
        <form className="flex items-stretch gap-2.5 max-[959px]:gap-2" onSubmit={handleSubmit} role="search">
          <div className={SEARCH_FIELD}>
            <button
              type="button"
              className="grid place-items-center w-6 h-6 shrink-0 border-0 rounded-[4px] bg-transparent text-secondary hover:bg-elevated hover:text-text"
              onClick={() => {
                setActiveTab(activeTab === 'search' ? 'browse' : 'search');
                // Hand focus straight back: the operator switches panes without
                // leaving the field they were typing in.
                inputRef.current?.focus();
              }}
              aria-label={activeTab === 'search' ? 'Switch to browse' : 'Switch to search'}
              title={activeTab === 'search' ? 'Browse chapters' : 'Search the Bible'}
            >
              <Icon name={activeTab === 'search' ? 'bible' : 'search'} size={15} />
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
                onChange={(event) => setQuery(event.target.value)}
                onKeyDown={handleSearchKeyDown}
                onFocus={() => setFieldFocused(true)}
                onBlur={() => setFieldFocused(false)}
                aria-label="Search the Bible"
                placeholder="Search references or words, e.g. John 3:16 or living water"
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
          <>
            <div className="flex items-center gap-2 flex-wrap">
              {/* Testament filter */}
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

              {/* Book filter */}
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

              {/* Mode filter */}
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

            <div aria-busy={pending}>
              {renderSearchResults()}
            </div>
          </>
        )}

        {/* ── BROWSE TAB ────────────────────────────────────────────── */}
        {activeTab === 'browse' && renderBrowse()}

        <p
          className={
            'pt-2.5 border-t border-border text-[10px] '
            + (action.status?.kind === 'error' ? 'text-error' : 'text-secondary')
          }
          aria-live="polite"
        >
          {action.status?.text ??
            'Preview and Send Live update the production panes. Add to Service fills the schedule.'}
        </p>
      </div>
    </section>
  );
}
