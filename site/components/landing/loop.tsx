import Link from 'next/link';
import type { ReactNode } from 'react';
import { cn } from '@/lib/cn';
import { ArrowRight, Container, Reveal, SectionHeading } from './ui';

// Facts from docs/concepts/guides-and-sensors.md. The refusal is a real one, from a session on
// 2026-10-31 (the pre-tool hook, rule.HookBackground in internal/rule/rule.go).

const guides: [string, ReactNode][] = [
  ['Skills', <>ness-entry, ness-launch, feature and profile skills in <code>.agents/skills</code></>],
  ['AGENTS.md', 'Managed blocks, read at the start of every session'],
  ['Profile slots', 'Your process text, rendered into the core skills'],
  ['The brief', 'What this session owes, written by its manager'],
  ['Your docs', 'Specs and records next to the code'],
];

const sensors: [string, string][] = [
  ['Pre-tool hooks', 'Polling loops, & backgrounding, frozen files, rungs above the ladder, the gate by hand'],
  ['Profile checks', 'At open, integrate and close; exit 4, nothing written'],
  ['The gate', 'ness integrate runs it in a throwaway checkout'],
  ['Contracts', 'ness contract check on frozen files'],
  ['Checkpoints', 'Handoffs need Done, Now, Next; close needs ## Lessons'],
  ['Doctor, stats, probes', 'Setup, cost per session, the running world'],
];

function Card({ kind, when, children, className }: { kind: string; when: string; children: ReactNode; className?: string }) {
  return (
    <div className={cn('rounded-2xl border border-line bg-ink-1 p-5 sm:p-6', className)}>
      <p className="font-mono text-eyebrow uppercase text-fg-subtle">{when}</p>
      <h3 className="mt-1.5 text-lg font-semibold text-fg">{kind}</h3>
      <ul className="mt-4 space-y-2.5 text-sm">{children}</ul>
    </div>
  );
}

function Item({ name, children, hot }: { name: string; children: ReactNode; hot?: ReactNode }) {
  return (
    <li className="grid grid-cols-[0.5rem_minmax(0,1fr)] gap-x-3">
      <span className="mt-[0.45rem] size-1.5 rounded-full bg-fg-subtle/60" aria-hidden />
      <div>
        <span className="font-medium text-fg">{name}</span>{' '}
        <span className="text-fg-muted [&_code]:font-mono [&_code]:text-[0.9em]">{children}</span>
        {hot}
      </div>
    </li>
  );
}

function Arrow({ dir, label, className }: { dir: 'right' | 'left'; label: string; className?: string }) {
  return (
    <div className={cn('flex items-center gap-2 font-mono text-[11px] text-fg-subtle', dir === 'left' && 'flex-row-reverse', className)}>
      <span className="h-px flex-1 bg-gradient-to-r from-transparent via-line-strong to-line-strong" aria-hidden />
      <span className="shrink-0">{label}</span>
      <ArrowRight className={cn('size-3.5 shrink-0 text-ember-ink', dir === 'right' ? 'max-lg:rotate-90' : 'max-lg:-rotate-90 lg:rotate-180')} />
    </div>
  );
}

export function GuidesSensors() {
  return (
    <section id="guides-sensors" className="relative scroll-mt-16 py-20 sm:py-28">
      <Container>
        <SectionHeading
          eyebrow="Guides and sensors"
          title="Guides before, sensors after."
          lead="Guides tell the agent the workflow before it acts. Sensors catch drift as it happens. Every refusal names the guide it enforces."
        />
        <Reveal className="mt-12 grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)_minmax(0,1fr)] lg:gap-6">
          <Card kind="Guides" when="Before it acts" className="max-lg:order-1">
            {guides.map(([n, b]) => (
              <Item
                key={n}
                name={n}
                hot={
                  n === 'Skills' && (
                    <span className="mt-1.5 flex w-fit items-center gap-1.5 rounded-md bg-ember-soft px-2 py-0.5 font-mono text-[11px] text-ember-ink ring-1 ring-ember/40">
                      ness-entry › Communicate
                    </span>
                  )
                }
              >
                {b}
              </Item>
            ))}
          </Card>

          {/* Below lg this column dissolves so the arrows sit between the cards: guides, act, sensors, refusal, back. */}
          <div className="max-lg:contents lg:space-y-4 lg:self-center">
            <Arrow dir="right" label="the agent acts" className="max-lg:order-2" />
            <figure className="terminal rounded-xl max-lg:order-4 border border-line-strong p-4 font-mono text-[11.5px] leading-relaxed shadow-lg">
              <figcaption className="mb-2 text-[10.5px] text-[color:var(--t-dim)]">pre-tool hook, a real refusal</figcaption>
              <p className="text-[color:var(--t-fg)]">
                ness: backgrounding with &amp; is refused: nothing wakes you when the job ends; run it with the Bash tool&apos;s
                run_in_background option and end your turn; the task notification wakes you{' '}
                <span className="rounded bg-[oklch(0.8_0.15_64/0.14)] px-1 text-[color:var(--t-amber)] ring-1 ring-[oklch(0.8_0.15_64/0.4)]">
                  (skill ness-entry, section Communicate)
                </span>
              </p>
            </figure>
            <Arrow dir="left" label="points back to its guide" className="max-lg:order-5" />
          </div>

          <Card kind="Sensors" when="As it happens" className="max-lg:order-3">
            {sensors.map(([n, b]) => (
              <Item key={n} name={n}>
                {b}
              </Item>
            ))}
          </Card>
        </Reveal>
        <Reveal className="mt-8 flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-fg-muted">
            A rule of your own is a skill section, a check, and the pointer between them. <code className="font-mono text-[0.9em]">ness sync</code> refuses a pointer to a heading that does not exist.
          </p>
          <Link href="/docs/concepts/guides-and-sensors" className="inline-flex shrink-0 items-center gap-1.5 text-sm font-medium text-ember-ink hover:underline">
            How the loop works <ArrowRight className="size-3.5" />
          </Link>
        </Reveal>
      </Container>
    </section>
  );
}
