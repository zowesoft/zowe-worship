import { useEffect, useRef, useState } from 'react';
import type { FormEvent, KeyboardEvent as ReactKeyboardEvent, ReactNode } from 'react';
import { Events } from '@wailsio/runtime';
import { BackendStatus } from '../components/BackendStatus';
import { Dropdown } from '../components/Dropdown';
import { Icon } from '../components/Icon';
import type { IconName } from '../components/Icon';
import { SlideMedia } from '../components/SlideMedia';
import { ThemedSlide } from '../components/ThemedSlide';
import { useStudio, type MediaAction } from '../stores/StudioProvider';
import { useTransportPlayer } from '../lib/mediaTransport';
import { ANALYZE_SECONDS, useDetection } from '../features/detection/DetectionProvider';
import type { BiblePassage } from '../features/bible/focus';
import { useShortcuts } from '../stores/ShortcutsProvider';
import { TEXT_GUARDED, actionForCombo, comboFromEvent } from '../lib/shortcuts';
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

// Only scripture carries a translation; the other kinds name themselves.
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
    } catch {}
  }

  async function sendItemLive(itemID: string) {
    try {
      await studio.previewItem(itemID);
      await studio.sendPreviewLive();
    } catch {}
  }

  async function moveItem(from: number, to: number) {
    try {
      await studio.reorderItems(moveWithin(items.map((item) => item.id), from, to));
    } catch {}
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
    } catch {}
  }

  async function create(event: FormEvent<HTMLFormElement>, close: () => void) {
    event.preventDefault();
    const trimmed = name.trim();
    if (trimmed === '') return;
    try {
      await studio.createService(trimmed);
      setName('');
      close();
    } catch {}
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
    } catch {}
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
          onClick={() => {
            // A monitor replugged while the menu was closed only shows up
            // if the list is reread on the way in.
            if (!open) void studio.refreshScreens().catch(() => undefined);
            toggle();
          }}
          title={
            display?.open
              ? `Audience display on ${display.screenName}`
              : display?.wanted
                ? `${display.screenName} is disconnected — reopen this menu to reconnect`
                : 'Choose a display for the audience'
          }
        >
          <Icon name="display" size={16} />
          <span>{display?.open ? display.screenName : 'No display'}</span>
          <Icon name="chevron" size={13} />
        </button>
      )}
      menu={({ close }) => (
        <>
          {display && !display.open && display.wanted && (
            <button
              type="button"
              className="flex items-center gap-2 p-[7px] px-2 mb-1 pb-[9px] border-0 border-b! border-border! rounded-md rounded-t-md! rounded-b-none! bg-transparent text-text text-[11.5px] text-left hover:bg-selected [&>span]:truncate [&_svg]:shrink-0 [&_svg]:text-teal"
              onClick={() => void run(studio.reconnectDisplay, close)}
            >
              <Icon name="link" size={13} />
              <span>Reconnect to {display.screenName}</span>
            </button>
          )}
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

// Media fills its own screen, so it has no theme and no picker.
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
      // The pane shows a frozen slide, so it must be rebuilt from the item for
      // the change to appear.
      await studio.previewItem(itemID);
    } catch (error: unknown) {
      setProblem(error instanceof Error ? error.message : String(error));
    }
  }

  return (
    <div className="flex items-center gap-2.5 border-t border-border px-2.5 py-[5px]">
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
  // The emergency button is the same F12 switch the detection panel uses.
  const detection = useDetection();
  const preview = studio.presentation?.preview ?? null;
  const live = studio.presentation?.live ?? null;
  const scheduleCount = studio.order?.items?.length ?? 0;
  const liveIsMedia = live?.kind === 'video' || live?.kind === 'audio';
  const previewIsMedia = preview?.kind === 'video' || preview?.kind === 'audio';
  const [playing, setPlaying] = useState(true);
  // The clip clock reported by the audience player, for the transport bar.
  const [clock, setClock] = useState({ timeMS: 0, durationMS: 0 });
  // The room volume persists across clips; the display re-applies the last
  // commanded value when a new element starts playing.
  const [volume, setVolume] = useState(100);

  // With no display connected the studio's own live pane is the player, so the
  // room clip is still watchable and the transport still works.
  const liveMediaRef = useRef<HTMLMediaElement>(null);
  useTransportPlayer(liveMediaRef, live?.source);

  // The preview player is local: it never reaches the room and keeps its own
  // clock, so the operator can check a clip before sending it live.
  const previewMediaRef = useRef<HTMLMediaElement>(null);
  const [previewPlaying, setPreviewPlaying] = useState(false);
  const [previewClock, setPreviewClock] = useState({ timeMS: 0, durationMS: 0 });

  useEffect(() => {
    setPreviewPlaying(false);
    setPreviewClock({ timeMS: 0, durationMS: 0 });
  }, [preview?.source]);

  useEffect(() => {
    const element = previewMediaRef.current;
    if (!element) return;

    const tick = () => setPreviewClock({
      timeMS: Math.round(element.currentTime * 1000),
      durationMS: Number.isFinite(element.duration) ? Math.round(element.duration * 1000) : 0,
    });
    const markPlaying = () => setPreviewPlaying(true);
    const markStopped = () => setPreviewPlaying(false);
    element.addEventListener('timeupdate', tick);
    element.addEventListener('loadedmetadata', tick);
    element.addEventListener('play', markPlaying);
    element.addEventListener('pause', markStopped);
    element.addEventListener('ended', markStopped);
    return () => {
      element.removeEventListener('timeupdate', tick);
      element.removeEventListener('loadedmetadata', tick);
      element.removeEventListener('play', markPlaying);
      element.removeEventListener('pause', markStopped);
      element.removeEventListener('ended', markStopped);
    };
  }, [preview?.source]);

  function runPreview(action: 'play' | 'pause' | 'seek', value?: number) {
    const element = previewMediaRef.current;
    if (!element) return;
    if (action === 'play') element.play().catch(() => undefined);
    if (action === 'pause') element.pause();
    if (action === 'seek' && Number.isFinite(element.duration)) {
      element.currentTime = Math.min(value ?? 0, 1) * element.duration;
    }
  }

  // A new clip starts from the beginning, so the button returns to Pause and
  // the slider to zero.
  useEffect(() => {
    setPlaying(true);
    setClock({ timeMS: 0, durationMS: 0 });
  }, [live?.source]);

  useEffect(() => Events.On('media-ended', () => setPlaying(false)), []);

  useEffect(() => Events.On('media-progress', (event) => {
    const progress = event.data as { timeMs: number; durationMs: number };
    setClock({ timeMS: progress.timeMs, durationMS: progress.durationMs });
  }), []);

  async function control(action: MediaAction, value?: number) {
    try {
      await studio.mediaControl(action, value);
      if (action === 'play' || action === 'restart') setPlaying(true);
      if (action === 'pause' || action === 'stop') setPlaying(false);
      if (action === 'volume' && value !== undefined) setVolume(value);
    } catch {}
  }

  async function sendLive() {
    try {
      await studio.sendPreviewLive();
    } catch {}
  }

  async function clearLive() {
    try {
      await studio.clearLive();
    } catch {}
  }

  async function step(direction: 'next' | 'previous') {
    try {
      if (direction === 'next') {
        await studio.nextItem();
      } else {
        await studio.previousItem();
      }
    } catch {}
  }

  return (
    <section className="grid grid-cols-2 gap-3.5" aria-label="Persistent presentation workspace">
      <section className="bg-panel border border-border rounded-[10px] overflow-hidden" aria-labelledby="preview-title">
        <div className="min-h-[40px] px-3 py-1 flex items-center justify-between gap-2.5 border-b border-border max-[959px]:px-2.5 max-[959px]:py-[7px]">
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

        {/* The screen is a wide strip, not a 16:9 miniature: its
            height is capped so the panes give the workspace below
            (Bible results, schedule) its vertical room back, while the
            width fills the panel so the pane reads as a wide rectangle
            instead of a letterboxed square. ThemedSlide sizes text in
            container units, so the strip shows the same theme, colors
            and wording as the air slide at thumbnail scale; wraps can
            differ, so the live output stays the source of truth. */}
        {preview ? (
          <div className="aspect-[16/9] w-full max-h-[128px] bg-[#0B1321]! p-0! flex flex-col items-center justify-center text-center">
            {previewIsMedia
              ? <SlideMedia content={preview} player autoPlay={false} mediaRef={previewMediaRef} />
              : <SlideContent content={preview} />}
          </div>
        ) : (
          <div className="aspect-[16/9] w-full max-h-[128px] bg-[#0B1321]! px-3 py-[18px] flex flex-col items-center justify-center text-center [&>svg]:text-secondary [&>svg]:opacity-60 [&>svg]:mb-2.5 [&>strong]:text-secondary [&>strong]:text-[11.5px] [&>strong]:font-medium [&>p]:text-[10.5px] [&>p]:text-secondary [&>p]:mt-1 [&>p]:max-w-[260px] max-[959px]:[&>p]:max-w-[180px]">
            <Icon name="layers" size={30} />
            <strong>No content selected</strong>
            <p>Search scripture or choose a schedule item to prepare a slide.</p>
          </div>
        )}

        {previewIsMedia && (
          <div className="border-t border-border px-2.5 py-1.5 flex items-center gap-2 text-secondary text-[9.5px] tabular-nums">
            <button
              type="button"
              className="inline-grid place-items-center w-7 h-7 shrink-0 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text"
              onClick={() => void runPreview(previewPlaying ? 'pause' : 'play')}
              aria-label={previewPlaying ? 'Pause the preview' : 'Play the preview'}
            >
              <Icon name={previewPlaying ? 'pause' : 'play'} size={13} />
            </button>
            <span className="w-[38px] shrink-0 text-right">{clockTime(previewClock.timeMS)}</span>
            <input
              type="range"
              className="flex-1 min-w-0 h-1 accent-blue"
              min={0}
              max={1}
              step={0.01}
              value={previewClock.durationMS > 0 ? previewClock.timeMS / previewClock.durationMS : 0}
              disabled={previewClock.durationMS <= 0}
              onChange={(event) => runPreview('seek', Number(event.target.value))}
              aria-label="Seek in the preview"
            />
            <span className="w-[38px] shrink-0">{clockTime(previewClock.durationMS)}</span>
          </div>
        )}

        {preview && <PreviewTheme preview={preview} />}

        <div className="flex items-center gap-[7px] border-t border-border min-h-7 px-3 py-1 text-secondary text-[9.5px]">
          <span className="inline-block w-1 h-1 rounded-full bg-secondary shrink-0" />
          {preview ? `Preview holds ${preview.label}.` : 'Preview is not connected to the presentation controller.'}
        </div>
      </section>

      <section className="bg-panel border border-border rounded-[10px] overflow-hidden border-[#5A4120]!" aria-labelledby="live-title">
        <div className="min-h-[40px] px-3 py-1 flex items-center justify-between gap-2.5 border-b border-border max-[959px]:px-2.5 max-[959px]:py-[7px]">
          <h2 id="live-title" className="flex items-center gap-[7px] text-[10px] tracking-[1.1px] uppercase text-secondary [&_svg]:text-secondary"><span className="inline-block w-[7px] h-[7px] rounded-full bg-teal shadow-[0_0_0_3px_rgba(33,214,162,0.14)]" /><Icon name="display" size={15} />Live Output</h2>
          <div className="flex items-center gap-1.5">
            <span className={`inline-flex items-center px-2 py-[3px] border rounded-full text-[9px] whitespace-nowrap ${live ? 'border-[#1E4D40]! bg-[#0E2A24]! text-[#6EE7C0]!' : 'border-[#2A3950] bg-elevated text-secondary'}`}>{live ? 'On air' : 'Off air'}</span>
            <span className={`inline-flex items-center px-2 py-[3px] border rounded-full text-[9px] whitespace-nowrap ${studio.display?.open ? 'border-[#1E4D40]! bg-[#0E2A24]! text-[#6EE7C0]!' : 'border-[#2A3950] bg-elevated text-secondary'}`}>
              {studio.display?.open ? studio.display.screenName : 'No display'}
            </span>
          </div>
        </div>

        {live ? (
          <div className="aspect-[16/9] w-full max-h-[128px] bg-[#060B14] p-0! flex flex-col items-center justify-center text-center">
            {liveIsMedia && !studio.display?.open
              ? <SlideMedia content={live} player mediaRef={liveMediaRef} onEnded={() => void App.MediaEnded()} />
              : <SlideContent content={live} />}
          </div>
        ) : (
          <div className="aspect-[16/9] w-full max-h-[128px] bg-[#060B14] px-3 py-[18px] flex flex-col items-center justify-center text-center [&>svg]:text-secondary [&>svg]:opacity-60 [&>svg]:mb-2.5 [&>strong]:text-secondary [&>strong]:text-[11.5px] [&>strong]:font-medium [&>p]:text-[10.5px] [&>p]:text-secondary [&>p]:mt-1 [&>p]:max-w-[260px] max-[959px]:[&>p]:max-w-[180px]">
            <Icon name="display" size={30} />
            <strong>No output connected</strong>
            <p>An audience display will appear here.</p>
          </div>
        )}

        {liveIsMedia && (
          <div className="border-t border-border px-2.5 py-1.5 flex items-center gap-2 text-secondary text-[9.5px] tabular-nums">
            <span className="w-[38px] shrink-0 text-right">{clockTime(clock.timeMS)}</span>
            <input
              type="range"
              className="flex-1 min-w-0 h-1 accent-blue"
              min={0}
              max={1}
              step={0.01}
              value={clock.durationMS > 0 ? clock.timeMS / clock.durationMS : 0}
              disabled={clock.durationMS <= 0}
              onChange={(event) => void control('seek', Number(event.target.value))}
              aria-label="Seek in the clip"
            />
            <span className="w-[38px] shrink-0">{clockTime(clock.durationMS)}</span>
            <button
              type="button"
              className="inline-grid place-items-center w-7 h-7 shrink-0 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text"
              disabled={clock.timeMS === 0 && clock.durationMS === 0}
              onClick={() => void control('stop')}
              aria-label="Stop the clip"
              title="Stop and return to the start"
            >
              <Icon name="square" size={13} />
            </button>
            <Icon name="volume" size={12} className="shrink-0" />
            <input
              type="range"
              className="w-[54px] shrink-0 h-1 accent-teal"
              min={0}
              max={100}
              step={1}
              value={volume}
              onChange={(event) => void control('volume', Number(event.target.value))}
              aria-label="Room volume"
            />
          </div>
        )}

        <div className="flex items-center justify-between gap-2.5 border-t border-border px-2.5 py-1">
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
            <button type="button" className="min-h-7 inline-flex items-center justify-center gap-1.5 px-2 py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap text-[#FFB1B7]! border-[#3A222A]! bg-[#221019]! disabled:text-[#B77F87]! disabled:bg-[#1C1217]! disabled:border-[#3A222A]!" disabled={detection.disabled} onClick={() => { void App.SetDetectionDisabled(true).then(() => detection.refresh()).catch(() => undefined); }} title="Stop new detections without changing what the audience sees (F12)">
              <Icon name="detection" size={12} />Emergency
            </button>
          </div>
        </div>
      </section>
    </section>
  );
}

// confidenceLabel spells the band out, because color alone must not carry the
// reading (PRD accessibility rule).
function confidenceLabel(band: string): string {
  if (band === 'high') return 'High';
  if (band === 'medium') return 'Medium';
  return 'Low';
}

function confidenceTone(band: string): string {
  if (band === 'high') return 'border-[#1E4D40]! bg-[#0E2A24]! text-[#6EE7C0]!';
  if (band === 'medium') return 'border-[#4D3F1E]! bg-[#2B2413]! text-amber!';
  return 'border-[#2A3950]! bg-elevated! text-secondary!';
}

function DetectionPanel({ onOpenBible }: { onOpenBible: (passage: BiblePassage) => void }) {
  const detection = useDetection();
  const [problem, setProblem] = useState<string | null>(null);
  // The transcript can fill the whole sidebar during a sermon; collapsed by
  // default so the queue stays in view, with a one-line tail as live preview.
  const [transcriptOpen, setTranscriptOpen] = useState(false);
  const transcriptRef = useRef<HTMLParagraphElement>(null);

  useEffect(() => {
    if (transcriptOpen && detection.analyzing && transcriptRef.current) {
      transcriptRef.current.scrollTop = transcriptRef.current.scrollHeight;
    }
  }, [detection.transcript, transcriptOpen, detection.analyzing]);

  // Keyboard path through the queue: the list takes focus, arrows move the
  // highlight and one key acts on it, so a reference can be served while the
  // operator's hands are on the keyboard rather than hunting for a button.
  const queueRef = useRef<HTMLUListElement>(null);
  const [queueIndex, setQueueIndex] = useState(0);
  const queueLength = detection.candidates.length;
  // Clamped, not stored: an action removes the highlighted card, and the next
  // render must not point past the end of the list.
  const activeIndex = queueLength === 0 ? -1 : Math.min(queueIndex, queueLength - 1);

  function highlight(index: number) {
    if (queueLength === 0) return;
    const next = Math.min(queueLength - 1, Math.max(0, index));
    setQueueIndex(next);
    // Keep the highlighted card visible without moving the panel header; the
    // sidebar is the only thing that scrolls.
    requestAnimationFrame(() => {
      queueRef.current
        ?.querySelector(`[data-queue-index="${next}"]`)
        ?.scrollIntoView({ block: 'nearest' });
    });
  }

  function handleQueueKeyDown(event: ReactKeyboardEvent<HTMLUListElement>) {
    // Only when the list itself has focus: keys bubbling from a card's buttons
    // must not act a second time on Enter or on their own letters.
    if (event.target !== event.currentTarget) return;
    const candidate = detection.candidates[activeIndex];
    if (!candidate) return;
    switch (event.key) {
      case 'ArrowDown':
        event.preventDefault();
        highlight(activeIndex + 1);
        return;
      case 'ArrowUp':
        event.preventDefault();
        highlight(activeIndex - 1);
        return;
      case 'Home':
        event.preventDefault();
        highlight(0);
        return;
      case 'End':
        event.preventDefault();
        highlight(queueLength - 1);
        return;
      case 'Enter':
      case 'p':
      case 'P':
        event.preventDefault();
        void run(() => detection.project(candidate.id));
        return;
      case 'a':
      case 'A':
        event.preventDefault();
        void run(() => detection.approve(candidate.id));
        return;
      case 'd':
      case 'D':
        event.preventDefault();
        void run(() => detection.dismiss(candidate.id));
        return;
      default:
    }
  }

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
        <div className="flex items-center gap-2 min-w-0 text-secondary">
          <span className={`inline-block w-1.5 h-1.5 rounded-full shrink-0 ${detection.listening ? 'bg-[#6EE7C0]' : 'bg-secondary'}`} />
          <strong className="flex-1 w-0 min-w-0 text-text text-[10.5px] font-medium truncate">{detection.listening ? `Listening · ${detection.bufferedSeconds.toFixed(0)} s buffered${detection.continuous ? (detection.segmenting ? ' · hearing' : ' · waiting for speech') : ''}` : 'Not listening'}</strong>
          {detection.segment > 0 && (
            <span className="text-[9px] text-secondary whitespace-nowrap">segment {detection.segment}</span>
          )}
          <Icon name="detection" size={22} className={`ml-auto ${detection.listening ? 'text-teal' : 'text-teal opacity-[.65]'}`} />
        </div>

        <div className="grid gap-2">
          <div className="flex items-center gap-2.5 p-2.5 border border-border rounded-lg bg-[#111D30]">
            <span className="inline-grid place-items-center w-[34px] h-[34px] border border-[#3B355C]! bg-[#24203E]! text-[#B3A4FF]! rounded-lg"><Icon name="mic" size={18} /></span>
            <div className="flex flex-1 w-0 flex-col gap-px min-w-0">
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
              // Disabled blocks starting, as F8 does, but never stopping, or the
              // microphone would be trapped on.
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
                    <button
            type="button"
            role="switch"
            aria-checked={detection.continuous}
            className="min-h-7 inline-flex items-center gap-2 px-2 py-1 border border-[#35445C] rounded-md bg-elevated text-[10px] text-text text-left"
            onClick={detection.toggleContinuous}
            title="Close an analysis segment when the speaker pauses"
          >
            <span className={`relative flex items-center w-[26px] h-3.5 rounded-full shrink-0 ${detection.continuous ? 'bg-[#6EE7C0]' : 'bg-[#2A3950]'}`} aria-hidden="true">
              <span className={`w-2.5 h-2.5 rounded-full ml-0.5 ${detection.continuous ? 'bg-app' : 'bg-secondary'}`} />
            </span>
            Continuous — segment automatically
          </button>
          {detection.lastError && (
            <p className="px-2 py-1.5 border border-[#4D1E1E] rounded-md bg-[#2B1313] text-[#FFB1B7] text-[9.5px] leading-[1.5]">{detection.lastError}</p>
          )}
<label className="min-h-7 inline-flex items-center justify-center gap-1.5 px-2 py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap text-text cursor-pointer has-disabled:opacity-50" title="Run a recording through the same pipeline — MP3, M4A or WAV; the whole file is analyzed in engine-safe chunks">
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
            <h3 id="transcript-title" className="text-[9.5px] uppercase tracking-[1px] text-secondary">
              <button
                type="button"
                className="inline-flex items-center gap-1.5 uppercase tracking-[1px] hover:text-text [&_svg]:transition-transform [&_svg]:duration-150"
                aria-expanded={transcriptOpen}
                aria-controls="transcript-body"
                onClick={() => setTranscriptOpen((open) => !open)}
              >
                <Icon name="chevron" size={12} className={transcriptOpen ? '' : '-rotate-90'} />
                Current transcript
              </button>
            </h3>
            {detection.analyzing && (
              <span className="inline-flex items-center gap-1.5 text-[9px] text-teal">
                <span className="w-1.5 h-1.5 rounded-full bg-teal animate-pulse" aria-hidden="true" />
                transcribing
              </span>
            )}
          </div>
          <div id="transcript-body" className="grid gap-2">
            {transcriptOpen ? (
              detection.transcript ? (
                <p ref={transcriptRef} className="p-2.5 border border-border rounded-md bg-app text-text text-[10px] leading-[1.6] max-h-52 overflow-y-auto">{detection.transcript}</p>
              ) : (
                <p className="p-2.5 border border-dashed border-border rounded-md bg-app text-secondary text-[10px] leading-[1.6]">
                  {detection.listening ? 'Nothing analyzed yet. Let the preacher name a verse, then analyze.' : 'Start listening, then analyze a chunk to transcribe it.'}
                </p>
              )
            ) : detection.transcript ? (
              <button
                type="button"
                className="block w-full truncate px-0.5 text-left text-[10px] text-secondary hover:text-text"
                title="Show the full transcript"
                onClick={() => setTranscriptOpen(true)}
              >
                …{detection.transcript.slice(-110)}
              </button>
            ) : null}
          </div>
        </section>

        <section className="grid gap-2 border-t border-border pt-[11px]" aria-labelledby="queue-title">
          <div className="flex items-center justify-between gap-2">
            <h3 id="queue-title" className="text-[9.5px] uppercase tracking-[1px] text-secondary">Pending queue</h3>
            <span className="inline-grid place-items-center min-w-[18px] h-[18px] px-[5px] rounded-full bg-elevated text-[9.5px]">{detection.candidates.length}</span>
            {detection.candidates.length > 0 && (
              <button type="button" className="text-secondary text-[9.5px] hover:text-text" onClick={() => void run(detection.clear)}>Clear all</button>
            )}
          </div>
          {detection.autoProjected.length > 0 && (
            <p className="px-2 py-1.5 border border-[#1E4D40] rounded-md bg-[#0E2A24] text-[#6EE7C0] text-[9.5px] leading-[1.5]">Auto-projected {detection.autoProjected.join(', ')} — every safety gate passed.</p>
          )}
          {detection.repeats > 0 && (
            <p className="px-2 py-1.5 border border-[#4D3F1E] rounded-md bg-[#2B2413] text-amber text-[9.5px] leading-[1.5]">Repeated reference — folded into the pending card instead of queuing a duplicate.</p>
          )}
          {detection.candidates.length === 0 ? (
            <p className="p-2.5 border border-dashed border-border rounded-md bg-app text-secondary text-[10px] leading-[1.6]">Nothing waiting for confirmation.</p>
          ) : (
            <ul
              ref={queueRef}
              className="m-0 p-0 list-none grid gap-1.5 rounded-[7px] focus-visible:outline-1 focus-visible:outline-blue"
              role="listbox"
              aria-label="Pending detections"
              tabIndex={0}
              onKeyDown={handleQueueKeyDown}
            >
              {detection.candidates.map((candidate, index) => (
                <li
                  key={candidate.id}
                  data-queue-index={index}
                  role="option"
                  aria-selected={index === activeIndex}
                  className={
                    'grid gap-1.5 p-2.5 border rounded-[7px] bg-app '
                    + (index === activeIndex
                      ? 'border-[#304C80] bg-selected shadow-[inset_3px_0_var(--color-blue)]'
                      : 'border-border')
                  }
                  onClick={(event) => {
                    // Acting on a button must not steal the list's keyboard path.
                    if ((event.target as HTMLElement).closest('button, summary, details')) return;
                    highlight(index);
                    queueRef.current?.focus();
                  }}
                >
                  <div className="flex items-baseline gap-2 min-w-0">
                    <Icon name="bible" size={14} className="text-[#B3A4FF] shrink-0 translate-y-px" />
                    <strong className="text-[11.5px] font-semibold truncate">{candidate.reference}</strong>
                    <span className="ml-auto text-secondary text-[9.5px] whitespace-nowrap">{candidate.createdAt}</span>
                  </div>
                  <p className="text-secondary text-[9.5px] leading-[1.5] line-clamp-2">“{candidate.transcript}”</p>
                  <p className="flex flex-wrap items-center gap-1.5 text-secondary text-[9px] leading-[1.4]">
                    <span className="inline-flex items-center px-1.5 py-px border border-[#3B355C]! rounded-full bg-[#24203E]! text-[#B3A4FF]! whitespace-nowrap">
                      {candidate.method === 'explicit-reference' ? 'Explicit reference' : candidate.method}
                    </span>
                    <span
                      className={`inline-flex items-center px-1.5 py-px border rounded-full whitespace-nowrap tabular-nums ${confidenceTone(candidate.confidenceBand)}`}
                      title="Phrasing evidence, not measured accuracy"
                    >
                      {Math.round(candidate.confidence * 100)}% {confidenceLabel(candidate.confidenceBand)}
                    </span>
                    <span className={`inline-flex items-center px-1.5 py-px border rounded-full whitespace-nowrap ${candidate.finalized ? 'border-[#1E4D40]! bg-[#0E2A24]! text-[#6EE7C0]!' : 'border-[#2A3950]! bg-elevated! text-secondary!'}`}>
                      {candidate.finalized ? 'Final' : 'Live'}
                    </span>
                    {candidate.repeats > 0 && (
                      <span className="text-amber whitespace-nowrap">heard ×{candidate.repeats + 1}</span>
                    )}
                    <span>{candidate.explanation}</span>
                  </p>
                  {candidate.autoProjection && (
                    <details className="px-2 py-1.5 border border-[#4D3F1E] rounded-md bg-[#2B2413] text-[9px] leading-[1.5]">
                      <summary className="text-amber cursor-pointer">Held back from auto-projection — {candidate.autoProjection.reason}</summary>
                      <ul className="mt-1 p-0 list-none grid gap-0.5 text-secondary">
                        {candidate.autoProjection.gates?.map((gate) => (
                          <li key={gate.id} className={gate.passed ? 'text-[#6EE7C0]' : 'text-amber'}>
                            {gate.passed ? '✓' : '✗'} {gate.label}
                          </li>
                        ))}
                      </ul>
                    </details>
                  )}
                  <div className="flex flex-wrap gap-1.5">
                    <button
                      type="button"
                      className="min-h-7 inline-flex flex-1 items-center justify-center gap-1.5 px-2 py-1 border border-blue! bg-blue! text-white! rounded-md text-[10.5px] font-medium whitespace-nowrap not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
                      onClick={() => void run(() => detection.project(candidate.id))}
                      title="Send this passage to the room now"
                    >
                      <Icon name="display" size={12} />Project live
                    </button>
                    <button
                      type="button"
                      className="min-h-7 inline-flex flex-1 items-center justify-center gap-1.5 px-2 py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap"
                      onClick={() => void run(() => detection.approve(candidate.id))}
                      title="Fill the preview without changing what the room sees"
                    >
                      <Icon name="check" size={12} />Approve to preview
                    </button>
                    <button
                      type="button"
                      className="min-h-7 inline-flex items-center justify-center gap-1.5 px-2 py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap"
                      onClick={() => onOpenBible({
                        bookId: candidate.bookId,
                        chapter: candidate.chapter,
                        verseStart: candidate.verseStart,
                        verseEnd: candidate.verseEnd,
                      })}
                      title="Open this reference in the Bible workspace"
                    >
                      <Icon name="search" size={12} />Search
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
          {detection.candidates.length > 0 && (
            <p className="text-secondary text-[9px] leading-[1.5]">
              Click the queue, then ↑↓ select · P or Enter project · A approve · D dismiss
            </p>
          )}
        </section>

        <section className="grid gap-2 border-t border-border pt-[11px]" aria-labelledby="detection-history-title">
          <div className="flex items-center justify-between gap-2">
            <h3 id="detection-history-title" className="text-[9.5px] uppercase tracking-[1px] text-secondary">Recent decisions</h3>
            {detection.history.length > 0 && (
              <span className="inline-grid place-items-center min-w-[18px] h-[18px] px-[5px] rounded-full bg-elevated text-[9.5px]">{detection.history.length}</span>
            )}
          </div>
          {detection.history.length === 0 ? (
            <p className="p-2.5 border border-dashed border-border rounded-md bg-app text-secondary text-[10px] leading-[1.6]">Nothing decided yet — approvals, dismissals and cleared candidates appear here.</p>
          ) : (
            <ul className="m-0 p-0 list-none grid gap-px">
              {detection.history.slice(0, 6).map((entry) => (
                <li key={entry.id} className="flex items-baseline gap-2 min-w-0 px-2 py-[5px] rounded-md hover:bg-elevated">
                  <span className="text-[10.5px] font-medium truncate">{entry.reference}</span>
                  <span className={`ml-auto text-[9px] whitespace-nowrap ${entry.decision === 'approved' || entry.decision === 'auto_projected' ? 'text-[#6EE7C0]' : 'text-secondary'}`}>{decisionLabel(entry.decision)}</span>
                  <span className="text-secondary text-[9px] whitespace-nowrap tabular-nums">{decisionTime(entry.createdAt)}</span>
                </li>
              ))}
            </ul>
          )}
        </section>

        {problem !== null && <p className="text-error text-[10.5px] leading-[1.5]" role="alert">{problem}</p>}

        <div className="flex items-start gap-2 text-secondary">
          <Icon name="bible" size={16} className="text-[#B3A4FF] mt-0.5" />
          <p className="text-[10px] leading-[1.6]">Scripture first. A candidate is a suggestion: Project live sends it to the room, while Approve only fills the preview for you to send yourself.</p>
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
    } catch {}
  }

  const iconBtn = 'inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50';

  return (
    <section className="bg-panel border border-border rounded-[10px]" aria-label="Production controls">
      <div className="min-h-11 px-3.5 py-2.5 flex items-center justify-between gap-2.5 border-b border-border">
        <h2 className="text-[11px] flex items-center gap-[7px] uppercase tracking-[0.7px] text-secondary [&_svg]:text-secondary"><Icon name="display" size={16} />Production</h2>
        <span className="inline-flex px-2 py-[3px] border border-[#2A3950]! rounded-[5px] bg-elevated! text-secondary! text-[9.5px] font-medium whitespace-nowrap">{detection.autoProjection && !detection.autoProjectionPaused ? 'Hybrid' : 'Manual'}</span>
      </div>

      <div className="grid gap-2.5 p-3">
        <button
          type="button"
          className="flex items-center gap-2.5 p-2.5 border border-border rounded-lg bg-[#111D30] text-left w-full"
          onClick={() => void run(detection.toggleListening)}
          title={detection.listening ? 'Stop listening (F8)' : 'Listen for scripture references (F8)'}
        >
          <span className="grid place-items-center w-[34px] h-[34px] border border-[#3B355C]! bg-[#24203E]! text-[#B3A4FF]! rounded-lg shrink-0"><Icon name="mic" size={18} /></span>
          <div className="flex flex-1 w-0 flex-col gap-px min-w-0">
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

                <button
          type="button"
          className="flex items-center gap-[9px] pt-[9px] border-t border-border w-full text-left"
          onClick={() => void run(() => detection.setAutoProjection(!detection.autoProjection))}
          title="Project high-confidence explicit references that pass every safety gate"
        >
          <span className={`relative flex items-center w-[30px] h-4 rounded-full shrink-0 ${detection.autoProjection && !detection.autoProjectionPaused ? 'bg-[#6EE7C0]' : 'bg-[#2A3950]'}`} aria-hidden="true">
            <span className={`w-3 h-3 rounded-full ml-0.5 ${detection.autoProjection && !detection.autoProjectionPaused ? 'bg-app' : 'bg-secondary'}`} />
          </span>
          <div className="min-w-0">
            <strong className="block text-[9.5px] font-medium">Auto Projection</strong>
            <span className="flex items-center gap-1 text-secondary text-[9px] mt-px">
              <span className={`inline-block w-[5px] h-[5px] rounded-full shrink-0 ${detection.autoProjection && !detection.autoProjectionPaused ? 'bg-[#6EE7C0]' : 'bg-secondary'}`} />
              {!detection.autoProjection ? 'Off' : detection.autoProjectionPaused ? 'Paused (F9)' : 'Hybrid — gates armed'}
            </span>
          </div>
        </button>

        <p className="text-secondary text-[9.5px] leading-[1.5]">
          {detection.autoProjection
            ? 'Only high-confidence explicit references that pass every safety gate are projected; everything else waits for you.'
            : 'Every detection waits in the queue for you to approve.'}
        </p>
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

// The panel keeps the wording short; these are the history table's CHECK values.
function decisionLabel(decision: string): string {
  switch (decision) {
    case 'auto_projected': return 'Auto-projected';
    case 'approved': return 'Approved';
    case 'dismissed': return 'Dismissed';
    case 'ignored': return 'Cleared';
    default: return decision;
  }
}

function decisionTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
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

// clockTime renders a clip clock as m:ss, the way a media player reads.
function clockTime(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '0:00';
  const totalSeconds = Math.floor(ms / 1000);
  return `${Math.floor(totalSeconds / 60)}:${String(totalSeconds % 60).padStart(2, '0')}`;
}

export function AppShell({ activePage, onNavigate, onOpenBible, children }: {
  activePage: PageId;
  onNavigate: (page: PageId) => void;
  // Switch to the Bible page with one passage open (the detection card's Search).
  onOpenBible: (passage: BiblePassage) => void;
  children: ReactNode;
}) {
  const activeLabel = navigation.find((item) => item.id === activePage)?.label ?? 'Bible';
  const workspaceRef = useRef<HTMLDivElement>(null);
  const studio = useStudio();
  const detection = useDetection();
  const { shortcuts } = useShortcuts();
  const [shortcutNotice, setShortcutNotice] = useState<string | null>(null);
  const noticeTimer = useRef<number | null>(null);

  function flashNotice(msg: string) {
    if (noticeTimer.current !== null) window.clearTimeout(noticeTimer.current);
    setShortcutNotice(msg);
    noticeTimer.current = window.setTimeout(() => setShortcutNotice(null), 3000);
  }

  // A keypress has no panel to show errors in, so shortcuts use the notice bar.
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

      const combo = comboFromEvent(event);
      const action = combo === null ? null : actionForCombo(shortcuts, combo);
      // Schedule navigation stays out of the way while the operator types; the
      // other actions are global.
      if (action !== null && !(TEXT_GUARDED.has(action) && inText)) {
        event.preventDefault();
        switch (action) {
          case 'sendPreviewLive':
            if (studio.presentation?.preview) {
              void studio.sendPreviewLive();
              flashNotice('Preview sent live');
            } else {
              flashNotice('No preview ready to send live');
            }
            break;
          case 'focusBibleSearch':
            onNavigate('bible');
            window.setTimeout(() => {
              const input = document.getElementById('bible-search-input') as HTMLInputElement | null;
              input?.focus();
              input?.select();
            }, 60);
            break;
          case 'toggleListening':
            // Stopping always works: if detection was disabled while the mic was
            // live, refusing the toggle would leave it capturing with no way off.
            if (detection.listening) {
              void detection.toggleListening();
              flashNotice('Stopped listening for scripture');
            } else if (detection.disabled) {
              flashNotice('Detection is disabled — re-enable it in Settings');
            } else {
              void run(() => detection.toggleListening(), () => flashNotice('Listening for scripture references'));
            }
            break;
          case 'toggleAutoProjection':
            if (!detection.autoProjection) {
              flashNotice('Auto-projection is off — enable it in Production');
              break;
            }
            void run(() => detection.toggleAutoProjectionPause(), () => flashNotice(
              detection.autoProjectionPaused ? 'Auto-projection resumed' : 'Auto-projection paused',
            ));
            break;
          case 'emergencyDisable':
            // Stops new candidates without touching what the audience sees.
            void run(() => App.SetDetectionDisabled(true), () => {
              void detection.refresh();
              flashNotice('Emergency: detection disabled (live display preserved)');
            });
            break;
          case 'schedulePrevious':
            void studio.previousItem();
            break;
          case 'scheduleNext':
            void studio.nextItem();
            break;
        }
        return;
      }

      // Escape only blurs a focused field; clearing live output needs an
      // explicit command, so a stray keypress cannot blank the room.
      if (event.key === 'Escape' && inText) {
        (activeEl as HTMLElement).blur();
      }
    }

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [shortcuts, studio, detection, onNavigate]);

  useEffect(() => {
    workspaceRef.current?.scrollTo({ top: 0 });
  }, [activePage]);

  const showDock = activePage !== 'home';
  // Bible pins its search header and scrolls only the reading pane, so that pane
  // needs a real height to overflow; other pages grow with their content.
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
        <div className="grid grid-cols-[minmax(0,1fr)] gap-3.5 content-start min-h-0 min-w-0 overflow-y-auto max-[959px]:overflow-visible">
          <DetectionPanel onOpenBible={onOpenBible} />
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
