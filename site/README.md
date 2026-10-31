# site

The ness landing page and docs. Fumadocs on Next.js, Tailwind v4, exported as static files.

## Run

| Task | Command |
| --- | --- |
| Install | `bun install` |
| Dev server on port 3210, all interfaces | `bun run dev` |
| Static build into `out/`, served at `/` | `bun run build` |
| Static build for GitHub Pages, served at `/ness` | `bun run build:pages` |
| Serve `out/` on port 3210 | `bun run start` |
| Type check | `bun run types:check` |

The base path comes from `NEXT_PUBLIC_BASE_PATH`. Left unset, the site lives at `/`.

## Where the docs come from

The docs are the repository's own `docs/` folder, read in place. Nothing is copied into `site/`.

| Rule | Where |
| --- | --- |
| Pages are plain GFM `.md` with `title` and `description` frontmatter | `docs/**` |
| Each folder orders its pages with a `meta.json` | `docs/**/meta.json` |
| Only `index.md` and six sections are read: `why`, `start`, `concepts`, `guides`, `reference`, `built-with-ness` | `source.config.ts` |
| GitHub alerts (`> [!NOTE]`, `TIP`, `IMPORTANT`, `WARNING`, `CAUTION`) render as callouts | `lib/remark-github-alerts.ts` |
| Numbered headings inside a `steps` block render as steps | `remarkSteps` from Fumadocs |

The collection is an allowlist. `docs/discovery/` is private and never matches. The archive folders (`design`, `spec`, `sessions`, `superpowers`, `profiles`) and `backlog.md` never match either. A new section needs a line in `source.config.ts`.

The pages render as GFM on GitHub too. Keep them free of MDX components.

## Design system

### Principles

- Dark first. Light mode uses the same tokens with lighter values.
- One accent, ember (a warm amber). No second hue. Green and yellow appear only as terminal state colours.
- Warm neutrals, not blue-grey.
- Texture only in the hero and the closing call: a faded grid, a dot field, one blurred glow.
- Motion reveals content once. `prefers-reduced-motion` turns every animation off and shows the final state.

### Tokens

All tokens live in `app/global.css` on `:root` (light) and `.dark`. Tailwind reads them through `@theme inline`, so `bg-ink-1` and `text-ember-ink` work as classes.

| Token | Use |
| --- | --- |
| `--ink-0` to `--ink-3` | Page, raised surface, sunken surface, hover |
| `--line`, `--line-strong` | Hairlines and stronger borders |
| `--fg`, `--fg-muted`, `--fg-subtle` | Text: primary, secondary, tertiary |
| `--ember` | Accent fills: buttons, the logo's root node, focus ring |
| `--ember-ink` | Accent text, tuned for contrast (4.8:1 on light ink-0) |
| `--ember-soft` | Accent tint for badges and the highlighted layer |
| `--on-ember` | Text on an ember fill |
| `--ok`, `--warn` | Session states and drift markers |
| `--term-bg`, `--term-fg` | Terminal surfaces, dark in both themes |
| `--radius-sm` to `--radius-xl` | 6, 10, 16, 22 px |

Fumadocs reads the same tokens through `--color-fd-*`, so docs and landing share one palette. Info callouts are neutral; the accent stays for the important ones.

### Type

| Class | Use |
| --- | --- |
| `text-display` | Hero headline, fluid with `clamp` |
| `text-h2` | Section titles |
| `text-lead` | Section leads and hero copy |
| Eyebrow (`Eyebrow` component) | Mono, uppercase, ember dot |

Geist Sans for text, Geist Mono for code, terminals and labels.

### Components

| Component | File | What it is |
| --- | --- | --- |
| `Mark`, `Wordmark` | `components/logo.tsx` | Nessie, the tile mark (also `public/icon.svg` and the OG card) |
| `Container`, `SectionHeading`, `Button`, `Reveal`, `SpotlightCard` | `components/landing/ui.tsx` | Landing primitives |
| `HowItWorks` | `components/landing/how.tsx` | Six steps, each with picked lines from a real capture |
| `GuidesSensors` | `components/landing/loop.tsx` | Guides before, sensors after: two cards and a real refusal pointing back to its skill section |
| `TermFrame` | `components/landing/shot.tsx` | Real command output |
| `HerdrHero`, `Capture`, `PhoneAlert`, `TabStrip` | `components/landing/capture.tsx` | The hero window from one round of the run (todo-undo-open); a capture from `captures/` in its captured colours, a few picked lines with the full capture behind "Show full capture" (`lib/ansi.ts`); one alert from the run's log; herdr's tab list in the 14:08:00 frame |
| `Machinery` | `components/landing/bento.tsx` | What is in the box: the six-part bento |
| `Why` | `components/landing/drift.tsx` | The argument, with three drifting harnesses |
| `Proof` | `components/landing/proof.tsx` | Stats and ness's own session tree |
| `withGlyphs` | `components/landing/glyph.tsx` | Draws `✓` and `⎿`, which Geist Mono lacks |
| `ogCard` | `lib/og.tsx` | Social card for `/og.png` and every docs page |

### Accessibility

- Every interactive element shows an ember focus ring on `:focus-visible`.
- A pending capture slot is one image with a text description in `aria-label`.
- Body text meets WCAG AA in both themes.
- Layouts hold from 360 px up with no horizontal scroll.

## Open items

- `captures/*.ansi` are whole-line excerpts of the 2026-10-31 run, cut by `captures/trim.sh` from the raw captures (kept outside the repo; the raw files hold local paths and session links).
- `docs/start/quickstart.md`: install steps wait until they are verified.
