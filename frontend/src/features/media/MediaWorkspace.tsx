import { useEffect, useMemo, useState } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';
import { Events } from '@wailsio/runtime';
import { App } from '../../../bindings/zopresent';
import type { MediaImportResult, MediaPlaylistInput, MediaPlaylistView, MediaView, PlaylistRunView } from '../../../bindings/zopresent/models';
import type { Aspect, FormatGroup } from '../../../bindings/zopresent/internal/media/models';
import { FittedImage } from '../../components/FittedImage';
import { Icon } from '../../components/Icon';
import { formatBytes } from '../../lib/format';
import { useStudio } from '../../stores/StudioProvider';

const filters = [
  { value: 'all', label: 'All' },
  { value: 'image', label: 'Images' },
  { value: 'video', label: 'Video' },
  { value: 'audio', label: 'Audio' },
] as const;

type Filter = (typeof filters)[number]['value'];

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

// The tile itself is the preview gesture, so its buttons have to opt out.
function fromActions(event: ReactMouseEvent<HTMLElement>): boolean {
  return (event.target as HTMLElement).closest('.media-tile-actions') !== null;
}

function formatDuration(ms: number): string {
  if (ms <= 0) return '';
  const total = Math.round(ms / 1000);
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`;
}

function kindIcon(kind: string) {
  if (kind === 'video') return <Icon name="media" size={26} />;
  if (kind === 'audio') return <Icon name="music" size={26} />;
  return <Icon name="layers" size={26} />;
}

// The backend owns the shape list, so the ratio is looked up rather than parsed
// from the label.
function ratioOf(aspects: Aspect[], value: string): number {
  return aspects.find((aspect) => aspect.value === value)?.ratio ?? 0;
}

const kindLabels: Record<string, string> = { image: 'Images', video: 'Video', audio: 'Audio' };

// The declaration comes from the backend - the same list the file dialog and
// the importer enforce - so this caption states it verbatim instead of
// promising a format the next import would reject.
function formatsSummary(groups: FormatGroup[]): string {
  return groups
    .map((group) => {
      const extensions = (group.extensions ?? []).map((ext) => ext.replace('.', '').toUpperCase()).join(', ');
      const codecs = group.codecs ? ` (${group.codecs})` : '';
      return `${kindLabels[group.kind] ?? group.kind}: ${extensions}${codecs}`;
    })
    .join(' · ');
}

export function MediaWorkspace() {
  const studio = useStudio();
  const [filter, setFilter] = useState<Filter>('all');
  const [query, setQuery] = useState('');
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState<{ kind: 'message' | 'error'; text: string } | null>(null);
  const [playlists, setPlaylists] = useState<MediaPlaylistView[]>([]);
  const [run, setRun] = useState<PlaylistRunView | null>(null);
  // The playlist being edited, and the name in its box until blur saves it.
  const [editing, setEditing] = useState<string | null>(null);
  const [draftName, setDraftName] = useState('');
  const [addChoice, setAddChoice] = useState('');
  // Declared formats come from the backend once, so the caption under the
  // import button can never promise something the picker would not offer.
  const [formats, setFormats] = useState<FormatGroup[]>([]);

  const shown = useMemo(() => studio.media.filter((item) => {
    if (filter !== 'all' && item.kind !== filter) return false;
    const text = query.trim().toLowerCase();
    return text === '' || item.title.toLowerCase().includes(text);
  }), [filter, query, studio.media]);

  async function chooseFiles() {
    setBusy(true);
    setStatus(null);
    try {
      const paths = await App.PickMediaFiles();
      if (!paths || paths.length === 0) return;
      report(await studio.importMedia(paths));
    } catch (error: unknown) {
      setStatus({ kind: 'error', text: errorMessage(error) });
    } finally {
      setBusy(false);
    }
  }

  function report(result: MediaImportResult) {
    const added = result.added?.length ?? 0;
    const problems = result.problems ?? [];
    if (added === 0 && problems.length === 0) {
      setStatus({ kind: 'error', text: 'Nothing was added.' });
      return;
    }
    const parts = [`${added} ${added === 1 ? 'item' : 'items'} added`];
    const duplicates = problems.filter((problem) => problem.duplicate).length;
    if (duplicates > 0) parts.push(`${duplicates} already in the library`);
    const rejected = problems.length - duplicates;
    if (rejected > 0) parts.push(`${rejected} unsupported`);
    setStatus({ kind: 'message', text: parts.join(' · ') });
  }

  const canAct = studio.bridgeAvailable && !busy;

  async function act(action: () => Promise<unknown>, message: string) {
    setBusy(true);
    setStatus(null);
    try {
      await action();
      setStatus({ kind: 'message', text: message });
    } catch (error: unknown) {
      setStatus({ kind: 'error', text: errorMessage(error) });
    } finally {
      setBusy(false);
    }
  }

  // The editor is a separate window, so the grid is reread whenever this tab is
  // mounted and picks up framing saved there.
  useEffect(() => {
    if (studio.bridgeAvailable) void studio.refreshMedia();
  }, []);

  // The format list is static, so it is read with the tab and never refreshed.
  useEffect(() => {
    if (!studio.bridgeAvailable) return;
    void (async () => setFormats((await App.MediaFormats()) ?? []))();
  }, [studio.bridgeAvailable]);

  // Playlists load with the tab, and the run badge follows the backend's
  // event - which clip is on the wall moves on its own while a playlist
  // plays, so the row must not depend on the operator clicking here.
  useEffect(() => {
    if (!studio.bridgeAvailable) return;
    void refreshPlaylists();
    return Events.On('playlist-run', (event) => setRun(event.data as PlaylistRunView));
  }, [studio.bridgeAvailable]);

  async function refreshPlaylists() {
    try {
      setPlaylists((await App.ListMediaPlaylists()) ?? []);
    } catch {
      // The bridge being gone is already reported by the provider.
    }
  }

  async function openEditor(item: MediaView) {
    setBusy(true);
    setStatus(null);
    try {
      await App.OpenMediaEditor(item.id);
    } catch (error: unknown) {
      setStatus({ kind: 'error', text: errorMessage(error) });
    } finally {
      setBusy(false);
    }
  }

  function sendLive(item: MediaView) {
    return act(async () => {
      await studio.previewMedia(item.id);
      await studio.projectMedia(item.id);
    }, `${item.title} is live.`);
  }

  async function remove(item: MediaView) {
    await act(() => studio.deleteMedia(item.id), `Removed ${item.title}.`);
  }

  // The picker and the adopt are two calls so a cancelled dialog stays silent:
  // only a chosen file can produce the "not the same file" report.
  async function relink(item: MediaView) {
    setBusy(true);
    setStatus(null);
    try {
      const path = await App.PickMediaFile();
      if (path === '') return;
      await studio.relinkMedia(item.id, path);
      setStatus({ kind: 'message', text: `Found ${item.title} again.` });
    } catch (error: unknown) {
      setStatus({ kind: 'error', text: errorMessage(error) });
    } finally {
      setBusy(false);
    }
  }

  const smallButton = 'min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]';

  async function createPlaylist() {
    setBusy(true);
    setStatus(null);
    try {
      const created = await App.SaveMediaPlaylist({
        id: '', name: `Playlist ${playlists.length + 1}`, mediaIds: [],
      });
      await refreshPlaylists();
      setEditing(created.id);
      setDraftName(created.name);
      setAddChoice('');
      setStatus({ kind: 'message', text: 'Playlist created - add clips below.' });
    } catch (error: unknown) {
      setStatus({ kind: 'error', text: errorMessage(error) });
    } finally {
      setBusy(false);
    }
  }

  async function updatePlaylist(playlist: MediaPlaylistInput, message: string) {
    await act(async () => {
      await App.SaveMediaPlaylist(playlist);
      await refreshPlaylists();
    }, message);
  }

  function playPlaylist(playlist: MediaPlaylistView) {
    void act(() => App.PlayMediaPlaylist(playlist.id), `${playlist.name} is playing - clips run back to back.`);
  }

  function stopPlaylist(playlist: MediaPlaylistView) {
    void act(() => studio.mediaControl('stop'), `Stopped ${playlist.name}.`);
  }

  async function removePlaylist(playlist: MediaPlaylistView) {
    await act(async () => {
      if (editing === playlist.id) setEditing(null);
      await App.DeleteMediaPlaylist(playlist.id);
      await refreshPlaylists();
    }, `Deleted ${playlist.name}.`);
  }

  async function savePlaylistName(playlist: MediaPlaylistView) {
    const name = draftName.trim();
    if (name === '' || name === playlist.name) {
      setDraftName(playlist.name);
      return;
    }
    await updatePlaylist({ id: playlist.id, name, mediaIds: playlist.mediaIds ?? [] }, `Renamed to ${name}.`);
  }

  function moveClip(playlist: MediaPlaylistView, index: number, step: -1 | 1) {
    const items = [...(playlist.mediaIds ?? [])];
    const target = index + step;
    if (target < 0 || target >= items.length) return;
    [items[index], items[target]] = [items[target], items[index]];
    void updatePlaylist({ id: playlist.id, name: playlist.name, mediaIds: items }, `Moved clip to position ${target + 1}.`);
  }

  return (
    <section className="bg-panel border border-border rounded-[10px] overflow-hidden" aria-labelledby="media-title">
      <div className="min-h-11 px-3.5 py-2.5 flex items-center justify-between gap-2.5 border-b border-border">
        <h2 id="media-title" className="text-[11px] flex items-center gap-[7px] uppercase tracking-[0.7px] text-secondary [&_svg]:text-secondary"><Icon name="media" size={16} />Media</h2>
        <div className="flex items-center gap-3">
          <span>{studio.media.length} items</span>
          <button
            type="button"
            className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-blue! rounded-md bg-blue! text-white! text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
            disabled={!studio.bridgeAvailable || busy}
            onClick={() => void chooseFiles()}
          >
            <Icon name="plus" size={14} />Add media
          </button>
        </div>
      </div>

      <div className="grid gap-3.5 p-4">
        {formats.length > 0 && (
          <p className="m-0 -mb-0.5 text-[10.5px] leading-[1.7] text-[#6F809B]">
            Accepted: {formatsSummary(formats)} — what WebView2 decodes. Containers it refuses, such as
            .mkv and .avi, will not import whatever codec they carry.
          </p>
        )}
        <div className="flex items-center gap-3 flex-wrap">
          <div className="flex-1 min-w-[220px] flex items-center gap-2 h-[34px] px-[11px] border border-[#35445C] rounded-[7px] bg-app focus-within:border-blue focus-within:shadow-[0_0_0_2px_rgba(71,120,255,0.18)] [&>svg]:text-secondary">
            <Icon name="search" size={15} />
            <input
              type="text"
              value={query}
              placeholder="Search by name"
              aria-label="Search media by name"
              autoComplete="off"
              className="flex-1 min-w-0 border-0 outline-none bg-transparent text-text text-[12px] placeholder:text-[#6F809B]"
              onChange={(event) => setQuery(event.target.value)}
            />
          </div>
          <div className="flex gap-1" role="group" aria-label="Filter by type">
            {filters.map((option) => (
              <button
                key={option.value}
                type="button"
                className={`px-[11px] py-1.5 border border-border rounded-md bg-transparent text-secondary text-[11px] hover:bg-elevated hover:text-text ${filter === option.value ? 'border-[#304C80]! bg-selected! text-text!' : ''}`}
                aria-pressed={filter === option.value}
                onClick={() => setFilter(option.value)}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        {studio.media.length === 0 ? (
          <div className="grid justify-items-center gap-2 py-[34px] px-5 border border-dashed border-border rounded-[10px] text-center [&_h3]:text-[14px] [&_p]:max-w-[560px] [&_p]:text-secondary [&_p]:text-[11.5px] [&_p]:leading-[1.7]">
            <span className="grid place-items-center w-12 h-12 border border-[#30466A] bg-selected text-[#8CABFF] rounded-[11px] shrink-0"><Icon name="media" size={24} /></span>
            <h3>No media yet</h3>
            <p>Use Add media to choose images, video or audio from any folder. Zopresent keeps its own copy, so moving a file later cannot break a service.</p>
          </div>
        ) : shown.length === 0 ? (
          <p className="p-[22px] text-secondary text-[11.5px] text-center">Nothing matches this filter.</p>
        ) : (
          <ul className="list-none grid grid-cols-[repeat(auto-fill,minmax(178px,1fr))] gap-3 m-0 p-0">
            {shown.map((item) => {
              const projectable = !item.missing;
              const editable = projectable;
              return (
                <li
                  key={item.id}
                  className={[
                    'relative grid gap-0 border border-border rounded-lg bg-app overflow-hidden hover:[&_.media-tile-actions]:opacity-100 focus-within:[&_.media-tile-actions]:opacity-100',
                    item.missing ? 'border-[#4D3F1E]!' : '',
                    projectable ? 'cursor-pointer select-none' : '',
                  ].filter(Boolean).join(' ')}
                  title={projectable
                    ? `Click to preview ${item.title} · double-click to send live`
                    : 'The library copy is missing. Use Find the file to point at it again.'}
                  onClick={(event) => {
                    if (!projectable || fromActions(event) || !canAct) return;
                    void act(() => studio.previewMedia(item.id), `Previewing ${item.title}.`);
                  }}
                  onDoubleClick={(event) => {
                    if (!projectable || fromActions(event) || !canAct) return;
                    void sendLive(item);
                  }}
                >
                  <div className="relative grid place-items-center aspect-[16/9] bg-[#0B1424] overflow-hidden [&_img]:w-full [&_img]:h-full [&_img]:object-cover [&_img]:block">
                    {item.hasThumbnail
                      ? (
                        <FittedImage
                          src={item.thumbnailUrl}
                          alt=""
                          ratio={ratioOf(studio.mediaAspects, item.aspect)}
                          fit={item.fit === 'cover' ? 'cover' : 'contain'}
                        />
                      )
                      : <span className="text-[#4B5F7C]">{kindIcon(item.kind)}</span>}
                    {formatDuration(item.durationMs) && <span className="absolute right-1.5 bottom-1.5 py-0.5 px-1.5 rounded-[4px] bg-[rgba(4,9,18,0.82)] text-text text-[10px] tabular-nums">{formatDuration(item.durationMs)}</span>}
                    {item.missing && <span className="inline-flex items-center px-2 py-[3px] border border-[#4D3F1E]! rounded-full bg-[#2B2413]! text-amber! text-[9px] whitespace-nowrap absolute left-1.5 top-1.5">Missing</span>}
                  </div>
                  <div className="px-2.5 pt-2 pb-2.5 min-w-0 [&_strong]:block [&_strong]:truncate [&_strong]:text-[11.5px] [&_strong]:font-medium [&_span]:block [&_span]:mt-0.5 [&_span]:text-secondary [&_span]:text-[9.5px] [&_span]:capitalize">
                    <strong title={item.title}>{item.title}</strong>
                    <span>{item.kind} · {formatBytes(item.sizeBytes)}</span>
                  </div>
                  <div className="media-tile-actions absolute inset-x-0 bottom-0 flex items-center justify-end gap-1.5 py-2 px-2 pl-2.5 bg-linear-to-t from-[rgba(6,12,22,0.94)] to-[rgba(6,12,22,0)] opacity-0 transition-opacity duration-120">
                    {editable && (
                      <button
                        type="button"
                        className={smallButton}
                        disabled={!canAct}
                        onClick={(event) => {
                          event.stopPropagation();
                          void openEditor(item);
                        }}
                      >
                        <Icon name="settings" size={13} />Edit
                      </button>
                    )}
                    {projectable && (
                      <button
                        type="button"
                        className={smallButton}
                        disabled={!canAct}
                        onClick={(event) => {
                          event.stopPropagation();
                          void act(() => studio.addMediaToService(item.id), `Added ${item.title} to the service.`);
                        }}
                      >
                        <Icon name="plus" size={13} />Add to Service
                      </button>
                    )}
                    {item.missing && (
                      <button
                        type="button"
                        className={smallButton}
                        disabled={!canAct}
                        onClick={(event) => {
                          event.stopPropagation();
                          void relink(item);
                        }}
                      >
                        <Icon name="link" size={13} />Find the file
                      </button>
                    )}
                    <button
                      type="button"
                      className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50"
                      disabled={!canAct}
                      aria-label={`Remove ${item.title} from the library`}
                      title="Remove from library"
                      onClick={(event) => {
                        event.stopPropagation();
                        void remove(item);
                      }}
                    >
                      <Icon name="close" size={14} />
                    </button>
                  </div>
                </li>
              );
            })}
          </ul>
        )}

        <div className="grid gap-2 border-t border-border pt-3" aria-labelledby="playlists-title">
          <div className="flex items-center justify-between gap-2">
            <h3 id="playlists-title" className="text-[11px] m-0 flex items-center gap-[7px] uppercase tracking-[0.7px] text-secondary [&_svg]:text-secondary"><Icon name="layers" size={14} />Playlists</h3>
            <button type="button" className={smallButton} disabled={!canAct} onClick={() => void createPlaylist()}>
              <Icon name="plus" size={13} />New playlist
            </button>
          </div>

          {playlists.length === 0 ? (
            <p className="m-0 text-secondary text-[11px] leading-[1.7]">
              A playlist runs clips back to back, one ending as the next begins. Clips only - video and audio, never stills.
            </p>
          ) : (
            <ul className="list-none grid gap-1.5 m-0 p-0">
              {playlists.map((playlist) => {
                const items = playlist.mediaIds ?? [];
                const playing = Boolean(run?.active && run.playlistId === playlist.id);
                const editingThis = editing === playlist.id;
                if (editingThis) {
                  const available = studio.media.filter((clip) => (
                    (clip.kind === 'video' || clip.kind === 'audio') && !clip.missing && !items.includes(clip.id)
                  ));
                  return (
                    <li key={playlist.id} className="grid gap-2 px-2.5 py-2.5 border border-[#304C80]! rounded-md bg-app">
                      <div className="flex items-center gap-2">
                        <input
                          type="text"
                          value={draftName}
                          aria-label="Playlist name"
                          className="flex-1 min-w-0 h-7 px-2 border border-[#35445C] rounded-md bg-elevated text-text text-[11.5px] outline-none focus:border-blue"
                          onChange={(event) => setDraftName(event.target.value)}
                          onBlur={() => void savePlaylistName(playlist)}
                        />
                        <span className="text-secondary text-[10px] whitespace-nowrap">{items.length} {items.length === 1 ? 'clip' : 'clips'}</span>
                        <button
                          type="button"
                          className={smallButton}
                          onClick={() => {
                            setEditing(null);
                            setAddChoice('');
                            setDraftName(playlist.name);
                          }}
                        >
                          Done
                        </button>
                      </div>

                      {items.length === 0 ? (
                        <p className="m-0 text-secondary text-[11px]">No clips yet - add one below.</p>
                      ) : (
                        <ul className="list-none grid gap-1 m-0 p-0">
                          {items.map((mediaID, index) => {
                            const clip = studio.media.find((item) => item.id === mediaID);
                            return (
                              <li key={`${playlist.id}-${index}`} className="flex items-center gap-2 px-1.5 py-1 border border-border rounded-md bg-app min-w-0 text-[11px]">
                                <span className="w-4 shrink-0 text-secondary tabular-nums text-[10px]">{index + 1}</span>
                                <span className="shrink-0 text-[#4B5F7C]">
                                  <Icon name={clip?.kind === 'audio' ? 'music' : 'media'} size={13} />
                                </span>
                                <span className={`flex-1 min-w-0 truncate ${clip ? '' : 'text-secondary'}`} title={clip?.title}>
                                  {clip?.title ?? 'Unknown clip'}
                                  {clip?.missing && <span className="text-amber"> · missing</span>}
                                </span>
                                <button
                                  type="button"
                                  className="inline-grid place-items-center w-6 h-6 border-0 rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-40"
                                  disabled={!canAct || index === 0}
                                  aria-label={`Move ${clip?.title ?? 'clip'} up`}
                                  onClick={() => moveClip(playlist, index, -1)}
                                >
                                  <Icon name="chevron" size={12} className="rotate-180" />
                                </button>
                                <button
                                  type="button"
                                  className="inline-grid place-items-center w-6 h-6 border-0 rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-40"
                                  disabled={!canAct || index === items.length - 1}
                                  aria-label={`Move ${clip?.title ?? 'clip'} down`}
                                  onClick={() => moveClip(playlist, index, 1)}
                                >
                                  <Icon name="chevron" size={12} />
                                </button>
                                <button
                                  type="button"
                                  className="inline-grid place-items-center w-6 h-6 border-0 rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-error disabled:opacity-40"
                                  disabled={!canAct}
                                  aria-label={`Remove ${clip?.title ?? 'clip'} from ${playlist.name}`}
                                  onClick={() => void updatePlaylist(
                                    { id: playlist.id, name: playlist.name, mediaIds: items.filter((_, i) => i !== index) },
                                    `Removed ${clip?.title ?? 'clip'} from ${playlist.name}.`,
                                  )}
                                >
                                  <Icon name="close" size={12} />
                                </button>
                              </li>
                            );
                          })}
                        </ul>
                      )}

                      {available.length > 0 && (
                        <div className="flex items-center gap-2">
                          <select
                            className="flex-1 min-w-0 min-h-7 px-2 border border-[#35445C] rounded-md bg-elevated text-text text-[11px]"
                            aria-label="Clip to add to the playlist"
                            value={addChoice}
                            onChange={(event) => setAddChoice(event.target.value)}
                          >
                            <option value="">Add a clip…</option>
                            {available.map((clip) => (
                              <option key={clip.id} value={clip.id}>{clip.title}</option>
                            ))}
                          </select>
                          <button
                            type="button"
                            className={smallButton}
                            disabled={!canAct || addChoice === ''}
                            onClick={() => {
                              const chosen = studio.media.find((item) => item.id === addChoice);
                              setAddChoice('');
                              void updatePlaylist(
                                { id: playlist.id, name: playlist.name, mediaIds: [...items, addChoice] },
                                `Added ${chosen?.title ?? 'clip'} to ${playlist.name}.`,
                              );
                            }}
                          >
                            <Icon name="plus" size={13} />Add
                          </button>
                        </div>
                      )}
                    </li>
                  );
                }
                return (
                  <li key={playlist.id} className="flex items-center gap-2 px-2 py-1.5 border border-border rounded-md bg-app min-w-0">
                    {playing ? (
                      <button
                        type="button"
                        className="inline-grid place-items-center w-7 h-7 border border-[#3A222A] rounded-md bg-[#221019] text-error shrink-0"
                        aria-label={`Stop ${playlist.name}`}
                        title="Stop the run"
                        disabled={!canAct}
                        onClick={() => stopPlaylist(playlist)}
                      >
                        <Icon name="square" size={12} />
                      </button>
                    ) : (
                      <button
                        type="button"
                        className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-40 shrink-0"
                        aria-label={`Play ${playlist.name}`}
                        title={items.length === 0 ? 'Add clips first' : `Play ${playlist.name} from the top`}
                        disabled={!canAct || items.length === 0}
                        onClick={() => playPlaylist(playlist)}
                      >
                        <Icon name="play" size={12} />
                      </button>
                    )}
                    <strong className="min-w-0 truncate text-[11.5px] font-medium" title={playlist.name}>{playlist.name}</strong>
                    <span className="text-secondary text-[10px] whitespace-nowrap">
                      {items.length} {items.length === 1 ? 'clip' : 'clips'}
                      {playing && run ? ` · playing ${run.index + 1}/${run.length}` : ''}
                    </span>
                    <span className="flex-1" />
                    <button
                      type="button"
                      className={smallButton}
                      disabled={!canAct}
                      onClick={() => {
                        setEditing(playlist.id);
                        setDraftName(playlist.name);
                        setAddChoice('');
                      }}
                    >
                      Edit
                    </button>
                    <button
                      type="button"
                      className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-error disabled:opacity-40 shrink-0"
                      aria-label={`Delete playlist ${playlist.name}`}
                      title="Delete playlist"
                      disabled={!canAct}
                      onClick={() => void removePlaylist(playlist)}
                    >
                      <Icon name="close" size={13} />
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        {status && (
          <p className={`text-[11.5px] ${status.kind === 'error' ? 'text-error' : 'text-teal'}`} aria-live="polite">
            {status.text}
          </p>
        )}

        <p className="text-[#6F809B] text-[10.5px]">Click a tile to preview it, double-click to send it live. Sound plays on the audience display.</p>
      </div>
    </section>
  );
}
