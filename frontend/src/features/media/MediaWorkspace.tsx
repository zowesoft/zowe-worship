import { useEffect, useMemo, useState } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';
import { App } from '../../../bindings/zowe-worship';
import type { MediaImportResult, MediaView } from '../../../bindings/zowe-worship/models';
import type { Aspect } from '../../../bindings/zowe-worship/internal/media/models';
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

export function MediaWorkspace() {
  const studio = useStudio();
  const [filter, setFilter] = useState<Filter>('all');
  const [query, setQuery] = useState('');
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState<{ kind: 'message' | 'error'; text: string } | null>(null);

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
            <p>Use Add media to choose images, video or audio from any folder. Zowe keeps its own copy, so moving a file later cannot break a service.</p>
            <p className="text-[10.5px]! text-[#6F809B]!">Video: MP4, MOV, WebM · Images: JPG, PNG, GIF, WebP, BMP · Audio: MP3, M4A, WAV, OGG, FLAC</p>
          </div>
        ) : shown.length === 0 ? (
          <p className="p-[22px] text-secondary text-[11.5px] text-center">Nothing matches this filter.</p>
        ) : (
          <ul className="list-none grid grid-cols-[repeat(auto-fill,minmax(178px,1fr))] gap-3 m-0 p-0">
            {shown.map((item) => {
              const projectable = !item.missing;
              const framable = projectable && item.kind === 'image';
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
                    {framable && (
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
