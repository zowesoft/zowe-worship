import { useMemo, useState } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';
import { App } from '../../../bindings/zowe-worship';
import type { MediaImportResult, MediaView } from '../../../bindings/zowe-worship/models';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import './media.css';

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

function formatSize(bytes: number): string {
  const mb = bytes / (1024 * 1024);
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`;
  return `${Math.max(1, Math.round(mb))} MB`;
}

function kindIcon(kind: string) {
  if (kind === 'video') return <Icon name="media" size={26} />;
  if (kind === 'audio') return <Icon name="music" size={26} />;
  return <Icon name="layers" size={26} />;
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

  function sendLive(item: MediaView) {
    return act(async () => {
      await studio.previewMedia(item.id);
      await studio.projectMedia(item.id);
    }, `${item.title} is live.`);
  }

  async function remove(item: MediaView) {
    await act(() => studio.deleteMedia(item.id), `Removed ${item.title}.`);
  }

  return (
    <section className="panel media-workspace" aria-labelledby="media-title">
      <div className="panel-heading">
        <h2 id="media-title"><Icon name="media" size={16} />Media</h2>
        <div className="media-heading-actions">
          <span className="quiet-label">{studio.media.length} items</span>
          <button
            type="button"
            className="button button--primary button--small"
            disabled={!studio.bridgeAvailable || busy}
            onClick={() => void chooseFiles()}
          >
            <Icon name="plus" size={14} />Add media
          </button>
        </div>
      </div>

      <div className="media-body">
        <div className="media-toolbar">
          <div className="media-search">
            <Icon name="search" size={15} />
            <input
              type="text"
              value={query}
              placeholder="Search by name"
              aria-label="Search media by name"
              autoComplete="off"
              onChange={(event) => setQuery(event.target.value)}
            />
          </div>
          <div className="media-filters" role="group" aria-label="Filter by type">
            {filters.map((option) => (
              <button
                key={option.value}
                type="button"
                className={`media-filter${filter === option.value ? ' media-filter--active' : ''}`}
                aria-pressed={filter === option.value}
                onClick={() => setFilter(option.value)}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>

        {studio.media.length === 0 ? (
          <div className="media-empty">
            <span className="icon-tile"><Icon name="media" size={24} /></span>
            <h3>No media yet</h3>
            <p>Use Add media to choose images, video or audio from any folder. Zowe keeps its own copy, so moving a file later cannot break a service.</p>
            <p className="media-formats">Video: MP4, MOV, WebM · Images: JPG, PNG, GIF, WebP, BMP · Audio: MP3, M4A, WAV, OGG, FLAC</p>
          </div>
        ) : shown.length === 0 ? (
          <p className="media-none">Nothing matches this filter.</p>
        ) : (
          <ul className="media-grid">
            {shown.map((item) => {
              const projectable = item.kind === 'image';
              return (
                <li
                  key={item.id}
                  className={[
                    'media-tile',
                    item.missing ? 'media-tile--missing' : '',
                    projectable ? 'media-tile--projectable' : '',
                  ].filter(Boolean).join(' ')}
                  title={projectable
                    ? `Click to preview ${item.title} · double-click to send live`
                    : 'Video and audio playback arrive with the next slice'}
                  onClick={(event) => {
                    if (!projectable || fromActions(event) || !canAct) return;
                    void act(() => studio.previewMedia(item.id), `Previewing ${item.title}.`);
                  }}
                  onDoubleClick={(event) => {
                    if (!projectable || fromActions(event) || !canAct) return;
                    void sendLive(item);
                  }}
                >
                  <div className="media-thumb">
                    {item.hasThumbnail
                      ? <img src={item.thumbnailUrl} alt="" loading="lazy" />
                      : <span className="media-thumb-placeholder">{kindIcon(item.kind)}</span>}
                    {formatDuration(item.durationMs) && <span className="media-duration">{formatDuration(item.durationMs)}</span>}
                    {item.missing && <span className="status-pill status-pill--warn">Missing</span>}
                  </div>
                  <div className="media-meta">
                    <strong title={item.title}>{item.title}</strong>
                    <span>{item.kind} · {formatSize(item.sizeBytes)}</span>
                  </div>
                  <div className="media-tile-actions">
                    {projectable && (
                      <button
                        type="button"
                        className="button button--small media-add"
                        disabled={!canAct}
                        onClick={(event) => {
                          event.stopPropagation();
                          void act(() => studio.addMediaToService(item.id), `Added ${item.title} to the service.`);
                        }}
                      >
                        <Icon name="plus" size={13} />Add to Service
                      </button>
                    )}
                    <button
                      type="button"
                      className="icon-button icon-button--small"
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
          <p className={`media-status${status.kind === 'error' ? ' media-status--error' : ''}`} aria-live="polite">
            {status.text}
          </p>
        )}

        <p className="media-note">Images preview and project now. Video and audio playback arrive with the next slice.</p>
      </div>
    </section>
  );
}
