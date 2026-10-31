'use client';
import Link from 'next/link';
import type { ComponentProps, PointerEvent, ReactNode } from 'react';
import { motion, useReducedMotion } from 'motion/react';
import { cn } from '@/lib/cn';

/** Page column: 16px gutters on phones, 24px from sm, max 72rem. */
export function Container({ className, ...props }: ComponentProps<'div'>) {
  return <div className={cn('mx-auto w-full max-w-6xl px-4 sm:px-6', className)} {...props} />;
}

export function Eyebrow({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <p className={cn('font-mono text-eyebrow uppercase text-fg-subtle', className)}>
      <span className="mr-2 inline-block size-1.5 -translate-y-px rounded-full bg-ember align-middle" />
      {children}
    </p>
  );
}

export function SectionHeading({
  eyebrow,
  title,
  lead,
  className,
}: {
  eyebrow: string;
  title: ReactNode;
  lead?: ReactNode;
  className?: string;
}) {
  return (
    <Reveal className={cn('max-w-2xl', className)}>
      <Eyebrow>{eyebrow}</Eyebrow>
      <h2 className="mt-4 text-h2 font-semibold text-balance text-fg">{title}</h2>
      {lead && <p className="mt-4 text-lead text-pretty text-fg-muted">{lead}</p>}
    </Reveal>
  );
}

type ButtonProps = { href: string; children: ReactNode; variant?: 'primary' | 'ghost'; className?: string };

export function Button({ href, children, variant = 'primary', className }: ButtonProps) {
  const external = href.startsWith('http');
  const cls = cn(
    'group inline-flex h-11 items-center justify-center gap-2 rounded-full px-5 text-sm font-medium transition-[background-color,border-color,color,box-shadow] duration-200',
    variant === 'primary'
      ? 'bg-ember text-on-ember shadow-[0_0_0_1px_var(--ember),0_8px_30px_-8px_var(--ember)] hover:brightness-105'
      : 'border border-line-strong bg-ink-1/60 text-fg backdrop-blur hover:border-fg-subtle hover:bg-ink-2',
    className,
  );
  if (external)
    return (
      <a href={href} className={cls} target="_blank" rel="noreferrer">
        {children}
      </a>
    );
  return (
    <Link href={href} className={cls}>
      {children}
    </Link>
  );
}

/** Fade-and-rise on first view. Off under prefers-reduced-motion. */
export function Reveal({ children, className, delay = 0 }: { children: ReactNode; className?: string; delay?: number }) {
  const reduce = useReducedMotion();
  return (
    <motion.div
      data-reveal
      className={className}
      initial={reduce ? false : { opacity: 0, y: 14 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: '-80px' }}
      transition={{ duration: 0.6, delay, ease: [0.22, 1, 0.36, 1] }}
    >
      {children}
    </motion.div>
  );
}

/** Card with a pointer-following ember wash (Magic UI "magic card" pattern, CSS only). */
export function SpotlightCard({ className, children, ...props }: ComponentProps<'div'>) {
  function onMove(e: PointerEvent<HTMLDivElement>) {
    const r = e.currentTarget.getBoundingClientRect();
    e.currentTarget.style.setProperty('--x', `${e.clientX - r.left}px`);
    e.currentTarget.style.setProperty('--y', `${e.clientY - r.top}px`);
  }
  return (
    <div
      onPointerMove={onMove}
      className={cn(
        'spotlight overflow-hidden rounded-xl border border-line bg-ink-1 transition-colors duration-300 hover:border-line-strong',
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export function ArrowRight({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 16 16" aria-hidden className={cn('size-4 transition-transform duration-200 group-hover:translate-x-0.5', className)}>
      <path d="M3 8h9M8.5 4.5 12 8l-3.5 3.5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function GitHubIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" aria-hidden className={cn('size-4', className)} fill="currentColor">
      <path d="M12 .5a11.5 11.5 0 0 0-3.64 22.41c.58.1.79-.25.79-.56v-2c-3.2.7-3.88-1.37-3.88-1.37-.52-1.33-1.28-1.69-1.28-1.69-1.05-.71.08-.7.08-.7 1.16.08 1.77 1.19 1.77 1.19 1.03 1.77 2.7 1.26 3.36.96.1-.75.4-1.26.73-1.55-2.55-.29-5.24-1.28-5.24-5.68 0-1.26.45-2.28 1.19-3.09-.12-.29-.52-1.46.11-3.05 0 0 .97-.31 3.17 1.18a11 11 0 0 1 5.77 0c2.2-1.49 3.17-1.18 3.17-1.18.63 1.59.23 2.76.11 3.05.74.81 1.19 1.83 1.19 3.09 0 4.41-2.69 5.38-5.25 5.67.41.36.78 1.06.78 2.14v3.17c0 .31.21.67.8.56A11.5 11.5 0 0 0 12 .5Z" />
    </svg>
  );
}
