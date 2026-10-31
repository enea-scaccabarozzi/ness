> [!NOTE]
> Archive: the record of the session that prepared the public release. Names of private projects were replaced; the content is otherwise as written.

# Public docs: structure, narrative, site (proposal r1, 2026-10-31)

Built on the agreed understanding (round 2, `understanding.md`).

## 1. The narrative: one arc, read top to bottom
The docs follow a single story that the landing page also tells, in a shorter form:

**Thesis → Story → Machinery → Build your harness → Reference → Proof.**

Every page ends with a "Next" link, so a first-time reader can walk the arc. A returning reader
uses the sidebar and search.

## 2. Docs tree (in the repo, `docs/`)
```
docs/
  index.md                    Introduction: there is no universal harness (N1-N6, one page)
  why/
    philosophy.md             The story: three harnesses, shared machinery that drifted, ness
    principles.md             P1-P7, and what ness is not
    defaults.md               The opinions in the shipped defaults (D), and how to swap them
    limits.md                 Not claimed: scale, platforms, Codex
  start/
    install.md                Prerequisites (Linux, systemd, herdr, Claude Code), mise, binary
    quickstart.md             ness init, your first quick task, your first full session
    first-week.md             Worker install, phone channel, reading status, stats
  concepts/                   The machinery, M1-M6
    sessions.md               Tree, ownership, sizes, promotion of a conversation
    messages-and-worker.md    send/reply/inbox, waking, compaction, limits, stalls
    session-record.md         brief, specs, plan, decisions, report: explainability
    profiles-and-sync.md      Extensibility surface: profiles, slots, local profile, safe sync
    atoms.md                  Seams and their implementations
    features.md               probes, notify, contracts (overview)
    models.md                 The ladder, selectors, enforcement
    guards.md                 Hooks, refusals that teach, the gate, supported-or-refused
  guides/                     Build your harness (how-to)
    local-profile.md          Grow your own harness (today's profiles/authoring.md)
    checks.md                 Give a rule a check
    program.md                Steer a multi-week program
    probes.md                 Ask the running world a question
    notify.md                 Post to Slack, Telegram, GitHub
    contracts.md              Freeze acceptance files
    integration.md            squash, merge, rebase, PR; release-please
    models.md                 Tune the ladder, write a selector
    codex.md                  Codex (experimental)
    doctor.md                 Troubleshoot
    upgrade.md                Upgrading ness in a project (today's manual path)
  reference/
    cli/                      GENERATED from cobra (one page per command), checked in the gate
    config.md                 .ness/ness.toml
    profile-format.md         profile.toml
    probe-format.md           Probe file frontmatter
    builtin-profiles.md       The five built-ins
    files.md                  What ness writes where (file table, sync rules)
    environment.md            NESS_UI, NO_COLOR, ... and exit codes
  built-with-ness/            The proof (public, anonymised, "altered for release" banner)
    index.md                  How ness was built with ness, the field rounds, numbers anonymised
    roadmap.md                Today's backlog.md
  design/                     stays here (path used by tests and links): design and decisions
  spec/                       stays here (testkit checks requirement ids against it)
  sessions/                   stays here (ness writes new sessions here by config)
  superpowers/plans/          stays here: the v1 and field plans
```
Root files: `README.md` (pitch, a 60-second quickstart, links into the arc), `CONTRIBUTING.md`
(dev setup, the gate, test discipline, architecture of `internal/`, releasing), `LICENSE`,
`SECURITY.md`, `CODE_OF_CONDUCT.md`. `docs/discovery/` does not exist in the public history (D30).

The archive folders (`design/`, `spec/`, `sessions/`, `superpowers/`) keep their paths, because
code and tests use them. The site mounts them under "Built with ness".

## 3. One source, no drift
- The docs are plain GitHub Markdown in `docs/`. They read well on GitHub, and the site renders
  the same files. No page is ever copied into the site.
- The only site-specific additions are nav order and titles: frontmatter (`title`,
  `description`) plus one `meta.json` per folder. Callouts use GitHub's `> [!NOTE]` syntax, which
  both renderers understand.
- The CLI reference is generated from the cobra commands and committed. The gate checks it
  (regenerate, then diff), so `--help` and the docs cannot disagree.
- CI builds the site on every PR. A broken link, a page missing from the nav, or a nav entry
  without a page fails the build.
- Leak guard: the existing private-name denylist scan (built-in profiles today) extends to
  `docs/`, `README.md` and `site/`. It makes the rule "docs do not refer to things invisible to the
  outside world" (FF3-D10) a test, not a convention.

## 4. Site
Recommended: **Fumadocs** (Next.js, React, Tailwind) in `site/`, exported as static files.
- The docs half looks and feels like Mintlify: sidebar, search, TOC, code blocks with copy,
  dark/light themes. It reads the repo's `docs/` folder directly as its content source.
- The landing page is a React page in the same app, so shadcn and 21st.dev components drop in
  as they are (both are React + Tailwind + motion).
- Static export, so it hosts free on GitHub Pages (or Vercel or Cloudflare), deployed by an
  Action on main.

Rejected:
- **Mintlify**: hosted and paid at scale, has its own MDX/nav dialect (drift-prone), and no
  free-form React landing.
- **Astro Starlight**: excellent, but the docs look less like Mintlify, and 21st.dev components
  would need React islands.
- **Nextra**: weaker search and theming than Fumadocs.

Landing page, five beats, following the arc:
1. Hero: "There is no universal harness. Build yours." Under it, `ness` in a live terminal
   animation (a conversation promotes itself, children open, a status tree).
2. The drift: three harnesses diverging, then the machinery pulled out.
3. The machinery: M1-M6 as a bento grid.
4. Proof: "ness builds ness", with the anonymised field numbers.
5. Get started: the install command, then into the docs.

## 5. Decisions for the operator
- **License**: reserved (publishing). Recommend **Apache-2.0** (patent grant, common for dev
  tools). MIT is the simpler alternative.
- **Hosting and domain**: GitHub Pages on `<user>.github.io/ness` is free. A custom domain
  costs money and is reserved.
- **Tagline**: "There is no universal harness. Build yours." Alternatives: "The machinery under
  your agent harness." / "Steer the product. ness carries the harness."

## 6. Next steps after this is agreed
1. Prototype the site locally: the landing plus the docs shell with 3 to 4 real pages (index,
   philosophy, quickstart, one concept). Served on this box for you to open, with screenshots
   sent too. We iterate until you like it.
2. Then write the full docs, restructure the archive with release banners and anonymisation,
   generate the CLI reference, add the leak scan, and write the root files.
3. Last: the fresh-history publication (irreversible, asked separately).
