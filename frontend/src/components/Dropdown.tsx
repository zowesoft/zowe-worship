import { useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';

type DropdownState = {
  open: boolean;
  toggle: () => void;
  close: () => void;
};

// The menus that hang off a control - the service switcher, the display picker,
// the theme size field - all need the same thing: a trigger that opens a
// floating panel, and a click anywhere outside that panel to close it again.
// The trigger and the panel are both handed the open state so a caller can draw
// its own chevron rotation and close the menu when an option is picked.
export function Dropdown({ trigger, menu, align = 'left', offset = 12, role = 'menu', wrapperClassName = 'relative', panelClassName = '' }: {
  trigger: (state: DropdownState) => ReactNode;
  menu: (state: DropdownState) => ReactNode;
  align?: 'left' | 'right';
  offset?: number;
  role?: 'menu' | 'listbox';
  wrapperClassName?: string;
  panelClassName?: string;
}) {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;

    function dismiss(event: MouseEvent) {
      if (!wrapRef.current?.contains(event.target as Node)) setOpen(false);
    }

    document.addEventListener('mousedown', dismiss);
    return () => document.removeEventListener('mousedown', dismiss);
  }, [open]);

  const state: DropdownState = { open, toggle: () => setOpen((value) => !value), close: () => setOpen(false) };

  return (
    <div className={wrapperClassName} ref={wrapRef}>
      {trigger(state)}
      {open && (
        <div
          className={`absolute ${align === 'right' ? 'right-0' : 'left-0'} z-30 grid gap-0.5 p-1.5 border border-border rounded-lg bg-elevated shadow-[0_16px_36px_rgba(2,6,14,0.6)] ${panelClassName}`}
          style={{ top: `calc(100% + ${offset}px)` }}
          role={role}
        >
          {menu(state)}
        </div>
      )}
    </div>
  );
}
