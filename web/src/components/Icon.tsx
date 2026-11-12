import type { ComponentChildren } from 'preact';

export type IconName = 'align' | 'connect' | 'settings' | 'log' | 'start' | 'camera' | 'sun' | 'sparkle' | 'clock' | 'moon';

const paths: Record<IconName, ComponentChildren> = {
  align: (
    <>
      <circle cx="12" cy="12" r="7.5" />
      <path d="M12 2.5v4M12 17.5v4M2.5 12h4M17.5 12h4" />
      <circle cx="12" cy="12" r="1.2" fill="currentColor" />
    </>
  ),
  connect: (
    <>
      <path d="M2.5 9a15 15 0 0 1 19 0M5.5 12.5a10.5 10.5 0 0 1 13 0M8.5 16a5.5 5.5 0 0 1 7 0" />
      <circle cx="12" cy="19.6" r="1.1" fill="currentColor" />
    </>
  ),
  settings: (
    <>
      <path d="M3.5 6h9M17.5 6h3M3.5 12h3M11.5 12h9M3.5 18h11M19.5 18h1" />
      <circle cx="15" cy="6" r="2.3" />
      <circle cx="9" cy="12" r="2.3" />
      <circle cx="17" cy="18" r="2.3" />
    </>
  ),
  log: <path d="M5 5.5h14M5 10.5h14M5 15.5h9M5 20.5h6" />,
  start: (
    <>
      <circle cx="12" cy="12" r="8" />
      <path d="M12 4v16M4 12h16" opacity=".5" />
      <circle cx="15.5" cy="8.5" r="1.8" fill="currentColor" />
    </>
  ),
  camera: (
    <>
      <path d="M3 8.5h4l1.5-2.5h7L17 8.5h4V19H3z" />
      <circle cx="12" cy="13.5" r="3.5" />
    </>
  ),
  sun: (
    <>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 3v2M12 19v2M3 12h2M19 12h2M5.6 5.6l1.4 1.4M17 17l1.4 1.4M5.6 18.4L7 17M17 7l1.4-1.4" />
    </>
  ),
  sparkle: (
    <>
      <path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8z" />
      <path d="M19 16l.8 2.2L22 19l-2.2.8L19 22l-.8-2.2L16 19l2.2-.8z" />
    </>
  ),
  clock: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 7v5l3.2 2" />
    </>
  ),
  moon: <path d="M20 14.5A8.5 8.5 0 0 1 9.5 4a8.5 8.5 0 1 0 10.5 10.5z" />,
};

export function Icon({ name, size }: { name: IconName; size?: number }) {
  return (
    <svg class="icon" viewBox="0 0 24 24" aria-hidden="true" style={size ? { width: size, height: size } : undefined}>
      {paths[name]}
    </svg>
  );
}

export function Logo() {
  return (
    <svg class="logo" viewBox="0 0 24 24" aria-hidden="true">
      <path fill="currentColor" d="M12 1.5l2.3 8.2L22.5 12l-8.2 2.3L12 22.5l-2.3-8.2L1.5 12l8.2-2.3z" />
    </svg>
  );
}
