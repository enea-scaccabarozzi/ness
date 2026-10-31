import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { Wordmark } from '@/components/logo';
import { githubUrl } from './shared';

/** Top links for the landing; the docs sidebar already lists these pages. */
export const homeLinks: BaseLayoutProps['links'] = [
  { text: 'Docs', url: '/docs', active: 'nested-url' },
  { text: 'Philosophy', url: '/docs/why/philosophy' },
  { text: 'Principles', url: '/docs/why/principles' },
];

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      title: <Wordmark />,
    },
    githubUrl,
  };
}
