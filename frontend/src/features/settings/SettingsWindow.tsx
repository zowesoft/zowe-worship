import { useEffect, useState } from 'react';
import { Window } from '@wailsio/runtime';
import { App } from '../../../bindings/zowe-worship';
import type { StorageView } from '../../../bindings/zowe-worship/models';
import { Icon } from '../../components/Icon';
import { formatBytes, formatDateTime } from '../../lib/format';
import './settings-window.css';

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

export function SettingsWindow() {
  const [storage, setStorage] = useState<StorageView | null>(null);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);

  useEffect(() => {
    App.GetStorage()
      .then((view) => setStorage(view))
      .catch((error: unknown) => setProblem(errorMessage(error)));
  }, []);

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

  const translations = storage?.translations ?? [];
  const backups = storage?.backups ?? [];
  const mediaItems = storage?.mediaItems ?? 0;
  const defaultTranslationId = storage?.defaultTranslation || 'kjv';

  return (
    <main className="settings-window" aria-label="Settings">
      <header className="settings-header">
        <div>
          <p className="eyebrow">Zowe Worship {storage?.version ?? ''}</p>
          <h2>Settings</h2>
        </div>
        <button type="button" className="icon-button" onClick={() => Window.Close()} aria-label="Close settings" title="Close">
          <Icon name="close" size={16} />
        </button>
      </header>

      <div className="settings-body">
        <section className="settings-card" aria-labelledby="settings-storage-title">
          <h3 id="settings-storage-title">Where your data lives</h3>
          <p className="settings-path">{storage?.dataRoot ?? 'Not available yet'}</p>
          <dl className="settings-rows">
            <div className="settings-row">
              <dt>Library database</dt>
              <dd>{formatBytes(storage?.databaseBytes ?? 0)}</dd>
            </div>
            <div className="settings-row">
              <dt>Media</dt>
              <dd>
                {formatBytes(storage?.mediaBytes ?? 0)}
                {' · '}
                {mediaItems} {mediaItems === 1 ? 'item' : 'items'}
              </dd>
            </div>
          </dl>
          <p className="settings-note">
            Everything stays on this computer. Copying the folder above moves the songs, services and media to
            another machine.
          </p>
        </section>

        <section className="settings-card" aria-labelledby="settings-translations-title">
          <div className="settings-card-head">
            <h3 id="settings-translations-title">Bible translations</h3>
            <span className="settings-count">{translations.length}</span>
          </div>
          {translations.length === 0 ? (
            <p className="settings-note">No translation is installed yet.</p>
          ) : (
            <ul className="settings-list">
              {translations.map((translation) => {
                const isDefault = defaultTranslationId === translation.id;
                return (
                  <li key={translation.id}>
                    <span className="settings-list-main">
                      <strong>{translation.abbreviation || translation.name}</strong>
                      <span>{translation.name}</span>
                    </span>
                    <span className="settings-list-side">
                      <span>{translation.verseCount.toLocaleString()} verses</span>
                      {isDefault ? (
                        <span className="status-pill status-pill--on" style={{ marginLeft: 8 }}>Default</span>
                      ) : (
                        <button
                          type="button"
                          className="button button--small"
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

        <section className="settings-card" aria-labelledby="settings-backups-title">
          <div className="settings-card-head">
            <h3 id="settings-backups-title">Backups</h3>
            <button
              type="button"
              className="button button--small button--primary"
              disabled={busy}
              onClick={() => void backUp()}
            >
              {busy ? 'Saving…' : 'Back up now'}
            </button>
          </div>
          {backups.length === 0 ? (
            <p className="settings-note">
              No backup has been taken. The app saves one automatically before an update changes the data.
            </p>
          ) : (
            <>
              <ul className="settings-list">
                {backups.map((backup) => (
                  <li key={backup.name}>
                    <span className="settings-list-main">
                      <strong>{formatDateTime(backup.createdAt)}</strong>
                      <span>{backup.name}</span>
                    </span>
                    <span className="settings-list-side">{formatBytes(backup.sizeBytes)}</span>
                  </li>
                ))}
              </ul>
              {storage !== null && storage.backupCount > backups.length && (
                <p className="settings-note">
                  {storage.backupCount - backups.length} older {storage.backupCount - backups.length === 1 ? 'copy' : 'copies'} in the backups folder.
                </p>
              )}
            </>
          )}
        </section>

        {problem !== null && <p className="settings-error" role="alert">{problem}</p>}
      </div>
    </main>
  );
}
