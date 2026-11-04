import { useEffect, useState } from 'react';
import { Window } from '@wailsio/runtime';
import { App } from '../../../bindings/zowe-worship';
import type { EngineOption, StorageView } from '../../../bindings/zowe-worship/models';
import { Icon } from '../../components/Icon';
import { formatBytes, formatDateTime } from '../../lib/format';

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

// Select value for a spec typed by hand rather than discovered on disk.
const CUSTOM_ENGINE = '__custom';

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
  }, []);

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

  const translations = storage?.translations ?? [];
  const backups = storage?.backups ?? [];
  const mediaItems = storage?.mediaItems ?? 0;
  const defaultTranslationId = storage?.defaultTranslation || 'kjv';

  return (
    <main className="h-full grid grid-rows-[auto_minmax(0,1fr)] gap-3.5 pt-3.5 px-4 pb-4 overflow-hidden" aria-label="Settings">
      <header className="flex items-end justify-between gap-3.5 [&_h2]:text-[17px] [&_h2]:font-semibold [&_h2]:mt-[3px]">
        <div>
          <p className="text-[9.5px] uppercase tracking-[1.3px] font-semibold text-secondary">Zowe Worship {storage?.version ?? ''}</p>
          <h2>Settings</h2>
        </div>
        <button type="button" className="inline-grid place-items-center w-[34px] h-[34px] border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50" onClick={() => Window.Close()} aria-label="Close settings" title="Close">
          <Icon name="close" size={16} />
        </button>
      </header>

      <div className="min-h-0 overflow-y-auto grid gap-3 content-start pr-1">
        <section className="grid gap-[9px] py-3 px-[13px] bg-panel border border-border rounded-[10px] [&_h3]:text-[10px] [&_h3]:tracking-[1.1px] [&_h3]:uppercase [&_h3]:text-secondary" aria-labelledby="settings-storage-title">
          <h3 id="settings-storage-title">Where your data lives</h3>
          <p className="px-2.5 py-2 border border-border rounded-md bg-app font-mono text-[11px] leading-[1.5] break-all select-text">{storage?.dataRoot ?? 'Not available yet'}</p>
          <dl className="m-0 grid gap-[5px]">
            <div className="flex items-baseline justify-between gap-3.5 text-[11.5px] [&_dt]:text-secondary [&_dd]:m-0">
              <dt>Library database</dt>
              <dd>{formatBytes(storage?.databaseBytes ?? 0)}</dd>
            </div>
            <div className="flex items-baseline justify-between gap-3.5 text-[11.5px] [&_dt]:text-secondary [&_dd]:m-0">
              <dt>Media</dt>
              <dd>
                {formatBytes(storage?.mediaBytes ?? 0)}
                {' · '}
                {mediaItems} {mediaItems === 1 ? 'item' : 'items'}
              </dd>
            </div>
          </dl>
          <p className="text-[#6F809B] text-[10.5px] leading-[1.6]">
            Everything stays on this computer. Copying the folder above moves the songs, services and media to
            another machine.
          </p>
        </section>

        <section className="grid gap-[9px] py-3 px-[13px] bg-panel border border-border rounded-[10px] [&_h3]:text-[10px] [&_h3]:tracking-[1.1px] [&_h3]:uppercase [&_h3]:text-secondary" aria-labelledby="settings-translations-title">
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

        <section className="grid gap-[9px] py-3 px-[13px] bg-panel border border-border rounded-[10px] [&_h3]:text-[10px] [&_h3]:tracking-[1.1px] [&_h3]:uppercase [&_h3]:text-secondary" aria-labelledby="settings-scripture-title">
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

        <section className="grid gap-[9px] py-3 px-[13px] bg-panel border border-border rounded-[10px] [&_h3]:text-[10px] [&_h3]:tracking-[1.1px] [&_h3]:uppercase [&_h3]:text-secondary" aria-labelledby="settings-detection-title">
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
          {engines.length > 0 ? (
            <label className="grid gap-1.5">
              <span className="text-[10.5px] text-secondary">Speech engine</span>
              <select
                className="min-h-8 px-2.5 border border-[#35445C] rounded-md bg-app text-[11.5px] text-text"
                value={!customEngine && engines.some((engine) => engine.spec === engineSpec) ? engineSpec : customEngine ? CUSTOM_ENGINE : ''}
                disabled={busy}
                onChange={(event) => void chooseEngine(event.target.value)}
                aria-label="Speech engine"
              >
                <option value="" disabled>{engineSpec ? 'Engine not found on disk' : 'Choose an engine…'}</option>
                {engines.map((engine) => (
                  <option key={engine.spec} value={engine.spec}>{engine.label}</option>
                ))}
                <option value={CUSTOM_ENGINE}>Custom command…</option>
              </select>
            </label>
          ) : (
            <p className="text-[#6F809B] text-[10.5px] leading-[1.6]">
              No speech engine was found on this computer. Download one with
              {' '}<code className="font-mono">wails3 task asr:bench:fetch</code>, or point
              the app at an engine you already have:
            </p>
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
          {!customEngine && engines.length > 0 && (
            <p className="text-[#6F809B] text-[10px] leading-[1.6]">
              Engines are found automatically in the app's benchmarks folder and the
              engines folder in your data directory.
            </p>
          )}
        </section>

        <section className="grid gap-[9px] py-3 px-[13px] bg-panel border border-border rounded-[10px] [&_h3]:text-[10px] [&_h3]:tracking-[1.1px] [&_h3]:uppercase [&_h3]:text-secondary" aria-labelledby="settings-backups-title">
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
                    <span className="shrink-0 text-secondary text-[10.5px] inline-flex items-center gap-2">{formatBytes(backup.sizeBytes)}</span>
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

        {problem !== null && <p className="text-error text-[11.5px]" role="alert">{problem}</p>}
      </div>
    </main>
  );
}
