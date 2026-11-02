import { useState } from 'react';
import type { FormEvent } from 'react';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import { moveWithin, useDragOrder } from '../../lib/useDragOrder';
import type * as models from '../../../bindings/zowe-worship/models';
import './service-order.css';

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

function formatDuration(seconds?: number): string | null {
  if (!seconds || seconds <= 0) return null;
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  if (m > 0 && s > 0) return `${m}m ${s}s`;
  if (m > 0) return `${m}m`;
  return `${s}s`;
}

function formatKindLabel(item: models.ServiceItemView): string {
  if (item.kind === 'scripture') {
    return `${item.translationId ? item.translationId.toUpperCase() : 'KJV'} · Scripture`;
  }
  if (item.kind === 'song') return 'Song';
  if (item.kind === 'media') return 'Media';
  if (item.kind === 'text') return 'Text Slide';
  if (item.kind === 'announcement') return 'Announcement';
  if (item.kind === 'blank') return 'Blank Divider';
  return item.kind;
}

function StatusBadge({ status }: { status?: string }) {
  if (status === 'live') {
    return (
      <span className="status-pill status-pill--on" title="Currently showing on live display">
        <span className="pulse-dot" />Live
      </span>
    );
  }
  if (status === 'next') {
    return (
      <span className="status-pill status-pill--warn" title="Next in line / currently in preview">
        Next
      </span>
    );
  }
  if (status === 'completed') {
    return (
      <span className="status-pill status-pill--done" title="Item presentation finished">
        <Icon name="check" size={11} />Done
      </span>
    );
  }
  return (
    <span className="status-pill status-pill--subtle" title="Upcoming schedule item">
      Upcoming
    </span>
  );
}

export function ServiceOrderWorkspace() {
  const studio = useStudio();
  const [name, setName] = useState('Sunday Service');
  const [renaming, setRenaming] = useState(false);
  const [draft, setDraft] = useState('');
  const [busy, setBusy] = useState<string | null>(null);
  const [status, setStatus] = useState<{ kind: 'message' | 'error'; text: string } | null>(null);

  // Add Custom Item dialog state
  const [showAddModal, setShowAddModal] = useState(false);
  const [addKind, setAddKind] = useState<'text' | 'announcement' | 'blank'>('text');
  const [addTitle, setAddTitle] = useState('');
  const [addContent, setAddContent] = useState('');
  const [addNotes, setAddNotes] = useState('');
  const [addMinutes, setAddMinutes] = useState('');

  // Inline meta editing state
  const [editingItemId, setEditingItemId] = useState<string | null>(null);
  const [editNotes, setEditNotes] = useState('');
  const [editMinutes, setEditMinutes] = useState('');

  const service = studio.order?.service ?? null;
  const items = studio.order?.items ?? [];
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

  async function moveItem(from: number, to: number) {
    await perform(
      'reorder',
      () => studio.reorderItems(moveWithin(items.map((item) => item.id), from, to)),
      'Saved the new order.',
    );
  }

  async function submitRename() {
    const trimmed = draft.trim();
    if (trimmed === '' || service === null) return;
    await perform('rename', () => studio.renameService(service.id, trimmed), `Renamed to ${trimmed}.`);
    setRenaming(false);
  }

  async function handleDuplicate() {
    if (!service) return;
    const copyName = `${service.name} (Copy)`;
    await perform(
      'duplicate',
      () => studio.duplicateService(service.id, copyName),
      `Duplicated as ${copyName}.`,
    );
  }

  async function handleAddCustom(event: FormEvent) {
    event.preventDefault();
    const title = addTitle.trim() || (addKind === 'blank' ? 'Blank Slide' : '');
    if (!title) return;

    const durationSeconds = Math.max(0, Math.round((parseFloat(addMinutes) || 0) * 60));

    await perform(
      'add-custom',
      () =>
        studio.addCustomItem({
          kind: addKind,
          title,
          content: addContent.trim(),
          notes: addNotes.trim(),
          duration: durationSeconds,
        }),
      `Added ${title} to the service.`,
    );

    setShowAddModal(false);
    setAddTitle('');
    setAddContent('');
    setAddNotes('');
    setAddMinutes('');
  }

  function startEditingMeta(item: models.ServiceItemView) {
    setEditingItemId(item.id);
    setEditNotes(item.notes || '');
    setEditMinutes(item.duration ? String(Math.round((item.duration / 60) * 10) / 10) : '');
  }

  async function saveMeta(itemID: string) {
    const durationSeconds = Math.max(0, Math.round((parseFloat(editMinutes) || 0) * 60));
    await perform(
      `meta-${itemID}`,
      () => studio.updateItemMeta(itemID, editNotes.trim(), durationSeconds),
      'Updated slide details.',
    );
    setEditingItemId(null);
  }

  const drag = useDragOrder((from, to) => void moveItem(from, to));

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
                <p>Create a service, then add scripture, songs, media, text slides, announcements, or blank dividers to build your schedule.</p>
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
              {renaming ? (
                <form
                  className="service-rename"
                  onSubmit={(event) => {
                    event.preventDefault();
                    void submitRename();
                  }}
                >
                  <input
                    type="text"
                    value={draft}
                    aria-label="Service name"
                    autoComplete="off"
                    autoFocus
                    onChange={(event) => setDraft(event.target.value)}
                  />
                  <button type="submit" className="button button--small button--primary" disabled={draft.trim() === ''}>
                    Save
                  </button>
                  <button type="button" className="button button--small" onClick={() => setRenaming(false)}>
                    Cancel
                  </button>
                </form>
              ) : (
                <div>
                  <p className="eyebrow">Current service</p>
                  <h3>{service.name}</h3>
                </div>
              )}

              <div className="service-order-head-actions">
                <span className="quiet-label">{items.length} {items.length === 1 ? 'item' : 'items'}</span>
                {!renaming && (
                  <>
                    <button
                      type="button"
                      className="button button--small"
                      disabled={!canAct}
                      onClick={() => {
                        setDraft(service.name);
                        setRenaming(true);
                      }}
                      title="Rename this service"
                    >
                      Rename
                    </button>
                    <button
                      type="button"
                      className="button button--small"
                      disabled={!canAct}
                      onClick={() => void handleDuplicate()}
                      title="Duplicate this service schedule"
                    >
                      <Icon name="copy" size={13} />Duplicate
                    </button>
                    <button
                      type="button"
                      className="button button--small button--primary"
                      disabled={!canAct}
                      onClick={() => setShowAddModal(true)}
                      title="Add a text slide, announcement, or blank divider"
                    >
                      <Icon name="plus" size={13} />Add Slide
                    </button>
                  </>
                )}
              </div>
            </div>

            {/* Modal for adding custom slides */}
            {showAddModal && (
              <div className="custom-item-modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="custom-modal-title">
                <form className="custom-item-modal" onSubmit={(e) => void handleAddCustom(e)}>
                  <div className="custom-modal-header">
                    <h3 id="custom-modal-title">Add Slide to Service</h3>
                    <button
                      type="button"
                      className="icon-button icon-button--small"
                      onClick={() => setShowAddModal(false)}
                      aria-label="Close"
                    >
                      <Icon name="close" size={14} />
                    </button>
                  </div>

                  <div className="custom-modal-tabs" role="tablist">
                    <button
                      type="button"
                      role="tab"
                      aria-selected={addKind === 'text'}
                      className={`custom-modal-tab${addKind === 'text' ? ' custom-modal-tab--active' : ''}`}
                      onClick={() => setAddKind('text')}
                    >
                      Text Slide
                    </button>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={addKind === 'announcement'}
                      className={`custom-modal-tab${addKind === 'announcement' ? ' custom-modal-tab--active' : ''}`}
                      onClick={() => setAddKind('announcement')}
                    >
                      Announcement
                    </button>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={addKind === 'blank'}
                      className={`custom-modal-tab${addKind === 'blank' ? ' custom-modal-tab--active' : ''}`}
                      onClick={() => setAddKind('blank')}
                    >
                      Blank Divider
                    </button>
                  </div>

                  <div className="custom-modal-fields">
                    <label className="field-label">
                      <span>Title {addKind !== 'blank' && <strong className="required">*</strong>}</span>
                      <input
                        type="text"
                        className="modal-input"
                        placeholder={addKind === 'blank' ? 'e.g. Silent Prayer (optional)' : addKind === 'announcement' ? 'e.g. Youth Camp This Saturday' : 'e.g. Call to Worship'}
                        value={addTitle}
                        onChange={(e) => setAddTitle(e.target.value)}
                        autoFocus
                        required={addKind !== 'blank'}
                      />
                    </label>

                    {addKind !== 'blank' && (
                      <label className="field-label">
                        <span>Slide Text</span>
                        <textarea
                          className="modal-textarea"
                          rows={4}
                          placeholder={addKind === 'announcement' ? 'Enter announcement details or instructions…' : 'Enter lines to present…'}
                          value={addContent}
                          onChange={(e) => setAddContent(e.target.value)}
                        />
                      </label>
                    )}

                    <div className="field-row">
                      <label className="field-label">
                        <span>Estimated Duration (mins)</span>
                        <input
                          type="number"
                          step="0.5"
                          min="0"
                          className="modal-input"
                          placeholder="e.g. 2"
                          value={addMinutes}
                          onChange={(e) => setAddMinutes(e.target.value)}
                        />
                      </label>
                      <label className="field-label">
                        <span>Operator Notes</span>
                        <input
                          type="text"
                          className="modal-input"
                          placeholder="e.g. Dim lights, quiet background music"
                          value={addNotes}
                          onChange={(e) => setAddNotes(e.target.value)}
                        />
                      </label>
                    </div>
                  </div>

                  <div className="custom-modal-actions">
                    <button
                      type="button"
                      className="button button--small"
                      onClick={() => setShowAddModal(false)}
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      className="button button--small button--primary"
                      disabled={addKind !== 'blank' && addTitle.trim() === ''}
                    >
                      Add to Service
                    </button>
                  </div>
                </form>
              </div>
            )}

            {items.length === 0 ? (
              <div className="service-order-empty">
                <Icon name="order" size={26} />
                <strong>No items yet</strong>
                <p>Add scripture from Bible search, song sections, media, or click “Add Slide” for custom text slides and announcements.</p>
              </div>
            ) : (
              <ol className="service-order-list">
                {items.map((item, index) => {
                  const isLive = item.status === 'live';
                  const isPreview = item.status === 'next';
                  const isDropTarget = drag.over === index && drag.dragging !== null && drag.dragging !== index;
                  const isEditingThis = editingItemId === item.id;
                  const durationLabel = formatDuration(item.duration);

                  return (
                    <li
                      key={item.id}
                      className={[
                        'service-order-item',
                        isLive ? 'service-order-item--live' : '',
                        isPreview ? 'service-order-item--preview' : '',
                        item.status === 'completed' ? 'service-order-item--completed' : '',
                        drag.dragging === index ? 'service-order-item--dragging' : '',
                        isDropTarget ? 'service-order-item--target' : '',
                      ].filter(Boolean).join(' ')}
                      title="Drag to reorder · Double-click to preview"
                      onDoubleClick={() => void perform(
                        `preview-${item.id}`,
                        () => studio.previewItem(item.id),
                        `Previewing ${item.title}.`,
                      )}
                      {...drag.rowProps(index)}
                    >
                      <span className="service-order-grip" aria-hidden="true" title="Drag to reorder"><Icon name="grip" size={14} /></span>
                      <span className="service-order-position">{item.position}</span>

                      <div className="service-order-copy">
                        <div className="service-order-title-row">
                          <strong>{item.title}</strong>
                          {durationLabel && <span className="item-duration-tag">⏱ {durationLabel}</span>}
                        </div>
                        <div className="service-order-meta-row">
                          <span>{formatKindLabel(item)}</span>
                          {item.notes && <span className="item-notes-text">· 📝 {item.notes}</span>}
                        </div>

                        {/* Inline editor for notes & duration */}
                        {isEditingThis && (
                          <div className="inline-meta-editor" onClick={(e) => e.stopPropagation()}>
                            <div className="inline-meta-inputs">
                              <input
                                type="text"
                                className="inline-input"
                                placeholder="Operator notes…"
                                value={editNotes}
                                onChange={(e) => setEditNotes(e.target.value)}
                              />
                              <input
                                type="number"
                                step="0.5"
                                min="0"
                                className="inline-input inline-input--narrow"
                                placeholder="Mins"
                                value={editMinutes}
                                onChange={(e) => setEditMinutes(e.target.value)}
                              />
                            </div>
                            <div className="inline-meta-actions">
                              <button
                                type="button"
                                className="button button--small button--primary"
                                onClick={() => void saveMeta(item.id)}
                              >
                                Save Details
                              </button>
                              <button
                                type="button"
                                className="button button--small"
                                onClick={() => setEditingItemId(null)}
                              >
                                Cancel
                              </button>
                            </div>
                          </div>
                        )}
                      </div>

                      <StatusBadge status={item.status} />

                      <div className="service-order-row-actions">
                        <button
                          type="button"
                          className="button button--small"
                          disabled={!canAct}
                          onClick={(e) => {
                            e.stopPropagation();
                            void perform(
                              `preview-${item.id}`,
                              () => studio.previewItem(item.id),
                              `Previewing ${item.title}.`,
                            );
                          }}
                          title="Preview slide"
                        >
                          <Icon name="preview" size={13} />Preview
                        </button>
                        <button
                          type="button"
                          className="button button--small button--primary"
                          disabled={!canAct}
                          onClick={(e) => {
                            e.stopPropagation();
                            void perform(
                              `live-${item.id}`,
                              async () => {
                                await studio.previewItem(item.id);
                                await studio.sendPreviewLive();
                              },
                              `${item.title} is now live.`,
                            );
                          }}
                          title="Send directly to live audience display"
                        >
                          <Icon name="arrow" size={13} />Live
                        </button>
                        <button
                          type="button"
                          className="icon-button icon-button--small"
                          disabled={!canAct}
                          onClick={(e) => {
                            e.stopPropagation();
                            startEditingMeta(item);
                          }}
                          title="Edit notes and duration"
                          aria-label="Edit notes and duration"
                        >
                          <Icon name="more" size={13} />
                        </button>
                        <button
                          type="button"
                          className="icon-button icon-button--small"
                          disabled={!canAct}
                          aria-label={`Remove ${item.title} from the service`}
                          title="Remove from service"
                          onClick={(e) => {
                            e.stopPropagation();
                            void perform(
                              `remove-${item.id}`,
                              () => studio.removeItem(item.id),
                              `Removed ${item.title}.`,
                            );
                          }}
                        >
                          <Icon name="close" size={14} />
                        </button>
                      </div>
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

        <p className="service-order-note">
          Drag rows to reorder. Double-click to preview. Schedule items automatically track live progress through Completed, Live, Next, and Upcoming states.
        </p>
      </div>
    </section>
  );
}
