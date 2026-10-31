> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Sessions: open, close, status, integrate, environment checks (SESS)

Packages: `internal/session`, `internal/atom/{herdr,claude,codex,merge,squash,pr}`, cli `open`,
`close`, `status`, `integrate`. Sources: §2 (seams), §3 (session tree, release), §4, §5
(Sessions, Integration), §6, §16 (#4, #5, #7, #10, #12), migration hot-fixes F5 and P5
(launch race, never-ran close, global herdr names), D6, D14, D15.

## Environment checks (§4)

- **R-SESS-001** `ness open` MUST refuse when herdr is not installed, printing that herdr is
  required; there MUST be no tmux or plain-terminal fallback.
  Verify: testscript — `herdr` absent from PATH; exit non-zero; no worktree, branch, tab or row.
- **R-SESS-002** `ness open` MUST refuse when the installed herdr is not at least the tested
  version within the same major version, printing both versions.
  Verify: testscript — fake herdr reports an older version; stderr has both.
- **R-SESS-003** `ness open` MUST refuse when the client is not at least the tested version within
  the same major version, printing the found and tested versions.
  Verify: testscript — fake `claude --version` returns an untested version.
- **R-SESS-004** `ness open` MUST refuse when any tier's model id does not resolve, naming the
  tier row.
  Verify: testscript — tier table with an unknown model id; stderr names the tier.
- **R-SESS-005** `ness open` MUST refuse when a global skill has the exact name of a project
  skill in `.agents/skills`, naming both paths.
  Verify: testscript — `$HOME/.claude/skills/deploy/` and `.agents/skills/deploy/`; stderr names
  both directories.
- **R-SESS-006** The shadowing check MUST compare project skill names against
  `~/.claude/skills`, the Claude Code enterprise skills path, `~/.agents/skills` and
  `/etc/codex/skills`.
  Verify: unit — a match in each location refuses; injected filesystem roots.
- **R-SESS-014** Before the shadowing check is implemented, a spike MUST confirm the Claude Code
  enterprise skills path on Linux against the installed client and record it with its source in
  the session's decisions.md; the path MUST be a constant in the claude atom.
  Verify: review — decisions entry with the path and evidence; unit — the constant equals it.
- **R-SESS-015** v1 MUST support Linux only; on another OS every command except `--version` and
  `--help` MUST refuse.
  Verify: unit — injected GOOS `darwin` → refusal exit 4.
- **R-SESS-016** The tested herdr, Claude Code and Codex versions MUST be embedded in the binary.
  Verify: unit — `ness doctor --json` reports the embedded tested versions; changing them
  requires a new build.
- **R-SESS-007** The shadowing check MUST NOT refuse for plugin skills (namespaced) or for
  non-identical names.
  Verify: unit — `deploy` vs `deploy-prod` and a plugin skill `x:deploy` pass.
- **R-SESS-008** Every command MUST run the binary-vs-pin check (R-CFG-011..013) before acting.
  Verify: testscript — see core-config.md.
- **R-SESS-009** When a tier's model becomes unavailable during a run, the session MUST be
  stopped and its manager told; `--fallback-model` MUST be passed to the client only if the
  project config sets a fallback.
  Verify: contract(fake+real) — fake client reports model unavailable; session stopped, item in
  the manager's inbox; launch args contain `--fallback-model` only when configured.
- **R-SESS-010** Every refusal from an environment check MUST happen before any side effect
  (worktree, branch, file, store row, tab).
  Verify: testscript — tree, `git branch`, store dump and fake-herdr tab list unchanged.

## `ness open`: launch

- **R-SESS-020** `ness open` MUST create a worktree at `.worktrees/<scope>-<id>/` on a new branch
  `session/<scope>-<id>`.
  Verify: testscript — `git worktree list` and `git branch` show both.
- **R-SESS-021** A child's branch MUST start from its manager's branch; a root's from `--base REF`
  or the configured base.
  Verify: testscript — `git merge-base --is-ancestor` of the parent head.
- **R-SESS-022** `ness open --brief FILE` MUST copy the brief to the new session's record
  directory, nested under its parent's: a root at `<sessions>/<root>/`, a child at
  `<sessions>/<root>/<child>/`, a grandchild at `<sessions>/<root>/<child>/<grandchild>/`.
  Verify: testscript — open root, child, grandchild; each `brief.md` at its nested path with
  FILE's bytes.
- **R-SESS-023** `--tier` MUST be optional: without it the selector picks the rung (R-MOD-040),
  `--kind NAME` feeding the declared selector; a declared selector with no kind gives base with
  reason `base: no kind`, not a refusal; an unknown `--kind` MUST be refused listing the kinds. An explicit rung above the recommendation MUST be refused
  unless `--reason TEXT` is given; `--tier up|down` MUST need `--resume` and MUST refuse a stored
  rung off the ladder or above the ceiling.
  Verify: testscript — no `--tier` opens base; `--tier frontier --kind implement` refused naming
  `--reason`; `--tier up` without `--resume` refused.
- **R-SESS-024** `--tier` MUST accept only the rungs of the project's ladder (`models.ladder`) up
  to the ceiling; a tier-table row off the ladder is not a launch tier.
  Verify: testscript — `--tier ultra` refused, listing valid tiers.
- **R-SESS-025** The client MUST be started without a prompt argument. Once herdr reports it
  ready for input, the launch MUST submit the one-line prompt
  `You are ness session <ID> (<scope>). Read <path>/brief.md, then load ness-entry.`
  through the runtime's prompt call (`herdr agent prompt --wait`), which returns only once the
  client took it (R-SESS-150). While the agent is `blocked` (a trust
  or approval dialog) the prompt waits; still blocked at `launch.ready_timeout`, the launch
  stalls (R-SESS-033) and `ness open --resume` submits it. A ready mark written before the prompt
  went in (the hook runs at the client's start, or once the dialog is accepted) does not make the
  session ready: status shows its launch phase, the worker leaves it alone, and `--resume`
  continues a live client or relaunches a dead one. (Amended after field run 1: with the
  prompt in argv herdr never saw the client idle and every launch waited out its 120 s timeout.
  Amended by PS-1: herdr returned once the text and Enter were written, and a client still
  starting dropped the Enter, leaving the prompt in its box and the session idle.)
  Verify: contract(fake) — agent start args carry no prompt; one submission to the session's
  pane with exactly that text; a blocked agent gets none and the launch exits 4 as stalled;
  `--resume` after unblocking submits it.
- **R-SESS-026** The brief MUST NOT be typed or pasted into the terminal (launch race fix, F5).
  Verify: contract(fake) — the fake runtime records zero type-text calls during a launch.
- **R-SESS-027** The client MUST be launched with a session id (`--session-id` for Claude Code)
  recorded in the store, binding the transcript.
  Verify: contract(fake+real) — argv contains `--session-id <uuid>`; store row has the same
  uuid; real run: the transcript file name equals it.
- **R-SESS-028** Effort MUST be passed per launch (`--effort` value or the tier's effort), never
  through `CLAUDE_CODE_EFFORT_LEVEL`.
  Verify: contract(fake) — argv contains the effort flag; the launch env has no
  `CLAUDE_CODE_EFFORT_LEVEL`.
- **R-SESS-017** `--effort` MUST accept `low|medium|high|xhigh|max`, validated by the client atom
  (a value the client does not support is refused naming the client).
  Verify: contract(fake) — `--effort max` accepted by claude atom; `--effort huge` → exit 2.
- **R-SESS-029** `--client` MUST select the client atom for this launch, overriding the
  configured client.
  Verify: testscript — `--client codex` with fake codex; the codex fake receives the launch.
- **R-SESS-030** The launch MUST open a herdr tab for the session (titled from `--title` when
  given) in its manager's workspace, the workspace of the manager's tab; a session whose manager
  has no tab (a root) opens in the workspace of the process that runs `ness open`
  (`HERDR_WORKSPACE_ID`), else where herdr puts a new tab. `ness open --resume` does the same.
  (Amended after field run 1: without a workspace herdr used the one the operator had focused.)
  Verify: contract(fake+real) — tab exists with that title; a child of a root whose tab is in
  `w7` opens in `w7`; a root with `HERDR_WORKSPACE_ID=w9` opens in `w9`; neither → no
  `--workspace`.
- **R-SESS-031** Each launch phase MUST be persisted in the store before its step's side effect
  starts.
  Verify: unit — fault-injecting runtime fails at step k; the store shows phase k; no effect of
  step k+1 exists.
- **R-SESS-032** The launch MUST wait for the ready mark the SessionStart hook writes
  (R-HOOK-003) and only then report success.
  Verify: contract(fake) — fake client delays the hook; open does not return before the mark.
- **R-SESS-033** When the ready mark does not arrive within `launch.ready_timeout` (default 60 s),
  the session MUST be shown as *stalled launch* in `ness status` and open MUST exit 4 naming
  `ness open --resume ID`.
  Verify: unit (synctest) — no mark; at 60 s status shows `stalled launch`; with `ready_timeout =
  "120s"` not before 120 s.
- **R-SESS-034** Paths listed in `worktree.link` MUST be symlinked, and paths in `worktree.copy`
  copied, from the primary checkout into the new worktree (gitignored data, §16 #10).
  Verify: testscript — `data/` linked (`readlink` → primary checkout); `.env.example` copied
  (regular file, equal bytes).
- **R-SESS-035** The herdr agent name of a session MUST be `<project>-<scope>`, truncated to
  herdr's name-length limit.
  Verify: contract(fake) — the name passed to herdr equals `infra-lab-sync` for project
  `infra-lab`, scope `sync`; a 60-char scope is truncated to the limit.
- **R-SESS-036** The herdr agent name MUST be unique against herdr's live agent names (all repos),
  not only against this project's store; on collision ness MUST append characters of the session
  id until the name is unique (still within the limit) and MUST never attach to the other agent.
  Verify: contract(fake+real) — fake herdr reports a live agent `fin-harness-hotfixes` from
  another repo; the launch uses `fin-harness-hotfixes-<id chars>`; the other agent receives no
  input.
- **R-SESS-037** A process MUST be treated as session `X` when `NESS_SESSION=X` is set (with
  `NESS_RUN` naming the run), and as the operator when `NESS_SESSION` is unset; the launch MUST
  set both variables for the client.
  Verify: contract(fake) — the client's launch env contains both; testscript — commands without
  the variable act as operator; R-SESS-114 adds the adopted conversation.
- **R-SESS-041** `ness open --root` MUST be refused when run by a session (only the operator opens
  a root).
  Verify: testscript — with `NESS_SESSION` set; exit 4.
- **R-SESS-114** A process without `NESS_SESSION` whose `CLAUDE_CODE_SESSION_ID` equals the client
  session of an open root adopted with `--here` MUST act as that root, in every command and hook;
  `NESS_SESSION` wins when both are set, and the id of a closed adopted root identifies the
  operator. Its transcript MUST be looked up under the worktree's client project directory when it
  is there (a conversation adopted from inside its worktree), else under the primary checkout's.
  Verify: testscript — after adoption, with only the conversation id set: `open --root` refused
  (R-SESS-041), `send --to operator` recorded from the root, `hook session-start` prints the
  root's identity; after the root closes the same id runs as the operator.
- **R-SESS-042** Session ids MUST be `s-` followed by 12 lowercase hex characters.
  Verify: unit — generated ids match `^s-[0-9a-f]{12}$`.
- **R-SESS-043** `--scope` MUST be required and match `^[a-z][a-z0-9-]{0,31}$`; `--title` MUST
  default to the scope.
  Verify: testscript — missing scope → exit 2; `Sync_1` → exit 2; tab title = scope.
- **R-SESS-044** Launch phases MUST keep the current harness phase names (incl. `starting-agent`,
  `sending-brief`), listed as constants frozen by a golden test.
  Verify: unit — golden list of phase names.
- **R-SESS-038** `ness open --root --here` MUST register the running conversation as a root
  without creating a new tab or client process.
  Verify: testscript — fake runtime records no new tab; store has the root bound to the current
  pane and transcript.
- **R-SESS-115** `--here` MUST make the root's branch, worktree, linked and copied files and brief
  exactly as for a launched root (R-SESS-020, R-SESS-022, R-SESS-034), print the worktree path,
  and MUST NOT switch the branch of or write into the primary checkout; its close follows the same
  landed check and teardown, without closing the operator's tab.
  Verify: testscript — the primary keeps its branch and an untracked file, the brief is in the
  worktree's record; an integrated adopted root closes with worktree and branch gone.
- **R-SESS-039** `--here` without `--root` MUST be refused.
  Verify: testscript — exit non-zero.
- **R-SESS-040** `--base REF` MUST be refused without `--root`.
  Verify: testscript — child open with `--base` exits non-zero.
- **R-SESS-152** A new child MUST be refused (exit 4) while `launch.max_children` children are open
  across the project (roots not counted, nor a child parked until a later time, R-MSG-034); unset
  or 0, the cap is half the cores the process may use, at least 1. The refusal names the count, the
  cap and the ways out (wait for a child to land, or raise the cap), and lists the children holding
  the slots, `<id> <scope> (manager <id>) idle <age>`, idle from the transcript's last real work as
  R-SESS-080 reads it (`-` when unknown); it never suggests a subagent instead.
  Verify: testscript — GOMAXPROCS=4: two children open, a third refused, both listed; a root still
  opens; closing one frees its place; a child parked until 2099 frees one and is not listed, one
  parked without a time does not; `max_children = 5` wins over the cores.

## `ness open --resume`

- **R-SESS-045** `ness open --resume ID` on a stalled launch MUST continue from the recorded
  phase without repeating completed steps (no second worktree, branch or tab).
  Verify: unit — stall at each phase in turn; resume; exactly one worktree, branch and tab.
- **R-SESS-046** `--resume` on a stalled launch MUST NOT refuse because the outcome of the last
  step is uncertain; it MUST check the pane once and continue (F5.3, "ambiguous receipt"
  regression).
  Verify: unit — fake runtime returns an ambiguous pane state; resume succeeds.
- **R-SESS-047** `ness open --resume ID` on an exited session MUST relaunch the client in a new
  run, resuming its transcript.
  Verify: contract(fake+real) — client argv contains the resume flag with the stored session id.
- **R-SESS-048** Every new run MUST clear the previous terminal binding and bind to the new pane
  (F5.4 "Terminal binding changed" regression).
  Verify: unit — stored pane id after resume equals the new pane; the old id is not referenced.
- **R-SESS-049** `--resume ID --brief FILE` MUST replace the session's brief.md with FILE.
  Verify: testscript — brief.md bytes equal FILE.
- **R-SESS-050** `--resume` of a running (not exited, not stalled) session MUST be refused.
  Verify: testscript — exit 4; no new tab.
- **R-SESS-051** `--resume ID` MUST accept `--tier` and `--effort` to relaunch the session one
  tier up (or with another effort); the store MUST record the new tier for the new run.
  `--tier up|down` steps the stored rung one place, staying put at the bottom and the ceiling.
  Verify: contract(fake) — `--resume ID --tier frontier` → argv carries the frontier model;
  status shows frontier.

## `ness close`

- **R-SESS-060** `ness close ID` MUST be allowed only to the session's manager (for a child), the
  operator (for a root) or the root itself with `--authorized` (R-SESS-121), and refused otherwise.
  Verify: testscript — sibling closes → refused; root closes child → ok; root closes itself
  without `--authorized` → refused, telling it to ask the operator.
- **R-SESS-121** `ness close <own root id> --authorized WORDS` MUST be accepted when WORDS (at
  least two) appear as consecutive whole words, case-insensitively, in a message the operator
  typed in the session's client transcript: a user line the client marks as typed by a human
  (at the prompt, or queued while the agent was busy),
  never a tool result, a meta or system line, a sidechain line, a text ness types itself (a
  worker hint, `/compact`, `/rename`, the resume text, the launch prompt) or what the client
  records of its own commands and their output. Otherwise, and for a client that keeps no transcript, it MUST be
  refused telling the agent to ask the operator; `--authorized` from a child, or on another
  session's root, MUST be refused (CX-19).
  Verify: testscript — fixture transcript with the words typed → close succeeds; the words only
  in a tool result, a hint or a meta line → exit 4; no transcript → exit 4; a child passing
  `--authorized` → exit 4. Unit — the transcript reader over a fixture `.jsonl` per line kind.
- **R-SESS-122** Closing a root MUST NOT close its tab or its client, launched or adopted: the
  conversation stays readable. The report, obligation, landed and integration checks apply as for
  any close (CX-19).
  Verify: testscript — a launched root closed by the operator: the fake herdr tab is still
  there, the worktree and branch are gone, the row is closed.
- **R-SESS-124** The operator's close of a root MUST be refused while the root's agent is
  working: its tab stays (R-SESS-122), so nothing stops it before the landed check, and a commit
  made before the worktree is removed would be lost. The root closing itself with `--authorized`
  is not refused. The refusal touches nothing.
  Verify: testscript — fake herdr root agent `working`: operator close → exit 4, worktree, branch
  and row unchanged; the root's own `--authorized` close while working → succeeds; agent `idle`
  → the operator's close succeeds.
- **R-SESS-125** Every launch and resume MUST start the client in its skip-permissions mode
  (Claude Code `--dangerously-skip-permissions`, Codex `--dangerously-bypass-approvals-and-sandbox`)
  unless `launch.permissions = "client"`, which passes neither and leaves the client's own
  settings; any other value MUST be refused by config. ness's hooks still run in either mode
  (D43). A client's one-time acceptance of the mode is the operator's, once per machine.
  Verify: testscript — default → the flag on both clients; `client` → no flag; `yolo` → refused.
- **R-SESS-150** The launch MUST confirm that its first prompt was submitted: the prompt call
  waits for the agent's first state change after the submission (`--wait`, every `--until`
  state, a capped `--timeout`). When herdr reports `agent_prompt_stalled` (no change within its
  5 s: the client dropped the Enter), the launch MUST press Enter in the pane
  (`herdr pane send-keys PANE Enter`, not `agent send-keys`, which the client ignored) only while
  the agent is not blocked and the client's input box, read through the client's screen reader,
  starts with the prompt text (an Enter on Claude Code's trust dialog answers "No, exit"); after
  each Enter it waits up to 5 s for the agent to turn `working` or `blocked`, at most 3 times;
  then the launch MUST fail naming the pane, its phase left where `ness open --resume` submits
  again. The text MUST be typed at most once per run (the client may draw it in its box seconds
  after the stall; a second typing appends a copy): a prompt that never shows up stalls the
  launch at `launch.ready_timeout`, and a resume presses Enter on a prompt still in the box
  instead of typing it again. A client without a screen reader fails the launch at the stall. A
  launch MUST NOT end in a session left idle with its prompt unsubmitted.
  Verify: contract(fake) — the fake herdr drops the next N Enters: N=1 → one Enter, one
  submission of exactly the prompt, phase ready; N above the attempts → the launch fails naming
  the pane after 3 Enters, phase short of ready, and a resume presses Enter once, one submission;
  blocked at the stall → no key; text never in the box → one prompt call, no key, stalled; real
  run: four back-to-back launches in one workspace all submit exactly one prompt (field run: two
  of four stuck; first fix run: three of four doubled).
- **R-SESS-151** The worker MUST NOT submit a launch prompt it finds in a box: the launch owns
  its first prompt (R-SESS-150) and the worker never types into or submits a non-empty box
  (R-WRK-014). A session whose prompt is pending is in a launch phase, which the worker leaves
  alone.
  Verify: by R-WRK-014's test (a draft on the prompt line, no input) and R-SESS-025's (a pending
  prompt is not ready).
- **R-SESS-061** `ness close` MUST refuse when `<sessions>/…/report.md` of the session does not
  exist (except R-SESS-066).
  Verify: testscript — no report → exit non-zero naming the expected path.
- **R-SESS-153** `ness close` MUST refuse (exit 4), with `--discard` too, a report without a
  `## Lessons` heading (level 2, outside a fenced block), naming the heading and ending in
  `(skill ness-entry, section Lessons)`. It reads the report from the worktree and, when the
  worktree is gone, from the session's branch (R-SESS-061). One line under the heading, such as
  `None.`, passes; a session that never ran (R-SESS-066) is exempt.
  Verify: unit `CheckClose` over both paths; testscript `close_lessons`.
- **R-SESS-154** Migration 10 MUST add `sessions.size TEXT NOT NULL DEFAULT ''` and leave every
  existing row empty. The first command that opens the store MUST give each unsized root the
  project's default size (`Resolved.DefaultSize`: the last size a profile adds, else `full`), so a
  project running `program` keeps its program checks on its old roots; children keep `''`.
  Verify: unit — a store at version 9 migrated, `FillSizes`; testscript `size_program`.
- **R-SESS-155** A root's size is recorded at launch: `ness open --root --size SIZE` stores it, and
  without `--size` the root takes the project's default size (R-SESS-154). A size a profile adds (`sizes`, R-PROF-085) is accepted
  only when that profile is on. A child's size stays empty.
  Verify: unit `Launch.Size` round trip and `SetSize`; testscripts `size_open`, `size_program`.
- **R-SESS-156** `ness integrate` MUST run the project's setup command and its gate with
  `NESS_BASE` set to the fold base (the commit the branch is folded onto), so a gate can compare
  the branch against it. The base MUST be a commit hash (`^[0-9a-f]{40,64}$`) before it is quoted
  into the shell line.
  Verify: unit `TestIntegrateGivesGateAndSetupTheFoldBase` — setup and gate each print the variable.
- **R-SESS-157** A `quick` root whose branch is integrated (`IntegratedAt` set) is `done`:
  `Shown` returns `done` for its phase, and `session.Done` is true; the worker treats it as done
  only while no turn began after the integration (R-WRK-098). Done changes nothing else: the
  root still closes only with the operator's words (R-SESS-145).
  Verify: unit `session.Done` and `Shown` for quick/full, integrated or not.
- **R-SESS-158** `ness open --root --size X` MUST refuse (exit 2) a size that is not `quick`, `full`
  or one a profile adds, listing the valid ones and ending in `(skill ness-entry, section Work
  sizes)`.
  Verify: testscript `size_open`.
- **R-SESS-159** `ness open` MUST refuse a child under a `quick` root, saying a quick root
  delegates to subagents and naming `ness size full`, ending in `(skill ness-entry, section Quick
  root)`; and MUST refuse `--size` without `--root` (children have no size).
  Verify: testscript `size_open`.
- **R-SESS-160** `ness size SIZE` MUST change the root's size to a valid one; it MUST refuse a
  child ("children have no size"), an operator call without `--session ID`, and `quick` while the
  root has an open child. Only the operator moves a root off a size a profile declared (to `quick`,
  `full` or another profile size); growing from `quick` or `full` is free for the root.
  Verify: testscript `size_cmd`.
- **R-SESS-161** `ness size` without an argument MUST print the current size and the valid sizes;
  after a change it prints the new size and what the size owes. The operator names the root with
  `--session ID`.
  Verify: testscript `size_cmd`.
- **R-SESS-162** `ness status` MUST show a root's size in place of its tier word (`standard full`),
  and `ness status --json` MUST carry `size` on every session (empty for a child).
  Verify: testscripts `size_cmd`, `status_json`.
- **R-SESS-163** A profile check that declares `size = [...]` (R-PROF-084) MUST run only for a root
  of one of those sizes; the size of a child's check, and of a close check whoever closes, is its top root's. A project with a `program`
  root and a `full` root runs the program checks for the first only.
  Verify: testscripts `size_program`, `size_close`.
- **R-SESS-062** `ness close` MUST refuse while the session has open children.
  Verify: testscript — open grandchild → refused naming it.
- **R-SESS-063** `ness close` MUST refuse while the session has open obligations, naming them.
  Verify: testscript — an undisposed request to the session → refused naming the item.
- **R-SESS-064** (CHANGED: a contained branch passes, R-SESS-167) `ness close` MUST refuse when no integration is recorded, unless `--discard`.
  Verify: testscript — report, no integrate → refused; with `--discard` → ok.
- **R-SESS-167** With no integration recorded, `ness close` MUST accept a session whose branch is
  contained in its base (merging it changes nothing in the base's tree: an ancestor, a hand fold, a
  rebase), record the base's sha as its integration and say so; the landed check of a destructive
  close MUST accept the same containment, and only then: with an integration recorded, the
  integrator alone answers. Ceiling: containment is judged against the fork point, so a hand fold
  followed by a commit that reverts part of it, with nothing recorded, reads as contained and the
  revert is dropped. Integrate's "nothing to integrate" and close's "no
  integration recorded" then never block each other. The uncommitted-files refusal MUST name
  `--keep-work REASON`.
  Verify: testscripts `close_contained`, `close_handfold`.
- **R-SESS-065** A successful close MUST remove the worktree and, for a child, the herdr tab
  (a root's stays, R-SESS-122), and MUST mark the session `closed` in the store, keeping its row
  (tier, role, scope, ids, close note) for status `--all` and stats. (This amends design §5, which
  says the store row is removed.)
  Verify: testscript — `git worktree list` lacks it; a child's fake herdr tab gone; store row
  status `closed`.
- **R-SESS-072** Close MUST delete the session's local branch only when its integration is
  recorded, and MUST keep it on `--discard` and `--keep-work`.
  Verify: testscript — integrated → `git branch` lacks it; `--discard` → branch present.
- **R-SESS-066** A session that never ran — no ready mark was ever written — MUST be closable
  without report and integration, and its close MUST succeed when its tab is already gone
  (never-ran close regression, P5).
  Verify: testscript — stall a launch, close the tab out of band, `ness close ID` exits 0 and
  removes the worktree; a session that reached ready is not exempt.
- **R-SESS-067** `--keep-work REASON` MUST keep the worktree and branch and record REASON.
  Verify: testscript — worktree still present; REASON visible in the close record/log.
- **R-SESS-068** `--note TEXT` MUST be recorded with the close.
  Verify: testscript — worker.log / close record contains the note.
- **R-SESS-069** Closing a session MUST retire undelivered items addressed to it (see R-WRK-020).
  Verify: testscript — pending item to the closed session is retired, not left as a zombie.
- **R-SESS-123** Closing a child MUST dispose every open item it sent to its manager as
  `accepted` with the answer `closed with the child`, without a reply item: the manager owes
  nothing for a child it closed, and a handoff needs no reply. Items to anyone else stay open,
  and so do a root's items to the operator. (Added after field run 1, F6.)
  Verify: testscript — a child's open handoff, decision and fyi to its manager → after close the
  manager's inbox and obligations are empty and the items show `accepted`; the child's item to a
  sibling stays open; unit — no reply item is created.
- **R-SESS-130** `ness close ID --withdraw` MUST settle every open item the caller sent the
  session (deferred ones included) as `withdrawn`, with the close note as the reason
  (`withdrawn at close` without one), without a notice item, and close in one step: those items
  do not block the close. (CX-21.)
  Verify: testscript — a manager's open request and deferred decision to a child; close
  `--withdraw --note` → closed, both `withdrawn` with the note.
- **R-SESS-131** Without `--withdraw`, the caller's own open items MUST still block the close
  (R-SESS-063), and the refusal MUST name `--withdraw`.
  Verify: testscript — close without the flag → exit 4 naming the items and `--withdraw`.
- **R-SESS-070** Closing a session that owns active probes MUST transfer their ownership to its
  manager (R-PRB-070).
  Verify: testscript — status shows the manager as owner.
- **R-SESS-071** With the `pr` integrator, close MUST refuse (exit 4) while the recorded PR is not
  merged (§16 #12).
  Verify: contract(fake+real) — fake GitHub reports open PR → exit 4, worktree kept; merged →
  close succeeds.

## `ness status`

- **R-SESS-080** `ness status` MUST print the active session tree (closed sessions excluded) with,
  per session: tier, phase, context size, minutes idle, open obligations and whom they wait on,
  and what the session waits on (R-WRK-099) as `on <kind> <ref>, ...` (a park as `parked` or
  `parked until MM-DD HH:MM`, UTC), nothing when it waits on nothing; `--json` carries them as
  `waits: [{kind, ref, note, until}]`, `until` RFC 3339 or "".
  "Minutes idle" MUST be measured from the last real work entry of the transcript (a `user` or
  `assistant` entry that is not meta and not a compaction summary), never the file's modification
  time, which the client's own bookkeeping entries (away summaries, turn durations, metadata
  rewrites) refresh.
  Verify: testscript — root + two children; each line has those fields, a child's item and a park
  with its until-time on the row and in the JSON; a closed child absent;
  unit — a transcript whose last real entry is hours older than later bookkeeping entries and the
  file mtime reports the real entry's time; a transcript with no timestamped entry falls back to
  the mtime.
- **R-SESS-086** `ness status --all` MUST also list closed sessions, marked closed.
  Verify: testscript — the closed child appears with `closed`.
- **R-SESS-081** `ness status` MUST list active probes with their state and evidence summary.
  Verify: testscript — one GREEN and one paused probe shown with states.
- **R-SESS-082** `ness status` MUST show the worker's heartbeat age and version.
  Verify: testscript — fake heartbeat row; status shows age and version; no worker → shown as
  not running.
- **R-SESS-083** A probe paused for lack of a live owner MUST be shown as red in `ness status`.
  Verify: testscript — `--json` state `paused`; plain output marks it red (ANSI on TTY, unless
  `NO_COLOR`); rich output marks it red (R-UI-027).
- **R-SESS-084** `ness status --json` MUST contain the same information as the text output.
  Verify: testscript — every field of the text tree is present in JSON.
- **R-SESS-085** A stalled launch MUST be shown as `stalled launch`.
  Verify: see R-SESS-033.

## `ness integrate`

- **R-SESS-090** `ness integrate` MUST default `--from` to the calling session's branch and
  `--into` to its manager's branch; a root's default target is the base.
  Verify: testscript — child integrate lands in the root branch; root integrate lands in base.
- **R-SESS-091** `ness integrate` MUST run the configured gate command in a throwaway worktree of
  the folded result before updating the target.
  Verify: testscript — gate writes its cwd to a file; it is a temporary worktree, not the
  session's.
- **R-SESS-103** `ness integrate` MUST refuse when the source session's worktree has uncommitted
  changes.
  Verify: testscript — dirty file in the session worktree → exit 4; target unchanged.
- **R-SESS-104** When the target branch is checked out in the manager's worktree, integrate MUST
  fast-forward it there after the fold, and MUST refuse before folding if that worktree is dirty.
  Verify: testscript — manager worktree HEAD equals the new target sha; dirty manager worktree →
  exit 4, nothing folded.
- **R-SESS-105** `gate` MUST be configured; integrate MUST NOT run without it.
  Verify: testscript — see R-CFG-040 (config without `gate` is refused).
- **R-SESS-092** When the gate fails, integrate MUST leave the target branch unchanged, push
  nothing, record nothing and exit non-zero.
  Verify: testscript — gate `false`; target sha unchanged; no integration row.
- **R-SESS-093** The throwaway worktree MUST be removed after integrate, on success and failure.
  Verify: testscript — `git worktree list` after shows no extra entry.
- **R-SESS-094** The `merge` integrator MUST fold preserving the source commits.
  Verify: contract(fake+real git) — every source commit is an ancestor of the target after.
- **R-SESS-095** The `squash` integrator MUST fold the source as one commit on the target.
  Verify: contract(real git) — target advanced by exactly one commit with the combined diff.
- **R-SESS-118** The `rebase` integrator MUST replay, oldest first, every source commit whose change
  the target lacks onto the target, with no merge commit and without rewriting the source branch;
  "integrated?" MUST compare patches, so the replayed source counts as integrated until it gains a
  new commit.
  Verify: contract(real git) — target gains one commit per source commit, no merge commit; a
  later source commit is not integrated and a second fold replays only it.
- **R-SESS-119** With `integrate.push = true`, integrate MUST fetch the target from `origin` first
  and fast-forward the local target when origin is ahead, so the fold lands on what others pushed;
  a local target that diverged from origin MUST be refused; a failed fetch is logged and the
  integrate goes on (R-SESS-098 then covers the push).
  Verify: testscript — a commit pushed to origin by another clone is under the folded commits and
  the push succeeds; a diverged local main is refused.
- **R-SESS-120** (CHANGED: named `--remote-control`) A Claude Code session ness launches or resumes MUST be started with
  `--name <project>/<title>` (the project from `.ness/ness.toml`, the title of its tab), so the
  client's session list, transcripts and the mobile apps show the same name as the tab, and the
  client never titles it on its own (D42), and with `--remote-control=<project>/<title>` (the same
  name, in one argv element; the bare `--remote-control` when the launch has no name), so its
  transcript records the URL that opens it in the Claude app (R-WRK-082) and the app lists it under
  that name. It goes right after the session id, before `--model`. The argv is
  `--session-id <id>` (or `--resume <id>`), `--remote-control=<name>`, `--model`, `--effort`, then
  `--fallback-model`, `--name` and `--dangerously-skip-permissions` when set. FF3-D12 asked for
  `<project> · <title>`; the launch carries only the `--name` value, so that is the name used.
  Verify: unit — `Args` carries `--name` before the prompt on a first launch and a resume, and
  `--remote-control=infra-lab/backups` third on both (bare without a name); testscript — `open.txtar`
  matches `--session-id <uuid> --remote-control=infra-lab/prog --model … --name infra-lab/prog`,
  `open_resume.txtar` `--resume <uuid> --remote-control=p/prog --model …`.
- **R-SESS-096** The `pr` integrator MUST open (or update) a pull request from source to target
  and record its URL; "integrated?" MUST be true only once it is merged.
  Verify: contract(fake+real) — fake GitHub records the PR; integrated? false then true after
  merge.
- **R-SESS-097** When the fold conflicts, integrate MUST leave the target unchanged and exit
  non-zero naming the conflicting paths.
  Verify: contract(real git) — conflicting change; target sha unchanged.
- **R-SESS-098** integrate MUST push the target only when the project config enables pushing.
  Verify: testscript — bare remote; with push off the remote ref is unchanged; on → updated.
- **R-SESS-099** integrate MUST record the integration commit (or PR) in the store, which close
  reads.
  Verify: testscript — store has the target sha; close then passes R-SESS-064.
- **R-SESS-100** integrate MUST NOT post to channels, start probes or touch environments
  ("nothing else").
  Verify: testscript — fake channel and probe dir untouched.
- **R-SESS-101** `--from REF --into REF` with refs not owned by the caller's session tree (e.g. a
  promotion between long-lived branches, §16 #4) MUST be allowed only to roots and the operator.
  Verify: testscript — root `--from dev --into prod` folds after the gate; a child doing it →
  exit 4.
- **R-SESS-102** Each integrator MUST answer "integrated?" for a source/target pair.
  Verify: contract(fake+real) — before fold false, after true, for merge, squash, rebase and pr.
- **R-SESS-140** With `integrate.mode = "squash"`, `ness integrate` MUST take the subject of the
  squash commit from `--subject`, a Conventional Commit subject (`type(scope)!: summary`, type in
  lower-case letters, scope and `!` optional, a summary that does not start with a space), and
  MUST refuse (exit 4) before folding when it is missing or malformed, naming the expected form
  (D44).
  Verify: testscript — no `--subject` and `--subject 'stuff'` → exit 4, target unchanged;
  `--subject 'feat(cli): x'` lands a commit with that subject.
- **R-SESS-141** The squash commit's body MUST list the subject of every folded commit, oldest
  first, merge commits left out, each between `BEGIN_NESTED_COMMIT` and `END_NESTED_COMMIT` lines
  (release-please reads each as its own change), then the `Ness-Integrates: <source sha>` trailer.
  Verify: contract(real git) — two source commits and a merge of the target into the source → the
  body holds the two subjects in order, the merge's subject absent, and the trailer.
- **R-SESS-142** A source squashed before and integrated again (no longer an ancestor of the
  target) MUST fold only what it gained since its newest squash on the target: the merge base is
  the source sha that squash's trailer names, the body lists only the commits after it, and a
  landed change the target modified since MUST NOT conflict.
  Verify: contract(real git) — squash, change a landed line on the target, add a source commit,
  fold again → no conflict, the target keeps its change, the body lists only the new commit.
- **R-SESS-143** A squash that changes nothing in the target's tree MUST be refused as nothing
  to integrate (exit 4), creating no commit.
  Verify: contract(real git) — a source that only merged the target after its squash →
  `ErrNothingToIntegrate`.
- **R-SESS-145** integrate MUST run at most one gate at a time per project: a gate that finds
  another integrate's gate running MUST say so in its log and wait for it, not fail.
  Verify: unit — with the project's gate lock held, Integrate logs that it waits and runs no gate;
  once the lock is released the gate runs and the target moves.
- **R-SESS-146** When `integrate.setup` is set, `ness integrate` MUST run it with `sh -c` in the
  throwaway worktree, after the fold and inside the gate lock, immediately before the gate; a
  non-zero exit MUST fail the integrate like a failing gate (`ErrGate`, target unmoved, throwaway
  removed) without running the gate; its output goes to the gate log. Unset, nothing runs.
  Verify: unit and testscript.
- **R-SESS-164** Two integrates into one target MUST queue, not refuse: the second logs
  `ness: waiting for <holder>'s integrate into <target>` (the holder is the session id, or the
  operator) and waits for the per-target lock.
  Verify: unit — with the target lock held, Integrate logs the holder and runs no gate until it
  is released, then lands.
- **R-SESS-165** When the target moved while the gate ran, integrate MUST fold again onto the
  moved target, under the target lock, instead of failing the fast-forward. When the new fold
  differs from the gated one only under `sessions` (the session records) it lands without a
  second gate; otherwise the gate runs once more, on the new fold. A fold that conflicts, or a
  second move outside the records, MUST fail with a plain message naming the target and the next
  step (`ErrTargetMoved`, exit 1), never git's own output. A gate that reads the records (a link
  check over `docs/sessions`) is not re-run for a record-only move.
  Verify: unit — record-only move: one gate run, both changes land; code move: two runs, the
  second sees the moved target as `NESS_BASE`; conflicting move and a second code move:
  `ErrTargetMoved`, target at the moved sha.
- **R-SESS-166** The push after a fold MUST carry SSH keepalives (`core.sshCommand` with
  `ServerAliveInterval`, unless the project set `core.sshCommand` or the environment sets
  `GIT_SSH` or `GIT_SSH_COMMAND`: the pair would beat a `GIT_SSH` wrapper) and MUST be retried once
  when it fails on a dropped connection, judged by git's and ssh's own lines (a failing pre-push
  hook that prints "connection reset" mid-line is not re-run). A project's pre-push hook
  that re-runs what the gate just passed holds the connection idle for its whole run; skipping it
  for ness's push is the project's call (ness never passes `--no-verify`).
  Verify: unit — a first push failing "closed by remote host" is retried and lands; a rejected
  push is not retried; a project `core.sshCommand` is left alone.
- **R-SESS-144** `--subject` with any mode other than `squash` MUST be ignored with a warning on
  stderr (`--subject ignored: integrate.mode merge writes its own message`) and the integrate MUST
  proceed (merge, rebase and pr write their own messages).
  Verify: testscript — `integrate.mode = "merge"` with `--subject` → exit 0, the warning, the
  target moved, the merge commit's message is not the subject.

## Runtime and client contracts

- **R-SESS-110** The runtime atom contract (open tab, run command, type text, read screen,
  idle/working, focused) MUST be satisfied by both the herdr atom and the test fake.
  Verify: contract(fake+real) — one suite, both implementations.
- **R-SESS-111** The client atom contract (launch args, hooks wiring, transcript → context size
  and usage, limit/ready notices) MUST be satisfied by the Claude Code atom and its fake.
  Verify: contract(fake+real).
- **R-SESS-112** 20 parallel real launches MUST all reach ready with none stuck (L6 exit).
  Verify: e2e(real client) — 20 concurrent `ness open` economy-tier trivial briefs; 20 ready
  marks; zero stalled.
- **R-SESS-113** The Codex atom MUST pass its trust/update flags at launch so no interactive
  dialog blocks the launch.
  Verify: contract(fake+real) — argv contains the flags; real run reaches ready without input.
