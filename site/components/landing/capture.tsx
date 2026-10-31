import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import type { CSSProperties, ReactNode } from 'react';
import { cn } from '@/lib/cn';
import { columns, parseAnsi, reflow, type Line } from '@/lib/ansi';

/*
 * Real terminal captures from site/captures, rendered in the colours they were captured with.
 * The files are whole-line excerpts of the 2026-10-31 run (captures/trim.sh), nothing rewritten.
 * Lines are re-wrapped to the frame's width (lib/ansi.ts reflow), so nothing scrolls sideways.
 */

function load(file: string) {
  return parseAnsi(readFileSync(join(process.cwd(), 'captures', file), 'utf8'));
}

/** Lines `pick` selects (1-based, inclusive ranges) or all; a gap between ranges shows as a blank line. */
export function captureLines(file: string, pick?: [number, number][], joinWraps = true): (Line | null)[] {
  const raw = load(file);
  const cols = columns(raw);
  if (!pick) return reflow(raw, cols, joinWraps);
  return pick.flatMap(([a, b], i) => [...(i ? [null] : []), ...reflow(raw.slice(a - 1, b), cols, joinWraps)]);
}

/** Terminal lines with hanging indents; `replay` reveals them one by one (CSS only, final state under reduced motion). */
export function TermLines({ lines, replay, className }: { lines: (Line | null)[]; replay?: number; className?: string }) {
  return (
    <div className={cn('font-mono leading-[1.55] break-words whitespace-pre-wrap', replay !== undefined && 'replay', className)}>
      {lines.map((l, i) => (
        <div
          key={i}
          style={
            {
              '--i': i,
              '--d': `${replay ?? 0}s`,
              backgroundColor: l?.bg,
              paddingLeft: l ? `${l.indent}ch` : undefined,
              textIndent: l ? `-${l.indent}ch` : undefined,
            } as CSSProperties
          }
        >
          {!l || l.segs.length === 0
            ? ' '
            : l.segs.map((s, j) => (
                <span key={j} style={s.style}>
                  {s.text}
                </span>
              ))}
        </div>
      ))}
    </div>
  );
}

function Dots() {
  return (
    <span className="flex gap-1.5" aria-hidden>
      <span className="size-2 rounded-full bg-white/10" />
      <span className="size-2 rounded-full bg-white/10" />
      <span className="size-2 rounded-full bg-white/10" />
    </span>
  );
}

/**
 * One capture as a terminal frame: the picked lines, and under them a native disclosure with the
 * whole capture plus any `more` (ness's own output, the record).
 */
export function Capture({
  file,
  title,
  pick,
  joinWraps = true,
  more,
  className,
}: {
  file: string;
  title: string;
  pick?: [number, number][];
  joinWraps?: boolean;
  more?: ReactNode;
  className?: string;
}) {
  return (
    <figure className={cn('terminal overflow-hidden rounded-xl border border-line-strong shadow-lg', className)}>
      <figcaption className="flex h-9 items-center gap-3 border-b border-white/[0.07] bg-white/[0.02] px-4 font-mono text-[11px] text-[color:var(--t-dim)]">
        <Dots />
        <span className="truncate">{title}</span>
      </figcaption>
      <TermLines
        lines={captureLines(file, pick, joinWraps)}
        className="px-4 py-3.5 text-[11px] text-[color:var(--t-fg)] sm:px-5 sm:text-[12px]"
      />
      {(pick || more) && (
        <details className="group border-t border-white/[0.07]">
          <summary className="flex cursor-pointer list-none items-center gap-2 px-4 py-2.5 font-mono text-[11px] text-[color:var(--t-dim)] select-none hover:text-[color:var(--t-fg)] sm:px-5 [&::-webkit-details-marker]:hidden">
            <span className="transition-transform group-open:rotate-90" aria-hidden>
              ›
            </span>
            <span className="group-open:hidden">Show full capture</span>
            <span className="hidden group-open:inline">Hide full capture</span>
          </summary>
          <div className="space-y-4 border-t border-white/[0.07] px-4 pt-3.5 pb-4 sm:px-5">
            {pick && (
              <TermLines lines={captureLines(file, undefined, joinWraps)} className="text-[11px] text-[color:var(--t-fg)]" />
            )}
            {more}
          </div>
        </details>
      )}
    </figure>
  );
}

/** ness's own lines, which the chat folds into "Ran N shell commands". Verbatim from the root's transcript. */
export function NessOutput({ lines }: { lines: string[] }) {
  return (
    <div>
      <p className="font-mono text-[11px] text-[color:var(--t-dim)]">What ness printed, folded in the chat as “Ran N shell commands”:</p>
      <pre className="mt-1.5 font-mono text-[11px] leading-[1.55] break-all whitespace-pre-wrap text-[color:var(--t-fg)] opacity-80">
        {lines.join('\n')}
      </pre>
    </div>
  );
}

/** One operator alert, verbatim from the run's alert log; the session link is withheld. */
export function PhoneAlert({
  emoji,
  kind,
  project,
  session,
  body,
  className,
}: {
  emoji: string;
  kind: string;
  project: string;
  session: string;
  body: string;
  className?: string;
}) {
  return (
    <div className={cn('rounded-2xl rounded-bl-md border border-line-strong bg-ink-1 p-3.5 text-[13px] leading-snug text-fg shadow-xl', className)}>
      <p>
        <span aria-hidden>{emoji}</span> <b className="font-semibold">{kind}</b> · {project}
      </p>
      <p className="mt-0.5">
        <b
          className="cursor-default font-semibold text-ember-ink underline decoration-ember/40 underline-offset-2"
          title="Links to the session in Claude Code. The link is withheld here."
        >
          {session}
        </b>
      </p>
      <p className="mt-2.5 text-pretty text-fg-muted">{body}</p>
    </div>
  );
}

/** herdr's tab list in the 14:08:00 frame of the round (`herdr tab list`): the operator's tab and both children. */
const roundTabs = [
  { label: '1', status: 'done' },
  { label: 'todo undo', status: 'working' },
  { label: 'todo list --open', status: 'working' },
];

/** The round's integrate ask, verbatim from the alert log; the session link is withheld. */
export const integrateAlert = {
  emoji: '🟠',
  kind: 'Needs you',
  project: 'todo',
  session: 'todo-undo-open',
  body: 'list --open ready (2b6c5d1, tests pass). It makes unknown list flags an error (previously ignored) — keep? Recommend keep. OK to integrate both undo and list-open? Only conflict is the usage line; I\'ll merge it.',
};

export function TabStrip({ workspace = 'todo', className }: { workspace?: string; className?: string }) {
  return (
    <div className={cn('flex items-center gap-1 overflow-hidden px-2 py-2 font-mono text-[11px]', className)}>
      <span className="mr-1 hidden shrink-0 px-1.5 text-[color:var(--t-dim)] sm:inline">{workspace}</span>
      {roundTabs.map((t, i) => (
        <span
          key={t.label}
          className={cn(
            'flex min-w-0 items-center gap-1.5 rounded-md px-2 py-1',
            i === 0 ? 'shrink-0 bg-white/[0.08] text-[color:var(--t-fg)]' : 'text-[color:var(--t-dim)] ring-1 ring-white/[0.08]',
          )}
        >
          <span
            className={cn(
              'size-1.5 shrink-0 rounded-full',
              t.status === 'working' ? 'animate-pulse bg-[color:var(--t-amber)] motion-reduce:animate-none' : 'bg-[color:var(--t-ok)]',
            )}
            aria-hidden
          />
          <span className="truncate">{t.label}</span>
          <span className="hidden text-[color:var(--t-dim)] md:inline">{t.status}</span>
        </span>
      ))}
    </div>
  );
}

/**
 * The hero: one round of the run in a herdr window. The tab strip and ness status share the
 * 14:08:00 frame; the root's pane is the 14:07:30 frame with the proposal and the operator's yes;
 * the alert is the integrate ask the root sent at 14:10, shown as the phone card behind the window,
 * not inside it. Lines replay once, then rest.
 */
export function HerdrHero() {
  const chat = captureLines('proposal.ansi', [[1, 2], [9, 9], [11, 16], [27, 31]]);
  return (
    <figure className="relative">
      <div
        aria-hidden
        className="group absolute -top-20 -right-4 z-0 hidden w-64 rotate-3 opacity-90 transition duration-300 ease-out hover:z-20 hover:-translate-y-2 hover:rotate-0 hover:scale-105 hover:opacity-100 motion-reduce:transition-none lg:block xl:-top-[5.5rem] xl:-right-24 xl:w-72"
      >
        <p className="mb-1.5 flex items-center gap-1.5 pl-1 font-mono text-[10px] tracking-wider text-fg-subtle uppercase transition-colors group-hover:text-ember-ink">
          <span className="size-1.5 rounded-full bg-ember" />
          On your phone · from the root
        </p>
        <PhoneAlert className="replay-pop transition-shadow group-hover:shadow-[0_20px_60px_-15px_var(--ember)] group-hover:ring-1 group-hover:ring-ember/50" {...integrateAlert} />
      </div>
      <div className="terminal relative z-10 overflow-hidden rounded-2xl border border-line-strong text-left shadow-2xl">
        <div className="flex items-center gap-3 border-b border-white/[0.07] bg-white/[0.02] pl-4">
          <Dots />
          <TabStrip className="min-w-0 flex-1" />
        </div>
        <div className="min-w-0">
          <p className="px-4 pt-3 font-mono text-[10.5px] text-[color:var(--t-dim)] sm:px-5">tab 1 · Claude Code</p>
          <TermLines lines={chat} replay={0.4} className="px-4 pt-2 pb-4 text-[11px] text-[color:var(--t-fg)] sm:px-5 sm:text-[12px]" />
        </div>
        <div className="border-t border-white/[0.07] bg-black/20">
          <p className="px-4 pt-3 font-mono text-[10.5px] text-[color:var(--t-dim)] sm:px-5">ness status</p>
          <TermLines
            lines={captureLines('status.ansi', undefined, false)}
            replay={2.4}
            className="px-4 pt-2 pb-4 text-[10.5px] text-[color:var(--t-fg)] sm:px-5 sm:text-[11.5px]"
          />
        </div>
      </div>
      <figcaption className="mt-3 text-center text-xs text-fg-subtle">
        Captured from one real run, 2026-10-31. Moments of one round, in one window.
      </figcaption>
    </figure>
  );
}
