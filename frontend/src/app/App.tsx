import { useState } from 'react';
import { Icon } from '../components/Icon';
import type { IconName } from '../components/Icon';
import { BibleWorkspace } from '../features/bible/BibleWorkspace';
import { AudienceDisplay } from '../features/display/AudienceDisplay';
import { MediaWorkspace } from '../features/media/MediaWorkspace';
import { SongsWorkspace } from '../features/songs/SongsWorkspace';
import { ServiceOrderWorkspace } from '../features/service-order/ServiceOrderWorkspace';
import { AppShell } from '../layouts/AppShell';
import type { PageId } from '../layouts/AppShell';
import { StudioProvider } from '../stores/StudioProvider';

type FeaturePage = {
  icon: IconName;
  title: string;
  description: string;
  purpose: string;
  next: string;
};

const featurePages: Record<Exclude<PageId, 'bible' | 'service' | 'songs' | 'media'>, FeaturePage> = {
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

// The audience display is the same bundle loaded by a second window. It has no
// studio to talk to, so it skips the provider and the shell entirely.
const isAudienceView = new URLSearchParams(window.location.search).get('view') === 'audience';

export default function App() {
  const [activePage, setActivePage] = useState<PageId>('bible');

  if (isAudienceView) {
    return <AudienceDisplay />;
  }

  return (
    <StudioProvider>
      <AppShell activePage={activePage} onNavigate={setActivePage}>
        {activePage === 'bible'
          ? <BibleWorkspace />
          : activePage === 'songs'
            ? <SongsWorkspace />
            : activePage === 'media'
              ? <MediaWorkspace />
              : activePage === 'service'
                ? <ServiceOrderWorkspace />
                : <FeatureScaffold page={featurePages[activePage]} />}
      </AppShell>
    </StudioProvider>
  );
}
