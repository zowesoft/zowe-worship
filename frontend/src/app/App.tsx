import { useState } from 'react';
import { BibleWorkspace } from '../features/bible/BibleWorkspace';
import { AudienceDisplay } from '../features/display/AudienceDisplay';
import { MediaEditorWindow } from '../features/media/MediaEditorWindow';
import { MediaWorkspace } from '../features/media/MediaWorkspace';
import { SettingsWindow } from '../features/settings/SettingsWindow';
import { SongsWorkspace } from '../features/songs/SongsWorkspace';
import { SongEditorWindow } from '../features/songs/SongEditorWindow';
import { ServiceOrderWorkspace } from '../features/service-order/ServiceOrderWorkspace';
import { AppShell } from '../layouts/AppShell';
import type { PageId } from '../layouts/AppShell';
import { StudioProvider } from '../stores/StudioProvider';

// Secondary windows are the same bundle loaded at a different address. They have
// no studio to talk to, so they skip the provider and the shell entirely.
const viewParams = new URLSearchParams(window.location.search);
const windowView = viewParams.get('view');

export default function App() {
  const [activePage, setActivePage] = useState<PageId>('bible');

  if (windowView === 'audience') {
    return <AudienceDisplay />;
  }
  if (windowView === 'media-editor') {
    return <MediaEditorWindow mediaID={viewParams.get('media') ?? ''} />;
  }
  if (windowView === 'song-editor') {
    return <SongEditorWindow songID={viewParams.get('song') ?? ''} />;
  }
  if (windowView === 'settings') {
    return <SettingsWindow />;
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
              : <ServiceOrderWorkspace />}
      </AppShell>
    </StudioProvider>
  );
}
