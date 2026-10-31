import { defineConfig, defineDocs } from 'fumadocs-mdx/config';
import { metaSchema, pageSchema } from 'fumadocs-core/source/schema';
import { remarkSteps } from 'fumadocs-core/mdx-plugins';
import { remarkGithubAlerts } from './lib/remark-github-alerts';
import { remarkArchiveLinks } from './lib/remark-archive-links';

// One source: the repository's own docs/ folder, read in place, never copied.
// An allowlist, not a denylist: docs/discovery (private) and the archive folders
// (design, spec, sessions, superpowers, profiles, backlog.md) never match.
const sections = ['why', 'start', 'concepts', 'guides', 'reference', 'built-with-ness'];

export const docs = defineDocs({
  dir: '../docs',
  docs: {
    schema: pageSchema,
    files: ['index.md', ...sections.map((s) => `${s}/**/*.md`)],
    postprocess: { includeProcessedMarkdown: true },
  },
  meta: {
    schema: metaSchema,
    files: ['meta.json', ...sections.map((s) => `${s}/**/meta.json`)],
  },
});

export default defineConfig({
  mdxOptions: {
    remarkPlugins: [remarkGithubAlerts, remarkSteps, remarkArchiveLinks(sections)],
  },
});
