import { useState } from 'react';
import type { FormEvent } from 'react';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import './service-order.css';

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

export function ServiceOrderWorkspace() {
  const studio = useStudio();
  const [name, setName] = useState('Sunday Service');
  const [busy, setBusy] = useState<string | null>(null);
  const [status, setStatus] = useState<{ kind: 'message' | 'error'; text: string } | null>(null);

  const service = studio.order?.service ?? null;
  const items = studio.order?.items ?? [];
  const liveID = studio.presentation?.live?.itemId ?? '';
  const previewID = studio.presentation?.preview?.itemId ?? '';
  const canAct = studio.bridgeAvailable && busy === null;

  async function perform(key: string, action: () => Promise<unknown>, success: string) {
    setBusy(key);
    setStatus(null);
    try {
      await action();
      setStatus({ kind: 'message', text: success });
    } catch (error: unknown) {
      setStatus({ kind: 'error', text: errorMessage(error) });
    } finally {
      setBusy(null);
    }
  }

  function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = name.trim();
    if (trimmed === '') return;
    void perform('create', () => studio.createService(trimmed), `Created ${trimmed}.`);
  }

  return (
    <section className="panel service-order" aria-labelledby="service-order-title">
      <div className="panel-heading">
        <h2 id="service-order-title"><Icon name="order" size={16} />Service Order</h2>
        <span className="quiet-label">{service ? `${items.length} items` : 'No service yet'}</span>
      </div>

      <div className="service-order-body">
        {service === null ? (
          <>
            <div className="service-order-intro">
              <span className="icon-tile"><Icon name="order" size={24} /></span>
              <div>
                <h3>Start a service</h3>
                <p>Create a service, then add scripture from the Bible workspace. The schedule appears in the sidebar and drives preview and live output.</p>
              </div>
            </div>

            <form className="service-create" onSubmit={handleCreate}>
              <div className="service-create-field">
                <Icon name="order" size={16} />
                <input
                  type="text"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  aria-label="Service name"
                  placeholder="Sunday Service"
                  autoComplete="off"
                  disabled={!studio.bridgeAvailable || busy !== null}
                />
              </div>
              <button
                type="submit"
                className="button button--primary"
                disabled={!canAct || name.trim() === ''}
              >
                Create service
              </button>
            </form>

            {!studio.bridgeAvailable && (
              <p className="service-order-note">Open the Zowe Worship desktop app to store a service.</p>
            )}
          </>
        ) : (
          <>
            <div className="service-order-head">
              <div>
                <p className="eyebrow">Current service</p>
                <h3>{service.name}</h3>
              </div>
              <span className="quiet-label">{items.length} {items.length === 1 ? 'item' : 'items'}</span>
            </div>

            {items.length === 0 ? (
              <div className="service-order-empty">
                <Icon name="bible" size={26} />
                <strong>No items yet</strong>
                <p>Add scripture from the Bible workspace with “Add to Service”.</p>
              </div>
            ) : (
              <ol className="service-order-list">
                {items.map((item) => {
                  const isLive = liveID !== '' && liveID === item.id;
                  const isPreview = previewID !== '' && previewID === item.id;
                  return (
                    <li
                      key={item.id}
                      className={`service-order-item${isLive ? ' service-order-item--live' : ''}${isPreview ? ' service-order-item--preview' : ''}`}
                    >
                      <span className="service-order-position">{item.position}</span>
                      <div className="service-order-copy">
                        <strong>{item.title}</strong>
                        <span>{item.translationId.toUpperCase()} · {item.kind}</span>
                      </div>
                      {isLive && <span className="status-pill status-pill--on">Live</span>}
                      {!isLive && isPreview && <span className="status-pill status-pill--warn">Preview</span>}
                      <button
                        type="button"
                        className="button button--small"
                        disabled={!canAct}
                        onClick={() => void perform(
                          `preview-${item.id}`,
                          () => studio.previewItem(item.id),
                          `Previewing ${item.title}.`,
                        )}
                      >
                        <Icon name="preview" size={13} />Preview
                      </button>
                      <button
                        type="button"
                        className="icon-button icon-button--small"
                        disabled={!canAct}
                        aria-label={`Remove ${item.title} from the service`}
                        title="Remove from service"
                        onClick={() => void perform(
                          `remove-${item.id}`,
                          () => studio.removeItem(item.id),
                          `Removed ${item.title}.`,
                        )}
                      >
                        <Icon name="close" size={14} />
                      </button>
                    </li>
                  );
                })}
              </ol>
            )}
          </>
        )}

        {status && (
          <p className={`service-order-status${status.kind === 'error' ? ' service-order-status--error' : ''}`} aria-live="polite">
            {status.text}
          </p>
        )}

        <p className="service-order-note">Reordering, songs and media arrive with the full service editor.</p>
      </div>
    </section>
  );
}
