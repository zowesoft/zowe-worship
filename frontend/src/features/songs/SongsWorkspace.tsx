import { useEffect, useRef, useState } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';
import { App } from '../../../bindings/zowe-worship';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import type { SongSectionInput } from '../../stores/StudioProvider';

// The lyric list borrows the Bible tab's row/action look, so these mirror the
// strings in BibleWorkspace.tsx.
const ROW_ACTION =
  'inline-flex items-center gap-[5px] px-[7px] py-1.5 border border-transparent ' +
  'rounded-[5px] bg-transparent text-secondary text-[10px] whitespace-nowrap ' +
  'disabled:text-[#6F809B] max-[1279px]:[&_span]:hidden';

const STATUS_PANEL =
  'min-h-[210px] flex flex-col items-center justify-center gap-2 p-6 border ' +
  'border-dashed border-border rounded-lg bg-app text-secondary text-center ' +
  '[&>svg]:opacity-70 [&_strong]:text-text [&_strong]:text-[12px] ' +
  '[&_strong]:font-medium [&_p]:max-w-[440px] [&_p]:text-[11px] [&_p]:leading-[1.7]';

const ROW_TEXT =
  'min-w-0 truncate text-secondary text-[11px]';

// A section row inside the song list and the lyric-search list share this look.
const SONG_ROW =
  'flex items-center gap-2 px-2 py-px border border-transparent rounded-[7px] ' +
  'bg-app cursor-pointer select-none hover:bg-[#0C1523]';

const SONG_ROW_MAIN =
  'flex-1 min-w-0 grid grid-cols-[190px_minmax(0,1fr)] items-center gap-2 ' +
  'py-[9px] px-1 border-0 bg-transparent text-left text-inherit ' +
  'max-[1279px]:grid-cols-[150px_minmax(0,1fr)] max-[959px]:grid-cols-[120px_minmax(0,1fr)]';

const SONG_REF =
  'text-[11px] font-semibold truncate text-text';

const SEARCH_DEBOUNCE_MS = 250;

function fromActionArea(event: ReactMouseEvent<HTMLElement>): boolean {
  return (event.target as HTMLElement).closest('[data-row-actions]') !== null;
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
    <section className="bg-panel border border-border rounded-[10px] overflow-hidden" aria-labelledby="songs-workspace-title">
      <header className="flex items-center justify-between gap-[18px] min-h-[54px] px-3.5 py-2.5 border-b border-border max-[959px]:gap-3 max-[959px]:px-2.5 max-[959px]:py-2">
        <div className="flex items-center gap-2 [&>svg]:text-secondary [&_h2]:text-[14px] [&_h2]:tracking-[-0.2px] max-[959px]:[&_h2]:text-[13px]">
          <Icon name="music" size={18} />
          <h2 id="songs-workspace-title">Songs</h2>
        </div>
        <div className="flex items-center gap-2 ml-auto">
          <button
            type="button"
            className="min-h-8 inline-flex items-center justify-center gap-1.5 px-3 py-1.5 border border-[#35445C] rounded-md bg-elevated text-[11px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]"
            disabled={!canAct}
            onClick={() => void openEditor('')}
          >
            <Icon name="plus" size={13} />
            New song
          </button>
        </div>
      </header>

      <div className="grid gap-3 p-3.5 max-[959px]:p-2.5">
        <div className="flex items-stretch gap-2.5 max-[959px]:gap-2">
          <div
            className={
              'flex-1 min-w-0 flex items-center gap-2 min-h-[38px] px-3 border border-[#35445C] rounded-[7px] bg-app '
              + '[&>svg]:text-secondary focus-within:border-blue focus-within:shadow-[0_0_0_2px_rgba(71,120,255,0.18)]'
            }
          >
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
              className="flex-1 min-w-0 h-9 border-0 outline-none bg-transparent text-text text-[12px] placeholder:text-[#6F809B] disabled:text-secondary"
            />
            {query !== '' && (
              <button
                type="button"
                className="grid place-items-center w-6 h-6 border-0 rounded-[4px] bg-transparent text-secondary hover:bg-elevated hover:text-text"
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
          <span className="text-secondary text-[10px] ml-0.5" aria-live="polite">
            {pending
              ? 'Searching…'
              : trimmedQuery === ''
                ? `${songs.length} ${songs.length === 1 ? 'song' : 'songs'}`
                : `${hits.length} ${hits.length === 1 ? 'match' : 'matches'}`}
          </span>
        </div>

        {trimmedQuery !== '' ? (
          hits.length === 0 ? (
            <div className={STATUS_PANEL} role="status">
              <Icon name="music" size={30} />
              <strong>No lyrics found</strong>
              <p>Nothing matches "{trimmedQuery}". Try a shorter phrase.</p>
            </div>
          ) : (
            <ul className="list-none grid gap-1 m-0 p-0" aria-label="Lyric matches">
              {hits.map((hit) => {
                const label = `${hit.songTitle} · ${hit.label}`;
                const section = { songId: hit.songId, sectionId: hit.sectionId };
                return (
                  <li
                    key={`${hit.songId}-${hit.sectionId}`}
                    className={SONG_ROW}
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
                    <button type="button" className={SONG_ROW_MAIN}>
                      <span className={SONG_REF}>{label}</span>
                      <span className={ROW_TEXT}>{firstLine(hit.text)}</span>
                    </button>
                    <div className="flex items-center gap-0.5" data-row-actions>
                      <button
                        type="button"
                        className={ROW_ACTION}
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
                        className={ROW_ACTION}
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
                        className={ROW_ACTION}
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
          <div className={STATUS_PANEL} role="status">
            <Icon name="music" size={30} />
            <strong>No songs yet</strong>
            <p>Add a song with its sections, and each section becomes a slide you can preview or project.</p>
          </div>
        ) : (
          <div className="grid gap-1.5">
            {songs.map((song) => {
              const defaultArr = (song.arrangements ?? []).find((a) => a.isDefault)
                ?? (song.arrangements ?? [])[0]
                ?? null;
              const arrangementItems = defaultArr?.items ?? [];

              return (
                <details
                  className="group border border-border rounded-lg bg-app overflow-hidden"
                  key={song.id}
                >
                  <summary
                    className={
                      'flex items-center gap-2.5 py-[9px] px-[11px] cursor-pointer list-none '
                      + '[&::-webkit-details-marker]:hidden '
                      + "before:content-[''] before:w-1.5 before:h-1.5 before:shrink-0 "
                      + 'before:border-r-[1.6px] before:border-b-[1.6px] before:border-secondary '
                      + 'before:rotate-[-45deg] before:transition-transform before:duration-[120ms] '
                      + 'group-open:before:rotate-45'
                    }
                  >
                    <span className="text-[11.5px] font-semibold">{song.title}</span>
                    <span className="text-secondary text-[10px]">
                      {(song.sections ?? []).length} {(song.sections ?? []).length === 1 ? 'section' : 'sections'}
                      {song.author ? ` · ${song.author}` : ''}
                      {song.copyright ? ` · ${song.copyright}` : ''}
                      {song.ccliNumber ? ` · CCLI #${song.ccliNumber}` : ''}
                    </span>
                    <div className="ml-auto flex items-center gap-2 shrink-0">
                      {defaultArr && (
                        <button
                          type="button"
                          className={ROW_ACTION}
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
                        className={ROW_ACTION}
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
                        className={ROW_ACTION}
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
                        className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50"
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
                    <ul className="list-none grid gap-1 m-0 p-0 pt-1 px-2 pb-2.5" aria-label="Arrangement">
                      {arrangementItems.map((item) => {
                        const label = `${song.title} · ${item.label}`;
                        const input = { songId: song.id, sectionId: item.sectionId };
                        return (
                          <li
                            key={item.id}
                            className={SONG_ROW}
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
                            <button type="button" className={SONG_ROW_MAIN}>
                              <span className={SONG_REF}>{item.label}</span>
                              <span className={ROW_TEXT}>{firstLine(item.text)}</span>
                            </button>
                            <div className="flex items-center gap-0.5" data-row-actions>
                              <button
                                type="button"
                                className={ROW_ACTION}
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
                                className={ROW_ACTION}
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
                                className={ROW_ACTION}
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
                    <ul className="list-none grid gap-1 m-0 p-0 pt-1 px-2 pb-2.5" aria-label="Sections">
                      {(song.sections ?? []).map((section) => {
                        const label = `${song.title} · ${section.label}`;
                        const input = { songId: song.id, sectionId: section.id };
                        return (
                          <li
                            key={section.id}
                            className={SONG_ROW}
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
                            <button type="button" className={SONG_ROW_MAIN}>
                              <span className={SONG_REF}>{section.label}</span>
                              <span className={ROW_TEXT}>{firstLine(section.text)}</span>
                            </button>
                            <div className="flex items-center gap-0.5" data-row-actions>
                              <button
                                type="button"
                                className={ROW_ACTION}
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
                                className={ROW_ACTION}
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
                                className={ROW_ACTION}
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

        <p className={'pt-2.5 border-t border-border text-[10px] ' + (status?.kind === 'error' ? 'text-error' : 'text-secondary')} aria-live="polite">
          {status?.text ?? 'Click a section to preview it, double-click to send it live.'}
        </p>
      </div>
    </section>
  );
}
