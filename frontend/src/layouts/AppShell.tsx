import type { ReactNode } from 'react';
import { BackendStatus } from '../components/BackendStatus';
import { Icon } from '../components/Icon';
import type { IconName } from '../components/Icon';

export type PageId = 'home' | 'service' | 'media' | 'bible' | 'detection' | 'settings';

export const navigation: { id: PageId; label: string; icon: IconName }[] = [
  { id: 'home', label: 'Home', icon: 'home' },
  { id: 'service', label: 'Service Order', icon: 'order' },
  { id: 'media', label: 'Media Library', icon: 'media' },
  { id: 'bible', label: 'Bible', icon: 'bible' },
  { id: 'detection', label: 'Live Detection', icon: 'detection' },
  { id: 'settings', label: 'Settings', icon: 'settings' },
];

function PresentationPanes() {
  return (
    <section className="presentation-panes" aria-label="Persistent presentation workspace">
      <section className="panel output-pane" aria-labelledby="live-title">
        <div className="panel-heading">
          <h2 id="live-title"><Icon name="display" size={16} /> Live output</h2>
          <span className="quiet-label">Not connected</span>
        </div>
        <div className="presentation-screen">
          <Icon name="display" size={32} />
          <strong>No output connected</strong>
          <p>An audience display will appear here.</p>
        </div>
        <div className="pane-caption"><span className="status-dot" /> Output integration is not implemented.</div>
      </section>
      <section className="panel output-pane" aria-labelledby="preview-title">
        <div className="panel-heading">
          <h2 id="preview-title"><Icon name="preview" size={16} /> Preview</h2>
          <span className="quiet-label">Placeholder</span>
        </div>
        <div className="presentation-screen presentation-screen--preview">
          <Icon name="layers" size={32} />
          <strong>No content selected</strong>
          <p>Prepare content before it goes live.</p>
        </div>
        <div className="pane-caption"><span className="status-dot" /> Content preview is not implemented.</div>
      </section>
    </section>
  );
}

function DetectionPanel() {
  return (
    <aside className="panel detection-panel" aria-labelledby="detection-panel-title">
      <div className="panel-heading">
        <h2 id="detection-panel-title"><Icon name="detection" size={18} /> Live Detection</h2>
        <span className="scaffold-dot" title="Scaffold only" aria-label="Scaffold only" />
      </div>
      <div className="detection-body">
        <div className="detection-intro">
          <span className="icon-tile icon-tile--violet"><Icon name="detection" size={26} /></span>
          <h3>Ready for the next layer</h3>
          <p>This space is reserved for live transcription and scripture candidates.</p>
        </div>
        <dl className="configuration-list">
          <div><dt>Speech model</dt><dd>No model loaded</dd></div>
          <div><dt>Audio input</dt><dd>Not configured</dd></div>
          <div><dt>Auto-projection</dt><dd className="off-value">Off</dd></div>
        </dl>
        <button className="button button--full" type="button" disabled aria-describedby="detection-unavailable">
          <Icon name="detection" size={16} /> Start detection
        </button>
        <p className="control-help" id="detection-unavailable">Unavailable: audio capture and speech recognition are not implemented.</p>
        <div className="detection-placeholder">
          <h3>Transcript</h3>
          <p>No transcript. Detection is unavailable in this scaffold.</p>
        </div>
        <div className="detection-placeholder">
          <h3>Scripture candidates</h3>
          <p>No candidates. Matching and review controls will be added here.</p>
        </div>
        <div className="safety-note"><Icon name="bible" size={18} /><p>Scripture first. Nothing is detected or projected automatically in this build.</p></div>
      </div>
    </aside>
  );
}

function ProductionBar() {
  return (
    <footer className="production-bar" aria-label="Production controls">
      <div className="production-status">
        <span className="production-icon"><Icon name="display" size={22} /></span>
        <div><strong>No output connected</strong><p>Audio input not configured</p></div>
      </div>
      <div className="auto-projection"><span className="status-dot" /><span>Auto-projection <strong>off</strong></span></div>
      <div className="production-actions">
        <div className="button-row">
          <button type="button" className="button" disabled aria-describedby="production-unavailable">Previous</button>
          <button type="button" className="button" disabled aria-describedby="production-unavailable">Next</button>
          <button type="button" className="button" disabled aria-describedby="production-unavailable">Clear output</button>
          <button type="button" className="button" disabled aria-describedby="production-unavailable" title="Disable automatic projection without clearing the audience display">Disable auto</button>
          <button type="button" className="button button--primary" disabled aria-describedby="production-unavailable">Go live <Icon name="arrow" size={15} /></button>
        </div>
        <p id="production-unavailable" className="control-help">Production controls are not implemented.</p>
      </div>
    </footer>
  );
}

export function AppShell({ activePage, onNavigate, children }: {
  activePage: PageId;
  onNavigate: (page: PageId) => void;
  children: ReactNode;
}) {
  const activeLabel = navigation.find((item) => item.id === activePage)?.label ?? 'Home';

  return (
    <div className="app-shell">
      <a className="skip-link" href="#workspace">Skip to workspace</a>
      <aside className="sidebar" aria-label="Application sidebar">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">z</span>
          <div><strong>Zowe Worship</strong><span>BY ZOWESOFT</span></div>
        </div>
        <div className="sidebar-content">
          <p className="eyebrow nav-section-label">Workspace</p>
          <nav aria-label="Main navigation">
            {navigation.map((item) => (
              <button key={item.id} type="button" className={`nav-item${activePage === item.id ? ' nav-item--active' : ''}`} aria-current={activePage === item.id ? 'page' : undefined} onClick={() => onNavigate(item.id)}>
                <Icon name={item.icon} /><span>{item.label}</span>
              </button>
            ))}
          </nav>
          <div className="sidebar-bottom">
            <div className="foundation-label"><Icon name="layers" size={16} /><span>Application scaffold</span></div>
            <BackendStatus />
            <p className="brand-line">Scripture First. Always.</p>
          </div>
        </div>
      </aside>
      <header className="app-header">
        <div className="page-title"><span className="eyebrow">Workspace</span><h1 id="page-title">{activeLabel}</h1></div>
        <div className="header-context"><span className="quiet-label">No service configured</span><span className="badge">Foundation build</span></div>
      </header>
      <main id="workspace" className="workspace" tabIndex={-1} aria-labelledby="page-title">
        <div className="workspace-primary">
          <PresentationPanes />
          <section className="page-content" aria-label={`${activeLabel} workspace`}>
            {children}
          </section>
        </div>
        <DetectionPanel />
      </main>
      <ProductionBar />
    </div>
  );
}
