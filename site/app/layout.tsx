import { Hydrated } from '@/components/hydrated';
import type { Metadata, Viewport } from 'next';
import { GeistSans } from 'geist/font/sans';
import { GeistMono } from 'geist/font/mono';
import { Provider } from '@/components/provider';
import { appName, basePath, description, siteUrl, tagline } from '@/lib/shared';
import './global.css';

export const metadata: Metadata = {
  metadataBase: new URL(`${siteUrl}${basePath}/`),
  title: { default: `${appName}: ${tagline}`, template: `%s · ${appName}` },
  description,
  icons: { icon: `${basePath}/icon.svg` },
  openGraph: {
    title: `${appName}: ${tagline}`,
    description,
    siteName: appName,
    type: 'website',
    images: `${basePath}/og.png`,
  },
  twitter: { card: 'summary_large_image' },
};

export const viewport: Viewport = {
  themeColor: [
    { media: '(prefers-color-scheme: dark)', color: '#0e0d0c' },
    { media: '(prefers-color-scheme: light)', color: '#fdfcfa' },
  ],
};

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <html lang="en" className={`${GeistSans.variable} ${GeistMono.variable}`} suppressHydrationWarning>
      <head>
        {/* Reveal animations hide content until React hydrates; if it never does (blocked or failed
            scripts), show everything after 3 s rather than a blank page. */}
        <script
          dangerouslySetInnerHTML={{
            __html:
              "setTimeout(function(){if(!window.__nessHydrated)document.documentElement.classList.add('no-hydrate')},3000)",
          }}
        />
      </head>
      <body className="flex min-h-screen flex-col font-sans">
        <Provider>
          <Hydrated />
          {children}
        </Provider>
      </body>
    </html>
  );
}
