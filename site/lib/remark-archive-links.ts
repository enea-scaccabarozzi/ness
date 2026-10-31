import { existsSync } from 'node:fs';
import path from 'node:path';
import type { Link, Root } from 'mdast';
import { visit } from 'unist-util-visit';
import { githubUrl, gitConfig } from './shared';

// A relative link from a docs page to a file the site does not render (the archive: design,
// spec, sessions, superpowers) points at that file on GitHub, so the same Markdown works on
// github.com and on the site. A target missing from the repository fails the build.
export function remarkArchiveLinks(sections: string[]) {
  const docsDir = path.resolve(process.cwd(), '../docs');
  return () => (tree: Root, file: { path?: string }) => {
    if (!file.path) return;
    visit(tree, 'link', (node: Link) => {
      if (/^[a-z]+:|^#|^\//i.test(node.url)) return;
      const [target, hash] = node.url.split('#');
      const rel = path.relative(docsDir, path.resolve(path.dirname(file.path!), target)).split(path.sep).join('/');
      if (rel.startsWith('..') || rel === 'index.md' || sections.includes(rel.split('/')[0])) return;
      if (!existsSync(path.join(docsDir, decodeURIComponent(rel))))
        throw new Error(`${path.relative(docsDir, file.path!)}: link "${node.url}" points at docs/${rel}, which does not exist`);
      const kind = target.endsWith('/') ? 'tree' : 'blob';
      node.url = `${githubUrl}/${kind}/${gitConfig.branch}/docs/${rel}${hash ? `#${hash}` : ''}`;
    });
  };
}
