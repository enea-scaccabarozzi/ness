> [!NOTE]
> Archive: the record of the session that prepared the public release. Names of private projects were replaced; the content is otherwise as written.

# Plan: public-release

1. Ground: field usage in the three projects (subagent, private findings in the scratchpad); run the quickstart and the commands by hand in a scratch repo.
2. Prototype: the Fumadocs site in site/ (landing plus docs shell, real pages: index, philosophy, quickstart, one concept), served locally; iterate with the operator until approved.
3. Write: the full docs tree, the CLI reference generator plus its gate check, the leak scan over docs/README/site, the archive banners and anonymisation, the root files (README, CONTRIBUTING, LICENSE Apache-2.0, SECURITY, CODE_OF_CONDUCT), and the site CI and Pages deploy.
4. Verify: every command in the docs re-run from a clean scratch repo; review.
5. Publish: fresh history to enea-scaccabarozzi/ness (irreversible; the operator's explicit go).

## Step 3 split (2026-10-31, after landing approval PR-D9)
Five parallel subagents, disjoint files, each with its own verification log `verification-<lane>.md` (merged into verification.md at step 4):
- **go**: CLI reference generator (cobra → docs/reference/cli/), gate diff check, leak denylist scan extended to docs/, README.md, site/.
- **start**: docs/start/ (install, quickstart, first-week), why/defaults.md, why/limits.md; every command run in a scratch repo.
- **concepts**: the remaining concept pages (messages-and-worker, session-record, profiles-and-sync, atoms, features, models).
- **guides**: docs/guides/* and docs/reference/ non-CLI pages, built-with-ness/index.md and roadmap.md.
- **release**: root files, site CI and Pages workflow, archive banners and the anonymisation pass on design/, spec/, sessions/, superpowers/.
