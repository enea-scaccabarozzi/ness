'use client';
import type { ReactNode } from 'react';
import { cn } from '@/lib/cn';
import { Container, Reveal, SectionHeading, SpotlightCard } from './ui';
import { withGlyphs } from './glyph';

function Tile({
  n,
  title,
  body,
  children,
  className,
  delay,
}: {
  n: string;
  title: string;
  body: string;
  children: ReactNode;
  className?: string;
  delay?: number;
}) {
  return (
    <Reveal delay={delay} className={cn('min-w-0', className)}>
      <SpotlightCard className="flex h-full flex-col">
        <div className="p-5 sm:p-6">
          <p className="font-mono text-[11px] text-fg-subtle">{n}</p>
          <h3 className="mt-2 text-lg font-semibold tracking-[-0.015em] text-fg">{title}</h3>
          <p className="mt-1.5 max-w-md text-sm leading-relaxed text-pretty text-fg-muted">{body}</p>
        </div>
        <div className="mt-auto px-5 pb-5 sm:px-6 sm:pb-6">{children}</div>
      </SpotlightCard>
    </Reveal>
  );
}

const panel = 'rounded-lg border border-line bg-ink-0/70 font-mono text-[11.5px] leading-relaxed';

function TreeVisual() {
  const rows: [string, string, string, string][] = [
    ['', 'checkout', 'root · full', 'ready'],
    ['├─ ', 'payments', 'child', 'done'],
    ['│  └─ ', 'refunds', 'child of payments', 'ready'],
    ['└─ ', 'receipts', 'child', 'ready'],
  ];
  return (
    <div className="grid gap-3 sm:grid-cols-[1fr_auto]">
      <div className={cn(panel, 'overflow-x-auto p-3')}>
        {rows.map(([pre, scope, role, phase]) => (
          <div key={scope} className="flex items-center whitespace-pre py-0.5">
            <span className="text-fg-subtle">{pre}</span>
            <span className={cn('mr-2 size-1.5 shrink-0 rounded-full', phase === 'done' ? 'bg-fg-subtle' : 'bg-ok')} />
            <span className="text-fg">{scope}</span>
            <span className="ml-2 text-fg-subtle">{role}</span>
            <span className={cn('ml-auto pl-4', phase === 'done' ? 'text-fg-subtle' : 'text-ok')}>{phase}</span>
          </div>
        ))}
      </div>
      <ul className={cn(panel, 'grid grid-cols-2 gap-x-4 gap-y-1 p-3 sm:grid-cols-1')}>
        <li className="text-fg-subtle">each session gets</li>
        {['a worktree', 'a branch', 'a terminal tab', 'a record dir'].map((x) => (
          <li key={x} className="text-fg">
            <span className="mr-1.5 text-ember-ink">+</span>
            {x}
          </li>
        ))}
      </ul>
    </div>
  );
}

function MessagesVisual() {
  const items: [string, string, string][] = [
    ['request', 'checkout → payments', 'Add Apple Pay to checkout'],
    ['handoff', 'payments → checkout', 'Done / Now / Next'],
    ['decision', 'checkout → operator', 'Ship behind a flag?'],
  ];
  return (
    <ul className="space-y-1.5">
      {items.map(([intent, route, body]) => (
        <li key={intent} className={cn(panel, 'px-3 py-2')}>
          <div className="flex items-center gap-2">
            <span className="rounded bg-ember-soft px-1.5 py-px text-[10.5px] text-ember-ink">{intent}</span>
            <span className="truncate text-fg-muted">{route}</span>
          </div>
          <p className="mt-1 truncate font-sans text-[12.5px] text-fg">{body}</p>
        </li>
      ))}
    </ul>
  );
}

function RecordVisual() {
  const files = ['brief.md', 'specs.md', 'plan.md', 'decisions.md', 'report.md'];
  return (
    <div className={cn(panel, 'p-3')}>
      <p className="text-fg-subtle">docs/sessions/checkout/</p>
      <ul className="mt-1.5 grid grid-cols-2 gap-x-3 gap-y-1">
        {files.map((f) => (
          <li key={f} className="flex items-center gap-1.5 text-fg">
            <span className="text-fg-subtle">▪</span>
            {f}
          </li>
        ))}
      </ul>
      <p className="mt-2 border-t border-line pt-2 text-fg-subtle">decisions: append-only, one id each</p>
    </div>
  );
}

function LayersVisual() {
  const layers: [string, string, boolean][] = [
    ['local profile', 'your rules, applied last', true],
    ['profiles', 'program · go-strict · …', false],
    ['features', 'probes · notify · contracts', false],
    ['core', 'sessions · messages · worker · sync', false],
  ];
  return (
    <div className="space-y-1.5">
      {layers.map(([name, what, mine], i) => (
        <div
          key={name}
          className={cn(
            panel,
            'flex items-center justify-between gap-3 px-3 py-2',
            mine && 'border-ember/50 bg-ember-soft',
          )}
          style={{ marginInline: `${i * 6}px` }}
        >
          <span className={mine ? 'text-ember-ink' : 'text-fg'}>{name}</span>
          <span className="truncate text-fg-subtle">{what}</span>
        </div>
      ))}
      <p className="pt-1 font-mono text-[11px] text-fg-subtle">
        sync never overwrites what you wrote: a conflict lands as <span className="text-fg-muted">*.ness-new</span>
      </p>
    </div>
  );
}

function DefaultsVisual() {
  const groups: [string, string[]][] = [
    ['features', ['probes', 'notify', 'contracts']],
    ['profiles', ['program', 'go-strict', 'behavior-specs', 'git-conventional', 'example-release']],
    ['atoms', ['herdr', 'Claude Code', 'Codex (exp.)', 'squash', 'merge', 'rebase', 'pr', 'Slack', 'Telegram', 'GitHub', 'systemd']],
  ];
  return (
    <div className="space-y-3">
      {groups.map(([g, chips]) => (
        <div key={g}>
          <p className="mb-1.5 font-mono text-[11px] text-fg-subtle">{g}</p>
          <ul className="flex flex-wrap gap-1">
            {chips.map((c) => (
              <li key={c} className="rounded-md border border-line bg-ink-0/70 px-2 py-0.5 font-mono text-[11px] text-fg-muted">
                {c}
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}

function GuardVisual() {
  const refusals: [string, string, string, string][] = [
    [
      'Monitor',
      '(ness inbox)',
      'polling with Monitor on ness inbox/status is refused: pull once with ness inbox, then end your turn; the worker wakes you when something arrives',
      '(skill ness-entry, section Communicate)',
    ],
    [
      'Edit',
      '(docs/spec/checkout.md)',
      'docs/spec/checkout.md is frozen project-wide ([contracts] frozen in .ness/ness.toml); change it on the base branch',
      '(skill ness-contracts, section Frozen files)',
    ],
  ];
  return (
    <div className="terminal space-y-3 rounded-lg border border-line-strong p-3 font-mono text-[11.5px] leading-relaxed text-[color:var(--t-fg)]">
      {refusals.map(([tool, arg, msg, ref]) => (
        <div key={tool}>
          <p>
            <span className="text-[color:var(--t-amber)]">● </span>
            <span className="font-semibold">{tool}</span>
            <span className="text-[color:var(--t-dim)]">{arg}</span>
          </p>
          <p className="flex gap-2 text-[color:var(--t-dim)]">
            <span className="shrink-0 pl-2">{withGlyphs('⎿')}</span>
            <span>
              <span className="text-[color:var(--t-fg)]">{msg}</span> {ref}
            </span>
          </p>
        </div>
      ))}
    </div>
  );
}

export function Machinery() {
  return (
    <section id="machinery" className="relative py-20 sm:py-28">
      <Container>
        <SectionHeading
          eyebrow="What is in the box"
          title="Six parts, one CLI."
          lead="Each part is tested on its own. Each default can be turned off or swapped per project."
        />
        <div className="mt-14 grid grid-cols-1 gap-4 md:grid-cols-6">
          <Tile
            n="01"
            title="Sessions"
            body="Each session is an agent conversation with a scope, a git worktree, a branch and a terminal tab. A root opens children; each child reports back to it."
            className="md:col-span-4"
          >
            <TreeVisual />
          </Tile>
          <Tile
            n="02"
            title="Inbox and worker"
            body="ness send and ness reply carry requests, handoffs and decisions. The worker wakes the session a message is for, and resumes sessions after usage limits."
            className="md:col-span-2"
            delay={0.05}
          >
            <MessagesVisual />
          </Tile>
          <Tile
            n="03"
            title="Session record"
            body="A full session commits its brief, plan, decisions and report under docs/sessions. You can read why each change was made."
            className="md:col-span-2"
          >
            <RecordVisual />
          </Tile>
          <Tile
            n="04"
            title="Your process as profiles"
            body="Your rules live in your repo as profiles: text, files, config and checks for named slots. ness sync renders them and never overwrites your edits."
            className="md:col-span-4"
            delay={0.05}
          >
            <LayersVisual />
          </Tile>
          <Tile
            n="05"
            title="Built-in defaults"
            body="Features, profiles and atoms that work out of the box. Turn each one on or off in .ness/ness.toml."
            className="md:col-span-3"
          >
            <DefaultsVisual />
          </Tile>
          <Tile
            n="06"
            title="Hooks that explain"
            body="Hooks refuse polling loops, edits to frozen files and models above the ladder. Each refusal names the rule and the fix."
            className="md:col-span-3"
            delay={0.05}
          >
            <GuardVisual />
          </Tile>
        </div>
      </Container>
    </section>
  );
}
