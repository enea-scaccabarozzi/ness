import { useId } from 'react';
import { cn } from '@/lib/cn';

/*
 * Nessie, the ness mark: an app tile with a neck, a head, two humps and an ember wave, on a 32px
 * grid. The eye is a mask cut-out, so it reads on any background. public/icon.svg and lib/og.tsx
 * draw the same shape with fixed colours.
 */
export function Mark({ className }: { className?: string }) {
  const id = useId();
  return (
    <svg viewBox="0 0 32 32" aria-hidden className={cn('size-6', className)}>
      <defs>
        <clipPath id={`${id}c`}>
          <rect width="32" height="21.6" />
        </clipPath>
        <mask id={`${id}m`}>
          <rect width="32" height="32" fill="white" />
          <circle cx="15.6" cy="8.9" r="0.95" fill="black" />
        </mask>
      </defs>
      <rect x="1" y="1" width="30" height="30" rx="8.5" className="fill-ink-2 stroke-line-strong" strokeWidth="1" />
      <g clipPath={`url(#${id}c)`} mask={`url(#${id}m)`} fill="currentColor" className="text-fg">
        <path d="M8 24 C7.7 16.6 8.8 12 12.3 10.4" fill="none" stroke="currentColor" strokeWidth="4" strokeLinecap="round" />
        <ellipse cx="14.6" cy="9.8" rx="4" ry="3" transform="rotate(-10 14.6 9.8)" />
        <path d="M15.2 21.6 a3.5 4 0 0 1 7 0z" />
        <path d="M22.9 21.6 a2.5 2.9 0 0 1 5 0z" />
      </g>
      <path
        d="M4.6 24.4 q1.9 -1.4 3.8 0 t3.8 0 t3.8 0 t3.8 0 t3.8 0 t3.8 0"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.7"
        strokeLinecap="round"
        className="text-ember"
      />
    </svg>
  );
}

export function Wordmark({ className }: { className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <Mark />
      <span className="font-mono text-[15px] font-semibold tracking-[-0.02em] text-fg">ness</span>
    </span>
  );
}
