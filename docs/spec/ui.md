> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Terminal output: plain and rich renderers (UI)

Packages: `internal/ui`, `internal/cli`, `cmd/ness`. Source: D41.

Every command has one plain rendering, the tab-separated lines the other specs pin, which
agents, scripts and tests read. A human at a terminal gets a second, rich rendering of the
same values. The rich renderer only draws; it never changes what a command does, writes or
exits with.

## Choosing the renderer

- **R-UI-001** ness MUST choose the renderer once per run, the first rule that applies
  winning: the global `--ui rich|plain` flag; then plain for an agent (`NESS_SESSION`,
  `CLAUDE_CODE_SESSION_ID` or `CLAUDECODE` set), whatever `NESS_UI` says, since agents inherit
  the operator's environment; then `NESS_UI=rich|plain`; then plain when stdout is not a
  terminal, `TERM` is `dumb` or `CI` is set; otherwise rich. `--ui auto` and `NESS_UI=auto` (or
  unset) mean the detection.
  Verify: unit — `ui.Resolve` over every row of the table, including a flag that beats the
  variable and an agent identity that beats `NESS_UI=rich` on a terminal; testscript — an agent's
  `send` prints the plain line under `NESS_UI=rich`.
- **R-UI-002** An unknown `--ui` or `NESS_UI` value MUST be refused as a usage error naming
  the accepted values, except by `ness hook`, which fails open (R-HOOK-016) with the plain
  renderer.
  Verify: testscript — `ness --ui fancy status` and `NESS_UI=fancy ness status` exit 2 and
  stderr lists `auto, rich, plain`; `NESS_UI=fancy ness hook pre-tool` exits 0.
- **R-UI-003** The plain renderer's output MUST be byte-identical to the output the other
  specs pin, except `--help`, which also documents the global `--ui` flag and the commands'
  examples. `ness hook`, `ness post`, `ness contract`, `ness probe` and `--json` MUST print the
  plain rendering, stdout and stderr, under every setting, `--ui rich` included.
  Verify: testscript — every existing script runs with stdout on a pipe (plain);
  `ui_mode.txtar` compares `ness hook session-start`, `ness doctor --json`, `ness inbox --json`
  and a blocking `ness hook pre-tool`'s stderr under `NESS_UI=rich` and `--ui rich` with the
  plain runs.

## Colour, theme, width, accessibility

- **R-UI-010** Inside the rich renderer, colour MUST follow the colour profile of stdout:
  none when `NO_COLOR` is set or stdout is not a terminal (unless `CLICOLOR_FORCE` is set),
  else the terminal's profile; the layout (glyphs, tables, borders) stays.
  Verify: testscript — `NESS_UI=rich` output on a pipe has no escape byte; with
  `CLICOLOR_FORCE=1` it has `\x1b[`; with `NO_COLOR=1 CLICOLOR_FORCE=1` it has no colour
  sequence (bold may stay, as no-color.org allows).
- **R-UI-011** The rich palette MUST come from the terminal background (dark or light);
  `NESS_THEME=dark|light` MUST override the detection, and ness MUST NOT query the terminal
  when it is set or when the renderer is plain. (The libraries' own widgets, rich help and the
  form, may still ask the terminal for its background; their palette follows the setting.)
  Verify: unit — `ui.Resolve` with `NESS_THEME` set never calls the detection function; with
  plain never calls it.
- **R-UI-012** The rich renderer MUST lay out to the terminal width, else `COLUMNS`, else 100
  columns.
  Verify: unit — `ui.Resolve` width precedence.
- **R-UI-013** With `ACCESSIBLE` set, the rich renderer MUST NOT animate (no spinner, no
  full-screen view) and `ness init` MUST prompt one line at a time.
  Verify: unit — the resolved settings disable animation under `ACCESSIBLE`, and `--watch` and
  the spinner run only when they animate (review: `status --watch` checks `Animate`).

## Rich surfaces

- **R-UI-020** Rich help MUST list the same eighteen commands, flags and descriptions as plain
  help, styled.
  Verify: testscript — `NESS_UI=rich ness --help` names every command and `--ui`.
- **R-UI-021** A rich error MUST start with `✗` on stderr instead of `ness:`, keep the same
  message and exit code, and a usage error MUST add a `--help` hint naming the command.
  Verify: testscript — `NESS_UI=rich ness send --bogus` exits 2, stderr matches `✗` and
  `ness send --help`.
- **R-UI-022** Rich `ness doctor` MUST print one line per check with `✓`, `!` or `✗`, the fix
  of a failing or warning check on the line under it, and a final count of ok, warn and fail.
  Verify: testscript — `doctor_rich.txtar` matches the glyph lines, the `fix` line and
  `\d+ ok · \d+ warn · \d+ fail`.
- **R-UI-023** Rich `ness stats` MUST print a bordered table with the same rows and columns as
  plain, the total row last.
  Verify: testscript — `stats_rich.txtar` matches the border characters, each model row and
  the total.
- **R-UI-024** Rich `ness inbox` MUST print a table of the same items; rich `ness inbox --show`
  MUST render the body as Markdown.
  Verify: testscript — `inbox_rich.txtar`: the table rows; a `# Title` body printed without
  the `#`.
- **R-UI-025** Rich `ness sync` MUST mark each changed target with `+` (create), `~` (update),
  `-` (remove) or `!` (conflict), and render release notes as Markdown.
  Verify: testscript — `sync_rich.txtar` matches the glyph lines.
- **R-UI-026** Rich `ness init` on a terminal MUST ask for the missing values through one
  interactive form; plain prompting (R-CFG-051) and `--yes` (R-CFG-024) are unchanged.
  Verify: testscript — `ui_rich.txtar`: a rich init on a terminal with every value given asks
  nothing and succeeds; `init_tty.txtar` (plain prompting) still passes; review — the form is
  built from the same missing-value list.
- **R-UI-027** Rich `ness status` MUST draw the session tree with each session's id, scope,
  tier, phase, context, idle time and waits, the probes, and the worker; a paused probe MUST be
  red.
  Verify: testscript — `status_rich.txtar` matches tree connectors (`├`/`└`) and the fields.
- **R-UI-028** `ness status --watch` MUST show a live view refreshed every 2 s by one refresh loop, with
  keys `↑`/`↓` (select), `a` (closed sessions), `r` (refresh) and `q` (quit); it MUST be
  refused (exit 4) under the plain renderer or `ACCESSIBLE`.
  Verify: unit — the dashboard model over fixed loads: selection moves and clamps, `a`
  reloads with closed sessions, a failed load is shown and kept, a superseded load is dropped
  and schedules nothing; testscript — refused on a pipe.
- **R-UI-029** Rich result lines (send, reply, open, close, integrate, worker) MUST
  start with `✓`; rich `ness integrate` MUST indent the gate's log under a `▸ gate` header.
  Verify: testscript — `ui_results.txtar`.
