import type { ReactNode } from 'react';
import { cn } from '@/lib/cn';
import { Container, Reveal, SectionHeading } from './ui';
import { TermFrame, t } from './shot';
import { Capture, NessOutput, PhoneAlert, TabStrip, integrateAlert } from './capture';

/*
 * Everything in this section is real and from 2026-10-31 (ness 0.1.0+b13684c9f547, Claude Code
 * 2.1.316, herdr 0.9.3). Step 01 is `ness init` in a fresh scratch repo. Steps 02-06 are one round
 * of one root session (todo-undo-open) in a scratch Go CLI called todo, the same round as the hero:
 * each frame shows a few whole lines of its capture (site/captures, cut by captures/trim.sh) and
 * opens to the full one. The ness output lines come from the root's transcript, the tabs from
 * `herdr tab list`, the alerts from the alert log, with their session links withheld.
 */

const initLines = [
  '.agents/skills/ness-entry/SKILL.md',
  '.agents/skills/ness-launch/SKILL.md',
  '.claude/agents/ness-review.md',
  '.claude/settings.json:hooks.SessionStart[ness hook session-start]',
  '.claude/settings.json:hooks.PreToolUse[ness hook pre-tool]',
  'AGENTS.md#core/router',
];

type Step = { title: string; body: ReactNode; artifact: ReactNode };

const steps: Step[] = [
  {
    title: 'Add ness to your repository',
    body: (
      <>
        <code>ness init</code> writes the agent&apos;s skills, hooks and settings into your repo. It also sets your gate:
        the command that must pass before any work lands.
      </>
    ),
    artifact: (
      <TermFrame title="ness init --yes --gate true   (6 of 11 lines)">
        {initLines.map((l) => (
          <div key={l} className="break-all whitespace-pre-wrap">
            <span className={t.ok}>create</span> {l}
          </div>
        ))}
      </TermFrame>
    ),
  },
  {
    title: 'Ask for the work. The agent proposes a plan.',
    body: 'You talk to Claude Code as usual. When the work calls for sessions, the agent proposes them first: the root, the children and their tiers. Nothing opens until you say yes.',
    artifact: <Capture file="proposal.ansi" title="tab 1 · Claude Code" pick={[[1, 2], [9, 9], [17, 18], [21, 22], [27, 31]]} />,
  },
  {
    title: 'On your yes, children open in their own tabs',
    body: 'Your conversation becomes the root session. It writes each child a brief and opens it in its own worktree, branch and herdr tab.',
    artifact: (
      <div className="terminal overflow-hidden rounded-xl border border-line-strong shadow-lg">
        <TabStrip className="border-b border-white/[0.07] bg-white/[0.02]" />
        <Capture className="rounded-none border-0 shadow-none" file="child-undo.ansi" title="tab todo undo · a child, economy tier" pick={[[1, 1], [5, 6]]} />
        <Capture
          className="rounded-none border-0 border-t border-white/[0.07] shadow-none"
          file="child-list-open.ansi"
          title="tab todo list --open · a child, economy tier"
          pick={[[1, 1], [5, 6]]}
          more={
            <NessOutput
              lines={[
                'opened s-8446bf13aa78 scope undo branch session/undo-8446bf13aa78 record docs/sessions/todo-undo-open/undo tier economy (explicit)',
                'opened s-0c8a4af43c06 scope list-open branch session/list-open-0c8a4af43c06 record docs/sessions/todo-undo-open/list-open tier economy (explicit)',
              ]}
            />
          }
        />
      </div>
    ),
  },
  {
    title: 'Sessions report through an inbox',
    body: 'Each child commits on its own branch and hands off: done, now, next. The worker wakes the root when the message arrives.',
    artifact: (
      <div className="space-y-3">
        <Capture file="child-undo.ansi" title="tab todo undo · the child writes its handoff" pick={[[9, 13]]} />
        <Capture file="status.ansi" title="ness status · the root waits on undo" joinWraps={false} />
      </div>
    ),
  },
  {
    title: 'When a decision is yours, your phone pings',
    body: 'The root asks in its chat and files the question. The worker sends a short alert to your phone. You answer in the chat.',
    artifact: (
      <div>
        <Capture file="ask.ansi" title="tab 1 · the root" pick={[[3, 3], [10, 13], [15, 15]]} />
        <PhoneAlert className="mt-3 ml-auto max-w-sm sm:mr-4" {...integrateAlert} />
      </div>
    ),
  },
  {
    title: 'Say integrate. The work lands through your gate.',
    body: (
      <>
        <code>ness integrate</code> runs your gate and merges only when it passes. The session&apos;s report sits next to
        the code, so the reasons stay in the repo.
      </>
    ),
    artifact: (
      <Capture
        file="integrate.ansi"
        title="tab 1 · the root"
        pick={[[1, 2], [29, 34]]}
        more={
          <>
            <NessOutput
              lines={[
                'go vet ./... && go test ./...',
                'ok  \ttodo\t0.004s',
                'integrated session/todo-undo-open-51faa1fd840a into main at 1f722a26a0d3029cc25403576f1e697c5040e07e',
              ]}
            />
            <PhoneAlert
              className="max-w-sm"
              emoji="🟢"
              kind="Checkpoint"
              project="todo"
              session="todo-undo-open"
              body="Close the root when you like."
            />
          </>
        }
      />
    ),
  },
];

export function HowItWorks() {
  return (
    <section id="how" className="relative scroll-mt-16 pt-6 pb-20 sm:pt-10 sm:pb-28">
      <Container>
        <SectionHeading
          eyebrow="How it works"
          title="You talk to one agent. It opens the rest."
          lead="Six steps, from an empty repo to merged work. Steps 2 to 6 follow one root session in todo, a small Go CLI, adding undo and list --open."
        />
        <Reveal>
          <p className="mt-5 inline-flex items-center gap-2 rounded-full border border-line bg-ink-1 px-3 py-1 font-mono text-[11px] text-fg-muted">
            <span className="size-1.5 rounded-full bg-ok" aria-hidden />
            Captured from a real run · 2026-10-31
          </p>
        </Reveal>
        <ol className="mt-12 space-y-14 sm:mt-14 md:space-y-20">
          {steps.map((s, i) => (
            <li
              key={s.title}
              className={cn(
                'grid items-center gap-6 md:gap-12',
                i % 2 ? 'md:grid-cols-[minmax(0,7fr)_minmax(0,5fr)]' : 'md:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]',
              )}
            >
              <Reveal className={cn(i % 2 === 1 && 'md:order-2')}>
                <span className="font-mono text-sm text-ember-ink">0{i + 1}</span>
                <h3 className="mt-2 text-xl font-semibold tracking-[-0.015em] text-balance text-fg sm:text-2xl">{s.title}</h3>
                <p className="mt-3 max-w-md text-pretty leading-relaxed text-fg-muted [&_code]:rounded [&_code]:bg-ink-2 [&_code]:px-1 [&_code]:py-px [&_code]:font-mono [&_code]:text-[0.88em] [&_code]:text-fg">
                  {s.body}
                </p>
              </Reveal>
              <Reveal delay={0.08} className="min-w-0">
                {s.artifact}
              </Reveal>
            </li>
          ))}
        </ol>
      </Container>
    </section>
  );
}
