import { useEffect, useState } from 'react';
import { Events } from '@wailsio/runtime';
import { App } from '../../../bindings/zopresent';
import type { SpeechAttributionView, SpeechEngineView } from '../../../bindings/zopresent/models';
import { Icon } from '../../components/Icon';
import { formatBytes } from '../../lib/format';

// The download runs on the Go side and reports progress through events; this
// component only renders it. The payload has no generated model, so its shape
// is mirrored here.
type SpeechProgressView = {
  engineId: string;
  file: string;
  done: number;
  total: number;
  bytes: number;
  totalBytes: number;
};

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

export function SpeechEngineCard({
  onEngineChanged,
}: {
  onEngineChanged: () => void;
}) {
  const [engines, setEngines] = useState<SpeechEngineView[]>([]);
  const [progress, setProgress] = useState<SpeechProgressView | null>(null);
  const [installing, setInstalling] = useState<string | null>(null);
  const [problem, setProblem] = useState<string | null>(null);
  const [attribution, setAttribution] = useState<SpeechAttributionView | null>(null);
  const [showLicenses, setShowLicenses] = useState(false);

  const refresh = () => {
    App.ListSpeechEngines()
      .then((list) => setEngines(list ?? []))
      .catch((error: unknown) => setProblem(errorMessage(error)));
  };

  // Notices only exist once an engine is on disk, so this is re-read after an
  // install rather than only on mount.
  const loadAttribution = () => {
    App.GetSpeechAttribution()
      .then(setAttribution)
      .catch(() => {
        // Attribution is informational; a failure leaves the section hidden.
      });
  };

  useEffect(() => {
    refresh();
    loadAttribution();
  }, []);

  // An empty file in the progress event clears the bar; speech:config carries
  // the outcome.
  useEffect(() => {
    const offProgress = Events.On('speech:progress', (event) => {
      const update = event.data as SpeechProgressView;
      if (!update.file) {
        setProgress(null);
        return;
      }
      setProgress(update);
    });
    const offConfig = Events.On('speech:config', (event) => {
      const result = event.data as { engineId: string; ok: boolean; error: string };
      setInstalling(null);
      setProgress(null);
      if (!result.ok && result.error) {
        setProblem(result.error);
      } else {
        setProblem(null);
        onEngineChanged();
      }
      refresh();
      loadAttribution();
    });
    return () => {
      offProgress();
      offConfig();
    };
  }, []);

  async function install(id: string) {
    setProblem(null);
    setInstalling(id);
    try {
      await App.InstallSpeechEngine(id);
    } catch (error: unknown) {
      setInstalling(null);
      setProblem(errorMessage(error));
    }
  }

  const percent = progress && progress.totalBytes > 0
    ? Math.min(100, Math.round((progress.bytes * 100) / progress.totalBytes))
    : 0;

  return (
    <div className="grid gap-[9px]">
      <span className="text-[10.5px] text-secondary">Speech engine</span>
      <ul className="m-0 p-0 list-none grid gap-1.5">
        {engines.map((engine) => {
          const busy = installing === engine.id;
          const isActive = engine.active;
          return (
            <li
              key={engine.id}
              className="grid gap-1.5 p-2.5 border border-border rounded-[7px] bg-app"
            >
              <div className="flex items-baseline gap-2 min-w-0">
                <Icon name="detection" size={14} className="text-[#B3A4FF] shrink-0 translate-y-px" />
                <strong className="text-[11.5px] font-semibold truncate">{engine.name}</strong>
                {isActive && (
                  <span className="ml-auto inline-flex items-center px-2 py-[3px] border border-[#1E4D40]! rounded-full bg-[#0E2A24]! text-[#6EE7C0]! text-[9px] whitespace-nowrap">
                    In use
                  </span>
                )}
                {!isActive && engine.bundled && engine.installed && (
                  <span className="ml-auto text-secondary text-[9.5px] whitespace-nowrap">Bundled</span>
                )}
              </div>
              <p className="text-secondary text-[10px] leading-[1.5]">{engine.description}</p>

              {busy && progress ? (
                <div className="grid gap-1">
                  <div className="h-1.5 rounded-full bg-elevated overflow-hidden" role="progressbar" aria-valuenow={percent} aria-valuemin={0} aria-valuemax={100}>
                    <div className="h-full bg-blue transition-[width] duration-150" style={{ width: `${percent}%` }} />
                  </div>
                  <span className="text-secondary text-[9.5px]">
                    Downloading {progress.file} · {percent}%{progress.totalBytes > 0 ? ` of ${formatBytes(progress.totalBytes)}` : ''}
                  </span>
                </div>
              ) : (
                <div className="flex gap-1.5">
                  {!engine.supported ? (
                    <span className="text-[#6F809B] text-[10px]">Not available for this platform.</span>
                  ) : engine.installed ? (
                    isActive ? (
                      <span className="text-secondary text-[10px]">Ready to use.</span>
                    ) : (
                      <button
                        type="button"
                        className="min-h-7 inline-flex flex-1 items-center justify-center gap-1.5 px-2 py-1 border border-blue! bg-blue! text-white! rounded-md text-[10.5px] font-medium whitespace-nowrap not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
                        onClick={() => void choose(engine.id)}
                      >
                        <Icon name="check" size={12} />Use this engine
                      </button>
                    )
                  ) : (
                    <>
                      <button
                        type="button"
                        className="min-h-7 inline-flex flex-1 items-center justify-center gap-1.5 px-2 py-1 border border-blue! bg-blue! text-white! rounded-md text-[10.5px] font-medium whitespace-nowrap not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
                        disabled={installing !== null}
                        onClick={() => void install(engine.id)}
                      >
                        <Icon name="play" size={12} />Get engine
                      </button>
                      <span className="text-secondary text-[9.5px] self-center whitespace-nowrap">~140 MB</span>
                    </>
                  )}
                </div>
              )}
            </li>
          );
        })}
      </ul>

      {problem !== null && <p className="text-error text-[10.5px] leading-[1.5]" role="alert">{problem}</p>}
      <p className="text-[#6F809B] text-[10px] leading-[1.6]">
        Engines are found in the app folder and your data folder. Whisper and other
        models can be added later without changing anything here.
      </p>

      {attribution && attribution.components && attribution.components.length > 0 && (
        <div className="grid gap-1.5 pt-1 border-t border-border">
          <button
            type="button"
            className="justify-self-start inline-flex items-center gap-1.5 text-[10.5px] text-secondary hover:text-text"
            onClick={() => setShowLicenses((v) => !v)}
            aria-expanded={showLicenses}
          >
            <Icon
              name="chevron"
              size={12}
              className={`transition-transform ${showLicenses ? '' : '-rotate-90'}`}
            />
            Bundled engine licenses
          </button>
          {showLicenses && (
            <div className="grid gap-2">
              <ul className="m-0 p-0 list-none grid gap-1 text-[10.5px] text-secondary">
                {attribution.components.map((c) => (
                  <li key={c.name} className="flex items-baseline justify-between gap-3">
                    <span className="truncate">{c.name}</span>
                    <span className="shrink-0">{c.license}</span>
                  </li>
                ))}
              </ul>
              {attribution.noticesPath && (
                <button
                  type="button"
                  className="justify-self-start text-[10.5px] text-blue hover:underline"
                  onClick={() => void App.OpenPath(attribution.noticesPath!)}
                >
                  Open the full notices file
                </button>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );

  // The backend builds and stores the command line, so no path reaches the UI.
  async function choose(id: string) {
    setProblem(null);
    try {
      await App.SelectSpeechEngine(id);
      onEngineChanged();
      refresh();
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    }
  }
}
