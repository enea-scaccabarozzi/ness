import type { ReactNode } from 'react';
import { cn } from '@/lib/cn';

/** A terminal frame for real command output. The title is the command that produced it. */
export function TermFrame({ title, children, className }: { title: string; children: ReactNode; className?: string }) {
  return (
    <figure className={cn('terminal overflow-hidden rounded-xl border border-line-strong shadow-lg', className)}>
      <figcaption className="flex h-9 items-center gap-3 border-b border-white/[0.07] bg-white/[0.02] px-4 font-mono text-[11px] text-[color:var(--t-dim)]">
        <span className="flex gap-1.5" aria-hidden>
          <span className="size-2 rounded-full bg-white/10" />
          <span className="size-2 rounded-full bg-white/10" />
          <span className="size-2 rounded-full bg-white/10" />
        </span>
        <span className="truncate">{title}</span>
      </figcaption>
      <pre className="overflow-x-auto p-4 font-mono text-[11.5px] leading-[1.7] text-[color:var(--t-fg)] sm:px-5">
        {children}
      </pre>
    </figure>
  );
}

export const t = {
  dim: 'text-[color:var(--t-dim)]',
  ok: 'text-[color:var(--t-ok)]',
  amber: 'text-[color:var(--t-amber)]',
  bold: 'font-semibold text-[color:var(--t-fg)]',
  pill: 'rounded-[3px] bg-[color:var(--t-amber)] px-1 font-semibold text-[color:var(--t-bg)]',
};
