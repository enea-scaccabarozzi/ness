import { ImageResponse } from 'next/og';

// One social card for the landing and every docs page: dark ink, one ember accent.
export function ogCard({ title, description, eyebrow = 'ness' }: { title: string; description?: string; eyebrow?: string }) {
  return new ImageResponse(
    (
      <div
        style={{
          width: '100%',
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'space-between',
          padding: '72px 80px',
          background: '#0c0b0a',
          color: '#f3efe9',
          borderBottom: '12px solid #f2a43c',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 18 }}>
          {/* Mark from components/logo.tsx; the eye is drawn in the tile colour (Satori has no masks) */}
          <svg width="56" height="56" viewBox="0 0 32 32">
            <rect x="1" y="1" width="30" height="30" rx="8.5" fill="#1d1b19" stroke="#3a3632" strokeWidth="1" />
            <path d="M8 19.6 C7.8 16.2 8.8 12 12.3 10.4" fill="none" stroke="#f3efe9" strokeWidth="4" strokeLinecap="round" />
            <ellipse cx="14.6" cy="9.8" rx="4" ry="3" transform="rotate(-10 14.6 9.8)" fill="#f3efe9" />
            <path d="M15.2 21.6 a3.5 4 0 0 1 7 0z M22.9 21.6 a2.5 2.9 0 0 1 5 0z" fill="#f3efe9" />
            <circle cx="15.6" cy="8.9" r="0.95" fill="#1d1b19" />
            <path d="M4.6 24.4 q1.9 -1.4 3.8 0 t3.8 0 t3.8 0 t3.8 0 t3.8 0 t3.8 0" fill="none" stroke="#f2a43c" strokeWidth="1.7" strokeLinecap="round" />
          </svg>
          <span style={{ fontSize: 34, fontWeight: 600, letterSpacing: '-0.02em' }}>{eyebrow}</span>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 24 }}>
          <div style={{ fontSize: 68, fontWeight: 700, lineHeight: 1.08, letterSpacing: '-0.035em', maxWidth: 1000 }}>{title}</div>
          {description && <div style={{ fontSize: 30, lineHeight: 1.4, color: '#a8a199', maxWidth: 980 }}>{description}</div>}
        </div>
      </div>
    ),
    { width: 1200, height: 630 },
  );
}
