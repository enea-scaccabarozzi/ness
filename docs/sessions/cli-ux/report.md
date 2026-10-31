> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# cli-ux report

## Delivered

1. **Rich renderer** (`f96e9ee`, review fixes `066b5b1`; spec `docs/spec/ui.md`, D41). A person
   at a terminal gets styled help with examples (fang), `✗` errors with a help hint, doctor
   with glyphs, fixes and counts behind a spinner, stats and inbox tables, Markdown for
   `inbox --show` and release notes, sync glyphs and a colored dry-run diff, an interactive
   `ness init` form, the status tree with a detail per session, `ness status --watch` (a live
   full-screen view), `✓` result lines and the integrate gate log under a header. Plain output
   is byte-identical (every existing script passes); agents and the agent-facing commands
   (hook, post, contract, probe, `--json`) are plain under every setting. Controls: `--ui`,
   `NESS_UI`, `NO_COLOR`, `CLICOLOR_FORCE`, `NESS_THEME`, `ACCESSIBLE`.
2. **Promoted roots' context and stats** (`39c3825`, reworked in `066b5b1`): read from the
   transcript under the primary checkout, where the conversation started.
3. **Session names** (`a0ea21f`, D42, R-SESS-120, R-WRK-071): Claude Code sessions are launched and
   resumed with `--name <project>/<tab title>`; a promoted root is renamed by the worker typing
   `/rename <project>/<title>` once, when its pane is idle and free.

## Evidence

- `make validate` exits 0 at `a0ea21f` (build, both lints, race and repeated tests, audit,
  govulncheck after raising goldmark to v1.8.6, `sync --check`).
- New tests: `internal/ui` (mode table, theme, width, accessibility, NO_COLOR, the dashboard's
  selection, toggle, failed load, single refresh loop), `ui_mode.txtar`, `ui_rich.txtar`,
  `status_adopted.txtar` (fails without the fix), `TestArgsName`, `TestRenamesAnAdoptedRootOnceWhenFree`.
- Live, in herdr: `status --watch` against the infrastructure lab tree (select, toggle, quit); the init form
  filled and submitted; `claude -p --name` writes `custom-title` and `agent-name` and no
  `ai-title`; an interactive session given `/rename nametest/live-rename` showed "Session renamed
  to" and kept the name across a later turn with no new generated title.
- A fresh reviewer (opus) found one crash (an empty init form) and five important issues; all
  fixed or ruled in decisions CX-11 to CX-13.

## Not done

- Codex sessions get no client name: Codex has no session-name flag.
- The installed binary and the infrastructure lab worker still run the previous build until the operator
  installs it and restarts the worker (mutating systemctl is the operator's).

## Lessons

- A testscript's stdout is a pipe, so a TTY-only path (the init form) needs a forced renderer and
  a forced TTY to be exercised at all; the crash hid exactly there.
- Variables the operator puts in their shell profile reach every agent they start: an identity
  check has to beat the variable, not follow it.
- Claude Code keeps a conversation's transcript under the directory it started in; anything that
  moves a running conversation to a worktree has to remember where it began.
