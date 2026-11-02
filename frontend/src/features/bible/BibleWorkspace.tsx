import { useEffect, useRef, useState, useCallback } from 'react';
import type { FormEvent, MouseEvent as ReactMouseEvent } from 'react';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import { App } from '../../../bindings/zowe-worship';
import * as models from '../../../bindings/zowe-worship/models';
import './bible.css';

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

function fromActionArea(event: ReactMouseEvent<HTMLElement>): boolean {
  return (event.target as HTMLElement).closest('.bible-result-actions') !== null;
}

/** Render a snippet that may contain <mark> tags safely. */
function SnippetText({ snippet, text }: { snippet?: string; text: string }) {
  if (snippet) {
    // snippet uses <mark> tags from FTS5; we trust the backend to only emit those.
    const safe = snippet.replace(/<(?!\/?mark>)[^>]+>/gi, '');
    return <span className="bible-text" dangerouslySetInnerHTML={{ __html: safe }} />;
  }
  return <span className="bible-text">{text}</span>;
}

// ─── types ───────────────────────────────────────────────────────────────────

type SearchState =
  | { kind: 'idle' }
  | { kind: 'error'; message: string }
  | { kind: 'results'; results: models.BibleVerse[]; hasMore: boolean };

type BrowseState =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'error'; message: string }
  | { kind: 'results'; results: models.BibleVerse[]; bookId: number; chapter: number };

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
  const [activeTab, setActiveTab] = useState<ActiveTab>('search');

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

  const requestId = useRef(0);
  const debounceRef = useRef<number | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // — browse state ——————————————————————————————————————————————
  const [browseBookId, setBrowseBookId] = useState(1);
  const [browseChapter, setBrowseChapter] = useState(1);
  const [browseState, setBrowseState] = useState<BrowseState>({ kind: 'idle' });

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

  // Books filtered by current testament selection (for "All Books" dropdown)
  const filteredBooks =
    testament === '' ? books : books.filter((b) => b.testament === testament);

  // Chapter count for currently selected browse book
  const browseBook = books.find((b) => b.id === browseBookId);
  const browseChapters = browseBook?.chapters ?? 1;

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

  async function loadChapter(bookId: number, chapter: number) {
    if (!bridgeAvailable || translationId === '') return;
    setBrowseState({ kind: 'loading' });
    try {
      const response = await App.BrowsePassage({
        translationId,
        bookId,
        chapter,
        verseStart: 0,
        verseEnd: 0,
      });
      setBrowseState({
        kind: 'results',
        results: response.results ?? [],
        bookId,
        chapter,
      });
    } catch (error: unknown) {
      setBrowseState({ kind: 'error', message: errorMessage(error) });
    }
  }

  function handleBrowseBookChange(id: number) {
    setBrowseBookId(id);
    setBrowseChapter(1);
    void loadChapter(id, 1);
  }

  function handleBrowseChapterChange(chapter: number) {
    setBrowseChapter(chapter);
    void loadChapter(browseBookId, chapter);
  }

  // ─── renderers ────────────────────────────────────────────────────────────

  function renderVerseRow(result: models.BibleVerse, showRef = true) {
    const selected = selectedReference === result.reference;
    return (
      <li
        key={`${result.bookId}-${result.chapter}-${result.verse}`}
        className={`bible-result${selected ? ' bible-result--selected' : ''}`}
        title={`Click to preview ${result.reference} · double-click to send live`}
        onClick={(event) => {
          if (fromActionArea(event) || !canAct) return;
          void perform('preview', result);
        }}
        onDoubleClick={(event) => {
          if (fromActionArea(event) || !canAct) return;
          void perform('live', result);
        }}
      >
        <button type="button" className="bible-result-main" aria-pressed={selected}>
          {showRef && <span className="bible-reference">{result.reference}</span>}
          {!showRef && (
            <span className="bible-reference">
              <span className="bible-verse-num">{result.verse}</span>
            </span>
          )}
          <span className="bible-badge">{activeTranslation?.abbreviation ?? ''}</span>
          <SnippetText snippet={result.snippet} text={result.text} />
        </button>
        <div className="bible-result-actions">
          <button
            type="button"
            className="bible-action"
            disabled={!canAct}
            onClick={(e) => { e.stopPropagation(); void perform('preview', result); }}
            aria-label={`Preview ${result.reference}`}
            title={`Preview ${result.reference}`}
          >
            <Icon name="preview" size={13} /><span>Preview</span>
          </button>
          <button
            type="button"
            className="bible-action"
            disabled={!canAct}
            onClick={(e) => { e.stopPropagation(); void perform('live', result); }}
            aria-label={`Send ${result.reference} live`}
            title={`Send ${result.reference} live`}
          >
            <Icon name="display" size={13} /><span>Send Live</span>
          </button>
          <button
            type="button"
            className="bible-action"
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
        <div className="bible-message" role="status">
          <Icon name="bible" size={30} />
          <strong>Bible search unavailable</strong>
          <p>{translationError}</p>
        </div>
      );
    }
    if (loadingTranslations) {
      return (
        <div className="bible-message" role="status">
          <strong>Loading translations…</strong>
          <p>Reading installed scripture packages.</p>
        </div>
      );
    }
    if (translations.length === 0) {
      return (
        <div className="bible-message" role="status">
          <Icon name="bible" size={30} />
          <strong>No translations installed</strong>
          <p>Install a Bible translation package to search scripture.</p>
        </div>
      );
    }
    if (search.kind === 'error') {
      return (
        <div className="bible-message bible-message--error" role="alert">
          <strong>Search failed</strong>
          <p>{search.message}</p>
        </div>
      );
    }
    if (pending) {
      return (
        <div className="bible-message" role="status">
          <strong>Searching {activeTranslation?.abbreviation ?? ''}…</strong>
          <p>Reading the local translation package.</p>
        </div>
      );
    }
    if (search.kind === 'results') {
      const { results, hasMore } = search;
      if (results.length === 0) {
        return (
          <div className="bible-message" role="status">
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
          <ul className="bible-results" aria-label="Search results">
            {results.map((r) => renderVerseRow(r))}
          </ul>
          {hasMore && (
            <div className="bible-load-more">
              <button
                type="button"
                className="button"
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
      <div className="bible-message" role="status">
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
        <div className="bible-message" role="status">
          <Icon name="bible" size={30} />
          <strong>No translations installed</strong>
          <p>Install a Bible translation package to browse chapters.</p>
        </div>
      );
    }
    return (
      <div className="bible-browse">
        {/* Book + chapter selectors */}
        <div className="bible-browse-nav">
          <select
            className="bible-filter bible-filter--grow"
            aria-label="Select book"
            value={browseBookId}
            onChange={(e) => handleBrowseBookChange(Number(e.target.value))}
          >
            {books.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
          <select
            className="bible-filter"
            aria-label="Select chapter"
            value={browseChapter}
            onChange={(e) => handleBrowseChapterChange(Number(e.target.value))}
          >
            {Array.from({ length: browseChapters }, (_, i) => i + 1).map((ch) => (
              <option key={ch} value={ch}>
                Ch {ch}
              </option>
            ))}
          </select>
          <button
            type="button"
            className="button button--primary"
            disabled={!bridgeAvailable || translationId === ''}
            onClick={() => void loadChapter(browseBookId, browseChapter)}
            aria-label="Load chapter"
          >
            Go
          </button>
        </div>

        {/* Browse results */}
        <div className="bible-results-region">
          {browseState.kind === 'idle' && (
            <div className="bible-message" role="status">
              <Icon name="bible" size={28} />
              <strong>Select a chapter to read</strong>
              <p>Choose a book and chapter above, then press Go.</p>
            </div>
          )}
          {browseState.kind === 'loading' && (
            <div className="bible-message" role="status">
              <strong>Loading chapter…</strong>
            </div>
          )}
          {browseState.kind === 'error' && (
            <div className="bible-message bible-message--error" role="alert">
              <strong>Could not load chapter</strong>
              <p>{browseState.message}</p>
            </div>
          )}
          {browseState.kind === 'results' && (
            <>
              <div className="bible-browse-heading">
                <strong>
                  {books.find((b) => b.id === browseState.bookId)?.name} {browseState.chapter}
                </strong>
                <span className="bible-badge">{activeTranslation?.abbreviation ?? ''}</span>
              </div>
              <ul className="bible-results bible-results--chapter" aria-label="Chapter verses">
                {browseState.results.map((r) => renderVerseRow(r, false))}
              </ul>
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
    <section className="panel bible-workspace" aria-labelledby="bible-workspace-title">
      <header className="bible-header">
        <div className="bible-heading">
          <Icon name="bible" size={18} />
          <h2 id="bible-workspace-title">Bible</h2>
        </div>

        <div className="bible-tabs" role="tablist" aria-label="Bible views">
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'search'}
            className={`bible-tab${activeTab === 'search' ? ' bible-tab--active' : ''}`}
            onClick={() => setActiveTab('search')}
          >
            <Icon name="search" size={14} />Search
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'browse'}
            className={`bible-tab${activeTab === 'browse' ? ' bible-tab--active' : ''}`}
            onClick={() => setActiveTab('browse')}
          >
            <Icon name="bible" size={14} />Browse
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={false}
            className="bible-tab"
            disabled
            title="Cross references are not implemented."
          >
            <Icon name="link" size={14} />Cross References
          </button>
        </div>

        <div className="bible-header-actions">
          <select
            className="bible-translation"
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
        </div>
      </header>

      <div className="bible-body">
        {/* ── SEARCH TAB ────────────────────────────────────────────── */}
        {activeTab === 'search' && (
          <>
            <form className="bible-search" onSubmit={handleSubmit} role="search">
              <div className="bible-search-field">
                <Icon name="search" size={16} />
                <input
                  id="bible-search-input"
                  ref={inputRef}
                  type="text"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  aria-label="Search the Bible"
                  placeholder="Search references or words, e.g. John 3:16 or living water"
                  autoComplete="off"
                  spellCheck={false}
                  disabled={Boolean(translationError) || loadingTranslations}
                />
                {query !== '' && (
                  <button
                    type="button"
                    className="bible-search-clear"
                    onClick={clearSearch}
                    aria-label="Clear search"
                    title="Clear search"
                  >
                    <Icon name="close" size={14} />
                  </button>
                )}
              </div>
              <button
                type="submit"
                className="button button--primary bible-submit"
                disabled={!canSearch}
              >
                Search
              </button>
            </form>

            <div className="bible-filters">
              {/* Testament filter */}
              <select
                className="bible-filter"
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
                className="bible-filter"
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
                className="bible-filter"
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

              <span className="bible-result-count" aria-live="polite">
                {pending
                  ? 'Searching…'
                  : resultCount !== null
                    ? `${resultCount} result${resultCount === 1 ? '' : 's'}`
                    : ''}
              </span>
            </div>

            <div className="bible-results-region" aria-busy={pending}>
              {renderSearchResults()}
            </div>
          </>
        )}

        {/* ── BROWSE TAB ────────────────────────────────────────────── */}
        {activeTab === 'browse' && renderBrowse()}

        <p
          className={`bible-note${action.status?.kind === 'error' ? ' bible-note--error' : ''}`}
          aria-live="polite"
        >
          {action.status?.text ??
            'Preview and Send Live update the production panes. Add to Service fills the schedule.'}
        </p>
      </div>
    </section>
  );
}
