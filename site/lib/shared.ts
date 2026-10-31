import { createGetUrl } from 'fumadocs-core/source';

export const appName = 'ness';
export const tagline = 'There is no universal harness. Build yours.';
export const description =
  'ness is a CLI that turns Claude Code into a team of sessions in your repo. Each session gets its own worktree, branch and terminal tab, an inbox, and a written record.';
export const docsRoute = '/docs';
export const docsImageRoute = '/og/docs';
export const docsContentRoute = '/llms.mdx/docs';

/** Where the site is served from; '' locally, '/ness' on GitHub Pages. */
export const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? '';
export const siteUrl = process.env.NEXT_PUBLIC_SITE_URL ?? 'https://enea-scaccabarozzi.github.io';

export const gitConfig = {
  user: 'enea-scaccabarozzi',
  repo: 'ness',
  branch: 'main',
};
export const githubUrl = `https://github.com/${gitConfig.user}/${gitConfig.repo}`;

const getContentUrl = createGetUrl(docsContentRoute);

export function getPageMarkdownUrl(page: { slugs: string[]; locale?: string }) {
  const segments = [...page.slugs, 'content.md'];
  return { segments, url: getContentUrl(segments, page.locale) };
}

const getImageUrl = createGetUrl(docsImageRoute);

export function getPageImageUrl(page: { slugs: string[]; locale?: string }) {
  const segments = [...page.slugs, 'image.png'];
  return { segments, url: getImageUrl(segments, page.locale) };
}
