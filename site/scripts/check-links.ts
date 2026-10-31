// Fails when a page of the static export links to a page, file or heading that does not exist.
// Usage: bun scripts/check-links.ts [out]   (after `bun run build` or `bun run build:pages`)
// Reads the base path from NEXT_PUBLIC_BASE_PATH, as the build does. External links are skipped.
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';

const out = path.resolve(process.argv[2] ?? 'out');
const base = process.env.NEXT_PUBLIC_BASE_PATH ?? '';

function walk(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = path.join(dir, name);
    return statSync(p).isDirectory() ? walk(p) : [p];
  });
}

// The file a URL path is served from, as GitHub Pages resolves it, or undefined.
function target(urlPath: string): string | undefined {
  const rel = decodeURIComponent(urlPath).replace(/^\/+/, '');
  const p = path.join(out, rel);
  return [p, `${p}.html`, path.join(p, 'index.html')].find((c) => existsSync(c) && statSync(c).isFile());
}

const idsCache = new Map<string, Set<string>>();
function ids(file: string): Set<string> {
  let set = idsCache.get(file);
  if (!set) {
    set = new Set([...readFileSync(file, 'utf8').matchAll(/\sid="([^"]+)"/g)].map((m) => m[1]));
    idsCache.set(file, set);
  }
  return set;
}

const pages = walk(out).filter((f) => f.endsWith('.html'));
const broken: string[] = [];
for (const page of pages) {
  const html = readFileSync(page, 'utf8');
  const from = '/' + path.relative(out, page);
  for (const [, attr, raw] of html.matchAll(/\s(href|src)="([^"]*)"/g)) {
    const url = raw.replaceAll('&amp;', '&');
    if (/^([a-z][a-z0-9+.-]*:|\/\/)/i.test(url) || url === '') continue; // external, mailto:, data:
    const [pathAndQuery, hash] = url.split('#');
    const urlPath = pathAndQuery.split('?')[0];
    let file: string | undefined = page;
    if (urlPath !== '') {
      if (!urlPath.startsWith('/')) {
        file = target(path.posix.join(path.posix.dirname(from), urlPath));
      } else if (base && !urlPath.startsWith(`${base}/`) && urlPath !== base) {
        broken.push(`${from}: ${attr}="${raw}" is outside the base path ${base}`);
        continue;
      } else {
        file = target(urlPath.slice(base.length));
      }
    }
    if (!file) broken.push(`${from}: ${attr}="${raw}" has no page or file`);
    else if (hash && file.endsWith('.html') && !ids(file).has(decodeURIComponent(hash)))
      broken.push(`${from}: ${attr}="${raw}" names no heading on its page`);
  }
}

const unique = [...new Set(broken)].sort();
for (const line of unique) console.error(line);
console.log(`${pages.length} pages checked, ${unique.length} broken links`);
process.exit(unique.length > 0 ? 1 : 0);
