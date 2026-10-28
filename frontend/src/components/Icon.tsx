import type { ReactNode } from 'react';

export type IconName =
  | 'home'
  | 'order'
  | 'media'
  | 'bible'
  | 'detection'
  | 'settings'
  | 'display'
  | 'preview'
  | 'arrow'
  | 'check'
  | 'layers'
  | 'square';

const paths: Record<IconName, ReactNode> = {
  home: <><path d="m3 10 9-7 9 7" /><path d="M5 9v12h5v-7h4v7h5V9" /></>,
  order: <><path d="M9 5h12M9 12h12M9 19h12" /><path d="M3 5h1M3 12h1M3 19h1" /></>,
  media: <><rect x="3" y="3" width="18" height="18" rx="3" /><circle cx="8" cy="8" r="1.5" /><path d="m3 17 5-5 4 4 4-6 5 7" /></>,
  bible: <><path d="M12 5v16M12 5C8 2 4 3 2 4v16c3-2 7-2 10 1 3-3 7-3 10-1V4c-2-1-6-2-10 1Z" /></>,
  detection: <><path d="M3 10v4M7 6v12M12 3v18M17 6v12M21 10v4" /></>,
  settings: <><path d="M4 7h16M4 17h16" /><circle cx="9" cy="7" r="3" /><circle cx="15" cy="17" r="3" /></>,
  display: <><rect x="2" y="3" width="20" height="14" rx="2" /><path d="M8 21h8M12 17v4" /></>,
  preview: <><rect x="2" y="3" width="20" height="18" rx="2" /><path d="m10 8 6 4-6 4Z" /></>,
  arrow: <><path d="M5 12h14m-5-5 5 5-5 5" /></>,
  check: <path d="m5 12 4 4L19 6" />,
  layers: <><path d="m12 3 10 5-10 5L2 8l10-5ZM2 12l10 5 10-5M2 16l10 5 10-5" /></>,
  square: <rect x="5" y="5" width="14" height="14" rx="2" />,
};

export function Icon({ name, size = 20 }: { name: IconName; size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false">
      {paths[name]}
    </svg>
  );
}
