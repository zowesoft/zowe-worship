import { useState } from 'react';
import { BibleWorkspace } from '../features/bible/BibleWorkspace';
import { AudienceDisplay } from '../features/display/AudienceDisplay';
import { HomeWorkspace } from '../features/home/HomeWorkspace';
import { MediaEditorWindow } from '../features/media/MediaEditorWindow';
import { MediaWorkspace } from '../features/media/MediaWorkspace';
import { SettingsWindow } from '../features/settings/SettingsWindow';
import { SongsWorkspace } from '../features/songs/SongsWorkspace';
import { SongEditorWindow } from '../features/songs/SongEditorWindow';
import { ServiceOrderWorkspace } from '../features/service-order/ServiceOrderWorkspace';
import { ThemesWindow } from '../features/themes/ThemesWindow';
import { AppShell } from '../layouts/AppShell';
import type { PageId } from '../layouts/AppShell';
import { StudioProvider } from '../stores/StudioProvider';

// Secondary windows are the same bundle loaded at a different address. They have
// no studio to talk to, so they skip the provider and the shell entirely.
const viewParams = new URLSearchParams(window.location.search);
const windowView = viewParams.get('view');

export default function App() {
  const [activePage, setActivePage] = useState<PageId>('home');

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
  if (windowView === 'themes') {
    return <ThemesWindow />;
  }

  return (
    <StudioProvider>
      <AppShell activePage={activePage} onNavigate={setActivePage}>
        {activePage === 'home'
          ? <HomeWorkspace onNavigate={setActivePage} />
          : activePage === 'bible'
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
