import type { ComponentChildren } from 'preact';

export type IconName = 'home' | 'projects' | 'palette' | 'skills' | 'settings' | 'plus' | 'search' | 'arrow' | 'chevron' | 'back' | 'close' | 'web' | 'mobile' | 'desktop' | 'code' | 'eye' | 'refresh' | 'external' | 'copy' | 'check' | 'stop' | 'folder' | 'file' | 'motion' | 'clone' | 'deck' | 'connect' | 'menu' | 'send' | 'download' | 'clock' | 'activity' | 'warning' | 'message';
const paths: Record<IconName, ComponentChildren> = {
  activity: <><path d="M3 12h4l3-8 4 16 3-8h4"/></>,
  warning: <><path d="m12 3 10 18H2Z"/><path d="M12 9v5M12 17h.01"/></>,
  message: <><path d="M21 12a8 8 0 0 1-8 8H4l-2 2V12a9 9 0 0 1 19 0Z"/><path d="M8 10h8M8 14h5"/></>,
  home: <><path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1Z"/></>,
  projects: <><rect x="5" y="3" width="15" height="17" rx="2"/><path d="M5 7H2v14a1 1 0 0 0 1 1h13M9 7h7M9 11h7"/></>,
  palette: <><path d="M12 3a9 9 0 1 0 0 18h1a2 2 0 0 0 1.4-3.4 1.5 1.5 0 0 1 1-2.6H18a3 3 0 0 0 3-3 9 9 0 0 0-9-9Z"/><path d="M7.5 10h.01M10 6.8h.01M14.2 7h.01M17.2 10h.01" stroke-width="3"/></>,
  skills: <><path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5ZM20 2v4M18 4h4"/></>,
  settings: <><path d="m9 4 1-2h4l1 2 2 1 2-.2 2 3.4-1.2 1.8v2l1.2 1.8-2 3.4-2-.2-2 1-1 2h-4l-1-2-2-1-2 .2-2-3.4L4.2 12v-2L3 8.2l2-3.4L7 5Z"/><circle cx="12" cy="11" r="3"/></>,
  plus: <path d="M12 5v14M5 12h14"/>,
  search: <><circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/></>,
  arrow: <path d="M12 20V4m-6 6 6-6 6 6"/>,
  chevron: <path d="m9 6 6 6-6 6"/>,
  back: <path d="M20 12H4m6-6-6 6 6 6"/>,
  close: <path d="m6 6 12 12M6 18 18 6"/>,
  web: <><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 9h18M7 6.5h.01M10 6.5h.01"/></>,
  mobile: <><rect x="6" y="2" width="12" height="20" rx="2.5"/><path d="M10 5h4M11 19h2"/></>,
  desktop: <><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/></>,
  code: <path d="m7 6-5 6 5 6m10-12 5 6-5 6M14 3l-4 18"/>,
  eye: <><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"/><circle cx="12" cy="12" r="3"/></>,
  refresh: <><path d="M20 7v5h-5M4 17v-5h5"/><path d="M5.5 7A7.5 7.5 0 0 1 19 8l1 4M4 12l1 4a7.5 7.5 0 0 0 13.5 1"/></>,
  external: <><path d="M14 3h7v7m0-7L10 14"/><path d="M10 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-5"/></>,
  copy: <><rect x="8" y="8" width="13" height="13" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/></>,
  check: <path d="m5 12 4 4L19 6"/>,
  stop: <rect x="6" y="6" width="12" height="12" rx="2"/>,
  folder: <path d="M3 6a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/>,
  file: <><path d="M14 2H5v20h14V7Z"/><path d="M14 2v5h5M8 12h8M8 16h6"/></>,
  motion: <><path d="m10 4 11 8-11 8Z"/><path d="M3 6h3M1 12h5M3 18h3"/></>,
  clone: <><rect x="3" y="3" width="13" height="13" rx="2"/><path d="M9 19v2h12V9h-2M7 7h5M7 11h3"/></>,
  deck: <><rect x="2" y="4" width="20" height="13" rx="2"/><path d="M8 8.5h7M8 11.5h4.5"/><path d="m9 21 3-3 3 3"/></>,
  connect: <><path d="m8 3 2 2-5 5-2-2m11 11 2 2 5-5-2-2M8 8l8 8M7 12l-3 3a4 4 0 0 0 5 5l3-3M12 7l3-3a4 4 0 0 1 5 5l-3 3"/></>,
  menu: <path d="M4 6h16M4 12h16M4 18h16"/>,
  send: <><path d="m3 3 19 9-19 9 3-9Z"/><path d="M6 12h16"/></>,
  download: <><path d="M12 3v12m-5-5 5 5 5-5M4 17v4h16v-4"/></>,
  clock: <><circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/></>,
};
export function Icon({ name, size = 19, class: className = '' }: { name: IconName; size?: number; class?: string }) {
  return <svg class={`icon ${className}`} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">{paths[name]}</svg>;
}
// Engine badge for the omp harness; provider/model controls use ModelIcon.
export function AgentMark({ size = 24 }: { size?: number }) {
  return <span class="agent-mark" aria-hidden="true" style={{ width: size, height: size, fontSize: Math.round(size * 0.36) }}>omp</span>;
}
export function Logo({ size = 32 }: { size?: number }) {
  return <img src="/brand.svg" width={size} height={size} class="brand-mark" alt="" />;
}
