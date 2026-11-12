import { useCallback, useEffect, useState } from 'react';
import { App } from '../../../bindings/zowe-worship';
import type * as models from '../../../bindings/zowe-worship/models';
import { useStudio } from '../../stores/StudioProvider';
import { Icon } from '../../components/Icon';
import type { IconName } from '../../components/Icon';

// The history is written server-side as each projection lands, so the panel
// re-reads it whenever presentation state is republished. A preview move carries
// no new row, so it costs one query that changes nothing — cheaper than tracking
// which changes write and missing an edge, such as the same slide sent twice.
export function HistoryPanel() {
  const studio = useStudio();
  const [entries, setEntries] = useState<models.ProjectionView[]>([]);
  const [problem, setProblem] = useState<string | null>(null);
  const [restoring, setRestoring] = useState<string | null>(null);
  const presentation = studio.presentation;
  // The history belongs to the service that is running, so it is re-read when
  // the operator switches to another one: the rows of the service before it
  // are not this service's rows.
  const serviceID = studio.order?.service?.id ?? '';

  const reload = useCallback(async () => {
    try {
      setEntries((await App.RecentProjections()) ?? []);
      setProblem(null);
    } catch (error: unknown) {
      setProblem(error instanceof Error ? error.message : String(error));
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload, presentation, serviceID]);

  // Restore puts a past slide back on the wall live. The presentation event
  // republished by the backend re-runs reload, so the row and the list refresh
  // on their own.
  async function restore(entry: models.ProjectionView) {
    setRestoring(entry.id);
    try {
      await App.RestoreProjection(entry.id);
      setProblem(null);
    } catch (error: unknown) {
      setProblem(error instanceof Error ? error.message : String(error));
    } finally {
      setRestoring(null);
    }
  }

  return (
    <aside className="min-w-0 bg-panel border border-border rounded-[10px]" aria-labelledby="history-panel-title">
      <div className="min-h-11 px-3.5 py-2.5 flex items-center justify-between gap-2.5 border-b border-border">
        <h2 id="history-panel-title" className="text-[11px] flex items-center gap-[7px] uppercase tracking-[0.7px] text-secondary [&_svg]:text-secondary">
          <Icon name="history" size={16} />Presentation history
        </h2>
        <span className="inline-grid place-items-center min-w-[18px] h-[18px] px-[5px] rounded-full bg-elevated text-[9.5px]">{entries.length}</span>
      </div>

      <div className="grid gap-2 p-3.5">
        {problem !== null && <p className="text-error text-[10.5px] leading-[1.5]" role="alert">{problem}</p>}

        {entries.length === 0 ? (
          <p className="p-2.5 border border-dashed border-border rounded-md bg-app text-secondary text-[10px] leading-[1.6]">
            Nothing projected yet. Slides appear here as you send them live.
          </p>
        ) : (
          <ul className="m-0 p-0 list-none grid gap-1.5 max-h-[min(42vh,360px)] overflow-y-auto">
            {entries.map((entry) => (
              <li key={entry.id} className="flex items-center gap-2 p-2 border border-border rounded-[7px] bg-app min-w-0">
                <Icon name={iconFor(entry.kind)} size={14} className={`shrink-0 ${toneFor(entry.kind)}`} />
                <div className="flex flex-1 w-0 flex-col gap-px min-w-0">
                  <strong className="text-[11px] font-medium truncate">{entry.title}</strong>
                  {entry.label !== '' && entry.label !== entry.title && (
                    <span className="text-secondary text-[9.5px] truncate">{entry.label}</span>
                  )}
                </div>
                <time
                  className="ml-auto text-secondary text-[9.5px] whitespace-nowrap tabular-nums"
                  dateTime={entry.projectedAt}
                  title={new Date(entry.projectedAt).toLocaleString()}
                >
                  {clockTime(entry.projectedAt)}
                </time>
                {entry.restorable && (
                  <button
                    type="button"
                    className="shrink-0 inline-grid place-items-center min-w-[18px] h-[18px] px-1 border border-[#35445C] rounded-[5px] bg-elevated text-secondary hover:not-disabled:bg-[#1C2A40] hover:not-disabled:text-text disabled:opacity-50"
                    disabled={restoring !== null}
                    onClick={() => void restore(entry)}
                    aria-label={`Send ${entry.title} live again`}
                    title={`Send ${entry.title} live again`}
                  >
                    <Icon name="display" size={11} />
                  </button>
                )}
              </li>
            ))}
          </ul>
        )}

        {entries.length > 0 && (
          <div className="flex items-start gap-2 text-secondary">
            <Icon name="history" size={16} className="mt-0.5" />
            <p className="text-[10px] leading-[1.6]">Newest first. Clearing is recorded too. The display icon sends that slide live again — schedule items only, since ad hoc slides have nothing stored to rebuild from.</p>
          </div>
        )}
      </div>
    </aside>
  );
}

// clockTime shows the wall time an operator recognises, with a fallback for a
// stamp the browser cannot parse rather than rendering "Invalid Date".
function clockTime(stamp: string): string {
  const when = new Date(stamp);
  if (Number.isNaN(when.getTime())) {
    return '—';
  }
  return when.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function iconFor(kind: string): IconName {
  switch (kind) {
    case 'song':
      return 'music';
    case 'media':
    case 'image':
    case 'video':
    case 'audio':
      return 'media';
    case 'text':
      return 'order';
    case 'announcement':
      return 'bell';
    case 'blank':
    case 'clear':
      return 'square';
    default:
      return 'bible';
  }
}

function toneFor(kind: string): string {
  switch (kind) {
    case 'clear':
    case 'blank':
      return 'text-secondary';
    case 'song':
      return 'text-teal';
    case 'media':
    case 'image':
    case 'video':
    case 'audio':
      return 'text-blue';
    default:
      return 'text-[#B3A4FF]';
  }
}
