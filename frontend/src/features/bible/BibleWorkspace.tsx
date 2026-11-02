import { useEffect, useRef, useState } from 'react';
import type { FormEvent, MouseEvent as ReactMouseEvent } from 'react';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import { App } from '../../../bindings/zowe-worship';
import * as models from '../../../bindings/zowe-worship/models';
import './bible.css';

const modes = [
  { value: 'auto', label: 'References + text' },
  { value: 'words', label: 'Full text' },
  { value: 'phrase', label: 'Exact phrase' },
  { value: 'prefix', label: 'Word prefix' },
] as const;

type Mode = (typeof modes)[number]['value'];

type SearchState =
  | { kind: 'idle' }
  | { kind: 'error'; message: string }
  | { kind: 'results'; response: models.BibleSearchResponse };

const SEARCH_DEBOUNCE_MS = 250;
const MIN_QUERY_LENGTH = 2;

type ActionState = {
  busy: string | null;
  status: { kind: 'message' | 'error'; text: string } | null;
};

function fromActionArea(event: ReactMouseEvent<HTMLElement>): boolean {
  return (event.target as HTMLElement).closest('.bible-result-actions') !== null;
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

export function BibleWorkspace() {
  const studio = useStudio();
  const bridgeAvailable = studio.bridgeAvailable;
  const [translations, setTranslations] = useState<models.TranslationView[]>([]);
  const [translationId, setTranslationId] = useState('');
  const [translationError, setTranslationError] = useState<string | null>(
    bridgeAvailable ? null : 'Open the Zowe Worship desktop app to search installed translations.',
  );
  const [loadingTranslations, setLoadingTranslations] = useState(bridgeAvailable);
  const [query, setQuery] = useState('');
  const [mode, setMode] = useState<Mode>('auto');
  const [search, setSearch] = useState<SearchState>({ kind: 'idle' });
  const [pending, setPending] = useState(false);
  const [selectedReference, setSelectedReference] = useState<string | null>(null);
  const [action, setAction] = useState<ActionState>({ busy: null, status: null });
  const requestId = useRef(0);
  const debounceRef = useRef<number | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!bridgeAvailable) return;

    let active = true;
    Promise.all([
      App.ListTranslations(),
      App.GetSetting('default_translation_id').catch(() => ''),
    ])
      .then(([items, defaultId]) => {
        if (!active) return;
        const installed = items ?? [];
        setTranslations(installed);
        const preferred =
          (defaultId ? installed.find((item) => item.id === defaultId) : undefined) ??
          installed.find((item) => item.id === 'kjv') ??
          installed[0];
        setTranslationId(preferred?.id ?? '');
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

  const activeTranslation = translations.find((item) => item.id === translationId);
  const trimmedQuery = query.trim();
  const canSearch = bridgeAvailable && translationId !== '' && trimmedQuery.length >= MIN_QUERY_LENGTH;
  const canAct = bridgeAvailable && translationId !== '' && action.busy === null;

  function cancelDebounce() {
    if (debounceRef.current !== null) {
      window.clearTimeout(debounceRef.current);
      debounceRef.current = null;
    }
  }

  async function executeSearch(text: string, targetTranslation: string) {
    const currentRequest = ++requestId.current;
    setPending(true);

    try {
      const response = await App.SearchBible({
        translationId: targetTranslation,
        query: text,
        mode,
        limit: 50,
      });
      if (currentRequest !== requestId.current) return;
      setSearch({ kind: 'results', response });
      setSelectedReference(response.results?.[0]?.reference ?? null);
    } catch (error: unknown) {
      if (currentRequest !== requestId.current) return;
      setSearch({ kind: 'error', message: errorMessage(error) });
    } finally {
      if (currentRequest === requestId.current) setPending(false);
    }
  }

  useEffect(() => {
    if (!bridgeAvailable || translationId === '') return;

    cancelDebounce();

    if (trimmedQuery === '') {
      requestId.current += 1;
      setPending(false);
      setSearch({ kind: 'idle' });
      setSelectedReference(null);
      return;
    }
    if (trimmedQuery.length < MIN_QUERY_LENGTH) return;

    setAction({ busy: null, status: null });
    debounceRef.current = window.setTimeout(() => {
      debounceRef.current = null;
      void executeSearch(trimmedQuery, translationId);
    }, SEARCH_DEBOUNCE_MS);
  }, [trimmedQuery, mode, translationId, bridgeAvailable]);

  useEffect(() => cancelDebounce, []);

  function searchNow() {
    cancelDebounce();
    if (!canSearch) return;
    setAction({ busy: null, status: null });
    void executeSearch(trimmedQuery, translationId);
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    searchNow();
  }

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
        status: { kind: 'message', text: `Added ${result.reference} to ${order.service?.name ?? 'the service'}.` },
      });
    } catch (error: unknown) {
      setAction({ busy: null, status: { kind: 'error', text: errorMessage(error) } });
    }
  }

  function clearSearch() {
    cancelDebounce();
    requestId.current += 1;
    setPending(false);
    setQuery('');
    setSearch({ kind: 'idle' });
    setSelectedReference(null);
    inputRef.current?.focus();
  }

  function renderResults() {
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

    if (search.kind === 'results') {
        const results = search.response.results ?? [];

        if (results.length === 0 && pending) {
          return (
            <div className="bible-message" role="status">
              <strong>Searching {activeTranslation?.abbreviation ?? ''}…</strong>
              <p>Reading the local translation package.</p>
            </div>
          );
        }

        if (results.length === 0) {
          return (
            <div className="bible-message" role="status">
              <strong>No verses found</strong>
              <p>Nothing in {activeTranslation?.abbreviation ?? 'this translation'} matches “{trimmedQuery}”. Try another reference or fewer words.</p>
            </div>
          );
        }

        return (
          <ul className="bible-results" aria-label="Search results">
            {results.map((result) => {
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
                    <span className="bible-reference">{result.reference}</span>
                    <span className="bible-badge">{activeTranslation?.abbreviation ?? ''}</span>
                    <span className="bible-text">{result.text}</span>
                  </button>
                  <div className="bible-result-actions">
                    <button
                      type="button"
                      className="bible-action"
                      disabled={!canAct}
                      onClick={(event) => {
                        event.stopPropagation();
                        void perform('preview', result);
                      }}
                      aria-label={`Preview ${result.reference}`}
                      title={`Preview ${result.reference}`}
                    >
                      <Icon name="preview" size={13} /><span>Preview</span>
                    </button>
                    <button
                      type="button"
                      className="bible-action"
                      disabled={!canAct}
                      onClick={(event) => {
                        event.stopPropagation();
                        void perform('live', result);
                      }}
                      aria-label={`Send ${result.reference} live`}
                      title={`Send ${result.reference} live`}
                    >
                      <Icon name="display" size={13} /><span>Send Live</span>
                    </button>
                    <button
                      type="button"
                      className="bible-action"
                      disabled={!canAct}
                      onClick={(event) => {
                        event.stopPropagation();
                        void perform('add', result);
                      }}
                      aria-label={`Add ${result.reference} to the service`}
                      title={`Add ${result.reference} to the service`}
                    >
                      <Icon name="plus" size={13} /><span>Add to Service</span>
                    </button>
                    <button type="button" className="bible-action bible-action--icon" disabled title="More actions are not implemented." aria-label={`More actions for ${result.reference}`}>
                      <Icon name="more" size={14} />
                    </button>
                  </div>
                </li>
              );
            })}
          </ul>
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

    return (
      <div className="bible-message" role="status">
        <Icon name="search" size={30} />
        <strong>Search the Bible</strong>
        <p>Start typing a reference like John 3:16, a chapter like Psalm 23, or words like living water.</p>
      </div>
    );
  }

  const resultCount = search.kind === 'results' ? (search.response.results?.length ?? 0) : null;

  return (
    <section className="panel bible-workspace" aria-labelledby="bible-workspace-title">
      <header className="bible-header">
        <div className="bible-heading">
          <Icon name="bible" size={18} />
          <h2 id="bible-workspace-title">Bible</h2>
        </div>

        <div className="bible-tabs" role="tablist" aria-label="Bible views">
          <button type="button" role="tab" aria-selected="true" className="bible-tab bible-tab--active">
            <Icon name="search" size={14} />Search
          </button>
          <button type="button" role="tab" aria-selected="false" className="bible-tab" disabled title="Browsing arrives with the Bible browser.">
            <Icon name="bible" size={14} />Browse
          </button>
          <button type="button" role="tab" aria-selected="false" className="bible-tab" disabled title="Cross references are not implemented.">
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
            {translations.map((translation) => (
              <option key={translation.id} value={translation.id}>{translation.abbreviation}</option>
            ))}
          </select>
          <button type="button" className="button" disabled title="Translation management is not implemented.">Translation</button>
        </div>
      </header>

      <div className="bible-body">
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
              <button type="button" className="bible-search-clear" onClick={clearSearch} aria-label="Clear search" title="Clear search">
                <Icon name="close" size={14} />
              </button>
            )}
          </div>
          <button type="submit" className="button button--primary bible-submit" disabled={!canSearch}>
            Search
          </button>
        </form>

        <div className="bible-filters">
          <select className="bible-filter" aria-label="Filter by book" disabled title="Book filtering arrives with the Bible browser.">
            <option>All Books</option>
          </select>
          <select className="bible-filter" aria-label="Filter by testament" disabled title="Testament filtering arrives with the Bible browser.">
            <option>All Testaments</option>
          </select>
          <select
            className="bible-filter"
            aria-label="Search mode"
            value={mode}
            onChange={(event) => setMode(event.target.value as Mode)}
          >
            {modes.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>
          <span className="bible-result-count" aria-live="polite">
            {pending
              ? 'Searching…'
              : resultCount !== null ? `${resultCount} ${resultCount === 1 ? 'result' : 'results'}` : ''}
          </span>
        </div>

        <div className="bible-results-region" aria-busy={pending}>
          {renderResults()}
        </div>

        <p className={`bible-note${action.status?.kind === 'error' ? ' bible-note--error' : ''}`} aria-live="polite">
          {action.status?.text ?? 'Preview and Send Live update the production panes. Add to Service fills the schedule.'}
        </p>
      </div>
    </section>
  );
}
