import { useEffect, useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { Icon } from '../../components/Icon';
import { hasDesktopBridge } from '../../lib/desktop';
import { ListTranslations, SearchBible } from '../../../wailsjs/go/main/App.js';
import { main } from '../../../wailsjs/go/models';
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
  | { kind: 'loading' }
  | { kind: 'error'; message: string }
  | { kind: 'results'; response: main.BibleSearchResponse };

const productionActionsPending = 'This action arrives with the presentation controller.';

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

export function BibleWorkspace() {
  const bridgeAvailable = hasDesktopBridge();
  const [translations, setTranslations] = useState<main.TranslationView[]>([]);
  const [translationId, setTranslationId] = useState('');
  const [translationError, setTranslationError] = useState<string | null>(
    bridgeAvailable ? null : 'Open the Zowe Worship desktop app to search installed translations.',
  );
  const [loadingTranslations, setLoadingTranslations] = useState(bridgeAvailable);
  const [query, setQuery] = useState('');
  const [mode, setMode] = useState<Mode>('auto');
  const [search, setSearch] = useState<SearchState>({ kind: 'idle' });
  const [selectedReference, setSelectedReference] = useState<string | null>(null);
  const requestId = useRef(0);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!bridgeAvailable) return;

    let active = true;
    ListTranslations()
      .then((items) => {
        if (!active) return;
        setTranslations(items);
        const preferred = items.find((item) => item.id === 'kjv') ?? items[0];
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
  const canSearch = bridgeAvailable
    && translationId !== ''
    && query.trim() !== ''
    && search.kind !== 'loading';

  async function runSearch(translationOverride?: string) {
    const text = query.trim();
    const targetTranslation = translationOverride ?? translationId;
    if (!bridgeAvailable || targetTranslation === '' || text === '') return;

    const currentRequest = ++requestId.current;
    setSearch({ kind: 'loading' });
    setSelectedReference(null);

    try {
      const response = await SearchBible(new main.BibleSearchRequest({
        translationId: targetTranslation,
        query: text,
        mode,
        limit: 50,
      }));
      if (currentRequest !== requestId.current) return;
      setSearch({ kind: 'results', response });
      setSelectedReference(response.results[0]?.reference ?? null);
    } catch (error: unknown) {
      if (currentRequest !== requestId.current) return;
      setSearch({ kind: 'error', message: errorMessage(error) });
    }
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void runSearch();
  }

  function handleTranslationChange(nextTranslation: string) {
    setTranslationId(nextTranslation);
    if (query.trim() !== '' && search.kind !== 'loading') {
      void runSearch(nextTranslation);
    }
  }

  function clearSearch() {
    requestId.current += 1;
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

    switch (search.kind) {
      case 'loading':
        return (
          <div className="bible-message" role="status">
            <strong>Searching {activeTranslation?.abbreviation ?? ''}…</strong>
            <p>Reading the local translation package.</p>
          </div>
        );

      case 'error':
        return (
          <div className="bible-message bible-message--error" role="alert">
            <strong>Search failed</strong>
            <p>{search.message}</p>
          </div>
        );

      case 'results': {
        if (search.response.results.length === 0) {
          return (
            <div className="bible-message" role="status">
              <strong>No verses found</strong>
              <p>Nothing in {activeTranslation?.abbreviation ?? 'this translation'} matches “{query.trim()}”. Try another reference or fewer words.</p>
            </div>
          );
        }

        return (
          <ul className="bible-results" aria-label="Search results">
            {search.response.results.map((result) => {
              const selected = selectedReference === result.reference;
              return (
                <li
                  key={`${result.bookId}-${result.chapter}-${result.verse}`}
                  className={`bible-result${selected ? ' bible-result--selected' : ''}`}
                >
                  <button
                    type="button"
                    className="bible-result-main"
                    aria-pressed={selected}
                    onClick={() => setSelectedReference(result.reference)}
                  >
                    <span className="bible-reference">{result.reference}</span>
                    <span className="bible-badge">{activeTranslation?.abbreviation ?? ''}</span>
                    <span className="bible-text">{result.text}</span>
                  </button>
                  <div className="bible-result-actions">
                    <button type="button" className="bible-action" disabled title={productionActionsPending} aria-label={`Preview ${result.reference}`}>
                      <Icon name="preview" size={13} /><span>Preview</span>
                    </button>
                    <button type="button" className="bible-action" disabled title={productionActionsPending} aria-label={`Send ${result.reference} live`}>
                      <Icon name="display" size={13} /><span>Send Live</span>
                    </button>
                    <button type="button" className="bible-action" disabled title={productionActionsPending} aria-label={`Add ${result.reference} to the service`}>
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

      default:
        return (
          <div className="bible-message" role="status">
            <Icon name="search" size={30} />
            <strong>Search the Bible</strong>
            <p>Enter a reference like John 3:16, a chapter like Psalm 23, or words like living water.</p>
          </div>
        );
    }
  }

  const resultCount = search.kind === 'results' ? search.response.results.length : null;

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
            onChange={(event) => handleTranslationChange(event.target.value)}
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
            {search.kind === 'loading' && 'Searching…'}
            {resultCount !== null && `${resultCount} ${resultCount === 1 ? 'result' : 'results'}`}
          </span>
        </div>

        <div className="bible-results-region" aria-busy={search.kind === 'loading'}>
          {renderResults()}
        </div>

        <p className="bible-note">Preview, Send Live and Add to Service become available with the presentation controller.</p>
      </div>
    </section>
  );
}
