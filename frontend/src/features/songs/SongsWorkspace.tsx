import { useEffect, useRef, useState } from 'react';
import type { FormEvent, MouseEvent as ReactMouseEvent } from 'react';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import type { SongSectionInput } from '../../stores/StudioProvider';
import './songs.css';

const SEARCH_DEBOUNCE_MS = 250;

type DraftSection = { label: string; text: string };

const emptySections: DraftSection[] = [{ label: 'Verse 1', text: '' }];

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
  const [creating, setCreating] = useState(false);
  const [title, setTitle] = useState('');
  const [author, setAuthor] = useState('');
  const [copyright, setCopyright] = useState('');
  const [sections, setSections] = useState<DraftSection[]>(emptySections);
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

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy('create');
    setStatus(null);
    try {
      await studio.createSong({
        title: title.trim(),
        author: author.trim(),
        copyright: copyright.trim(),
        sections: sections.map((section) => ({ label: section.label.trim(), text: section.text.trim() })),
      });
      setTitle('');
      setAuthor('');
      setCopyright('');
      setSections(emptySections);
      setCreating(false);
      setStatus({ kind: 'message', text: 'Song saved.' });
    } catch (error: unknown) {
      setStatus({ kind: 'error', text: errorMessage(error) });
    } finally {
      setBusy(null);
    }
  }

  function updateSection(index: number, change: Partial<DraftSection>) {
    setSections((current) => current.map((section, position) => (
      position === index ? { ...section, ...change } : section
    )));
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
            onClick={() => {
              setCreating((current) => !current);
              setStatus(null);
            }}
          >
            <Icon name={creating ? 'close' : 'plus'} size={13} />
            {creating ? 'Cancel' : 'New song'}
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

        {creating && (
          <form className="song-form" onSubmit={handleSubmit}>
            <div className="song-form-fields">
              <label>
                <span>Title</span>
                <input
                  type="text"
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  placeholder="Amazing Grace"
                  autoComplete="off"
                  required
                />
              </label>
              <label>
                <span>Author</span>
                <input
                  type="text"
                  value={author}
                  onChange={(event) => setAuthor(event.target.value)}
                  placeholder="John Newton"
                  autoComplete="off"
                />
              </label>
              <label>
                <span>Copyright</span>
                <input
                  type="text"
                  value={copyright}
                  onChange={(event) => setCopyright(event.target.value)}
                  placeholder="Public domain"
                  autoComplete="off"
                />
              </label>
            </div>

            {sections.map((section, index) => (
              <div className="song-section-edit" key={index}>
                <input
                  className="song-section-label"
                  type="text"
                  value={section.label}
                  onChange={(event) => updateSection(index, { label: event.target.value })}
                  placeholder="Verse 1"
                  aria-label={`Section ${index + 1} label`}
                />
                <textarea
                  rows={4}
                  value={section.text}
                  onChange={(event) => updateSection(index, { text: event.target.value })}
                  placeholder="Type the lyrics for this slide"
                  aria-label={`Section ${index + 1} lyrics`}
                />
                <button
                  type="button"
                  className="icon-button icon-button--small"
                  disabled={sections.length === 1}
                  onClick={() => setSections((current) => current.filter((_, position) => position !== index))}
                  aria-label={`Remove section ${index + 1}`}
                  title="Remove this section"
                >
                  <Icon name="close" size={14} />
                </button>
              </div>
            ))}

            <div className="song-form-actions">
              <button
                type="button"
                className="button"
                onClick={() => setSections((current) => [...current, { label: '', text: '' }])}
              >
                <Icon name="plus" size={13} />Add section
              </button>
              <button
                type="submit"
                className="button button--primary"
                disabled={busy !== null || title.trim() === ''}
              >
                Save song
              </button>
            </div>
            <p className="song-form-note">
              Each section becomes one slide. Lyrics are stored on this computer only, so add words you have the
              right to use.
            </p>
          </form>
        )}

        {trimmedQuery !== '' ? (
          hits.length === 0 ? (
            <div className="bible-message" role="status">
              <Icon name="music" size={30} />
              <strong>No lyrics found</strong>
              <p>Nothing matches “{trimmedQuery}”. Try a shorter phrase.</p>
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
            {songs.map((song) => (
              <details className="song" key={song.id}>
                <summary>
                  <span className="song-title">{song.title}</span>
                  <span className="song-meta">
                    {(song.sections ?? []).length} {(song.sections ?? []).length === 1 ? 'section' : 'sections'}
                    {song.author ? ` · ${song.author}` : ''}
                    {song.copyright ? ` · ${song.copyright}` : ''}
                  </span>
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
                </summary>

                <ul className="song-sections">
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
              </details>
            ))}
          </div>
        )}

        <p className={`bible-note${status?.kind === 'error' ? ' bible-note--error' : ''}`} aria-live="polite">
          {status?.text ?? 'Click a section to preview it, double-click to send it live.'}
        </p>
      </div>
    </section>
  );
}
