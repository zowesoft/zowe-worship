import { useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import type { PageId } from '../../layouts/AppShell';

function NewServiceForm({ onCreated }: { onCreated: () => void }) {
  const studio = useStudio();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  function expand() {
    setOpen(true);
    // The input does not exist until this render commits.
    setTimeout(() => inputRef.current?.focus(), 0);
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const trimmed = name.trim();
    if (!trimmed || busy) return;
    setBusy(true);
    try {
      await studio.createService(trimmed);
      setName('');
      setOpen(false);
      onCreated();
    } catch {
    } finally {
      setBusy(false);
    }
  }

  if (!open) {
    return (
      <button
        type="button"
        className="inline-flex items-center gap-[7px] min-h-9 px-4 border border-blue rounded-[7px] bg-[rgba(71,120,255,0.12)] text-[#8CABFF] text-[12px] font-medium whitespace-nowrap shrink-0 transition-[background,color] duration-120 hover:not-disabled:bg-[rgba(71,120,255,0.22)] hover:not-disabled:text-text disabled:opacity-50 disabled:cursor-not-allowed"
        onClick={expand}
        disabled={!studio.bridgeAvailable}
      >
        <Icon name="display" size={16} />
        New service
      </button>
    );
  }

  return (
    <form className="flex items-center gap-[7px] shrink-0 max-[959px]:flex-wrap" onSubmit={(e) => void submit(e)}>
      <input
        ref={inputRef}
        type="text"
        className="w-[260px] h-[34px] px-[11px] border border-blue rounded-[7px] bg-[rgba(71,120,255,0.08)] text-text text-[12px] focus:border-[#8CABFF] focus:bg-[rgba(71,120,255,0.12)] focus:outline-none placeholder:text-secondary max-[959px]:w-full"
        placeholder="Service name, e.g. Sunday 9am"
        value={name}
        maxLength={80}
        autoComplete="off"
        onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => { if (e.key === 'Escape') { setOpen(false); setName(''); } }}
      />
      <button
        type="submit"
        className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! border-blue! bg-blue! text-white! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
        disabled={!name.trim() || busy}
      >
        {busy ? 'Creating…' : 'Create'}
      </button>
      <button
        type="button"
        className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]"
        onClick={() => { setOpen(false); setName(''); }}
      >
        Cancel
      </button>
    </form>
  );
}

type ServiceCardProps = {
  id: string;
  name: string;
  isActive: boolean;
  itemCount: number | null;   // only known for the active service
  onSelect: (id: string) => void;
  onNavigate: (page: PageId) => void;
};

function ServiceCard({ id, name, isActive, itemCount, onSelect, onNavigate }: ServiceCardProps) {
  const [busy, setBusy] = useState(false);

  async function open() {
    if (isActive) {
      onNavigate('service');
      return;
    }
    setBusy(true);
    try {
      await onSelect(id);
      onNavigate('service');
    } catch {
    } finally {
      setBusy(false);
    }
  }

  return (
    <article
      className={[
        'flex items-center gap-[14px] px-[18px] py-4 border border-border rounded-[10px] bg-panel hover:border-[#304C80] hover:bg-[#0F1E35] transition-[border-color,background] duration-140',
        isActive
          ? 'border-[#304C80]! bg-[#0C1A30]! shadow-[0_0_0_1px_rgba(71,120,255,0.15)] [&>div:first-child]:border-[rgba(71,120,255,0.4)] [&>div:first-child]:bg-[rgba(71,120,255,0.12)] [&>div:first-child]:text-blue'
          : '',
      ].filter(Boolean).join(' ')}
      aria-label={`Service: ${name}${isActive ? ' (current)' : ''}`}
    >
      <div className="grid place-items-center w-[42px] h-[42px] border border-[#30466A] rounded-[10px] bg-selected text-[#8CABFF] shrink-0">
        <Icon name="order" size={20} />
      </div>

      <div className="grid gap-[3px] min-w-0 flex-1">
        <strong className="block text-[14px] font-semibold tracking-[-0.2px] truncate">{name}</strong>
        {isActive && itemCount !== null ? (
          <span className="block text-[10.5px] text-secondary">
            {itemCount} {itemCount === 1 ? 'item' : 'items'} scheduled
          </span>
        ) : (
          <span className="block text-[10.5px] text-secondary">Tap to open</span>
        )}
      </div>

      {isActive && <span className="inline-flex items-center px-2 py-[3px] border border-[#1E4D40]! rounded-full bg-[#0E2A24]! text-[#6EE7C0]! text-[9px] whitespace-nowrap shrink-0">Current</span>}

      <button
        type="button"
        className={[
          'min-h-7 inline-flex items-center justify-center gap-[5px] px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap shrink-0 [&_svg]:-rotate-90 disabled:text-secondary',
          isActive
            ? 'disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! border-blue! bg-blue! text-white! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!'
            : 'disabled:bg-[#142033] disabled:border-[#2A3950]',
        ].filter(Boolean).join(' ')}
        disabled={busy}
        onClick={() => void open()}
        aria-label={isActive ? `Open schedule for ${name}` : `Switch to ${name} and open schedule`}
      >
        {busy ? '…' : isActive ? 'Open schedule' : 'Switch & open'}
        <Icon name="chevron" size={12} />
      </button>
    </article>
  );
}

export function HomeWorkspace({ onNavigate }: { onNavigate: (page: PageId) => void }) {
  const studio = useStudio();
  const services = studio.services ?? [];
  const activeService = studio.order?.service ?? null;
  const activeItems = studio.order?.items ?? [];

  async function selectService(id: string) {
    await studio.useService(id);
  }

  const isEmpty = services.length === 0;

  return (
    <div className="grid gap-7 pb-7">
      {/* ── header ── */}
      <header className="flex items-end justify-between gap-5 pb-[22px] border-b border-border max-[959px]:flex-col max-[959px]:items-start">
        <div className="grid gap-[5px]">
          <span className="text-[9.5px] uppercase tracking-[1.3px] font-semibold text-secondary">Zopresent</span>
          <h1 className="text-[24px] font-bold tracking-[-0.5px] text-text">
            {isEmpty ? 'Get started' : 'Your services'}
          </h1>
          <p className="text-[12px] text-secondary leading-[1.6] max-w-[420px]">
            {isEmpty
              ? 'Create your first service to build a schedule and begin presenting.'
              : 'Choose a service to continue, or create a new one.'}
          </p>
        </div>

        <NewServiceForm onCreated={() => { /* list updates via store */ }} />
      </header>

      {/* ── empty state ── */}
      {isEmpty && (
        <div className="grid justify-items-center gap-3 px-8 py-12 border border-dashed border-border rounded-[10px] bg-panel text-center [&_strong]:text-[16px] [&_strong]:font-semibold [&_p]:text-[12px] [&_p]:text-secondary [&_p]:leading-[1.65] [&_p]:max-w-[380px]">
          <div className="grid place-items-center w-[68px] h-[68px] border border-[#30466A] rounded-[16px] bg-selected text-[#8CABFF] mb-1">
            <Icon name="order" size={36} />
          </div>
          <strong>No services yet</strong>
          <p>
            A service is your running order — scripture passages, song sections,
            images and video clips, all in one list you work through during the meeting.
          </p>
          <NewServiceForm onCreated={() => { }} />
        </div>
      )}

      {/* ── service list ── */}
      {!isEmpty && (
        <section className="grid gap-2" aria-label="Saved services">
          {/* Active service first */}
          {activeService && (
            <ServiceCard
              key={activeService.id}
              id={activeService.id}
              name={activeService.name}
              isActive={true}
              itemCount={activeItems.length}
              onSelect={selectService}
              onNavigate={onNavigate}
            />
          )}
          {/* Other services */}
          {services
            .filter((s) => s.id !== activeService?.id)
            .map((s) => (
              <ServiceCard
                key={s.id}
                id={s.id}
                name={s.name}
                isActive={false}
                itemCount={null}
                onSelect={selectService}
                onNavigate={onNavigate}
              />
            ))}
        </section>
      )}

      {/* ── quick jump ── */}
      {!isEmpty && (
        <section className="grid gap-3" aria-label="Quick access">
          <h2 className="text-[10px] uppercase tracking-[1.2px] text-secondary font-semibold">Jump to</h2>
          <div className="grid grid-cols-4 gap-2 max-[1150px]:grid-cols-2 max-[959px]:grid-cols-2">
            {([
              { id: 'bible'   as PageId, icon: 'bible'  as const, label: 'Bible search',   hint: 'Find and project scripture' },
              { id: 'songs'   as PageId, icon: 'music'  as const, label: 'Songs',           hint: 'Lyrics and arrangements' },
              { id: 'media'   as PageId, icon: 'media'  as const, label: 'Media library',   hint: 'Images, video and audio' },
              { id: 'service' as PageId, icon: 'order'  as const, label: 'Service order',   hint: 'Build and run the schedule' },
            ]).map((tile) => (
              <button
                key={tile.id}
                type="button"
                className="grid justify-items-start gap-1 pt-4 px-[14px] pb-[14px] border border-border rounded-[9px] bg-panel text-left transition-[border-color,background] duration-120 hover:border-[#304C80] hover:bg-elevated [&_strong]:block [&_strong]:text-[12px] [&_strong]:font-semibold [&_strong]:text-text [&>span]:block [&>span]:text-[10px] [&>span]:text-secondary [&>span]:leading-[1.5]"
                onClick={() => onNavigate(tile.id)}
              >
                <span className="grid place-items-center w-9 h-9 border border-[#30466A] rounded-[9px] bg-selected text-[#8CABFF] mb-1.5"><Icon name={tile.icon} size={18} /></span>
                <strong>{tile.label}</strong>
                <span>{tile.hint}</span>
              </button>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
