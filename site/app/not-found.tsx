import Link from 'next/link';
import { Wordmark } from '@/components/logo';

export default function NotFound() {
  return (
    <main className="relative flex flex-1 flex-col items-center justify-center overflow-hidden px-4 py-24 text-center">
      <div className="bg-grid mask-fade-b pointer-events-none absolute inset-0 opacity-60" aria-hidden />
      <div className="relative">
        <Link href="/" aria-label="ness home" className="inline-block">
          <Wordmark />
        </Link>
        <div className="terminal mx-auto mt-10 max-w-md rounded-xl border border-line-strong p-5 text-left font-mono text-[12.5px] leading-relaxed text-[color:var(--t-fg)] shadow-lg">
          <p>
            <span className="text-[color:var(--t-dim)]">&gt; </span>open this page
          </p>
          <p className="mt-2 text-[color:var(--t-amber)]">404: no page at this path, refused.</p>
          <p className="text-[color:var(--t-dim)]">It may have moved when the docs were reorganised.</p>
        </div>
        <div className="mt-8 flex justify-center gap-5 text-sm">
          <Link href="/" className="font-medium text-ember-ink hover:underline">
            Home
          </Link>
          <Link href="/docs" className="font-medium text-ember-ink hover:underline">
            Docs
          </Link>
        </div>
      </div>
    </main>
  );
}
