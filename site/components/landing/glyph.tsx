import type { ReactNode } from 'react';

// Geist Mono has no ✓ (U+2713) or ⎿ (U+23BF), and many systems have no fallback for ⎿.
// Draw both so the terminal replays look the same everywhere.
function Check() {
  return (
    <svg viewBox="0 0 10 10" aria-label="✓" className="inline-block h-[0.85em] w-[0.6em] -translate-y-[0.05em]">
      <path d="M1.5 5.5 4 8l4.5-6" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
function Elbow() {
  return (
    <span aria-label="⎿" className="inline-block h-[0.75em] w-[0.6em] -translate-y-[0.15em] border-b border-l border-current opacity-80" />
  );
}

/** Text with ✓ and ⎿ replaced by drawn glyphs that keep one monospace cell each. */
export function withGlyphs(text: string): ReactNode {
  if (!/[✓⎿]/.test(text)) return text;
  return text.split(/([✓⎿])/).map((part, i) =>
    part === '✓' ? <Check key={i} /> : part === '⎿' ? <Elbow key={i} /> : part,
  );
}
