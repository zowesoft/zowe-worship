import { useRef, useState } from 'react';
import type { FormEvent } from 'react';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import type { PageId } from '../../layouts/AppShell';
import './home.css';

// ── new-service inline form ───────────────────────────────────────────────────

function NewServiceForm({ onCreated }: { onCreated: () => void }) {
  const studio = useStudio();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  function expand() {
    setOpen(true);
    // Wait a tick for the element to mount
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
      // error visible in sidebar
    } finally {
      setBusy(false);
    }
  }

  if (!open) {
    return (
      <button
        type="button"
        className="home-new-btn"
        onClick={expand}
        disabled={!studio.bridgeAvailable}
      >
        <Icon name="display" size={16} />
        New service
      </button>
    );
  }

  return (
    <form className="home-new-form" onSubmit={(e) => void submit(e)}>
      <input
        ref={inputRef}
        type="text"
        className="home-new-input"
        placeholder="Service name, e.g. Sunday 9am"
        value={name}
        maxLength={80}
        autoComplete="off"
        onChange={(e) => setName(e.target.value)}
        onKeyDown={(e) => { if (e.key === 'Escape') { setOpen(false); setName(''); } }}
      />
      <button
        type="submit"
        className="button button--primary button--small"
        disabled={!name.trim() || busy}
      >
        {busy ? 'Creating…' : 'Create'}
      </button>
      <button
        type="button"
        className="button button--small"
        onClick={() => { setOpen(false); setName(''); }}
      >
        Cancel
      </button>
    </form>
  );
}

// ── service card ──────────────────────────────────────────────────────────────

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
      // studio error shown in sidebar
    } finally {
      setBusy(false);
    }
  }

  return (
    <article
      className={['home-service-card', isActive ? 'home-service-card--active' : ''].filter(Boolean).join(' ')}
      aria-label={`Service: ${name}${isActive ? ' (current)' : ''}`}
    >
      <div className="home-card-icon">
        <Icon name="order" size={20} />
      </div>

      <div className="home-card-body">
        <strong className="home-card-name">{name}</strong>
        {isActive && itemCount !== null ? (
          <span className="home-card-meta">
            {itemCount} {itemCount === 1 ? 'item' : 'items'} scheduled
          </span>
        ) : (
          <span className="home-card-meta">Tap to open</span>
        )}
      </div>

      {isActive && <span className="status-pill status-pill--on home-card-badge">Current</span>}

      <button
        type="button"
        className={['button button--small', isActive ? 'button--primary' : ''].filter(Boolean).join(' ')}
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

// ── root ──────────────────────────────────────────────────────────────────────

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
    <div className="home-workspace">
      {/* ── header ── */}
      <header className="home-header">
        <div className="home-header-copy">
          <span className="eyebrow">Zowe Worship</span>
          <h1 className="home-title">
            {isEmpty ? 'Get started' : 'Your services'}
          </h1>
          <p className="home-subtitle">
            {isEmpty
              ? 'Create your first service to build a schedule and begin presenting.'
              : 'Choose a service to continue, or create a new one.'}
          </p>
        </div>

        <NewServiceForm onCreated={() => { /* list updates via store */ }} />
      </header>

      {/* ── empty state ── */}
      {isEmpty && (
        <div className="home-empty">
          <div className="home-empty-icon">
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
        <section className="home-services" aria-label="Saved services">
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
        <section className="home-jump" aria-label="Quick access">
          <h2 className="home-jump-heading">Jump to</h2>
          <div className="home-jump-tiles">
            {([
              { id: 'bible'   as PageId, icon: 'bible'  as const, label: 'Bible search',   hint: 'Find and project scripture' },
              { id: 'songs'   as PageId, icon: 'music'  as const, label: 'Songs',           hint: 'Lyrics and arrangements' },
              { id: 'media'   as PageId, icon: 'media'  as const, label: 'Media library',   hint: 'Images, video and audio' },
              { id: 'service' as PageId, icon: 'order'  as const, label: 'Service order',   hint: 'Build and run the schedule' },
            ]).map((tile) => (
              <button
                key={tile.id}
                type="button"
                className="home-jump-tile"
                onClick={() => onNavigate(tile.id)}
              >
                <span className="home-jump-icon"><Icon name={tile.icon} size={18} /></span>
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
