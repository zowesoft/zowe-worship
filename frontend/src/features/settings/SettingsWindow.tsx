import { useEffect, useRef, useState } from 'react';
import type { KeyboardEvent as ReactKeyboardEvent } from 'react';
import { Window } from '@wailsio/runtime';
import { App } from '../../../bindings/zowe-worship';
import type { EngineOption, ShortcutsView, StorageView } from '../../../bindings/zowe-worship/models';
import { Icon } from '../../components/Icon';
import { formatBytes, formatDateTime } from '../../lib/format';
import { DEFAULT_SHORTCUTS, SHORTCUT_ACTIONS, comboFromEvent, formatCombo, isBindable } from '../../lib/shortcuts';
import type { ShortcutAction } from '../../lib/shortcuts';
import { SpeechEngineCard } from './SpeechEngineCard';

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

// Select value for a spec typed by hand rather than discovered on disk.
const CUSTOM_ENGINE = '__custom';

// The window is tall, so each group gets its own tab instead of stacking every
// card into one long scroll.
type SettingsTabId = 'bible' | 'slides' | 'detection' | 'shortcuts' | 'backups';

const SETTINGS_TABS: { id: SettingsTabId; label: string }[] = [
  { id: 'bible', label: 'Bible' },
  { id: 'slides', label: 'Slides' },
  { id: 'detection', label: 'Detection' },
  { id: 'shortcuts', label: 'Shortcuts' },
  { id: 'backups', label: 'Backups' },
];

// Every group shares this card look; the show/hide choice adds the display so a
// hidden group leaves no gap behind.
const SECTION = 'gap-[9px] py-3 px-[13px] bg-panel border border-border rounded-[10px] [&_h3]:text-[10px] [&_h3]:tracking-[1.1px] [&_h3]:uppercase [&_h3]:text-secondary';

export function SettingsWindow() {
  const [storage, setStorage] = useState<StorageView | null>(null);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const [versesPerSlide, setVersesPerSlide] = useState('one');
  const [engineSpec, setEngineSpec] = useState('');
  const [engineDraft, setEngineDraft] = useState('');
  const [engines, setEngines] = useState<EngineOption[]>([]);
  const [customEngine, setCustomEngine] = useState(false);
  const [detectionDisabled, setDetectionDisabled] = useState(false);
  const [engineSaved, setEngineSaved] = useState(false);
  // Restore arms per backup name; a second click on the armed row confirms.
  const [restoreArm, setRestoreArm] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);
  const [shortcuts, setShortcuts] = useState<ShortcutsView>(DEFAULT_SHORTCUTS);
  // The action whose capture button is armed; a window keydown listener fills it.
  const [capturing, setCapturing] = useState<ShortcutAction | null>(null);
  const [shortcutError, setShortcutError] = useState<string | null>(null);
  const [tab, setTab] = useState<SettingsTabId>('bible');
  const scroller = useRef<HTMLDivElement>(null);

  // A tab opens at its own top rather than wherever the last one was scrolled.
  useEffect(() => {
    scroller.current?.scrollTo({ top: 0 });
  }, [tab]);

  useEffect(() => {
    App.GetStorage()
      .then((view) => setStorage(view))
      .catch((error: unknown) => setProblem(errorMessage(error)));

    App.GetSetting('scripture_verses_per_slide')
      .then((verses) => {
        if (verses === 'all' || verses === 'one') setVersesPerSlide(verses);
      })
      .catch(() => {
        // An unreadable setting falls back to one verse per slide, which is what
        // the app uses when nothing has ever been written.
      });

    App.GetDetectionConfig()
      .then((config) => {
        setEngineSpec(config.engineSpec);
        setEngineDraft(config.engineSpec);
        setDetectionDisabled(config.disabled);
      })
      .catch(() => {
        // Without the bridge the detection card simply shows its empty state.
      });

    App.ListDetectionEngines()
      .then((found) => setEngines(found ?? []))
      .catch(() => setEngines([]));

    App.GetShortcuts()
      .then(setShortcuts)
      .catch(() => {
        // Without the bridge the card keeps its built-in defaults.
      });
  }, []);

  // While a row is armed, a capture-phase listener owns every keydown: Escape
  // cancels, a bare modifier keeps waiting, and anything else becomes the new
  // combo once it clears the bindable and duplicate checks. Capture phase so the
  // studio's own global handler never sees these keys.
  useEffect(() => {
    if (capturing === null) return;
    const target = capturing;
    function onKey(event: KeyboardEvent) {
      event.preventDefault();
      event.stopPropagation();
      if (event.key === 'Escape') {
        setCapturing(null);
        setShortcutError(null);
        return;
      }
      const combo = comboFromEvent(event);
      if (combo === null) return;
      if (!isBindable(combo)) {
        setShortcutError(`${formatCombo(combo)} needs Ctrl or Alt so it can't clash with typing.`);
        return;
      }
      const clash = SHORTCUT_ACTIONS.find(
        ({ action }) => action !== target && shortcuts[action] === combo,
      );
      if (clash !== undefined) {
        setShortcutError(`${clash.label} already uses ${formatCombo(combo)}.`);
        return;
      }
      const next: ShortcutsView = { ...shortcuts, [target]: combo };
      setCapturing(null);
      setShortcutError(null);
      setBusy(true);
      App.SetShortcuts(next)
        .then(() => setShortcuts(next))
        .catch((error: unknown) => setProblem(errorMessage(error)))
        .finally(() => setBusy(false));
    }
    window.addEventListener('keydown', onKey, true);
    return () => window.removeEventListener('keydown', onKey, true);
  }, [capturing, shortcuts]);

  async function resetShortcuts() {
    setBusy(true);
    setProblem(null);
    setShortcutError(null);
    try {
      setShortcuts(await App.ResetShortcuts());
      setCapturing(null);
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function chooseVerses(value: 'one' | 'all') {
    setBusy(true);
    setProblem(null);
    try {
      await App.SetSetting('scripture_verses_per_slide', value);
      setVersesPerSlide(value);
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function backUp() {
    setBusy(true);
    setProblem(null);
    try {
      setStorage(await App.BackupNow());
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function restore(name: string) {
    if (restoreArm !== name) {
      setRestoreArm(name);
      window.setTimeout(() => setRestoreArm((current) => (current === name ? null : current)), 6000);
      return;
    }
    setRestoreArm(null);
    setBusy(true);
    setProblem(null);
    try {
      setStorage(await App.RestoreBackup(name));
      setNote(`Restored ${name}. The current data was kept as its own backup.`);
    } catch (error: unknown) {
      setNote(null);
      setProblem(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function setDefault(id: string) {
    setProblem(null);
    try {
      setStorage(await App.SetDefaultTranslation(id));
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    }
  }

  async function saveEngine() {
    setBusy(true);
    setProblem(null);
    try {
      await App.SetDetectionEngine(engineDraft);
      setEngineSpec(engineDraft);
      setEngineSaved(true);
      window.setTimeout(() => setEngineSaved(false), 2500);
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  // Picking a discovered engine saves it immediately; the text field only
  // appears for a spec the scan did not find.
  async function chooseEngine(spec: string) {
    if (spec === CUSTOM_ENGINE) {
      setCustomEngine(true);
      return;
    }
    setCustomEngine(false);
    setBusy(true);
    setProblem(null);
    try {
      await App.SetDetectionEngine(spec);
      setEngineSpec(spec);
      setEngineDraft(spec);
      setEngineSaved(true);
      window.setTimeout(() => setEngineSaved(false), 2500);
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  async function toggleDetectionDisabled() {
    setBusy(true);
    setProblem(null);
    try {
      const next = !detectionDisabled;
      await App.SetDetectionDisabled(next);
      setDetectionDisabled(next);
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  // The groups that are not showing stay mounted but hidden, so a speech engine
  // download keeps its progress listener and comes back exactly where it was.
  const pane = (id: SettingsTabId) => `${SECTION} ${tab === id ? 'grid' : 'hidden'}`;

  function onTabKeyDown(event: ReactKeyboardEvent<HTMLDivElement>) {
    const current = SETTINGS_TABS.findIndex((entry) => entry.id === tab);
    let next = current;
    if (event.key === 'ArrowRight') next = (current + 1) % SETTINGS_TABS.length;
    else if (event.key === 'ArrowLeft') next = (current - 1 + SETTINGS_TABS.length) % SETTINGS_TABS.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = SETTINGS_TABS.length - 1;
    else return;
    event.preventDefault();
    const target = SETTINGS_TABS[next].id;
    setTab(target);
    document.getElementById(`settings-tab-${target}`)?.focus();
  }

  const translations = storage?.translations ?? [];
  const backups = storage?.backups ?? [];
  const defaultTranslationId = storage?.defaultTranslation || 'kjv';

  return (
    <main className="h-full grid grid-rows-[auto_auto_minmax(0,1fr)] gap-3.5 pt-3.5 px-4 pb-4 overflow-hidden" aria-label="Settings">
      <header className="flex items-end justify-between gap-3.5 [&_h2]:text-[17px] [&_h2]:font-semibold [&_h2]:mt-[3px]">
        <div>
          <p className="text-[9.5px] uppercase tracking-[1.3px] font-semibold text-secondary">Zowe Worship {storage?.version ?? ''}</p>
          <h2>Settings</h2>
        </div>
        <button type="button" className="inline-grid place-items-center w-[34px] h-[34px] border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50" onClick={() => Window.Close()} aria-label="Close settings" title="Close">
          <Icon name="close" size={16} />
        </button>
      </header>

      <div
        className="flex gap-1 p-[3px] bg-[#0F1722] rounded-[7px] border border-[#243449]"
        role="tablist"
        aria-label="Settings groups"
        onKeyDown={onTabKeyDown}
      >
        {SETTINGS_TABS.map(({ id, label }) => (
          <button
            key={id}
            type="button"
            role="tab"
            id={`settings-tab-${id}`}
            aria-selected={tab === id}
            aria-controls={`settings-panel-${id}`}
            tabIndex={tab === id ? 0 : -1}
            className={[
              'flex-1 border-0 rounded-[5px] bg-transparent text-secondary text-[11px] py-1.5 px-2 whitespace-nowrap transition-all duration-150 hover:text-text',
              tab === id ? 'bg-[#23344B]! text-text! font-medium shadow-[0_1px_3px_rgba(0,0,0,0.25)]' : '',
            ].filter(Boolean).join(' ')}
            onClick={() => setTab(id)}
          >
            {label}
          </button>
        ))}
      </div>

      <div
        ref={scroller}
        className="min-h-0 overflow-y-auto grid gap-3 content-start pr-1"
        role="tabpanel"
        id={`settings-panel-${tab}`}
        aria-labelledby={`settings-tab-${tab}`}
      >
        <section className={pane('bible')} aria-labelledby="settings-translations-title">
          <div className="flex items-center justify-between gap-3">
            <h3 id="settings-translations-title">Bible translations</h3>
            <span className="text-secondary text-[10px]">{translations.length}</span>
          </div>
          {translations.length === 0 ? (
            <p className="text-[#6F809B] text-[10.5px] leading-[1.6]">No translation is installed yet.</p>
          ) : (
            <ul className="m-0 p-0 list-none grid gap-px [&_li]:flex [&_li]:items-center [&_li]:justify-between [&_li]:gap-3.5 [&_li]:px-2 [&_li]:py-[7px] [&_li]:rounded-md [&_li:hover]:bg-elevated">
              {translations.map((translation) => {
                const isDefault = defaultTranslationId === translation.id;
                return (
                  <li key={translation.id}>
                    <span className="grid gap-0.5 min-w-0 [&_strong]:text-[11.5px] [&_strong]:font-semibold [&_span]:text-secondary [&_span]:text-[10px] [&_span]:overflow-hidden [&_span]:text-ellipsis [&_span]:whitespace-nowrap">
                      <strong>{translation.abbreviation || translation.name}</strong>
                      <span>{translation.name}</span>
                    </span>
                    <span className="shrink-0 text-secondary text-[10.5px] inline-flex items-center gap-2">
                      <span>{translation.verseCount.toLocaleString()} verses</span>
                      {isDefault ? (
                        <span className="inline-flex items-center px-2 py-[3px] border border-[#1E4D40]! rounded-full bg-[#0E2A24]! text-[#6EE7C0]! text-[9px] whitespace-nowrap" style={{ marginLeft: 8 }}>Default</span>
                      ) : (
                        <button
                          type="button"
                          className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]"
                          style={{ marginLeft: 8 }}
                          onClick={() => void setDefault(translation.id)}
                        >
                          Make default
                        </button>
                      )}
                    </span>
                  </li>
                );
              })}
            </ul>
          )}
        </section>

        <section className={pane('slides')} aria-labelledby="settings-scripture-title">
          <div className="flex items-center justify-between gap-3">
            <h3 id="settings-scripture-title">Scripture slides</h3>
          </div>
          <p className="text-[#6F809B] text-[10.5px] leading-[1.6]">
            How a passage with more than one verse becomes slides. One verse at a
            time fills the wall with the verse you are reading and puts every verse
            in its own row on the order; the whole passage keeps them together.
          </p>
          <div className="flex gap-2 flex-wrap">
            <button
              type="button"
              className={`min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border rounded-md text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! ${versesPerSlide === 'one' ? ' border-blue! bg-blue! text-white! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!' : ' border-[#35445C] bg-elevated'}`}
              disabled={busy}
              onClick={() => void chooseVerses('one')}
            >
              One verse per slide
            </button>
            <button
              type="button"
              className={`min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border rounded-md text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! ${versesPerSlide === 'all' ? ' border-blue! bg-blue! text-white! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!' : ' border-[#35445C] bg-elevated'}`}
              disabled={busy}
              onClick={() => void chooseVerses('all')}
            >
              Whole passage
            </button>
          </div>
        </section>

        <section className={pane('detection')} aria-labelledby="settings-detection-title">
          <div className="flex items-center justify-between gap-3">
            <h3 id="settings-detection-title">Live detection</h3>
            <button
              type="button"
              className={`min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border rounded-md text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! ${detectionDisabled ? 'border-[#35445C] bg-elevated' : 'border-[#1E4D40]! bg-[#0E2A24]! text-[#6EE7C0]!'}`}
              disabled={busy}
              onClick={() => void toggleDetectionDisabled()}
              title="F12 in the studio does the same thing without touching the live display"
            >
              {detectionDisabled ? 'Disabled (F12)' : 'Enabled'}
            </button>
          </div>
          <p className="text-[#6F809B] text-[10.5px] leading-[1.6]">
            The speech engine transcribes what the microphone hears and the detector pulls
            out scripture references. Nothing is ever projected on its own — every
            detection waits for your approval in the studio panel.
          </p>
          <SpeechEngineCard
            onEngineChanged={() => {
              App.GetDetectionConfig()
                .then((config) => {
                  setEngineSpec(config.engineSpec);
                  setEngineDraft(config.engineSpec);
                })
                .catch(() => {
                  // Leave the last-known spec in place if the bridge is down.
                });
            }}
          />
          {engines.length > 0 && (
            <label className="grid gap-1.5">
              <span className="text-[10.5px] text-secondary">Or pick an engine already on disk</span>
              <select
                className="min-h-8 px-2.5 border border-[#35445C] rounded-md bg-app text-[11.5px] text-text"
                value={!customEngine && engines.some((engine) => engine.spec === engineSpec) ? engineSpec : customEngine ? CUSTOM_ENGINE : ''}
                disabled={busy}
                onChange={(event) => void chooseEngine(event.target.value)}
                aria-label="Speech engine found on disk"
              >
                <option value="" disabled>{engineSpec ? 'Engine not found on disk' : 'Choose an engine…'}</option>
                {engines.map((engine) => (
                  <option key={engine.spec} value={engine.spec}>{engine.label}</option>
                ))}
                <option value={CUSTOM_ENGINE}>Custom command…</option>
              </select>
            </label>
          )}
          {(customEngine || engines.length === 0) && (
            <>
              <label className="grid gap-1.5">
                <span className="text-[10.5px] text-secondary">Engine command</span>
                <input
                  type="text"
                  className="min-h-8 px-2.5 border border-[#35445C] rounded-md bg-app font-mono text-[11px] text-text select-text"
                  value={engineDraft}
                  spellCheck={false}
                  onChange={(event) => setEngineDraft(event.target.value)}
                  placeholder="whisper.tiny=C:\path\whisper-cli.exe -m model.bin -np -f {audio}"
                  aria-label="Speech engine command"
                />
              </label>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-blue! bg-blue! text-white! rounded-md text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
                  disabled={busy || engineDraft === engineSpec || engineDraft.trim() === ''}
                  onClick={() => void saveEngine()}
                >
                  Save engine
                </button>
                {engineSaved && <span className="text-[#6EE7C0] text-[10px]">Saved</span>}
              </div>
            </>
          )}
          <p className="text-[#6F809B] text-[10px] leading-[1.6]">
            Moonshine ships with the app. The engine runs offline; only the
            download above touches the network.
          </p>
        </section>

        <section className={pane('shortcuts')} aria-labelledby="settings-shortcuts-title">
          <div className="flex items-center justify-between gap-3">
            <h3 id="settings-shortcuts-title">Shortcuts</h3>
            <button
              type="button"
              className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap hover:bg-[#1C2A40] disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]!"
              disabled={busy}
              onClick={() => void resetShortcuts()}
            >
              Reset to defaults
            </button>
          </div>
          <p className="text-[#6F809B] text-[10.5px] leading-[1.6]">
            Click a shortcut, then press the keys you want. A printable key needs
            Ctrl or Alt so it can't swallow ordinary typing. Escape cancels.
          </p>
          <ul className="m-0 p-0 list-none grid gap-px [&_li]:flex [&_li]:items-center [&_li]:justify-between [&_li]:gap-3.5 [&_li]:px-2 [&_li]:py-[7px] [&_li]:rounded-md [&_li:hover]:bg-elevated">
            {SHORTCUT_ACTIONS.map(({ action, label }) => {
              const isCapturing = capturing === action;
              return (
                <li key={action}>
                  <span className="text-[11.5px] font-medium">{label}</span>
                  <button
                    type="button"
                    className={`min-h-7 inline-flex items-center justify-center px-2.5 py-1 border rounded-md font-mono text-[11px] whitespace-nowrap ${isCapturing ? 'border-blue! bg-blue! text-white!' : 'border-[#35445C] bg-elevated hover:bg-[#1C2A40]'}`}
                    disabled={busy && !isCapturing}
                    onClick={() => {
                      setShortcutError(null);
                      setCapturing(isCapturing ? null : action);
                    }}
                    aria-pressed={isCapturing}
                  >
                    {isCapturing ? 'Press keys…' : formatCombo(shortcuts[action])}
                  </button>
                </li>
              );
            })}
          </ul>
          {shortcutError !== null && (
            <p className="text-error text-[10px] leading-snug" role="alert">{shortcutError}</p>
          )}
        </section>

        <section className={pane('backups')} aria-labelledby="settings-backups-title">
          <div className="flex items-center justify-between gap-3">
            <h3 id="settings-backups-title">Backups</h3>
            <button
              type="button"
              className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-blue! rounded-md bg-blue! text-white! text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
              disabled={busy}
              onClick={() => void backUp()}
            >
              {busy ? 'Saving…' : 'Back up now'}
            </button>
          </div>
          {backups.length === 0 ? (
            <p className="text-[#6F809B] text-[10.5px] leading-[1.6]">
              No backup has been taken. The app saves one automatically before an update changes the data.
            </p>
          ) : (
            <>
              <ul className="m-0 p-0 list-none grid gap-px [&_li]:flex [&_li]:items-center [&_li]:justify-between [&_li]:gap-3.5 [&_li]:px-2 [&_li]:py-[7px] [&_li]:rounded-md [&_li:hover]:bg-elevated">
                {backups.map((backup) => (
                  <li key={backup.name}>
                    <span className="grid gap-0.5 min-w-0 [&_strong]:text-[11.5px] [&_strong]:font-semibold [&_span]:text-secondary [&_span]:text-[10px] [&_span]:overflow-hidden [&_span]:text-ellipsis [&_span]:whitespace-nowrap">
                      <strong>{formatDateTime(backup.createdAt)}</strong>
                      <span>{backup.name}</span>
                    </span>
                    <span className="shrink-0 text-secondary text-[10.5px] inline-flex items-center gap-2">
                      {formatBytes(backup.sizeBytes)}
                      <button
                        type="button"
                        className="min-h-7 inline-flex items-center justify-center px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-text text-[10.5px] font-medium whitespace-nowrap hover:bg-[#1C2A40] disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]!"
                        disabled={busy}
                        onClick={() => void restore(backup.name)}
                        title="Replace the current data with this backup"
                      >
                        {restoreArm === backup.name ? 'Confirm?' : 'Restore'}
                      </button>
                    </span>
                  </li>
                ))}
              </ul>
              {storage !== null && storage.backupCount > backups.length && (
                <p className="text-[#6F809B] text-[10.5px] leading-[1.6]">
                  {storage.backupCount - backups.length} older {storage.backupCount - backups.length === 1 ? 'copy' : 'copies'} in the backups folder.
                </p>
              )}
            </>
          )}
        </section>

        {note !== null && <p className="text-[#8BBA8B] text-[11.5px]" role="status">{note}</p>}
        {problem !== null && <p className="text-error text-[11.5px]" role="alert">{problem}</p>}
      </div>
    </main>
  );
}
