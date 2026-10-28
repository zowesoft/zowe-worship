import { useState } from 'react';
import { Icon } from '../components/Icon';
import type { IconName } from '../components/Icon';
import { BibleWorkspace } from '../features/bible/BibleWorkspace';
import { AppShell } from '../layouts/AppShell';
import type { PageId } from '../layouts/AppShell';

type FeaturePage = {
  icon: IconName;
  title: string;
  description: string;
  purpose: string;
  next: string;
};

const featurePages: Record<Exclude<PageId, 'bible'>, FeaturePage> = {
  service: {
    icon: 'order',
    title: 'A place for the whole service',
    description: 'No service order yet. Planning and service storage are not implemented.',
    purpose: 'Organize scripture, songs and media into an ordered service without changing the live output.',
    next: 'Service storage, item editing and operator-controlled cues.',
  },
  songs: {
    icon: 'music',
    title: 'Songs and lyrics, ready for the set',
    description: 'No songs are stored yet. Lyrics management and display are not implemented.',
    purpose: 'Keep reusable songs with sections and arrangements, then prepare lyric slides for a service without changing the live output.',
    next: 'Song storage, sections, arrangements and lyric presentation.',
  },
  media: {
    icon: 'media',
    title: 'Your media library starts here',
    description: 'No media is available. Importing and indexing are not implemented.',
    purpose: 'Browse reusable media and prepare selected items for a service. Local media management will be handled by the Go backend.',
    next: 'Local imports, metadata indexing and media previews.',
  },
  settings: {
    icon: 'settings',
    title: 'Configure the workspace, in time',
    description: 'There are no editable settings yet. Configuration persistence is not implemented.',
    purpose: 'Provide one place for output displays, audio devices, speech models and application preferences, backed by desktop configuration services.',
    next: 'Typed configuration, device discovery and saved preferences.',
  },
};

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
  const [activePage, setActivePage] = useState<PageId>('bible');

  return (
    <AppShell activePage={activePage} onNavigate={setActivePage}>
      {activePage === 'bible'
        ? <BibleWorkspace />
        : <FeatureScaffold page={featurePages[activePage]} />}
    </AppShell>
  );
}
