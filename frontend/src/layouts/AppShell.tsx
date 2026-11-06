import { useEffect, useRef, useState } from 'react';
import type { FormEvent, ReactNode } from 'react';
import { Events } from '@wailsio/runtime';
import { BackendStatus } from '../components/BackendStatus';
import { Dropdown } from '../components/Dropdown';
import { Icon } from '../components/Icon';
import type { IconName } from '../components/Icon';
import { SlideMedia } from '../components/SlideMedia';
import { ThemedSlide } from '../components/ThemedSlide';
import { useStudio } from '../stores/StudioProvider';
import { ANALYZE_SECONDS, useDetection } from '../features/detection/DetectionProvider';
import { HistoryPanel } from '../features/presentation/HistoryPanel';
import { moveWithin, useDragOrder } from '../lib/useDragOrder';
import { App } from '../../bindings/zowe-worship';
import * as models from '../../bindings/zowe-worship/models';
import type { Content } from '../../bindings/zowe-worship/internal/presentation/models';
import type { Theme } from '../../bindings/zowe-worship/internal/themes/models';
import { Kind as ThemeKind } from '../../bindings/zowe-worship/internal/themes/models';

export type PageId = 'home' | 'bible' | 'songs' | 'media' | 'service';

export const navigation: { id: PageId; label: string; icon: IconName }[] = [
  { id: 'home',    label: 'Home',          icon: 'home' },
  { id: 'bible',   label: 'Bible',         icon: 'bible' },
  { id: 'songs',   label: 'Songs',         icon: 'music' },
  { id: 'media',   label: 'Media',         icon: 'media' },
  { id: 'service', label: 'Service Order', icon: 'order' },
];

function useClock() {
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), 30000);
    return () => window.clearInterval(timer);
  }, []);

  return now;
}

// Only scripture carries a translation, so the other kinds name themselves
// instead of leaving a blank line under the title.
function itemSubtitle(item: models.ServiceItemView): string {
  if (item.kind === 'media') return 'Media';
  if (item.kind === 'song') return 'Song';
  if (item.kind === 'text') return 'Text Slide';
  if (item.kind === 'announcement') return 'Announcement';
  if (item.kind === 'blank') return 'Blank Divider';
  return item.translationId ? `${item.translationId.toUpperCase()} · Scripture` : 'Scripture';
}

function ScheduleSidebar({ onNavigate }: { onNavigate: (page: PageId) => void }) {
  const studio = useStudio();
  const items = studio.order?.items ?? [];
  const service = studio.order?.service ?? null;
  const liveID = studio.presentation?.live?.itemId ?? '';
  const previewID = studio.presentation?.preview?.itemId ?? '';

  async function previewItem(itemID: string) {
    try {
      await studio.previewItem(itemID);
    } catch {
      // The studio error is shown below the schedule.
    }
  }

  async function sendItemLive(itemID: string) {
    try {
      await studio.previewItem(itemID);
      await studio.sendPreviewLive();
    } catch {
      // The studio error is shown below the schedule.
    }
  }

  async function moveItem(from: number, to: number) {
    try {
      await studio.reorderItems(moveWithin(items.map((item) => item.id), from, to));
    } catch {
      // The studio error is shown below the schedule.
    }
  }

  const drag = useDragOrder((from, to) => void moveItem(from, to));

  return (
    <aside className="col-start-1 row-span-full flex flex-col min-h-0 bg-panel border-r border-border" aria-label="Service schedule">
      <div className="flex items-center gap-[11px] h-(--header-height) px-[18px] shrink-0 border-b border-border max-[959px]:px-3 max-[959px]:gap-2">
        <span className="grid place-items-center w-8 h-8 border border-blue rounded-lg text-text bg-selected text-[25px] font-bold italic leading-none pb-1 max-[959px]:w-[29px] max-[959px]:h-[29px] max-[959px]:text-[22px]" aria-hidden="true">z</span>
        <div>
          <strong className="block text-[15px] tracking-[-0.3px] whitespace-nowrap max-[959px]:text-[13.5px]">Zowe Worship</strong>
          <span className="block text-secondary text-[8.5px] tracking-[1.6px] mt-0.5">BY ZOWESOFT</span>
        </div>
      </div>

      <div className="flex flex-col flex-1 min-h-0 overflow-y-auto pt-[18px] px-3 pb-0 max-[959px]:px-2">
        <section className="px-2 pt-0.5 pb-4 border-b border-border" aria-labelledby="service-summary-title">
          <p className="text-[9.5px] uppercase tracking-[1.3px] font-semibold text-secondary" id="service-summary-title">Current service</p>
          <strong className="block text-[12.5px] mt-1.5">{service?.name ?? 'No service selected'}</strong>
          <span className="block text-secondary text-[10.5px] leading-[1.6] mt-1">
            {service
              ? `${items.length} ${items.length === 1 ? 'item' : 'items'} scheduled`
              : 'Create a service to build the schedule.'}
          </span>
        </section>

        <section className="pt-[15px]" aria-labelledby="schedule-title">
          <div className="flex items-center justify-between gap-2 px-2 pb-2.5">
            <h2 id="schedule-title" className="text-[10px] uppercase tracking-[1.2px] text-secondary">Schedule</h2>
            <span className="text-secondary text-[9.5px]">{items.length} items</span>
          </div>

          {items.length === 0 ? (
            <div className="grid justify-items-center gap-[7px] p-[18px] px-3 border border-dashed border-border rounded-lg bg-app text-center">
              <span className="grid place-items-center w-[34px] h-[34px] border border-[#30466A] rounded-lg bg-selected text-[#8CABFF]"><Icon name="order" size={20} /></span>
              <p className="text-[11.5px] font-medium">No items scheduled.</p>
              <span className="text-secondary text-[10px] leading-[1.6]">Scripture, songs and media will appear here in service order.</span>
              <button type="button" className="w-full min-h-8 inline-flex items-center justify-center gap-1.5 px-3 py-1.5 border border-[#35445C] rounded-md bg-elevated text-[11px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950] mt-1" onClick={() => onNavigate('service')}>
                Open Service Order
              </button>
            </div>
          ) : (
            <ol className="list-none grid gap-1 m-0 p-0">
              {items.map((item, index) => {
                const isLive = liveID !== '' && liveID === item.id;
                const isPreview = previewID !== '' && previewID === item.id;
                const isDropTarget = drag.over === index && drag.dragging !== null && drag.dragging !== index;
                return (
                  <li
                    key={item.id}
                    className={[
                      'rounded-md cursor-pointer select-none',
                      drag.dragging === index ? 'opacity-40' : '',
                      isDropTarget ? 'shadow-[0_0_0_1px_var(--color-blue)]' : '',
                    ].filter(Boolean).join(' ')}
                    title={`Click to preview ${item.title} · double-click to send live · drag to reorder`}
                    onClick={() => void previewItem(item.id)}
                    onDoubleClick={() => void sendItemLive(item.id)}
                    {...drag.rowProps(index)}
                  >
                    <button
                      type="button"
                      aria-pressed={isPreview}
                      className={[
                        'w-full flex items-center gap-2 px-2 py-[7px] border rounded-md bg-transparent text-left text-inherit hover:bg-elevated',
                        isPreview ? 'border-[#304C80]! bg-selected!' : isLive ? 'border-[#1E4D40]! bg-[#0E2A24]!' : 'border-transparent',
                      ].filter(Boolean).join(' ')}
                    >
                      <span className="w-[11px] shrink-0 text-[#56698A] cursor-grab" aria-hidden="true"><Icon name="grip" size={13} /></span>
                      <span className="w-4 shrink-0 text-center text-secondary text-[9.5px]">{item.position}</span>
                      <span className="flex-1 min-w-0">
                        <strong className="block truncate text-[11px] font-medium">{item.title}</strong>
                        <span className="block mt-px text-secondary text-[9px]">{itemSubtitle(item)}</span>
                      </span>
                      {item.status === 'live' ? (
                        <span className="inline-flex items-center px-2 py-[3px] border border-[#1E4D40]! rounded-full bg-[#0E2A24]! text-[#6EE7C0]! text-[9px] whitespace-nowrap shrink-0">Live</span>
                      ) : item.status === 'next' ? (
                        <span className="inline-flex items-center px-2 py-[3px] border border-[#4D3F1E]! rounded-full bg-[#2B2413]! text-amber! text-[9px] whitespace-nowrap shrink-0">Next</span>
                      ) : item.status === 'completed' ? (
                        <span className="inline-flex items-center px-2 py-[3px] border border-[#2A3950] rounded-full bg-elevated text-secondary text-[9px] whitespace-nowrap shrink-0">Done</span>
                      ) : (
                        <span className="inline-flex items-center px-2 py-[3px] border border-[#2A3950]! rounded-full bg-[#151F2E]! text-[#7E92AB]! text-[9px] whitespace-nowrap shrink-0">Ready</span>
                      )}
                    </button>
                  </li>
                );
              })}
            </ol>
          )}

          {studio.error && <p className="mt-2.5 px-2.5 py-2 border border-[#5A2B35] rounded-md bg-[#1C1217] text-[#FFB1B7] text-[9.5px] leading-normal" role="status">{studio.error}</p>}
        </section>

        <div className="mt-auto pt-7">
          <BackendStatus />
          <p className="border-t border-border p-4 px-2 text-secondary text-[10px]">Scripture First. Always.</p>
        </div>
      </div>
    </aside>
  );
}

function ServiceSwitcher() {
  const studio = useStudio();
  const [name, setName] = useState('');
  const current = studio.order?.service ?? null;

  async function pick(serviceID: string, close: () => void) {
    close();
    try {
      await studio.useService(serviceID);
    } catch {
      // The studio error is shown in the sidebar.
    }
  }

  async function create(event: FormEvent<HTMLFormElement>, close: () => void) {
    event.preventDefault();
    const trimmed = name.trim();
    if (trimmed === '') return;
    try {
      await studio.createService(trimmed);
      setName('');
      close();
    } catch {
      // The studio error is shown in the sidebar.
    }
  }

  return (
    <Dropdown
      wrapperClassName="relative flex min-w-0"
      panelClassName="min-w-[240px]"
      trigger={({ open, toggle }) => (
        <button
          type="button"
          className="inline-flex items-center gap-1.5 p-0 border-0 bg-transparent text-text text-[15px] font-semibold tracking-[-0.2px] whitespace-nowrap disabled:text-text disabled:opacity-95 disabled:cursor-default hover:not-disabled:text-blue hover:not-disabled:[&_svg]:text-blue [&_svg]:text-secondary max-[959px]:text-[13px]"
          disabled={!studio.bridgeAvailable}
          aria-haspopup="true"
          aria-expanded={open}
          onClick={toggle}
        >
          <span>{current?.name ?? 'No service selected'}</span>
          <Icon name="chevron" size={14} />
        </button>
      )}
      menu={({ close }) => (
        <>
          {studio.services.map((service) => (
            <button
              key={service.id}
              type="button"
              role="menuitemradio"
              aria-checked={current?.id === service.id}
              className={[
                'flex items-center gap-2 p-[7px] px-2 border-0 rounded-md bg-transparent text-text text-[11.5px] text-left hover:bg-selected [&>span]:truncate [&_svg]:shrink-0',
                current?.id === service.id ? '[&_svg]:text-teal' : '[&_svg]:text-transparent',
              ].filter(Boolean).join(' ')}
              onClick={() => void pick(service.id, close)}
            >
              <Icon name="check" size={13} />
              <span>{service.name}</span>
            </button>
          ))}
          {studio.services.length === 0 && <p className="px-2 py-1.5 text-secondary text-[10.5px]">No saved services yet.</p>}

          <form className="flex gap-1.5 mt-1 pt-2 border-t border-border" onSubmit={(event) => void create(event, close)}>
            <input
              type="text"
              value={name}
              placeholder="New service"
              aria-label="New service name"
              autoComplete="off"
              onChange={(event) => setName(event.target.value)}
              className="flex-1 min-w-0 h-[30px] px-2.5 border border-[#35445C] rounded-md bg-app text-text text-[11.5px] focus:border-blue focus:outline-none placeholder:text-[#6F809B]"
            />
            <button type="submit" className="min-h-7 inline-flex items-center justify-center gap-1.5 px-2 py-1 border border-blue! rounded-md bg-blue! text-white! text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!" disabled={name.trim() === ''}>
              Add
            </button>
          </form>
        </>
      )}
    />
  );
}

function DisplaySelector() {
  const studio = useStudio();
  const display = studio.display;

  async function run(action: () => Promise<models.DisplayView>, close: () => void) {
    close();
    try {
      await action();
    } catch {
      // The studio error is shown in the sidebar.
    }
  }

  return (
    <Dropdown
      align="right"
      wrapperClassName="relative flex"
      panelClassName="min-w-[260px]"
      trigger={({ open, toggle }) => (
        <button
          type="button"
          className="inline-flex items-center gap-1.5 h-8 px-[9px] border border-[#2A3950] rounded-md bg-elevated text-text text-[10.5px] whitespace-nowrap disabled:text-secondary [&_svg:last-child]:text-secondary max-[1150px]:[&>span]:hidden"
          disabled={!studio.bridgeAvailable}
          aria-expanded={open}
          aria-haspopup="true"
          onClick={toggle}
          title={display?.open ? `Audience display on ${display.screenName}` : 'Choose a display for the audience'}
        >
          <Icon name="display" size={16} />
          <span>{display?.open ? display.screenName : 'No display'}</span>
          <Icon name="chevron" size={13} />
        </button>
      )}
      menu={({ close }) => (
        <>
          {studio.screens.map((screen) => (
            <button
              key={screen.id}
              type="button"
              role="menuitemradio"
              aria-checked={display?.open && display.screenId === screen.id}
              className={[
                'flex items-center gap-2 p-[7px] px-2 border-0 rounded-md bg-transparent text-text text-[11.5px] text-left hover:bg-selected [&>span]:truncate [&_svg]:shrink-0',
                display?.open && display.screenId === screen.id ? '[&_svg]:text-teal' : '[&_svg]:text-transparent',
              ].filter(Boolean).join(' ')}
              onClick={() => void run(() => studio.openDisplay(screen.id), close)}
            >
              <Icon name="check" size={13} />
              <span>{screen.name}</span>
            </button>
          ))}
          {studio.screens.length === 0 && (
            <p className="px-2 py-1.5 text-secondary text-[10.5px]">No second display detected. Connect a projector or monitor, then reopen this menu.</p>
          )}
          {display?.open && (
            <button
              type="button"
              className="flex items-center gap-2 p-[7px] px-2 border-0 rounded-md bg-transparent text-text text-[11.5px] text-left hover:bg-selected [&>span]:truncate [&_svg]:shrink-0 mt-1 pt-[9px] border-t! border-border! rounded-t-none! rounded-b-md! [&_svg]:text-error"
              onClick={() => void run(studio.closeDisplay, close)}
            >
              <Icon name="close" size={13} />
              <span>Close audience display</span>
            </button>
          )}
        </>
      )}
    />
  );
}

function AppHeader() {
  const studio = useStudio();
  const connected = studio.bridgeAvailable;
  const now = useClock();
  const dateLabel = now.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
  const timeLabel = now.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });

  return (
    <header className="col-start-2 row-start-1 flex items-center justify-between gap-[18px] px-5 border-b border-border bg-panel min-w-0 max-[959px]:px-3.5 max-[959px]:gap-2.5">
      <div className="flex items-center gap-[18px] min-w-0">
        <ServiceSwitcher />
        <div className="flex items-center gap-3.5 pl-[18px] border-l border-border text-secondary text-[10.5px] whitespace-nowrap max-[1000px]:hidden">
          <span>{dateLabel}</span>
          <span>{timeLabel}</span>
        </div>
        <span className={`inline-flex items-center gap-1.5 px-[9px] py-1 border rounded-[5px] text-[9.5px] whitespace-nowrap max-[959px]:px-[7px] ${connected ? 'border-[#1E4D40] bg-[#0E2A24] text-[#6EE7C0]' : 'border-[#4D3F1E]! bg-[#2B2413]! text-amber!'}`}>
          <span className={`inline-block w-1.5 h-1.5 rounded-full shrink-0 ${connected ? 'bg-teal' : 'bg-amber'}`} />{connected ? 'Desktop connected' : 'Browser preview'}
        </span>
      </div>

      <div className="flex items-center gap-[5px] shrink-0">
        <DisplaySelector />
        <button type="button" className="inline-grid place-items-center w-[34px] h-[34px] border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50" onClick={() => void studio.openSettings()} aria-label="Open settings" title="Settings">
          <Icon name="settings" size={18} />
        </button>
        <button type="button" className="inline-grid place-items-center w-[34px] h-[34px] border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50" disabled aria-label="Notifications unavailable" title="Notifications are not implemented.">
          <Icon name="bell" size={18} />
        </button>
        <div className="flex items-center gap-[9px] ml-1.5 pl-3.5 border-l border-border max-[959px]:pl-2 max-[959px]:ml-0.5" aria-label="Local operator">
          <span className="grid place-items-center w-[31px] h-[31px] border border-[#304C80] rounded-full bg-[#1D315A] text-[#AFC4FF]"><Icon name="user" size={17} /></span>
          <div className="max-[959px]:hidden">
            <strong className="block text-[10.5px] font-medium">Local operator</strong>
            <span className="block text-secondary text-[9.5px] mt-px">This device</span>
          </div>
        </div>
      </div>
    </header>
  );
}

function SlideContent({ content }: { content: Content }) {
  if (content.kind === 'image' || content.kind === 'video' || content.kind === 'audio') {
    // A studio pane is a monitor, never a second player: a clip shows its first
    // frame and stays quiet while the audience display carries the sound.
    return <SlideMedia content={content} player={false} />;
  }

  return (
    <ThemedSlide
      theme={content.theme}
      lines={content.lines ?? []}
      reference={content.label}
      urlFor={(mediaID) => `/media/file/${mediaID}`}
      className="w-full h-full"
    />
  );
}

// A slide of this shape is themed as this kind. Media fills its own screen, so
// it has no theme and no picker.
function themeKind(kind: string): ThemeKind | null {
  if (kind === 'scripture') return ThemeKind.KindScripture;
  if (kind === 'song') return ThemeKind.KindLyrics;
  return null;
}

function PreviewTheme({ preview }: { preview: Content }) {
  const studio = useStudio();
  const [themes, setThemes] = useState<Theme[] | null>(null);
  const [problem, setProblem] = useState<string | null>(null);

  useEffect(() => {
    App.ListThemes()
      .then((list) => setThemes(list ?? []))
      .catch((error: unknown) => setProblem(error instanceof Error ? error.message : String(error)));
  }, []);

  const kind = themeKind(preview.kind);
  const item = (studio.order?.items ?? []).find((candidate) => candidate.id === preview.itemId);
  const ofKind = (themes ?? []).filter((theme) => theme.kind === kind);
  if (kind === null || item === undefined || themes === null || ofKind.length === 0) return null;

  const fallback = ofKind.find((theme) => theme.isDefault);
  const itemID = item.id;
  const chosenTheme = item.themeId ?? '';

  async function choose(themeID: string) {
    setProblem(null);
    try {
      await studio.setItemTheme(itemID, themeID);
      // The pane shows the frozen slide, so it has to be rebuilt from the item
      // before the operator can see the change they just asked for.
      await studio.previewItem(itemID);
    } catch (error: unknown) {
      setProblem(error instanceof Error ? error.message : String(error));
    }
  }

  return (
    <div className="flex items-center gap-2.5 border-t border-border px-2.5 py-[7px]">
      <label className="flex items-center gap-[7px] text-secondary text-[9.5px] tracking-[0.7px] uppercase">
        <span>Theme</span>
        <select className="h-[26px] max-w-[170px] px-1.5 border border-[#35445C] rounded-md bg-app text-text text-[11px] normal-case tracking-normal" value={chosenTheme} onChange={(event) => void choose(event.target.value)}>
          <option value="">{fallback ? `Default — ${fallback.name}` : 'Kind default'}</option>
          {ofKind.map((theme) => <option key={theme.id} value={theme.id}>{theme.name}</option>)}
        </select>
      </label>
      <span className="ml-auto text-[#6F809B] text-[9.5px]">Goes on air when you send this slide live.</span>
      {problem && <span className="text-error text-[10px]">{problem}</span>}
    </div>
  );
}

function PresentationPanes() {
  const studio = useStudio();
  const preview = studio.presentation?.preview ?? null;
  const live = studio.presentation?.live ?? null;
  const scheduleCount = studio.order?.items?.length ?? 0;
  const liveIsMedia = live?.kind === 'video' || live?.kind === 'audio';
  const [playing, setPlaying] = useState(true);

  // Sending anything new starts a clip from the beginning, so the button goes
  // back to Pause without waiting to be told.
  useEffect(() => {
    setPlaying(true);
  }, [live?.source]);

  useEffect(() => Events.On('media-ended', () => setPlaying(false)), []);

  async function control(action: 'play' | 'pause' | 'restart') {
    try {
      await studio.mediaControl(action);
      setPlaying(action !== 'pause');
    } catch {
      // Reported through the studio error area.
    }
  }

  async function sendLive() {
    try {
      await studio.sendPreviewLive();
    } catch {
      // Reported through the studio error area.
    }
  }

  async function clearLive() {
    try {
      await studio.clearLive();
    } catch {
      // Reported through the studio error area.
    }
  }

  async function step(direction: 'next' | 'previous') {
    try {
      if (direction === 'next') {
        await studio.nextItem();
      } else {
        await studio.previousItem();
      }
    } catch {
      // Reported through the studio error area.
    }
  }

  return (
    <section className="grid grid-cols-2 gap-3.5" aria-label="Persistent presentation workspace">
      <section className="bg-panel border border-border rounded-[10px] overflow-hidden" aria-labelledby="preview-title">
        <div className="min-h-[46px] px-3 py-2 flex items-center justify-between gap-2.5 border-b border-border max-[959px]:px-2.5 max-[959px]:py-[7px]">
          <h2 id="preview-title" className="flex items-center gap-[7px] text-[10px] tracking-[1.1px] uppercase text-secondary [&_svg]:text-secondary"><span className="inline-block w-1.5 h-1.5 rounded-full bg-secondary" /><Icon name="preview" size={15} />Preview</h2>
          <div className="flex items-center gap-1.5">
            <button
              type="button"
              className="min-h-7 inline-flex items-center justify-center gap-1.5 px-2 py-1 border border-blue! rounded-md bg-blue! text-white! text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
              disabled={preview === null}
              onClick={() => void sendLive()}
              title={preview === null ? 'Choose scripture to preview first.' : 'Project the preview to the live output'}
            >
              <Icon name="arrow" size={13} />Send Live
            </button>
          </div>
        </div>

        {preview ? (
          <div className="aspect-[16/9] w-full bg-[#0B1321]! p-0! flex flex-col items-center justify-center text-center"><SlideContent content={preview} /></div>
        ) : (
          <div className="aspect-[16/9] w-full bg-[#0B1321]! px-3 py-[18px] flex flex-col items-center justify-center text-center [&>svg]:text-secondary [&>svg]:opacity-60 [&>svg]:mb-2.5 [&>strong]:text-secondary [&>strong]:text-[11.5px] [&>strong]:font-medium [&>p]:text-[10.5px] [&>p]:text-secondary [&>p]:mt-1 [&>p]:max-w-[260px] max-[959px]:[&>p]:max-w-[180px]">
            <Icon name="layers" size={30} />
            <strong>No content selected</strong>
            <p>Search scripture or choose a schedule item to prepare a slide.</p>
          </div>
        )}

        {preview && <PreviewTheme preview={preview} />}

        <div className="flex items-center gap-[7px] border-t border-border min-h-8 px-3 py-2 text-secondary text-[9.5px]">
          <span className="inline-block w-1 h-1 rounded-full bg-secondary shrink-0" />
          {preview ? `Preview holds ${preview.label}.` : 'Preview is not connected to the presentation controller.'}
        </div>
      </section>

      <section className="bg-panel border border-border rounded-[10px] overflow-hidden border-[#5A4120]!" aria-labelledby="live-title">
        <div className="min-h-[46px] px-3 py-2 flex items-center justify-between gap-2.5 border-b border-border max-[959px]:px-2.5 max-[959px]:py-[7px]">
          <h2 id="live-title" className="flex items-center gap-[7px] text-[10px] tracking-[1.1px] uppercase text-secondary [&_svg]:text-secondary"><span className="inline-block w-[7px] h-[7px] rounded-full bg-teal shadow-[0_0_0_3px_rgba(33,214,162,0.14)]" /><Icon name="display" size={15} />Live Output</h2>
          <div className="flex items-center gap-1.5">
            <span className={`inline-flex items-center px-2 py-[3px] border rounded-full text-[9px] whitespace-nowrap ${live ? 'border-[#1E4D40]! bg-[#0E2A24]! text-[#6EE7C0]!' : 'border-[#2A3950] bg-elevated text-secondary'}`}>{live ? 'On air' : 'Off air'}</span>
            <span className={`inline-flex items-center px-2 py-[3px] border rounded-full text-[9px] whitespace-nowrap ${studio.display?.open ? 'border-[#1E4D40]! bg-[#0E2A24]! text-[#6EE7C0]!' : 'border-[#2A3950] bg-elevated text-secondary'}`}>
              {studio.display?.open ? studio.display.screenName : 'No display'}
            </span>
          </div>
        </div>

        {live ? (
          <div className="aspect-[16/9] w-full bg-[#060B14] p-0! flex flex-col items-center justify-center text-center"><SlideContent content={live} /></div>
        ) : (
          <div className="aspect-[16/9] w-full bg-[#060B14] px-3 py-[18px] flex flex-col items-center justify-center text-center [&>svg]:text-secondary [&>svg]:opacity-60 [&>svg]:mb-2.5 [&>strong]:text-secondary [&>strong]:text-[11.5px] [&>strong]:font-medium [&>p]:text-[10.5px] [&>p]:text-secondary [&>p]:mt-1 [&>p]:max-w-[260px] max-[959px]:[&>p]:max-w-[180px]">
            <Icon name="display" size={30} />
            <strong>No output connected</strong>
            <p>An audience display will appear here.</p>
          </div>
        )}

        <div className="flex items-center justify-between gap-2.5 border-t border-border px-2.5 py-[7px]">
          <div className="flex items-center gap-[7px] border-0 min-h-0 p-0 text-secondary text-[9.5px]">
            <span className="inline-block w-1 h-1 rounded-full bg-secondary shrink-0" />
            {scheduleCount > 0 ? `${scheduleCount} ${scheduleCount === 1 ? 'item' : 'items'} in the schedule.` : 'No service order is running.'}
          </div>
          <div className="flex items-center gap-[3px]">
            {liveIsMedia && (
              <>
                <button
                  type="button"
                  className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50"
                  onClick={() => void control(playing ? 'pause' : 'play')}
                  aria-label={playing ? 'Pause the clip' : 'Play the clip'}
                  title={playing ? 'Pause' : 'Play'}
                >
                  <Icon name={playing ? 'pause' : 'play'} size={14} />
                </button>
                <button
                  type="button"
                  className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50"
                  onClick={() => void control('restart')}
                  aria-label="Restart the clip"
                  title="Restart from the beginning"
                >
                  <Icon name="restart" size={14} />
                </button>
              </>
            )}
            <button type="button" className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50 [&>svg]:rotate-90" disabled={scheduleCount === 0} onClick={() => void step('previous')} aria-label="Previous schedule item" title="Previous schedule item">
              <Icon name="chevron" size={14} />
            </button>
            <button type="button" className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50 [&>svg]:-rotate-90" disabled={scheduleCount === 0} onClick={() => void step('next')} aria-label="Next schedule item" title="Next schedule item">
              <Icon name="chevron" size={14} />
            </button>
            <button type="button" className="min-h-7 inline-flex items-center justify-center gap-1.5 px-2 py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]" disabled={live === null} onClick={() => void clearLive()} title="Clear the live output">Clear</button>
            <button type="button" className="min-h-7 inline-flex items-center justify-center gap-1.5 px-2 py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap text-[#FFB1B7]! border-[#3A222A]! bg-[#221019]! disabled:text-[#B77F87]! disabled:bg-[#1C1217]! disabled:border-[#3A222A]!" disabled title="Automatic projection is not implemented.">
              <Icon name="detection" size={12} />Emergency
            </button>
          </div>
        </div>
      </section>
    </section>
  );
}

function DetectionPanel() {
  const detection = useDetection();
  const [problem, setProblem] = useState<string | null>(null);

  async function run(action: () => Promise<unknown>) {
    setProblem(null);
    try {
      await action();
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    }
  }

  const status = detection.disabled
    ? 'Emergency disabled'
    : detection.listening
      ? 'Listening'
      : detection.engineConfigured
        ? 'Ready'
        : 'No engine';

  const statusTone = detection.disabled
    ? 'border-[#4D3F1E]! bg-[#2B2413]! text-amber!'
    : detection.listening
      ? 'border-[#1E4D40]! bg-[#0E2A24]! text-[#6EE7C0]!'
      : 'border-[#2A3950]! bg-elevated! text-secondary!';

  const deviceLabel = detection.listening
    ? (detection.devices.find((device) => device.deviceId === detection.deviceId)?.label
      || detection.devices[0]?.label
      || 'Default microphone')
    : 'Not listening';

  return (
    <aside className="min-w-0 bg-panel border border-border rounded-[10px]" aria-labelledby="detection-panel-title">
      <div className="min-h-11 px-3.5 py-2.5 flex items-center justify-between gap-2.5 border-b border-border">
        <h2 id="detection-panel-title" className="text-[11px] flex items-center gap-[7px] uppercase tracking-[0.7px] text-secondary [&_svg]:text-secondary"><Icon name="detection" size={16} />AI Scripture Detection</h2>
        <span className={`inline-flex px-2 py-[3px] border rounded-[5px] text-[9.5px] font-medium whitespace-nowrap ${statusTone}`}>{status}</span>
      </div>

      <div className="grid gap-3 p-3.5">
        <div className="flex items-center gap-2 text-secondary">
          <span className={`inline-block w-1.5 h-1.5 rounded-full shrink-0 ${detection.listening ? 'bg-[#6EE7C0]' : 'bg-secondary'}`} />
          <strong className="text-text text-[10.5px] font-medium">{detection.listening ? `Listening · ${detection.bufferedSeconds.toFixed(0)} s buffered` : 'Not listening'}</strong>
          <Icon name="detection" size={22} className={`ml-auto ${detection.listening ? 'text-teal' : 'text-teal opacity-[.65]'}`} />
        </div>

        <div className="grid gap-2">
          <div className="flex items-center gap-2.5 p-2.5 border border-border rounded-lg bg-[#111D30]">
            <span className="inline-grid place-items-center w-[34px] h-[34px] border border-[#3B355C]! bg-[#24203E]! text-[#B3A4FF]! rounded-lg"><Icon name="mic" size={18} /></span>
            <div className="flex flex-col gap-px min-w-0">
              <span className="text-secondary text-[9.5px]">Audio input</span>
              <strong className="text-[11px] font-medium truncate">{deviceLabel}</strong>
            </div>
            <span className="ml-auto inline-grid place-items-center w-14 h-1.5 rounded-full bg-[#1A2740] overflow-hidden" aria-hidden="true">
              <span className="h-full rounded-full bg-[#6EE7C0] transition-[width] duration-100" style={{ width: `${Math.round(detection.level * 100)}%` }} />
            </span>
          </div>
          <div className="flex gap-2">
            <button
              type="button"
              className={`min-h-7 inline-flex flex-1 items-center justify-center gap-1.5 px-2 py-1 border rounded-md text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! ${detection.listening ? 'border-[#35445C] bg-elevated' : 'border-blue! bg-blue! text-white! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!'}`}
              // The emergency switch stops new detections, so the button must
              // refuse to start the same way F8 does — but never block stopping,
              // or disabling while listening would trap the microphone on.
              disabled={detection.disabled && !detection.listening}
              title={detection.disabled ? 'Detection is disabled (F12) — re-enable it in Settings' : undefined}
              onClick={() => void run(detection.toggleListening)}
            >
              <Icon name={detection.listening ? 'pause' : 'mic'} size={13} />
              {detection.listening ? 'Stop listening' : 'Listen (F8)'}
            </button>
            <button
              type="button"
              className="min-h-7 inline-flex items-center justify-center gap-1.5 px-2 py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]!"
              disabled={!detection.listening || detection.analyzing || detection.disabled}
              onClick={() => void run(detection.analyze)}
            >
              {detection.analyzing ? 'Analyzing…' : `Analyze last ${ANALYZE_SECONDS} s`}
            </button>
          </div>
          <label className="min-h-7 inline-flex items-center justify-center gap-1.5 px-2 py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap text-text cursor-pointer has-disabled:opacity-50" title="Run a recording through the same pipeline — MP3, M4A or WAV (first 30 seconds)">
            <Icon name="play" size={13} />
            {detection.analyzing ? 'Analyzing…' : 'Test audio file…'}
            <input
              type="file"
              accept="audio/*,.mp3,.m4a,.wav,.ogg"
              className="sr-only"
              disabled={detection.analyzing || detection.disabled}
              onChange={(event) => {
                const file = event.target.files?.[0];
                event.target.value = '';
                if (file) void run(() => detection.analyzeFile(file));
              }}
            />
          </label>
          {detection.devices.length > 1 && (
            <select
              className="min-h-7 w-full min-w-0 px-2 border border-[#35445C] rounded-md bg-app text-[10.5px] text-text truncate"
              value={detection.deviceId}
              onChange={(event) => detection.setDevice(event.target.value)}
              aria-label="Audio input device"
            >
              <option value="">Default microphone</option>
              {detection.devices.map((device) => (
                <option key={device.deviceId} value={device.deviceId}>{device.label}</option>
              ))}
            </select>
          )}
        </div>

        <section className="grid gap-2 border-t border-border pt-[11px]" aria-labelledby="transcript-title">
          <div className="flex items-center justify-between gap-2">
            <h3 id="transcript-title" className="text-[9.5px] uppercase tracking-[1px] text-secondary">Current transcript</h3>
          </div>
          {detection.transcript ? (
            <p className="p-2.5 border border-border rounded-md bg-app text-text text-[10px] leading-[1.6] select-text">{detection.transcript}</p>
          ) : (
            <p className="p-2.5 border border-dashed border-border rounded-md bg-app text-secondary text-[10px] leading-[1.6]">
              {detection.listening ? 'Nothing analyzed yet. Let the preacher name a verse, then analyze.' : 'Start listening, then analyze a chunk to transcribe it.'}
            </p>
          )}
        </section>

        <section className="grid gap-2 border-t border-border pt-[11px]" aria-labelledby="queue-title">
          <div className="flex items-center justify-between gap-2">
            <h3 id="queue-title" className="text-[9.5px] uppercase tracking-[1px] text-secondary">Pending queue</h3>
            <span className="inline-grid place-items-center min-w-[18px] h-[18px] px-[5px] rounded-full bg-elevated text-[9.5px]">{detection.candidates.length}</span>
            {detection.candidates.length > 0 && (
              <button type="button" className="text-secondary text-[9.5px] hover:text-text" onClick={() => void run(detection.clear)}>Clear all</button>
            )}
          </div>
          {detection.candidates.length === 0 ? (
            <p className="p-2.5 border border-dashed border-border rounded-md bg-app text-secondary text-[10px] leading-[1.6]">Nothing waiting for confirmation.</p>
          ) : (
            <ul className="m-0 p-0 list-none grid gap-1.5">
              {detection.candidates.map((candidate) => (
                <li key={candidate.id} className="grid gap-1.5 p-2.5 border border-border rounded-[7px] bg-app">
                  <div className="flex items-baseline gap-2 min-w-0">
                    <Icon name="bible" size={14} className="text-[#B3A4FF] shrink-0 translate-y-px" />
                    <strong className="text-[11.5px] font-semibold truncate">{candidate.reference}</strong>
                    <span className="ml-auto text-secondary text-[9.5px] whitespace-nowrap">{candidate.createdAt}</span>
                  </div>
                  <p className="text-secondary text-[9.5px] leading-[1.5] line-clamp-2">“{candidate.transcript}”</p>
                  <div className="flex gap-1.5">
                    <button
                      type="button"
                      className="min-h-7 inline-flex flex-1 items-center justify-center gap-1.5 px-2 py-1 border border-blue! bg-blue! text-white! rounded-md text-[10.5px] font-medium whitespace-nowrap not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
                      onClick={() => void run(() => detection.approve(candidate.id))}
                    >
                      <Icon name="check" size={12} />Approve to preview
                    </button>
                    <button
                      type="button"
                      className="min-h-7 inline-flex items-center justify-center px-2 py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap"
                      onClick={() => void run(() => detection.dismiss(candidate.id))}
                    >
                      Dismiss
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </section>

        {problem !== null && <p className="text-error text-[10.5px] leading-[1.5]" role="alert">{problem}</p>}

        <div className="flex items-start gap-2 text-secondary">
          <Icon name="bible" size={16} className="text-[#B3A4FF] mt-0.5" />
          <p className="text-[10px] leading-[1.6]">Scripture first. A candidate is a suggestion: approving it fills the preview, and you send it live yourself.</p>
        </div>
      </div>
    </aside>
  );
}

function ProductionCard() {
  const studio = useStudio();
  const detection = useDetection();
  const preview = studio.presentation?.preview ?? null;
  const live = studio.presentation?.live ?? null;
  const scheduleCount = studio.order?.items?.length ?? 0;
  const deviceLabel = detection.devices.find((device) => device.deviceId === detection.deviceId)?.label
    || detection.devices[0]?.label
    || 'Default microphone';

  async function run(action: () => Promise<unknown>) {
    try {
      await action();
    } catch {
      // Reported through the studio error area.
    }
  }

  const iconBtn = 'inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50';

  return (
    <section className="bg-panel border border-border rounded-[10px]" aria-label="Production controls">
      <div className="min-h-11 px-3.5 py-2.5 flex items-center justify-between gap-2.5 border-b border-border">
        <h2 className="text-[11px] flex items-center gap-[7px] uppercase tracking-[0.7px] text-secondary [&_svg]:text-secondary"><Icon name="display" size={16} />Production</h2>
        <span className="inline-flex px-2 py-[3px] border border-[#2A3950]! rounded-[5px] bg-elevated! text-secondary! text-[9.5px] font-medium whitespace-nowrap">Manual</span>
      </div>

      <div className="grid gap-2.5 p-3">
        <button
          type="button"
          className="flex items-center gap-2.5 p-2.5 border border-border rounded-lg bg-[#111D30] text-left w-full"
          onClick={() => void run(detection.toggleListening)}
          title={detection.listening ? 'Stop listening (F8)' : 'Listen for scripture references (F8)'}
        >
          <span className="grid place-items-center w-[34px] h-[34px] border border-[#3B355C]! bg-[#24203E]! text-[#B3A4FF]! rounded-lg shrink-0"><Icon name="mic" size={18} /></span>
          <div className="flex flex-col gap-px min-w-0">
            <span className="text-secondary text-[9.5px]">Audio input</span>
            <strong className="text-[11px] font-medium truncate">{detection.listening ? deviceLabel : 'Not listening'}</strong>
          </div>
          <span className="ml-auto inline-grid place-items-center w-14 h-1.5 rounded-full bg-[#1A2740] overflow-hidden" aria-hidden="true">
            <span className="h-full rounded-full bg-[#6EE7C0] transition-[width] duration-100" style={{ width: `${Math.round(detection.level * 100)}%` }} />
          </span>
        </button>

        <div className="grid gap-1.5">
          <div className="flex items-center gap-2 px-[9px] py-[7px] border border-border rounded-[7px] bg-[#111D30] [&>svg]:text-secondary [&>div]:min-w-0 [&_strong]:block [&_strong]:text-[10.5px] [&_strong]:font-medium [&_strong]:leading-[1.5] [&_div>span]:block [&_div>span]:text-secondary [&_div>span]:text-[9.5px] [&_div>span]:mt-px">
            <Icon name="display" size={15} />
            <div><strong>Live output</strong><span>{live ? live.label : 'No content'}</span></div>
            <span className={`ml-auto inline-flex items-center px-2 py-[3px] border rounded-full text-[9px] whitespace-nowrap ${live ? 'border-[#1E4D40]! bg-[#0E2A24]! text-[#6EE7C0]!' : 'border-[#2A3950] bg-elevated text-secondary'}`}>{live ? 'On air' : 'Off air'}</span>
          </div>
          <div className="flex items-center gap-2 px-[9px] py-[7px] border border-border rounded-[7px] bg-[#111D30] [&>svg]:text-secondary [&>div]:min-w-0 [&_strong]:block [&_strong]:text-[10.5px] [&_strong]:font-medium [&_strong]:leading-[1.5] [&_div>span]:block [&_div>span]:text-secondary [&_div>span]:text-[9.5px] [&_div>span]:mt-px">
            <Icon name="preview" size={15} />
            <div><strong>Preview</strong><span>{preview ? preview.label : 'No content'}</span></div>
            <span className="ml-auto inline-flex items-center px-2 py-[3px] border border-[#4D3F1E]! rounded-full bg-[#2B2413]! text-amber! text-[9px] whitespace-nowrap">{preview ? 'Ready' : 'Idle'}</span>
          </div>
        </div>

        <div className="flex items-center gap-1">
          <button type="button" className={`${iconBtn} [&>svg]:rotate-90`} disabled={scheduleCount === 0} onClick={() => void run(() => studio.previousItem())} aria-label="Previous schedule item" title="Previous schedule item">
            <Icon name="chevron" size={15} />
          </button>
          <button type="button" className={`${iconBtn} [&>svg]:-rotate-90`} disabled={scheduleCount === 0} onClick={() => void run(() => studio.nextItem())} aria-label="Next schedule item" title="Next schedule item">
            <Icon name="chevron" size={15} />
          </button>
          <button type="button" className="min-h-7 inline-flex items-center justify-center gap-1.5 px-2 py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]" disabled={live === null} onClick={() => void run(studio.clearLive)} title="Clear the live output">Clear</button>
          <button type="button" className={`${iconBtn} ml-auto`} onClick={() => void run(studio.openSettings)} aria-label="Production settings" title="Open settings">
            <Icon name="settings" size={16} />
          </button>
        </div>

        <div className="flex items-center gap-[9px] pt-[9px] border-t border-border" title="Automatic projection is not implemented.">
          <span className="relative flex items-center w-[30px] h-4 rounded-full bg-[#2A3950] shrink-0" aria-hidden="true"><span className="w-3 h-3 rounded-full bg-secondary ml-0.5" /></span>
          <div className="min-w-0"><strong className="block text-[9.5px] font-medium">Auto Projection</strong><span className="flex items-center gap-1 text-secondary text-[9px] mt-px"><span className="inline-block w-[5px] h-[5px] rounded-full bg-secondary shrink-0" />Off</span></div>
        </div>

        <p className="text-secondary text-[9.5px] leading-[1.5]">Auto-projection arrives with offline scripture detection.</p>
      </div>
    </section>
  );
}

function WorkspaceTabs({ activePage, onNavigate }: {
  activePage: PageId;
  onNavigate: (page: PageId) => void;
}) {
  const studio = useStudio();

  const tabBase = 'inline-flex items-center gap-[7px] px-3 py-[7px] border border-transparent rounded-md bg-transparent text-secondary text-[11px] font-medium whitespace-nowrap transition-[background,color] duration-120 hover:bg-elevated hover:text-text max-[959px]:px-[9px] max-[959px]:py-1.5';
  const tabActive = 'bg-selected! border-[#304C80]! text-text! hover:bg-selected! hover:text-text!';

  return (
    <nav className="flex items-center gap-[3px] p-1 bg-panel border border-border rounded-lg overflow-x-auto" aria-label="Workspaces">
      {navigation.map((item) => (
        <button
          key={item.id}
          type="button"
          className={`${tabBase}${activePage === item.id ? ` ${tabActive} [&>svg]:text-[#8CABFF]` : ''}`}
          aria-current={activePage === item.id ? 'page' : undefined}
          onClick={() => onNavigate(item.id)}
        >
          <Icon name={item.icon} size={15} />
          <span>{item.label}</span>
        </button>
      ))}
      {/* Themes and Settings are windows of their own, so neither becomes the active page. */}
      <button type="button" className={tabBase} onClick={() => void studio.openThemes()}>
        <Icon name="palette" size={15} />
        <span>Themes</span>
      </button>
      <button type="button" className={tabBase} onClick={() => void studio.openSettings()}>
        <Icon name="settings" size={15} />
        <span>Settings</span>
      </button>
    </nav>
  );
}

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

function isTextInput(element: Element | null): boolean {
  if (!element) return false;
  const tagName = element.tagName.toLowerCase();
  if (tagName === 'input') {
    const type = (element as HTMLInputElement).type.toLowerCase();
    return !['checkbox', 'radio', 'button', 'submit', 'reset', 'range', 'color'].includes(type);
  }
  if (tagName === 'textarea') return true;
  if ((element as HTMLElement).isContentEditable) return true;
  return false;
}

export function AppShell({ activePage, onNavigate, children }: {
  activePage: PageId;
  onNavigate: (page: PageId) => void;
  children: ReactNode;
}) {
  const activeLabel = navigation.find((item) => item.id === activePage)?.label ?? 'Bible';
  const workspaceRef = useRef<HTMLDivElement>(null);
  const studio = useStudio();
  const detection = useDetection();
  const [shortcutNotice, setShortcutNotice] = useState<string | null>(null);
  const noticeTimer = useRef<number | null>(null);

  function flashNotice(msg: string) {
    if (noticeTimer.current !== null) window.clearTimeout(noticeTimer.current);
    setShortcutNotice(msg);
    noticeTimer.current = window.setTimeout(() => setShortcutNotice(null), 3000);
  }

  // Shortcut actions report their failure through the same notice bar, since
  // there is no panel under the operator's fingers when a key is pressed.
  async function run(action: () => Promise<unknown>, ok?: () => void) {
    try {
      await action();
      ok?.();
    } catch (error: unknown) {
      flashNotice(errorMessage(error));
    }
  }

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      const activeEl = document.activeElement;
      const inText = isTextInput(activeEl);

      // Alt+Enter: Project selected preview or candidate
      if (event.altKey && event.key === 'Enter') {
        event.preventDefault();
        if (studio.presentation?.preview) {
          void studio.sendPreviewLive();
          flashNotice('Preview sent live');
        } else {
          flashNotice('No preview ready to send live');
        }
        return;
      }

      // Ctrl+Shift+S: Open scripture search
      if ((event.ctrlKey || event.metaKey) && event.shiftKey && (event.key === 'S' || event.key === 's')) {
        event.preventDefault();
        onNavigate('bible');
        window.setTimeout(() => {
          const input = document.getElementById('bible-search-input') as HTMLInputElement | null;
          input?.focus();
          input?.select();
        }, 60);
        return;
      }

      // F8: Start/stop scripture listening
      if (event.key === 'F8') {
        event.preventDefault();
        // Stopping always works: if F12 disabled detection while the mic was
        // live, refusing the toggle would leave it capturing with no way off.
        if (detection.listening) {
          void detection.toggleListening();
          flashNotice('Stopped listening for scripture');
          return;
        }
        if (detection.disabled) {
          flashNotice('Detection is disabled — re-enable it in Settings');
          return;
        }
        void run(() => detection.toggleListening(), () => flashNotice('Listening for scripture references'));
        return;
      }

      // F9: Pause/resume automatic projection
      if (event.key === 'F9') {
        event.preventDefault();
        flashNotice('Automatic projection is not implemented — nothing to pause');
        return;
      }

      // F12: Emergency disable detection (per PRD: must not blank audience display)
      if (event.key === 'F12') {
        event.preventDefault();
        void run(() => App.SetDetectionDisabled(true), () => {
          void detection.refresh();
          flashNotice('Emergency: detection disabled (live display preserved)');
        });
        return;
      }

      // Left / Right: Previous / Next slide when not typing in text fields
      if (!inText) {
        if (event.key === 'ArrowLeft') {
          event.preventDefault();
          void studio.previousItem();
          return;
        }
        if (event.key === 'ArrowRight') {
          event.preventDefault();
          void studio.nextItem();
          return;
        }
      }

      // Escape: dismiss input focus; PRD: Escape must not clear live output by default
      if (event.key === 'Escape') {
        if (inText) {
          (activeEl as HTMLElement).blur();
        }
      }
    }

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [studio, detection, onNavigate]);

  useEffect(() => {
    workspaceRef.current?.scrollTo({ top: 0 });
  }, [activePage]);

  // Home has its own live hero so the presentation dock is redundant there.
  const showDock = activePage !== 'home';
  // Most workspaces grow as tall as their content and let this column scroll.
  // Bible is different: it keeps a search header on screen at all times and
  // scrolls only the reading pane under it. For that its pane needs a real
  // height to overflow, so the workspace is stretched to fill the column and
  // its own inner scroller does the scrolling — the column never moves.
  const selfScrolls = activePage === 'bible';

  return (
    <div className="min-w-[768px] grid grid-cols-[var(--sidebar-width)_minmax(0,1fr)] grid-rows-[var(--header-height)_minmax(0,1fr)] h-dvh overflow-hidden max-[1300px]:[--sidebar-width:212px] max-[959px]:[--sidebar-width:184px]">
      <a className="fixed z-10 top-3 left-3 px-4 py-2.5 bg-text text-app rounded-md -translate-y-[160%] focus:translate-y-0" href="#workspace">Skip to workspace</a>
      <ScheduleSidebar onNavigate={onNavigate} />
      <AppHeader />
      <main id="workspace" className="col-start-2 row-start-2 grid grid-cols-[minmax(0,1fr)_320px] grid-rows-[minmax(0,1fr)] items-stretch gap-[18px] p-[18px] overflow-hidden min-h-0 focus:outline-none focus:[outline-offset:-3px] min-[1600px]:grid-cols-[minmax(0,1fr)_352px] max-[1300px]:grid-cols-[minmax(0,1fr)_292px] max-[1300px]:gap-3.5 max-[1300px]:p-3.5 max-[1150px]:grid-cols-[minmax(0,1fr)_264px] max-[1150px]:gap-3 max-[1150px]:p-3 max-[959px]:grid-cols-[minmax(0,1fr)] max-[959px]:grid-rows-[auto] max-[959px]:p-3 max-[959px]:gap-3 max-[959px]:overflow-y-auto" tabIndex={-1} aria-label={`${activeLabel} workspace`}>
        <div className={`min-h-0 min-w-0 overflow-x-hidden max-[959px]:overflow-visible ${selfScrolls ? 'overflow-hidden flex flex-col' : 'overflow-y-auto'} `} ref={workspaceRef}>
          {showDock && (
            <div className="sticky top-0 z-4 grid gap-3.5 pb-3.5 bg-app after:absolute after:inset-x-0 after:bottom-[-8px] after:h-2 after:bg-linear-to-b after:from-app after:to-[rgba(8,15,28,0)] after:pointer-events-none max-[1300px]:gap-3 max-[1300px]:pb-3 max-[959px]:static max-[959px]:pb-0 max-[959px]:after:hidden">
              <PresentationPanes />
              <WorkspaceTabs activePage={activePage} onNavigate={onNavigate} />
            </div>
          )}
          {!showDock && (
            <div className="sticky top-0 z-4 grid gap-0 pb-2.5 bg-app max-[959px]:static max-[959px]:pb-0">
              <WorkspaceTabs activePage={activePage} onNavigate={onNavigate} />
            </div>
          )}
          <section className={selfScrolls ? 'min-w-0 flex-1 min-h-0 flex flex-col max-[959px]:block' : 'min-w-0'}>
            {children}
          </section>
        </div>
        <div className="grid gap-3.5 content-start min-h-0 min-w-0 overflow-y-auto max-[959px]:overflow-visible">
          <DetectionPanel />
          <HistoryPanel />
          <ProductionCard />
        </div>
      </main>
      {shortcutNotice && (
        <div className="fixed bottom-6 right-6 z-[1000] flex items-center gap-2.5 px-4 py-2.5 bg-elevated border border-border shadow-[0_10px_30px_rgba(0,0,0,0.45)] rounded-lg text-text text-[12px] font-medium animate-toast-in pointer-events-none" role="status">
          <Icon name="display" size={15} />
          <span>{shortcutNotice}</span>
        </div>
      )}
    </div>
  );
}
