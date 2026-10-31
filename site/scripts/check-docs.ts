// Fails when the docs nav and the pages disagree, or the landing page's proof numbers drift
// from docs/built-with-ness/index.md. Usage: bun scripts/check-docs.ts (from site/).
// Nav: every page under the sections of source.config.ts is listed in its folder's meta.json,
// and every meta.json entry is a page (name.md) or a folder. The repo uses no "..." rest entries.
import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';

const docs = path.resolve('../docs');
const sections = [...readFileSync('source.config.ts', 'utf8').match(/const sections = \[([^\]]*)\]/)![1].matchAll(/'([^']+)'/g)].map((m) => m[1]);
const errors: string[] = [];

function checkFolder(rel: string, only?: string[]) {
  const dir = path.join(docs, rel);
  const metaFile = path.join(rel, 'meta.json');
  const names = readdirSync(dir).filter((n) => !only || only.includes(n.replace(/\.md$/, '')));
  const pages = names.filter((n) => n.endsWith('.md')).map((n) => n.slice(0, -3));
  const folders = names.filter((n) => statSync(path.join(dir, n)).isDirectory());
  if (!existsSync(path.join(docs, metaFile))) {
    errors.push(`docs/${metaFile} is missing`);
  } else {
    const listed: string[] = JSON.parse(readFileSync(path.join(docs, metaFile), 'utf8')).pages ?? [];
    if (listed.some((p) => p.startsWith('...') || p.startsWith('---') || p.startsWith('['))) errors.push(`docs/${metaFile}: unsupported special entry; update scripts/check-docs.ts`);
    for (const p of [...pages, ...folders]) if (!listed.includes(p)) errors.push(`docs/${metaFile} does not list ${p}`);
    for (const p of listed) if (!pages.includes(p) && !folders.includes(p)) errors.push(`docs/${metaFile} lists ${p}, which has no page or folder`);
  }
  for (const f of folders) checkFolder(path.join(rel, f));
}
checkFolder('', ['index', ...sections]);

// Proof: each value in proof.tsx's stats array appears as a number in the docs page.
const proof = readFileSync('components/landing/proof.tsx', 'utf8').match(/const stats[^=]*=\s*\[([\s\S]*?)\n\];/)![1];
const page = readFileSync(path.join(docs, 'built-with-ness/index.md'), 'utf8');
const values = [...proof.matchAll(/value:\s*'([^']+)'/g)].map((m) => m[1]);
if (values.length === 0) errors.push('components/landing/proof.tsx: no stats values found');
for (const v of values) {
  const re = new RegExp(`(?<![\\d.,])${v.replace(/[.,]/g, '\\$&')}(?![\\d]|[.,]\\d)`);
  if (!re.test(page)) errors.push(`components/landing/proof.tsx shows ${v}, which docs/built-with-ness/index.md does not state`);
}

for (const e of errors) console.error(e);
console.log(`docs nav and proof numbers: ${errors.length} problems`);
process.exit(errors.length > 0 ? 1 : 0);
