import { useEffect, useRef, useState } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';
import { App } from '../../../bindings/zowe-worship';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import type { SongSectionInput } from '../../stores/StudioProvider';
import './songs.css';

const SEARCH_DEBOUNCE_MS = 250;

function fromActionArea(event: ReactMouseEvent<HTMLElement>): boolean {
  return (event.target as HTMLElement).closest('.song-actions') !== null;
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

function firstLine(text: string): string {
  return text.split('\n').map((line) => line.trim()).find((line) => line !== '') ?? '';
}

export function SongsWorkspace() {
  const studio = useStudio();
  const [query, setQuery] = useState('');
  const [pending, setPending] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [status, setStatus] = useState<{ kind: 'message' | 'error'; text: string } | null>(null);
  const debounceRef = useRef<number | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const trimmedQuery = query.trim();
  const songs = studio.songs?.songs ?? [];
  const hits = studio.songs?.hits ?? [];
  const canAct = studio.bridgeAvailable && busy === null;

  const refreshRef = useRef(studio.refreshSongs);
  refreshRef.current = studio.refreshSongs;
  const lastQueryRef = useRef('');

  // Songs are written in their own window, so this tab rereads the library when
  // it is mounted and shows edits made there.
  useEffect(() => {
    if (studio.bridgeAvailable) void refreshRef.current();
  }, []);

  useEffect(() => {
    if (!studio.bridgeAvailable || trimmedQuery === lastQueryRef.current) return;

    if (debounceRef.current !== null) {
      window.clearTimeout(debounceRef.current);
    }
    setPending(true);
    debounceRef.current = window.setTimeout(() => {
      debounceRef.current = null;
      lastQueryRef.current = trimmedQuery;
      void refreshRef.current(trimmedQuery).finally(() => setPending(false));
    }, SEARCH_DEBOUNCE_MS);

    return () => {
      if (debounceRef.current !== null) {
        window.clearTimeout(debounceRef.current);
        debounceRef.current = null;
      }
    };
  }, [trimmedQuery, studio.bridgeAvailable]);

  async function perform(kind: 'preview' | 'live' | 'add', section: SongSectionInput, label: string) {
    const key = `${kind}:${label}`;
    setBusy(key);
    setStatus(null);
    try {
      if (kind === 'preview') {
        await studio.previewSongSection(section);
        setStatus({ kind: 'message', text: `Previewing ${label}.` });
      } else if (kind === 'live') {
        await studio.projectSongSection(section);
        setStatus({ kind: 'message', text: `${label} is live.` });
      } else {
        await studio.addSongSectionToService(section);
        setStatus({ kind: 'message', text: `Added ${label} to the service.` });
      }
    } catch (error: unknown) {
      setStatus({ kind: 'error', text: errorMessage(error) });
    } finally {
      setBusy(null);
    }
  }

  async function openEditor(songID: string) {
    setBusy('editor');
    setStatus(null);
    try {
      await App.OpenSongEditor(songID);
    } catch (error: unknown) {
      setStatus({ kind: 'error', text: errorMessage(error) });
    } finally {
      setBusy(null);
    }
  }

  async function duplicateSong(songID: string, title: string) {
    setBusy(`duplicate:${songID}`);
    setStatus(null);
    try {
      await studio.duplicateSong(songID);
      setStatus({ kind: 'message', text: `Duplicated "${title}".` });
    } catch (error: unknown) {
      setStatus({ kind: 'error', text: errorMessage(error) });
    } finally {
      setBusy(null);
    }
  }

  async function addArrangementToService(songID: string, arrangementID: string, songTitle: string) {
    setBusy(`arr:${arrangementID}`);
    setStatus(null);
    try {
      await studio.addSongArrangementToService(songID, arrangementID);
      setStatus({ kind: 'message', text: `Added "${songTitle}" arrangement to service.` });
    } catch (error: unknown) {
      setStatus({ kind: 'error', text: errorMessage(error) });
    } finally {
      setBusy(null);
    }
  }

  return (
    <section className="panel songs-workspace" aria-labelledby="songs-workspace-title">
      <header className="songs-header">
        <div className="bible-heading">
          <Icon name="music" size={18} />
          <h2 id="songs-workspace-title">Songs</h2>
        </div>
        <div className="songs-header-actions">
          <button
            type="button"
            className="button"
            disabled={!canAct}
            onClick={() => void openEditor('')}
          >
            <Icon name="plus" size={13} />
            New song
          </button>
        </div>
      </header>

      <div className="songs-body">
        <div className="bible-search">
          <div className="bible-search-field">
            <Icon name="search" size={16} />
            <input
              ref={inputRef}
              type="text"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              aria-label="Search lyrics"
              placeholder="Search lyrics, e.g. grace or chains are gone"
              autoComplete="off"
              spellCheck={false}
              disabled={!studio.bridgeAvailable}
            />
            {query !== '' && (
              <button
                type="button"
                className="bible-search-clear"
                onClick={() => {
                  setQuery('');
                  inputRef.current?.focus();
                }}
                aria-label="Clear lyric search"
                title="Clear lyric search"
              >
                <Icon name="close" size={14} />
              </button>
            )}
          </div>
          <span className="bible-result-count" aria-live="polite">
            {pending
              ? 'Searching…'
              : trimmedQuery === ''
                ? `${songs.length} ${songs.length === 1 ? 'song' : 'songs'}`
                : `${hits.length} ${hits.length === 1 ? 'match' : 'matches'}`}
          </span>
        </div>

        {trimmedQuery !== '' ? (
          hits.length === 0 ? (
            <div className="bible-message" role="status">
              <Icon name="music" size={30} />
              <strong>No lyrics found</strong>
              <p>Nothing matches "{trimmedQuery}". Try a shorter phrase.</p>
            </div>
          ) : (
            <ul className="song-results" aria-label="Lyric matches">
              {hits.map((hit) => {
                const label = `${hit.songTitle} · ${hit.label}`;
                const section = { songId: hit.songId, sectionId: hit.sectionId };
                return (
                  <li
                    key={`${hit.songId}-${hit.sectionId}`}
                    className="song-result"
                    title={`Click to preview ${label} · double-click to send live`}
                    onClick={(event) => {
                      if (fromActionArea(event) || !canAct) return;
                      void perform('preview', section, label);
                    }}
                    onDoubleClick={(event) => {
                      if (fromActionArea(event) || !canAct) return;
                      void perform('live', section, label);
                    }}
                  >
                    <button type="button" className="song-result-main">
                      <span className="song-result-reference">{label}</span>
                      <span className="bible-text">{firstLine(hit.text)}</span>
                    </button>
                    <div className="song-actions">
                      <button
                        type="button"
                        className="bible-action"
                        disabled={!canAct}
                        onClick={(event) => {
                          event.stopPropagation();
                          void perform('preview', section, label);
                        }}
                        aria-label={`Preview ${label}`}
                        title={`Preview ${label}`}
                      >
                        <Icon name="preview" size={13} /><span>Preview</span>
                      </button>
                      <button
                        type="button"
                        className="bible-action"
                        disabled={!canAct}
                        onClick={(event) => {
                          event.stopPropagation();
                          void perform('live', section, label);
                        }}
                        aria-label={`Send ${label} live`}
                        title={`Send ${label} live`}
                      >
                        <Icon name="display" size={13} /><span>Send Live</span>
                      </button>
                      <button
                        type="button"
                        className="bible-action"
                        disabled={!canAct}
                        onClick={(event) => {
                          event.stopPropagation();
                          void perform('add', section, label);
                        }}
                        aria-label={`Add ${label} to the service`}
                        title={`Add ${label} to the service`}
                      >
                        <Icon name="plus" size={13} /><span>Add to Service</span>
                      </button>
                    </div>
                  </li>
                );
              })}
            </ul>
          )
        ) : songs.length === 0 ? (
          <div className="bible-message" role="status">
            <Icon name="music" size={30} />
            <strong>No songs yet</strong>
            <p>Add a song with its sections, and each section becomes a slide you can preview or project.</p>
          </div>
        ) : (
          <div className="song-list">
            {songs.map((song) => {
              const defaultArr = (song.arrangements ?? []).find((a) => a.isDefault)
                ?? (song.arrangements ?? [])[0]
                ?? null;
              const arrangementItems = defaultArr?.items ?? [];

              return (
                <details className="song" key={song.id}>
                  <summary>
                    <span className="song-title">{song.title}</span>
                    <span className="song-meta">
                      {(song.sections ?? []).length} {(song.sections ?? []).length === 1 ? 'section' : 'sections'}
                      {song.author ? ` · ${song.author}` : ''}
                      {song.copyright ? ` · ${song.copyright}` : ''}
                      {song.ccliNumber ? ` · CCLI #${song.ccliNumber}` : ''}
                    </span>
                    <div className="song-summary-actions">
                      {defaultArr && (
                        <button
                          type="button"
                          className="bible-action"
                          disabled={!canAct}
                          aria-label={`Add all sections of ${song.title} to service`}
                          title="Add arrangement to service"
                          onClick={(event) => {
                            event.preventDefault();
                            event.stopPropagation();
                            void addArrangementToService(song.id, defaultArr.id, song.title);
                          }}
                        >
                          <Icon name="plus" size={13} /><span>Add All</span>
                        </button>
                      )}
                      <button
                        type="button"
                        className="bible-action"
                        disabled={!canAct}
                        aria-label={`Edit ${song.title}`}
                        title="Edit song"
                        onClick={(event) => {
                          // The summary owns the open/close toggle, so both have to be stopped.
                          event.preventDefault();
                          event.stopPropagation();
                          void openEditor(song.id);
                        }}
                      >
                        <Icon name="more" size={13} /><span>Edit</span>
                      </button>
                      <button
                        type="button"
                        className="bible-action"
                        disabled={!canAct}
                        aria-label={`Duplicate ${song.title}`}
                        title="Duplicate song"
                        onClick={(event) => {
                          event.preventDefault();
                          event.stopPropagation();
                          void duplicateSong(song.id, song.title);
                        }}
                      >
                        <Icon name="plus" size={13} /><span>Duplicate</span>
                      </button>
                      <button
                        type="button"
                        className="icon-button icon-button--small"
                        disabled={!canAct}
                        aria-label={`Delete ${song.title}`}
                        title="Delete song"
                        onClick={(event) => {
                          event.preventDefault();
                          event.stopPropagation();
                          if (!canAct) return;
                          setBusy(`delete:${song.id}`);
                          void studio.deleteSong(song.id)
                            .then(() => setStatus({ kind: 'message', text: `Deleted ${song.title}.` }))
                            .catch((error: unknown) => setStatus({ kind: 'error', text: errorMessage(error) }))
                            .finally(() => setBusy(null));
                        }}
                      >
                        <Icon name="close" size={14} />
                      </button>
                    </div>
                  </summary>

                  {/* Show arrangement order if available, otherwise raw sections */}
                  {arrangementItems.length > 0 ? (
                    <ul className="song-sections" aria-label="Arrangement">
                      {arrangementItems.map((item) => {
                        const label = `${song.title} · ${item.label}`;
                        const input = { songId: song.id, sectionId: item.sectionId };
                        return (
                          <li
                            key={item.id}
                            className="song-result"
                            title={`Click to preview ${label} · double-click to send live`}
                            onClick={(event) => {
                              if (fromActionArea(event) || !canAct) return;
                              void perform('preview', input, label);
                            }}
                            onDoubleClick={(event) => {
                              if (fromActionArea(event) || !canAct) return;
                              void perform('live', input, label);
                            }}
                          >
                            <button type="button" className="song-result-main">
                              <span className="song-result-reference">{item.label}</span>
                              <span className="bible-text">{firstLine(item.text)}</span>
                            </button>
                            <div className="song-actions">
                              <button
                                type="button"
                                className="bible-action"
                                disabled={!canAct}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  void perform('preview', input, label);
                                }}
                                aria-label={`Preview ${label}`}
                                title={`Preview ${label}`}
                              >
                                <Icon name="preview" size={13} /><span>Preview</span>
                              </button>
                              <button
                                type="button"
                                className="bible-action"
                                disabled={!canAct}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  void perform('live', input, label);
                                }}
                                aria-label={`Send ${label} live`}
                                title={`Send ${label} live`}
                              >
                                <Icon name="display" size={13} /><span>Send Live</span>
                              </button>
                              <button
                                type="button"
                                className="bible-action"
                                disabled={!canAct}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  void perform('add', input, label);
                                }}
                                aria-label={`Add ${label} to the service`}
                                title={`Add ${label} to the service`}
                              >
                                <Icon name="plus" size={13} /><span>Add to Service</span>
                              </button>
                            </div>
                          </li>
                        );
                      })}
                    </ul>
                  ) : (
                    <ul className="song-sections" aria-label="Sections">
                      {(song.sections ?? []).map((section) => {
                        const label = `${song.title} · ${section.label}`;
                        const input = { songId: song.id, sectionId: section.id };
                        return (
                          <li
                            key={section.id}
                            className="song-result"
                            title={`Click to preview ${label} · double-click to send live`}
                            onClick={(event) => {
                              if (fromActionArea(event) || !canAct) return;
                              void perform('preview', input, label);
                            }}
                            onDoubleClick={(event) => {
                              if (fromActionArea(event) || !canAct) return;
                              void perform('live', input, label);
                            }}
                          >
                            <button type="button" className="song-result-main">
                              <span className="song-result-reference">{section.label}</span>
                              <span className="bible-text">{firstLine(section.text)}</span>
                            </button>
                            <div className="song-actions">
                              <button
                                type="button"
                                className="bible-action"
                                disabled={!canAct}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  void perform('preview', input, label);
                                }}
                                aria-label={`Preview ${label}`}
                                title={`Preview ${label}`}
                              >
                                <Icon name="preview" size={13} /><span>Preview</span>
                              </button>
                              <button
                                type="button"
                                className="bible-action"
                                disabled={!canAct}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  void perform('live', input, label);
                                }}
                                aria-label={`Send ${label} live`}
                                title={`Send ${label} live`}
                              >
                                <Icon name="display" size={13} /><span>Send Live</span>
                              </button>
                              <button
                                type="button"
                                className="bible-action"
                                disabled={!canAct}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  void perform('add', input, label);
                                }}
                                aria-label={`Add ${label} to the service`}
                                title={`Add ${label} to the service`}
                              >
                                <Icon name="plus" size={13} /><span>Add to Service</span>
                              </button>
                            </div>
                          </li>
                        );
                      })}
                    </ul>
                  )}
                </details>
              );
            })}
          </div>
        )}

        <p className={`bible-note${status?.kind === 'error' ? ' bible-note--error' : ''}`} aria-live="polite">
          {status?.text ?? 'Click a section to preview it, double-click to send it live.'}
        </p>
      </div>
    </section>
  );
}
