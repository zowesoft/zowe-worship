import { useState } from 'react';
import { Icon } from '../components/Icon';
import type { IconName } from '../components/Icon';
import { AppShell } from '../layouts/AppShell';
import type { PageId } from '../layouts/AppShell';

type FeaturePage = {
  icon: IconName;
  title: string;
  description: string;
  purpose: string;
  next: string;
};

const featurePages: Record<Exclude<PageId, 'home'>, FeaturePage> = {
  service: {
    icon: 'order',
    title: 'A place for the whole service',
    description: 'No service order yet. Planning and service storage are not implemented.',
    purpose: 'Organize scripture, songs and media into an ordered service without changing the live output.',
    next: 'Service storage, item editing and operator-controlled cues.',
  },
  media: {
    icon: 'media',
    title: 'Your media library starts here',
    description: 'No media is available. Importing and indexing are not implemented.',
    purpose: 'Browse reusable media and prepare selected items for a service. Local media management will be handled by the Go backend.',
    next: 'Local imports, metadata indexing and media previews.',
  },
  bible: {
    icon: 'bible',
    title: 'Scripture first. A clean foundation.',
    description: 'No Bible translations are installed. Import and search are not implemented.',
    purpose: 'Find passages in installed translations, review them in preview and explicitly choose when to project. No sample scripture is included.',
    next: 'Translation import, local full-text search and passage preview.',
  },
  detection: {
    icon: 'detection',
    title: 'Assistance, with the operator in control',
    description: 'No model loaded. Audio capture, transcription and scripture matching are not implemented.',
    purpose: 'The Go backend will coordinate local audio and speech services. Candidate review belongs in the persistent detection panel, separate from live output.',
    next: 'Audio configuration, model loading and reviewed scripture candidates. Auto-projection is off.',
  },
  settings: {
    icon: 'settings',
    title: 'Configure the workspace, in time',
    description: 'There are no editable settings yet. Configuration persistence is not implemented.',
    purpose: 'Provide one place for output displays, audio devices, speech models and application preferences, backed by desktop configuration services.',
    next: 'Typed configuration, device discovery and saved preferences.',
  },
};

const nextAreas: { page: PageId; icon: IconName; label: string; detail: string }[] = [
  { page: 'service', icon: 'order', label: 'Service Order', detail: 'Plan the flow of a service' },
  { page: 'bible', icon: 'bible', label: 'Bible workspace', detail: 'Prepare scripture for presentation' },
  { page: 'media', icon: 'media', label: 'Media Library', detail: 'Build a reusable media collection' },
];

function Home({ onNavigate }: { onNavigate: (page: PageId) => void }) {
  return (
    <>
      <div className="section-intro">
        <div><p className="eyebrow">Built for the booth</p><h2>Your workspace starts here.</h2></div>
        <p>The shell is in place. Explore the workspaces below; production features are still to come.</p>
      </div>
      <div className="home-grid">
        <section className="panel foundation-card" aria-labelledby="foundation-title">
          <h3 id="foundation-title">Foundation checklist</h3>
          <ul className="checklist">
            <li><span className="check-icon"><Icon name="check" size={14} /></span><div><strong>Persistent production shell</strong><span>Preview, live output and detection spaces</span></div></li>
            <li><span className="check-icon"><Icon name="check" size={14} /></span><div><strong>Six navigable workspaces</strong><span>Keyboard-accessible navigation</span></div></li>
            <li><span className="check-icon"><Icon name="check" size={14} /></span><div><strong>Desktop bridge status</strong><span>Reports the actual application environment</span></div></li>
          </ul>
        </section>
        <section className="panel next-card" aria-labelledby="next-title">
          <h3 id="next-title">Explore the next areas</h3>
          <div className="next-links">
            {nextAreas.map((area) => (
              <button key={area.page} type="button" onClick={() => onNavigate(area.page)} className="next-link">
                <Icon name={area.icon} size={18} /><span><strong>{area.label}</strong><small>{area.detail}</small></span><Icon name="arrow" size={16} />
              </button>
            ))}
          </div>
        </section>
      </div>
    </>
  );
}

function FeatureScaffold({ page }: { page: FeaturePage }) {
  return (
    <section className="panel feature-scaffold">
      <div className="feature-intro">
        <span className="icon-tile"><Icon name={page.icon} size={26} /></span>
        <div><p className="eyebrow">Workspace scaffold</p><h2>{page.title}</h2><p>{page.description}</p></div>
      </div>
      <div className="feature-details">
        <div><h3>Purpose</h3><p>{page.purpose}</p></div>
        <div><h3>Next implementation</h3><p>{page.next}</p></div>
      </div>
      <p className="feature-note">Navigation only changes this workspace. The production shell stays mounted.</p>
    </section>
  );
}

export default function App() {
  const [activePage, setActivePage] = useState<PageId>('home');

  return (
    <AppShell activePage={activePage} onNavigate={setActivePage}>
      {activePage === 'home'
        ? <Home onNavigate={setActivePage} />
        : <FeatureScaffold page={featurePages[activePage]} />}
    </AppShell>
  );
}
