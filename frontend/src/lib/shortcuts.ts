import type * as models from '../../bindings/zopresent/models';

export type ShortcutAction = keyof models.ShortcutsView;

// The configurable operator actions, in the order the Settings card lists them.
export const SHORTCUT_ACTIONS: { action: ShortcutAction; label: string }[] = [
  { action: 'sendPreviewLive', label: 'Send preview live' },
  { action: 'focusBibleSearch', label: 'Focus Bible search' },
  { action: 'toggleListening', label: 'Toggle scripture listening' },
  { action: 'toggleAutoProjection', label: 'Pause/resume auto-projection' },
  { action: 'emergencyDisable', label: 'Emergency disable detection' },
  { action: 'schedulePrevious', label: 'Previous schedule item' },
  { action: 'scheduleNext', label: 'Next schedule item' },
];

// First-paint fallback only; the backend owns the authoritative defaults and
// GetShortcuts returns them whenever nothing is stored.
export const DEFAULT_SHORTCUTS: models.ShortcutsView = {
  sendPreviewLive: 'Ctrl+L',
  focusBibleSearch: 'Ctrl+F',
  toggleListening: 'F8',
  toggleAutoProjection: 'F9',
  emergencyDisable: 'F12',
  schedulePrevious: 'ArrowLeft',
  scheduleNext: 'ArrowRight',
};

// Schedule navigation must not fire while the operator is typing; the rest are
// global on purpose, matching how they behaved before they were configurable.
export const TEXT_GUARDED: ReadonlySet<ShortcutAction> = new Set([
  'schedulePrevious',
  'scheduleNext',
]);

const MODIFIER_KEYS = new Set(['Control', 'Alt', 'Shift', 'Meta', 'CapsLock', 'AltGraph', 'OS']);

// Bases that are safe to bind on their own. Anything printable (a letter, digit,
// Space, Enter, Escape, Tab) needs Ctrl or Alt so a shortcut never swallows
// ordinary typing.
const STANDALONE_BASE = /^(F\d{1,2}|Arrow(Left|Right|Up|Down)|Page(Up|Down)|Home|End|Insert|Delete)$/;

const DISPLAY_BASE: Record<string, string> = {
  ArrowLeft: '←',
  ArrowRight: '→',
  ArrowUp: '↑',
  ArrowDown: '↓',
  Escape: 'Esc',
  Space: 'Space',
};

function baseKey(event: KeyboardEvent): string {
  if (event.key === ' ') return 'Space';
  if (event.key.length === 1) return event.key.toUpperCase();
  return event.key;
}

// comboFromEvent renders a keypress in the same canonical "Ctrl+Alt+Shift+Key"
// form the backend stores, or null when only a modifier was pressed. Ctrl also
// covers Cmd so one map works on every platform.
export function comboFromEvent(event: KeyboardEvent): string | null {
  if (MODIFIER_KEYS.has(event.key)) return null;
  const mods: string[] = [];
  if (event.ctrlKey || event.metaKey) mods.push('Ctrl');
  if (event.altKey) mods.push('Alt');
  if (event.shiftKey) mods.push('Shift');
  return [...mods, baseKey(event)].join('+');
}

export function isBindable(combo: string): boolean {
  const parts = combo.split('+');
  const base = parts[parts.length - 1];
  const mods = parts.slice(0, -1);
  if (STANDALONE_BASE.test(base)) return true;
  return mods.includes('Ctrl') || mods.includes('Alt');
}

export function formatCombo(combo: string): string {
  if (!combo) return '—';
  return combo
    .split('+')
    .map((part) => DISPLAY_BASE[part] ?? part)
    .join(' + ');
}

// Shortcut groups the operator sees, split by scope. The global map above is
// configurable and fires everywhere on the studio window; this one lives at
// the queue's keyboard path and is fixed on purpose — there is no reason to
// rebind "project the highlighted card" per machine, and there is a cost in
// Settings filling up with keys nobody reassigns.
export const QUEUE_SHORTCUTS = [
  { action: 'navigate', keys: '↑ / ↓', label: 'move the highlight' },
  { action: 'project', keys: 'Enter or P', label: 'project live' },
  { action: 'approve', keys: 'A or Alt+Enter', label: 'approve to preview' },
  { action: 'dismiss', keys: 'D or Esc', label: 'dismiss' },
  { action: 'ends', keys: 'Ctrl+↑ / Ctrl+↓', label: 'highlight first / last' },
] as const;

// actionForCombo resolves a pressed combo to the one action bound to it.
export function actionForCombo(
  shortcuts: models.ShortcutsView,
  combo: string,
): ShortcutAction | null {
  for (const { action } of SHORTCUT_ACTIONS) {
    if (shortcuts[action] === combo) return action;
  }
  return null;
}
