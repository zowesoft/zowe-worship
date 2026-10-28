import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { BackendStatus } from '../components/BackendStatus';
import { Icon } from '../components/Icon';
import type { IconName } from '../components/Icon';
import { hasDesktopBridge } from '../lib/desktop';

export type PageId = 'bible' | 'songs' | 'media' | 'service' | 'settings';

export const navigation: { id: PageId; label: string; icon: IconName }[] = [
  { id: 'bible', label: 'Bible', icon: 'bible' },
  { id: 'songs', label: 'Songs', icon: 'music' },
  { id: 'media', label: 'Media', icon: 'media' },
  { id: 'service', label: 'Service Order', icon: 'order' },
  { id: 'settings', label: 'Settings', icon: 'settings' },
];

function useClock() {
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), 30000);
    return () => window.clearInterval(timer);
  }, []);

  return now;
}

function ScheduleSidebar({ onNavigate }: { onNavigate: (page: PageId) => void }) {
  return (
    <aside className="sidebar" aria-label="Service schedule">
      <div className="brand">
        <span className="brand-mark" aria-hidden="true">z</span>
        <div><strong>Zowe Worship</strong><span>BY ZOWESOFT</span></div>
      </div>

      <div className="sidebar-content">
        <section className="service-summary" aria-labelledby="service-summary-title">
          <p className="eyebrow" id="service-summary-title">Current service</p>
          <strong>No service selected</strong>
          <span>Create a service to build the schedule.</span>
        </section>

        <section className="schedule" aria-labelledby="schedule-title">
          <div className="schedule-heading">
            <h2 id="schedule-title">Schedule</h2>
            <span className="schedule-count">0 items</span>
          </div>
          <div className="schedule-empty">
            <span className="schedule-icon"><Icon name="order" size={20} /></span>
            <p>No items scheduled.</p>
            <span>Scripture, songs and media will appear here in service order.</span>
            <button type="button" className="button button--full" onClick={() => onNavigate('service')}>
              Open Service Order
            </button>
          </div>
        </section>

        <div className="sidebar-bottom">
          <BackendStatus />
          <p className="brand-line">Scripture First. Always.</p>
        </div>
      </div>
    </aside>
  );
}

function AppHeader({ onNavigate }: { onNavigate: (page: PageId) => void }) {
  const connected = hasDesktopBridge();
  const now = useClock();
  const dateLabel = now.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
  const timeLabel = now.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });

  return (
    <header className="app-header">
      <div className="service-context">
        <button type="button" className="service-switcher" disabled title="Service selection is not implemented.">
          <span>No service selected</span>
          <Icon name="chevron" size={14} />
        </button>
        <div className="header-meta">
          <span>{dateLabel}</span>
          <span>{timeLabel}</span>
        </div>
        <span className={`connection-pill${connected ? ' connection-pill--on' : ' connection-pill--warn'}`}>
          <span className="status-dot" />{connected ? 'Desktop connected' : 'Browser preview'}
        </span>
      </div>

      <div className="header-actions">
        <button type="button" className="display-selector" disabled title="Display routing is not implemented.">
          <Icon name="display" size={16} />
          <span>No display</span>
          <Icon name="chevron" size={13} />
        </button>
        <button type="button" className="icon-button" onClick={() => onNavigate('settings')} aria-label="Open settings" title="Settings">
          <Icon name="settings" size={18} />
        </button>
        <button type="button" className="icon-button" disabled aria-label="Notifications unavailable" title="Notifications are not implemented.">
          <Icon name="bell" size={18} />
        </button>
        <div className="operator" aria-label="Local operator">
          <span className="avatar"><Icon name="user" size={17} /></span>
          <div><strong>Local operator</strong><span>This device</span></div>
        </div>
      </div>
    </header>
  );
}

function PresentationPanes() {
  return (
    <section className="presentation-panes" aria-label="Persistent presentation workspace">
      <section className="panel output-pane output-pane--preview" aria-labelledby="preview-title">
        <div className="pane-heading">
          <h2 id="preview-title"><span className="status-dot" /><Icon name="preview" size={15} />Preview</h2>
          <div className="pane-heading-actions">
            <button type="button" className="button button--small button--primary" disabled title="Preview selection is not implemented.">
              <Icon name="arrow" size={13} />Send Live
            </button>
          </div>
        </div>
        <div className="presentation-screen presentation-screen--preview">
          <Icon name="layers" size={30} />
          <strong>No content selected</strong>
          <p>Search scripture or choose media to prepare a slide.</p>
        </div>
        <div className="pane-caption"><span className="status-dot" />Preview is not connected to the presentation controller.</div>
      </section>

      <section className="panel output-pane output-pane--live" aria-labelledby="live-title">
        <div className="pane-heading">
          <h2 id="live-title"><span className="live-dot" /><Icon name="display" size={15} />Live Output</h2>
          <div className="pane-heading-actions">
            <span className="status-pill status-pill--off">Off air</span>
            <button type="button" className="button button--small pane-display-button" disabled title="Display routing is not implemented.">No display</button>
          </div>
        </div>
        <div className="presentation-screen">
          <Icon name="display" size={30} />
          <strong>No output connected</strong>
          <p>An audience display will appear here.</p>
        </div>
        <div className="pane-controls">
          <div className="pane-caption"><span className="status-dot" />No service order is running.</div>
          <div className="pane-buttons">
            <button type="button" className="icon-button icon-button--small icon-button--prev" disabled aria-label="Previous slide" title="Presentation controls are not implemented.">
              <Icon name="chevron" size={14} />
            </button>
            <button type="button" className="icon-button icon-button--small icon-button--next" disabled aria-label="Next slide" title="Presentation controls are not implemented.">
              <Icon name="chevron" size={14} />
            </button>
            <button type="button" className="button button--small" disabled title="Presentation controls are not implemented.">Clear</button>
            <button type="button" className="button button--small button--danger" disabled title="Presentation controls are not implemented.">
              <Icon name="detection" size={12} />Emergency
            </button>
          </div>
        </div>
      </section>
    </section>
  );
}

function DetectionPanel() {
  return (
    <aside className="panel detection-panel" aria-labelledby="detection-panel-title">
      <div className="panel-heading">
        <h2 id="detection-panel-title"><Icon name="detection" size={16} />AI Scripture Detection</h2>
        <span className="badge badge--muted">Scaffold</span>
      </div>

      <div className="detection-body">
        <div className="listening-row">
          <span className="status-dot" />
          <strong>Not listening</strong>
          <Icon name="detection" size={22} className="listening-wave" />
        </div>

        <div className="audio-input-card">
          <span className="icon-tile icon-tile--small icon-tile--violet"><Icon name="mic" size={18} /></span>
          <div><span>Audio input</span><strong>Not configured</strong></div>
          <Icon name="chevron" size={14} />
        </div>

        <section className="detection-block" aria-labelledby="transcript-title">
          <div className="detection-block-heading">
            <h3 id="transcript-title">Current transcript</h3>
            <span>00:00</span>
          </div>
          <p className="detection-empty-text">No transcript. Audio capture and speech recognition are not implemented.</p>
        </section>

        <section className="detection-block" aria-labelledby="candidate-title">
          <div className="detection-block-heading"><h3 id="candidate-title">Candidate</h3></div>
          <div className="candidate-empty">
            <Icon name="bible" size={22} />
            <p>No scripture candidates. Detected references will be reviewed here.</p>
          </div>
        </section>

        <section className="detection-block" aria-labelledby="queue-title">
          <div className="detection-block-heading">
            <h3 id="queue-title">Pending queue</h3>
            <span className="queue-count">0</span>
          </div>
          <p className="detection-empty-text">Nothing waiting for confirmation.</p>
        </section>

        <section className="detection-block" aria-labelledby="recent-title">
          <div className="detection-block-heading"><h3 id="recent-title">Recent detections</h3></div>
          <p className="detection-empty-text">No detections yet.</p>
        </section>

        <div className="safety-note">
          <Icon name="bible" size={16} />
          <p>Scripture first. Nothing is detected or projected automatically in this build.</p>
        </div>
      </div>
    </aside>
  );
}

function ProductionBar({ onNavigate }: { onNavigate: (page: PageId) => void }) {
  return (
    <footer className="production-bar" aria-label="Production controls">
      <div className="production-audio">
        <span className="production-icon"><Icon name="mic" size={20} /></span>
        <div className="production-copy"><strong>Audio input</strong><span>Not configured</span></div>
        <Icon name="chevron" size={14} />
      </div>

      <div className="level-meter" aria-hidden="true">
        {Array.from({ length: 18 }, (_, index) => <span key={index} />)}
      </div>

      <div className="production-output">
        <div className="production-chip">
          <Icon name="display" size={18} />
          <div><strong>Live output</strong><span>No display</span></div>
          <span className="status-pill status-pill--off">Off air</span>
        </div>
        <div className="production-chip">
          <Icon name="preview" size={18} />
          <div><strong>Preview</strong><span>No content</span></div>
          <span className="status-pill status-pill--warn">Idle</span>
        </div>
      </div>

      <div className="production-actions">
        <button type="button" className="icon-button icon-button--prev" disabled aria-label="Previous slide" title="Presentation controls are not implemented.">
          <Icon name="chevron" size={16} />
        </button>
        <button type="button" className="icon-button icon-button--next" disabled aria-label="Next slide" title="Presentation controls are not implemented.">
          <Icon name="chevron" size={16} />
        </button>
        <button type="button" className="button" disabled title="Presentation controls are not implemented.">Clear</button>
        <div className="auto-projection" title="Automatic projection is not implemented.">
          <span className="toggle toggle--off" aria-hidden="true"><span /></span>
          <div><strong>Auto Projection</strong><span><span className="status-dot" />Off</span></div>
        </div>
        <button type="button" className="icon-button" onClick={() => onNavigate('settings')} aria-label="Production settings" title="Open settings">
          <Icon name="settings" size={17} />
        </button>
      </div>
    </footer>
  );
}

function WorkspaceTabs({ activePage, onNavigate }: {
  activePage: PageId;
  onNavigate: (page: PageId) => void;
}) {
  return (
    <nav className="workspace-tabs" aria-label="Workspaces">
      {navigation.map((item) => (
        <button
          key={item.id}
          type="button"
          className={`workspace-tab${activePage === item.id ? ' workspace-tab--active' : ''}`}
          aria-current={activePage === item.id ? 'page' : undefined}
          onClick={() => onNavigate(item.id)}
        >
          <Icon name={item.icon} size={15} />
          <span>{item.label}</span>
        </button>
      ))}
    </nav>
  );
}

export function AppShell({ activePage, onNavigate, children }: {
  activePage: PageId;
  onNavigate: (page: PageId) => void;
  children: ReactNode;
}) {
  const activeLabel = navigation.find((item) => item.id === activePage)?.label ?? 'Bible';
  const workspaceRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    workspaceRef.current?.scrollTo({ top: 0 });
  }, [activePage]);

  return (
    <div className="app-shell">
      <a className="skip-link" href="#workspace">Skip to workspace</a>
      <ScheduleSidebar onNavigate={onNavigate} />
      <AppHeader onNavigate={onNavigate} />
      <main id="workspace" className="workspace" tabIndex={-1} aria-label={`${activeLabel} workspace`}>
        <div className="workspace-primary" ref={workspaceRef}>
          <div className="presentation-dock">
            <PresentationPanes />
            <WorkspaceTabs activePage={activePage} onNavigate={onNavigate} />
          </div>
          <section className="page-content">
            {children}
          </section>
        </div>
        <DetectionPanel />
      </main>
      <ProductionBar onNavigate={onNavigate} />
    </div>
  );
}
