> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# cli-ux: what this session builds

A rich terminal layer for the operator, on top of the plain output that agents, scripts and
tests read. The plain output does not change by a byte; the rich layer is a second renderer
chosen per run.

## The one rule

Two renderers, one data path. Every command builds the same values it builds today (the
`--json` structs); `plain` prints them as today's tab lines, `rich` draws them. Nothing an
agent reads can change, because agents never get the rich renderer:

| Input (first that applies wins) | Mode |
|---|---|
| `ness hook`, `post`, `contract`, `probe`, or `--json` | plain, always |
| `--ui rich\|plain` (global flag) | as given |
| an agent identity (`NESS_SESSION`, `CLAUDE_CODE_SESSION_ID`, `CLAUDECODE`) | plain |
| `NESS_UI=rich\|plain` | as given |
| stdout not a terminal, `TERM=dumb`, `CI` set | plain |
| otherwise | rich |

Inside rich, colour follows the terminal: `NO_COLOR` keeps the layout without colour,
`CLICOLOR_FORCE` forces colour on a pipe (for `--ui rich | less -R`), and the palette adapts
to a dark or light background (`NESS_THEME=dark|light` overrides the detection, which
multiplexers often swallow). `ACCESSIBLE=1` swaps animated widgets for line prompts and
plain progress lines (screen readers). Width comes from the terminal, else `COLUMNS`, else 100.

No per-user config file: R-CFG-001 keeps ness free of user config, and an environment
variable in the operator's shell profile covers every machine-level preference.

## What the operator sees (rich)

- **Help, version, errors**: styled help (fang) with examples, coloured flags and
  sections; `ness: …` becomes a red `✗` line with a "try `ness X --help`" hint on usage errors.
- **status**: a header (project, base, worker pill), the session tree drawn as a tree with
  phase badges, a context gauge and waits; probes as a table; closed sessions dimmed.
- **status --watch**: a live full-screen dashboard (bubbletea), refreshing every 2 s; `↑/↓`
  select a session, the detail pane shows branch, worktree, client, obligations; `a` toggles
  closed sessions, `r` refreshes, `q` quits. Refused off a terminal.
- **doctor**: a spinner while the checks run, then one `✓ ! ✗` line per check, the fix under a
  failing one, and a summary (`14 ok · 1 warn · 0 fail`).
- **stats**: a bordered table, numbers right-aligned and humanised, the total row bold.
- **inbox**: a table with intent badges; `inbox --show` renders the body as Markdown (glamour).
- **sync**: one glyph per action (`+` create, `~` update, `-` remove, `!` conflict), the dry-run
  diff coloured, release notes rendered as Markdown.
- **init**: an interactive form (huh): project, base, gate, client, profiles to pick from.
- **integrate**: the gate's log indented under a `▸ gate CMD` header, and the elapsed time.
- **send, reply, open, close, worker, probe, integrate** result lines: a green `✓` prefix.

## Not in scope (said once)
Shell completions, man pages (fang's are off: the command surface is pinned at sixteen), ASCII
glyph fallback, a project-level `[ui]` table.
