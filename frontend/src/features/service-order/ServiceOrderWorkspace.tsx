import { useState } from 'react';
import type { FormEvent } from 'react';
import { Icon } from '../../components/Icon';
import { useStudio } from '../../stores/StudioProvider';
import { moveWithin, useDragOrder } from '../../lib/useDragOrder';
import type * as models from '../../../bindings/zowe-worship/models';

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
      <span className="inline-flex items-center px-2 py-[3px] border border-[#1E4D40]! rounded-full bg-[#0E2A24]! text-[#6EE7C0]! text-[9px] whitespace-nowrap" title="Currently showing on live display">
        <span className="inline-block w-1.5 h-1.5 rounded-full bg-[#34D399] mr-[5px] shadow-[0_0_6px_#34D399] animate-pulse-dot" />Live
      </span>
    );
  }
  if (status === 'next') {
    return (
      <span className="inline-flex items-center px-2 py-[3px] border border-[#4D3F1E]! rounded-full bg-[#2B2413]! text-amber! text-[9px] whitespace-nowrap" title="Next in line / currently in preview">
        Next
      </span>
    );
  }
  if (status === 'completed') {
    return (
      <span className="inline-flex items-center gap-1 px-2 py-[3px] border border-[#204539]! rounded-full bg-[#112620]! text-teal! text-[9px] whitespace-nowrap" title="Item presentation finished">
        <Icon name="check" size={11} />Done
      </span>
    );
  }
  return (
    <span className="inline-flex items-center px-2 py-[3px] border border-[#2A3950]! rounded-full bg-[#151F2E]! text-[#7E92AB]! text-[9px] whitespace-nowrap" title="Upcoming schedule item">
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
    <section className="bg-panel border border-border rounded-[10px] overflow-hidden" aria-labelledby="service-order-title">
      <div className="min-h-11 px-3.5 py-2.5 flex items-center justify-between gap-2.5 border-b border-border">
        <h2 id="service-order-title" className="text-[11px] flex items-center gap-[7px] uppercase tracking-[0.7px] text-secondary [&_svg]:text-secondary"><Icon name="order" size={16} />Service Order</h2>
        <span>{service ? `${items.length} items` : 'No service yet'}</span>
      </div>

      <div className="grid gap-3.5 p-4 max-[959px]:p-3">
        {service === null ? (
          <>
            <div className="flex items-start gap-3.5 [&_h3]:text-[15px] [&_h3]:mt-1 [&_p]:max-w-[620px] [&_p]:mt-1.5 [&_p]:text-secondary [&_p]:text-[11.5px] [&_p]:leading-[1.7]">
              <span className="grid place-items-center w-12 h-12 border border-[#30466A] bg-selected text-[#8CABFF] rounded-[11px] shrink-0"><Icon name="order" size={24} /></span>
              <div>
                <h3>Start a service</h3>
                <p>Create a service, then add scripture, songs, media, text slides, announcements, or blank dividers to build your schedule.</p>
              </div>
            </div>

            <form className="flex gap-2" onSubmit={handleCreate}>
              <div className="flex-1 min-w-0 flex items-center gap-2 min-h-[38px] px-3 border border-[#35445C] rounded-[7px] bg-app focus-within:border-blue focus-within:shadow-[0_0_0_2px_rgba(71,120,255,0.18)] [&>svg]:text-secondary">
                <Icon name="order" size={16} />
                <input
                  type="text"
                  className="flex-1 min-w-0 h-9 border-0 outline-none bg-transparent text-text text-[12px] placeholder:text-[#6F809B] disabled:text-secondary"
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
                className="min-h-8 inline-flex items-center justify-center gap-1.5 px-3 py-1.5 border border-blue! rounded-md bg-blue! text-white! text-[11px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
                disabled={!canAct || name.trim() === ''}
              >
                Create service
              </button>
            </form>

            {!studio.bridgeAvailable && (
              <p className="pt-2.5 border-t border-border text-secondary text-[10px]">Open the Zowe Worship desktop app to store a service.</p>
            )}
          </>
        ) : (
          <>
            <div className="flex items-end justify-between gap-3 [&_h3]:text-[16px] [&_h3]:mt-1">
              {renaming ? (
                <form
                  className="flex items-center gap-1.5"
                  onSubmit={(event) => {
                    event.preventDefault();
                    void submitRename();
                  }}
                >
                  <input
                    type="text"
                    className="w-[220px] min-w-0 h-[30px] px-2.5 border border-[#35445C] rounded-md bg-app text-text text-[12px] focus:border-blue focus:outline-none"
                    value={draft}
                    aria-label="Service name"
                    autoComplete="off"
                    autoFocus
                    onChange={(event) => setDraft(event.target.value)}
                  />
                  <button type="submit" className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-blue! rounded-md bg-blue! text-white! text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!" disabled={draft.trim() === ''}>
                    Save
                  </button>
                  <button type="button" className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]" onClick={() => setRenaming(false)}>
                    Cancel
                  </button>
                </form>
              ) : (
                <div>
                  <p className="text-[9.5px] uppercase tracking-[1.3px] font-semibold text-secondary">Current service</p>
                  <h3>{service.name}</h3>
                </div>
              )}

              <div className="flex items-center gap-2.5">
                <span>{items.length} {items.length === 1 ? 'item' : 'items'}</span>
                {!renaming && (
                  <>
                    <button
                      type="button"
                      className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]"
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
                      className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]"
                      disabled={!canAct}
                      onClick={() => void handleDuplicate()}
                      title="Duplicate this service schedule"
                    >
                      <Icon name="copy" size={13} />Duplicate
                    </button>
                    <button
                      type="button"
                      className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-blue! rounded-md bg-blue! text-white! text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
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
              <div className="fixed inset-0 bg-[rgba(0,0,0,0.65)] backdrop-blur-[2px] flex items-center justify-center z-[1000] p-4" role="dialog" aria-modal="true" aria-labelledby="custom-modal-title">
                <form className="w-full max-w-[480px] bg-[#172232] border border-[#2C3E56] rounded-[10px] shadow-[0_16px_40px_rgba(0,0,0,0.5)] grid gap-3.5 p-[18px]" onSubmit={(e) => void handleAddCustom(e)}>
                  <div className="flex items-center justify-between [&_h3]:text-[15px]">
                    <h3 id="custom-modal-title">Add Slide to Service</h3>
                    <button
                      type="button"
                      className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50"
                      onClick={() => setShowAddModal(false)}
                      aria-label="Close"
                    >
                      <Icon name="close" size={14} />
                    </button>
                  </div>

                  <div className="flex gap-1 p-[3px] bg-[#0F1722] rounded-[7px] border border-[#243449]" role="tablist">
                    <button
                      type="button"
                      role="tab"
                      aria-selected={addKind === 'text'}
                      className={[
                        'flex-1 border-0 rounded-[5px] bg-transparent text-secondary text-[11.5px] py-1.5 px-2.5 transition-all duration-150 hover:text-text',
                        addKind === 'text' ? 'bg-[#23344B]! text-text! font-medium shadow-[0_1px_3px_rgba(0,0,0,0.25)]' : '',
                      ].filter(Boolean).join(' ')}
                      onClick={() => setAddKind('text')}
                    >
                      Text Slide
                    </button>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={addKind === 'announcement'}
                      className={[
                        'flex-1 border-0 rounded-[5px] bg-transparent text-secondary text-[11.5px] py-1.5 px-2.5 transition-all duration-150 hover:text-text',
                        addKind === 'announcement' ? 'bg-[#23344B]! text-text! font-medium shadow-[0_1px_3px_rgba(0,0,0,0.25)]' : '',
                      ].filter(Boolean).join(' ')}
                      onClick={() => setAddKind('announcement')}
                    >
                      Announcement
                    </button>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={addKind === 'blank'}
                      className={[
                        'flex-1 border-0 rounded-[5px] bg-transparent text-secondary text-[11.5px] py-1.5 px-2.5 transition-all duration-150 hover:text-text',
                        addKind === 'blank' ? 'bg-[#23344B]! text-text! font-medium shadow-[0_1px_3px_rgba(0,0,0,0.25)]' : '',
                      ].filter(Boolean).join(' ')}
                      onClick={() => setAddKind('blank')}
                    >
                      Blank Divider
                    </button>
                  </div>

                  <div className="grid gap-2.5">
                    <label className="flex flex-col gap-1 text-[11px] text-secondary [&_strong]:text-[#F87171] [&_strong]:ml-0.5">
                      <span>Title {addKind !== 'blank' && <strong>*</strong>}</span>
                      <input
                        type="text"
                        className="w-full px-2.5 py-[7px] border border-[#33465E] rounded-md bg-[#0F1722] text-text text-[12px] focus:border-blue focus:outline-none focus:shadow-[0_0_0_2px_rgba(71,120,255,0.15)]"
                        placeholder={addKind === 'blank' ? 'e.g. Silent Prayer (optional)' : addKind === 'announcement' ? 'e.g. Youth Camp This Saturday' : 'e.g. Call to Worship'}
                        value={addTitle}
                        onChange={(e) => setAddTitle(e.target.value)}
                        autoFocus
                        required={addKind !== 'blank'}
                      />
                    </label>

                    {addKind !== 'blank' && (
                      <label className="flex flex-col gap-1 text-[11px] text-secondary">
                        <span>Slide Text</span>
                        <textarea
                          className="w-full px-2.5 py-[7px] border border-[#33465E] rounded-md bg-[#0F1722] text-text text-[12px] [font-family:inherit] resize-y min-h-20 focus:border-blue focus:outline-none focus:shadow-[0_0_0_2px_rgba(71,120,255,0.15)]"
                          rows={4}
                          placeholder={addKind === 'announcement' ? 'Enter announcement details or instructions…' : 'Enter lines to present…'}
                          value={addContent}
                          onChange={(e) => setAddContent(e.target.value)}
                        />
                      </label>
                    )}

                    <div className="grid grid-cols-[140px_1fr] gap-2.5">
                      <label className="flex flex-col gap-1 text-[11px] text-secondary">
                        <span>Estimated Duration (mins)</span>
                        <input
                          type="number"
                          step="0.5"
                          min="0"
                          className="w-full px-2.5 py-[7px] border border-[#33465E] rounded-md bg-[#0F1722] text-text text-[12px] focus:border-blue focus:outline-none focus:shadow-[0_0_0_2px_rgba(71,120,255,0.15)]"
                          placeholder="e.g. 2"
                          value={addMinutes}
                          onChange={(e) => setAddMinutes(e.target.value)}
                        />
                      </label>
                      <label className="flex flex-col gap-1 text-[11px] text-secondary">
                        <span>Operator Notes</span>
                        <input
                          type="text"
                          className="w-full px-2.5 py-[7px] border border-[#33465E] rounded-md bg-[#0F1722] text-text text-[12px] focus:border-blue focus:outline-none focus:shadow-[0_0_0_2px_rgba(71,120,255,0.15)]"
                          placeholder="e.g. Dim lights, quiet background music"
                          value={addNotes}
                          onChange={(e) => setAddNotes(e.target.value)}
                        />
                      </label>
                    </div>
                  </div>

                  <div className="flex items-center justify-end gap-2 mt-1">
                    <button
                      type="button"
                      className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]"
                      onClick={() => setShowAddModal(false)}
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-blue! rounded-md bg-blue! text-white! text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
                      disabled={addKind !== 'blank' && addTitle.trim() === ''}
                    >
                      Add to Service
                    </button>
                  </div>
                </form>
              </div>
            )}

            {items.length === 0 ? (
              <div className="grid justify-items-center gap-[7px] py-[26px] px-4 border border-dashed border-border rounded-lg bg-app text-secondary text-center [&_svg]:opacity-65 [&_strong]:text-text [&_strong]:text-[12px] [&_strong]:font-medium [&_p]:max-w-[340px] [&_p]:text-[11px] [&_p]:leading-[1.6]">
                <Icon name="order" size={26} />
                <strong>No items yet</strong>
                <p>Add scripture from Bible search, song sections, media, or click “Add Slide” for custom text slides and announcements.</p>
              </div>
            ) : (
              <ol className="list-none grid gap-[5px] m-0 p-0">
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
                        'flex items-center gap-[9px] py-[7px] px-[9px] border border-transparent rounded-[7px] bg-app select-none max-[959px]:flex-wrap',
                        isLive ? 'border-[#1E4D40]! bg-[#0E2A24]!' : isPreview ? 'border-[#304C80]! bg-selected!' : '',
                        item.status === 'completed' ? 'opacity-75' : drag.dragging === index ? 'opacity-40' : '',
                        isDropTarget ? 'shadow-[0_0_0_1px_var(--color-blue)]' : '',
                      ].filter(Boolean).join(' ')}
                      title="Drag to reorder · Double-click to preview"
                      onDoubleClick={() => void perform(
                        `preview-${item.id}`,
                        () => studio.previewItem(item.id),
                        `Previewing ${item.title}.`,
                      )}
                      {...drag.rowProps(index)}
                    >
                      <span className="flex text-[#56698A] cursor-grab" aria-hidden="true" title="Drag to reorder"><Icon name="grip" size={14} /></span>
                      <span className="w-5 shrink-0 text-center text-secondary text-[10px]">{item.position}</span>

                      <div className="flex-1 min-w-0 max-[959px]:basis-[60%] [&_strong]:block [&_strong]:text-[11.5px] [&_strong]:font-medium [&_span]:block [&_span]:text-secondary [&_span]:text-[9.5px] [&_span]:mt-px">
                        <div className="flex items-center gap-2">
                          <strong>{item.title}</strong>
                          {durationLabel && <span className="text-[9px] font-medium px-1.5 py-px rounded-[4px] bg-[rgba(71,120,255,0.12)] text-[#8EB4FF] border border-[rgba(71,120,255,0.25)]">⏱ {durationLabel}</span>}
                        </div>
                        <div className="flex items-center gap-1.5 flex-wrap mt-0.5">
                          <span>{formatKindLabel(item)}</span>
                          {item.notes && <span className="text-[#A0B2C6] text-[9.5px] italic">· 📝 {item.notes}</span>}
                        </div>

                        {/* Inline editor for notes & duration */}
                        {isEditingThis && (
                          <div className="mt-1.5 px-2.5 py-2 rounded-md bg-[#16202E] border border-[#2B3D55] grid gap-1.5" onClick={(e) => e.stopPropagation()}>
                            <div className="flex gap-1.5">
                              <input
                                type="text"
                                className="flex-1 min-w-0 h-7 px-2 border border-[#354760] rounded-[5px] bg-[#0E1622] text-text text-[11px] focus:border-blue focus:outline-none"
                                placeholder="Operator notes…"
                                value={editNotes}
                                onChange={(e) => setEditNotes(e.target.value)}
                              />
                              <input
                                type="number"
                                step="0.5"
                                min="0"
                                className="min-w-0 w-[75px] flex-none h-7 px-2 border border-[#354760] rounded-[5px] bg-[#0E1622] text-text text-[11px] focus:border-blue focus:outline-none"
                                placeholder="Mins"
                                value={editMinutes}
                                onChange={(e) => setEditMinutes(e.target.value)}
                              />
                            </div>
                            <div className="flex gap-1.5 justify-end">
                              <button
                                type="button"
                                className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-blue! rounded-md bg-blue! text-white! text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
                                onClick={() => void saveMeta(item.id)}
                              >
                                Save Details
                              </button>
                              <button
                                type="button"
                                className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]"
                                onClick={() => setEditingItemId(null)}
                              >
                                Cancel
                              </button>
                            </div>
                          </div>
                        )}
                      </div>

                      <StatusBadge status={item.status} />

                      <div className="flex items-center gap-1 ml-auto">
                        <button
                          type="button"
                          className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary disabled:bg-[#142033] disabled:border-[#2A3950]"
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
                          className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-blue! rounded-md bg-blue! text-white! text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
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
                          className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50"
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
                          className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50"
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
          <p className={`text-[10.5px] ${status.kind === 'error' ? 'text-error!' : 'text-secondary'}`} aria-live="polite">
            {status.text}
          </p>
        )}

        <p className="pt-2.5 border-t border-border text-secondary text-[10px]">
          Drag rows to reorder. Double-click to preview. Schedule items automatically track live progress through Completed, Live, Next, and Upcoming states.
        </p>
      </div>
    </section>
  );
}
