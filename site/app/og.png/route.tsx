import { ogCard } from '@/lib/og';
import { tagline } from '@/lib/shared';

export const dynamic = 'force-static';

// The landing's social card, exported as /og.png.
export function GET() {
  return ogCard({ title: tagline, description: 'A CLI that turns Claude Code into a team of sessions in your repo. You talk to one agent; it opens the rest.' });
}
