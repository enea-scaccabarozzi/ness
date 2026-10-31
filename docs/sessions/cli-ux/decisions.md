> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# cli-ux decisions

- **CX-1** Two renderers, one data path: the plain one stays byte-identical, the rich one only
  draws. Agents, scripts and the 96 existing testscripts read plain; changing it would break
  every one of them for no reader's gain.
- **CX-2** The renderer is resolved once in `Main`, before cobra parses: help and flag errors
  render before any command runs. `--ui` is still a registered flag, so cobra documents and
  accepts it.
- **CX-3** No per-user config file (R-CFG-001 forbids one); preferences are environment
  variables (`NESS_UI`, `NESS_THEME`, `NO_COLOR`, `CLICOLOR_FORCE`, `ACCESSIBLE`), listed in
  R-CFG-002.
- **CX-4** An agent identity (`NESS_SESSION`, `CLAUDE_CODE_SESSION_ID`, `CLAUDECODE`) forces
  plain even over `NESS_UI` (agents inherit the operator's shell profile); only `--ui` beats it.
  Codex exports no stable identity variable; its commands run on pipes. (Amended after review.)
- **CX-5** `NO_COLOR` beats `CLICOLOR_FORCE` (colorprofile lets the force win); bold stays, as
  no-color.org allows.
- **CX-6** A bad `NESS_UI` never blocks `ness hook`: hooks fail open (R-HOOK-016), and an exit 2
  from pre-tool would block every tool call.
- **CX-7** The context colour (green < 100k, amber < 160k, red) is a fixed cue, not a gauge: no
  model window is known to ness, and a bar against a guessed window would mislead.
- **CX-8** Charm stack: lipgloss (styles, tables), huh (form, spinner), glamour (Markdown),
  bubbletea (live view), fang (help). goldmark is raised to v1.8.6 for GO-2026-5320.
- **CX-9** fang's `completion` and `man` commands stay off: R-CFG/R-QA pin the sixteen-command
  surface, and the hidden `help` command is hidden in both renderers.
- **CX-10** Found while testing: a promoted root's transcript lives where the conversation
  started, so status and stats saw none. Fixed in this session (separate commit), since the
  operator's the infrastructure lab roots are all promoted ones.
- **CX-11** Review fixes: an init whose values all came from flags skips the form (it panicked
  on an empty group); hook, post, contract and probe are plain under every setting, stderr
  included (the block reason a model reads); `status --watch` keeps one refresh loop (each key
  press added one); a spinner interrupt exits 1 with a message and never runs the checks twice;
  no success line after a failed init or sync; fang's palette follows `NESS_THEME`.
- **CX-12** The transcript fix moved from the Claude atom (a global search by id, which also
  changed `open --resume` of an adopted root) to status and stats only: an adopted root's
  transcript is under the primary checkout. Resume is unchanged.
- **CX-13** Accepted, not fixed: plain `--help` gains the `--ui` flag and examples (help is
  documentation; R-UI-003 says so); the libraries may query the terminal background in their
  own widgets; stderr shares stdout's color profile; the dashboard selection follows the row
  index; diffs show tabs as spaces; cobra's own argument errors get no help hint.
- **CX-14** Field run 1 (the infrastructure lab, `infra-lab-feedback.md`), operator rulings 2026-10-10: fixes
  F1-F6 go ahead; the Ness repository runs its own worker (`ness-Ness.service`, installed); the
  update path waits for the public repository (`docs/backlog.md`).
- **CX-15** Delivery holds only on the input box: a draft, or a draft changed within the quiet
  window. Focus and scrolling never hold a delivery (the operator may be reading an idle pane
  that must still be woken). Amends R-WRK-017.
- **CX-16** The operator is told they are needed, not what about: a short notification
  ("infra-lab/backups needs you: <first line>") through a swappable notify channel (Telegram
  first), sent when a session is idle and waiting on the operator, or blocked on a prompt. The
  full context lives in that session's chat and its operator item, written for an operator who
  has read nothing else (question, the facts needed to answer, options with a recommendation,
  what happens meanwhile). The operator answers in that session's tab.
- **CX-17** Any session may ask the operator directly, but only for what its manager cannot
  settle: a decision reserved to the operator, a fact only the operator has, an action only the
  operator can take. Its manager gets a copy. Everything else goes to the manager, which decides
  or answers "operator's call" (and the child then asks). Amends R-MSG-017.
- **CX-18** Autonomy is set per brief from the operator's words: plan first, report then act, or
  act within the brief. A root never lowers it for a child. When the operator approves a plan in
  advance (several children, or more than one session), its checkpoints are skipped; the
  operator is needed again only outside that plan's scope.
- **CX-19** A root may close itself, but only with the operator's explicit authorization, which
  ness checks against the operator's own messages in the transcript (`--authorized "<their
  words>"`). Closing a root never closes its tab or its client: the conversation stays readable.
- **CX-20** Sessions ness launches or resumes skip the client's permission prompts by default
  (Claude Code `--dangerously-skip-permissions`, Codex's bypass flag); one project key restores
  the client's own settings. ness's own hooks still apply. Sessions already running stay as
  they are. Given to `delivery` (m-64kuxw2f).
- **CX-21** A manager withdraws its own open request or decision: the sender may settle an item
  it sent as `withdrawn`, with a reason the recipient sees; `ness close --withdraw` settles what
  the manager sent that child and closes it in one step. By default an open request still blocks
  a close (option D of the withdraw analysis). Given to `withdraw`.
- **CX-22** Secrets in configuration get a designed, extensible mechanism instead of
  `token_env` alone; a value that is not secret (a Telegram `chat_id`) may be written inline or
  come from the same secret source. Design first, the operator approves. Given to `secrets`.
- **CX-23** ness integrates by squash merge, friendly to release-please: the commit that reaches
  the base carries a conventional subject. Plus the fixes from the harness review: CI
  commit-msg allowlist, the `FuzzTOMLKeys` failure, a lighter pre-push, `-count=4` only in CI.
  Given to `integrate-fixes`.
- **CX-24** The composability task is only N1 (Claude-specific wiring behind the client seam)
  and N3 (one list per set of names). N4 (`extending.md`) goes to the docs cleanup once the
  field feedback settles; N5 and N6 wait for the operator after an explanation. Given to `seams`.
