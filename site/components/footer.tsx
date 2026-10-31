import Link from 'next/link';
import { Wordmark } from './logo';
import { githubUrl, tagline } from '@/lib/shared';

const links: [string, string][] = [
  ['Introduction', '/docs'],
  ['Philosophy', '/docs/why/philosophy'],
  ['Principles', '/docs/why/principles'],
  ['Sessions', '/docs/concepts/sessions'],
];

export function Footer() {
  return (
    <footer className="border-t border-line">
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-8 px-4 py-10 sm:flex-row sm:items-start sm:justify-between sm:px-6">
        <div>
          <Wordmark />
          <p className="mt-3 max-w-xs text-sm text-fg-muted">{tagline}</p>
        </div>
        <nav aria-label="Footer" className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
          {links.map(([label, href]) => (
            <Link key={href} href={href} className="text-fg-muted transition-colors hover:text-fg">
              {label}
            </Link>
          ))}
          <a href={githubUrl} className="text-fg-muted transition-colors hover:text-fg" target="_blank" rel="noreferrer">
            GitHub
          </a>
        </nav>
      </div>
      <div className="mx-auto w-full max-w-6xl px-4 pb-10 font-mono text-[11.5px] text-fg-subtle sm:px-6">
        Apache-2.0 · Made by Enea Scaccabarozzi, with ness
      </div>
    </footer>
  );
}
