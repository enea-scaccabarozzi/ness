import Link from 'next/link';
import { ArrowRight, Container, Reveal, SectionHeading } from './ui';

// Verified 2026-10-31 from the stores of four projects run on ness (ness itself and three
// field projects), 2026-10-10 to 10-31: docs/sessions/public-release/verification.md.
const stats: { value: string; label: string; source: string }[] = [
  { value: '81', label: 'sessions across four projects', source: 'store: sessions' },
  { value: '74', label: 'integrated through each project\'s own gate', source: 'store: integrations' },
  { value: '1,722', label: 'store items: messages, handoffs and probe wakes', source: 'store: items' },
  { value: '1,484', label: 'probe runs watching deployed services', source: 'store: probe runs' },
];

// Real: the closed roots and children of ness's own development, from `ness status --all`.
const tree: [string, string][] = [
  ['○ s-4ad0a273d838', 'cli-ux'],
  ['├─ ○ s-62214e9d3763', 'delivery'],
  ['├─ ○ s-1eb79f19a3bf', 'operator-loop'],
  ['├─ ○ s-d72adf947492', 'seams'],
  ['└─ ○ s-d81c78bf72a9', 'autonomy'],
  ['○ s-c5a82a215386', 'field-feedback'],
  ['├─ ○ s-e2eccad430d0', 'e-pointers'],
  ['└─ ○ s-4776a53a4a19', 'batch-e'],
  ['○ s-cf09261a7b0e', 'model-select'],
  ['○ s-404360a52d03', 'field-feedback-3'],
  ['├─ ○ s-08e81663ef79', 'liveness'],
  ['└─ ○ s-109ce4c0febd', 'integrate'],
];

export function Proof() {
  return (
    <section id="proof" className="relative border-y border-line bg-ink-1/50 py-20 sm:py-28">
      <Container className="grid items-start gap-12 lg:grid-cols-[1fr_1.05fr] lg:gap-16">
        <div>
          <SectionHeading
            eyebrow="Proof"
            title="ness builds ness."
            lead="After its first steps, ness was developed with ness. The specs, session records and field rounds are public, anonymised."
          />
          <dl className="mt-10 grid grid-cols-2 gap-px overflow-hidden rounded-xl border border-line bg-line">
            {stats.map((s) => (
              <div key={s.label} className="relative bg-ink-0 p-5">
                <dt className="sr-only">{s.label}</dt>
                <dd className="font-mono text-3xl font-semibold tracking-[-0.04em] text-fg sm:text-4xl">{s.value}</dd>
                <dd className="mt-1.5 text-sm text-fg-muted">{s.label}</dd>
              </div>
            ))}
          </dl>
          <Link
            href="/docs/why/philosophy#ness-builds-ness"
            className="group mt-6 inline-flex items-center gap-1.5 text-sm font-medium text-ember-ink hover:underline"
          >
            How ness was built <ArrowRight />
          </Link>
        </div>

        <Reveal delay={0.1}>
          <figure className="terminal overflow-hidden rounded-xl border border-line-strong shadow-lg">
            <figcaption className="flex h-10 items-center justify-between border-b border-white/[0.07] px-4 font-mono text-[11px] text-[color:var(--t-dim)]">
              <span>ness status --all</span>
              <span>ness · base main</span>
            </figcaption>
            <pre className="overflow-x-auto p-4 font-mono text-[12px] leading-[1.75] text-[color:var(--t-fg)] sm:p-5">
              {tree.map(([pre, scope]) => {
                const [connector, id] = [pre.slice(0, pre.indexOf('s-')), pre.slice(pre.indexOf('s-'))];
                return (
                  <div key={id}>
                    <span className="text-[color:var(--t-dim)]">{connector}</span>
                    <span className="text-[color:var(--t-dim)]">{id}</span>
                    {'  '}
                    <span>{scope}</span>
                    {'  '}
                    <span className="text-[color:var(--t-dim)]">closed</span>
                  </div>
                );
              })}
              <div className="mt-1 text-[color:var(--t-dim)]">…</div>
            </pre>
          </figure>
          <p className="mt-3 text-center text-xs text-fg-subtle">An excerpt of the real tree. Every row has a committed record.</p>
        </Reveal>
      </Container>
    </section>
  );
}
