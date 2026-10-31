> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# cli-ux plan

Each step is TDD and its own commit; the gate (`make validate`) passes at every commit.

1. **Spec and decision.** `docs/spec/ui.md` (area UI) with the mode table, colour, theme,
   accessibility, and one requirement per rich surface; amend R-CFG-002 (allowed variables) and
   R-SESS-083 (paused probe red in both renderers); D41 in decisions.md.
2. **`internal/ui` foundation.** `ui.Resolve(Term, flag) Settings` (pure, table-tested), the
   theme (styles from a dark/light flag and a colour profile), glyphs, `Writer` wrapping with
   colorprofile. `cmd/ness` gathers the terminal facts (`Deps.Term`); the global `--ui` flag;
   `paint` and the package-level `isTerminal` var go (no package mutable state).
3. **Help, version, errors** through fang in rich mode; plain keeps cobra's text.
4. **doctor, stats, inbox, sync, init** rich renderers (`ui.Table`, `ui.Markdown`,
   `ui.Form`), each with a testscript under `NESS_UI=rich` that fails on the plain lines.
5. **status** rich tree and `status --watch` dashboard (`ui.Dashboard`, a bubbletea model
   tested through Update/View with fixed data).
6. **Result lines and integrate** (`✓` prefix, gate log indentation).
7. **Docs**: README section "Terminal output", CHANGELOG entry; report.
