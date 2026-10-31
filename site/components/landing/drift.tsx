import Link from 'next/link';
import { ArrowRight, Container, Reveal, SectionHeading } from './ui';

// Illustration of the argument in docs/why/philosophy.md: three hand-built harnesses, three
// processes, one machinery copied and drifting.
const rows: [string, string, string][] = [
  ['project a', 'strict code quality', 'hand-built'],
  ['project b', 'scheduled checks on a live service', 'copied from a, patched'],
  ['project c', 'multi-week program', 'copied from b, drifted'],
];

export function Why() {
  return (
    <section id="why" className="relative scroll-mt-16 py-20 sm:py-28">
      <Container className="grid items-center gap-10 lg:grid-cols-[0.9fr_1.2fr] lg:gap-16">
        <div>
          <SectionHeading
            eyebrow="Why ness"
            title="There is no universal harness."
            lead="Each project needs its own process. Under every process sit the same parts: sessions, messages, records and extension points. ness ships those parts once, tested. You keep the process."
          />
          <Link
            href="/docs/why/philosophy"
            className="group mt-6 inline-flex items-center gap-1.5 text-sm font-medium text-ember-ink hover:underline"
          >
            Read the argument <ArrowRight />
          </Link>
        </div>

        <Reveal delay={0.1} className="min-w-0">
          {/* A grid, not a table: rows stack on phones, three columns from sm. */}
          <ul className="overflow-hidden rounded-xl border border-line bg-ink-1 text-sm">
            <li className="hidden grid-cols-[96px_1fr_1fr] gap-4 border-b border-line px-4 py-3 font-mono text-[11px] uppercase tracking-[0.1em] text-fg-subtle sm:grid" aria-hidden>
              <span />
              <span>Process</span>
              <span>Machinery</span>
            </li>
            {[...rows, ['with ness', 'yours, per project', 'ness, shared and tested'] as const].map(([p, process, machinery]) => {
              const ness = p === 'with ness';
              return (
                <li
                  key={p}
                  className={`grid gap-1 px-4 py-3 sm:grid-cols-[96px_1fr_1fr] sm:gap-4 ${ness ? 'bg-ember-soft' : 'border-b border-line'}`}
                >
                  <span className={`font-mono text-xs ${ness ? 'font-semibold text-ember-ink' : 'text-fg-subtle'}`}>{p}</span>
                  <span className="text-fg">{process}</span>
                  <span className={ness ? 'text-fg' : 'text-fg-muted'}>
                    <span className={`mr-1.5 font-mono ${ness ? 'text-ember-ink' : 'text-warn'}`}>{ness ? '=' : '≠'}</span>
                    {machinery}
                  </span>
                </li>
              );
            })}
          </ul>
        </Reveal>
      </Container>
    </section>
  );
}
