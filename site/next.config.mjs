import path from 'node:path';
import { createMDX } from 'fumadocs-mdx/next';

const withMDX = createMDX();

// GitHub Pages serves the site under /ness; local dev runs at /.
const basePath = process.env.NEXT_PUBLIC_BASE_PATH ?? '';

/** @type {import('next').NextConfig} */
const config = {
  output: 'export',
  basePath,
  images: { unoptimized: true },
  reactStrictMode: true,
  devIndicators: false,
  // Hosts allowed to open the dev server from another machine, e.g. DEV_ORIGINS=192.168.1.5
  allowedDevOrigins: (process.env.DEV_ORIGINS ?? '').split(',').filter(Boolean),
  // docs/ lives one level up, outside site/: let the bundler read it.
  turbopack: { root: path.resolve(import.meta.dirname, '..') },
  outputFileTracingRoot: path.resolve(import.meta.dirname, '..'),
};

export default withMDX(config);
