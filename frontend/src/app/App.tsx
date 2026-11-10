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
import type { BibleFocus, BiblePassage } from '../features/bible/focus';
import { StudioProvider } from '../stores/StudioProvider';
import { DetectionProvider } from '../features/detection/DetectionProvider';
import { ShortcutsProvider } from '../stores/ShortcutsProvider';

// Secondary windows are the same bundle loaded at a different address. They have
// no studio to talk to, so they skip the provider and the shell entirely.
const viewParams = new URLSearchParams(window.location.search);
const windowView = viewParams.get('view');

export default function App() {
  const [activePage, setActivePage] = useState<PageId>('home');
  // A detection card's Search action switches to the Bible page and opens the
  // passage there. The workspace clears it once the jump is under way, so
  // returning to the Bible tab later does not reopen an old reference.
  const [bibleFocus, setBibleFocus] = useState<BibleFocus | null>(null);

  function openBibleAt(passage: BiblePassage) {
    setBibleFocus({ ...passage, nonce: Date.now() });
    setActivePage('bible');
  }

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
      <DetectionProvider>
        <ShortcutsProvider>
          <AppShell activePage={activePage} onNavigate={setActivePage} onOpenBible={openBibleAt}>
            {activePage === 'home'
              ? <HomeWorkspace onNavigate={setActivePage} />
              : activePage === 'bible'
                ? <BibleWorkspace focus={bibleFocus} onFocusHandled={() => setBibleFocus(null)} />
                : activePage === 'songs'
                  ? <SongsWorkspace />
                  : activePage === 'media'
                    ? <MediaWorkspace />
                    : <ServiceOrderWorkspace />}
          </AppShell>
        </ShortcutsProvider>
      </DetectionProvider>
    </StudioProvider>
  );
}
