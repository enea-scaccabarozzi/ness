> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Profile-declared checks (CHK)

Packages: `internal/profile` (format), `internal/check` (runner). A profile may declare checks
that run when a session opens, integrates or closes; a failing check refuses the command with the
declared message and a `(skill NAME, section HEADING)` pointer.

The declaration format is specified in [profiles.md](profiles.md) as R-PROF-045..047. The runner's
requirements (R-CHK-nnn: events, kinds, context, output and exit codes) are written with the
runner.

## Events and where they run

- **R-CHK-001** Checks MUST run at exactly three points and only the checks whose `on` names the
  event: `open` in the launch plan after the manager and `--here` checks and before any write,
  `integrate` before the throwaway worktree and the gate lock, `close` in the close preconditions
  before any state change. A check declared for another event MUST NOT run.
  Verify: unit — `Run` with a check for each event; testscripts `check_open`, `check_integrate`,
  `check_close` (a check of another event, whose command would fail, never refuses).
- **R-CHK-002** `ness open --resume` MUST NOT run checks: the child already passed them at its
  first open.
  Verify: testscript `check_open` — a resume with a failing check opens; the check's log is unchanged.
- **R-CHK-003** `for` filters on the session the command is about: at open the session being
  opened (a child unless the operator opens it with `--root`), at integrate the integrating
  session, at close the session being closed. `for = "child"` MUST skip a root and the operator's
  `open --root`; `for = "root"` MUST skip a child; no `for` runs for both.
  Verify: unit — `Run` with `Context.Child` true and false; testscripts `check_open`, `check_close`.

## Context

- **R-CHK-004** A `run` check MUST be handed the context as environment: `NESS_EVENT` (`open`,
  `integrate` or `close`), `NESS_SESSION` (the acting session, empty for the operator),
  `NESS_ROOT` (the top ancestor of the acting session, empty for the operator), `NESS_WORKTREE`
  (absolute: the opener's checkout at open, the integrating session's at integrate, the closed
  session's at close), `NESS_PRIMARY`; at open also `NESS_SCOPE`; at integrate and close also
  `NESS_BRANCH`, `NESS_INTO` and `NESS_RECORD`. A variable that does not apply to the event MUST
  NOT be set; one that applies but is empty MUST be set empty. The command's working directory is
  `NESS_WORKTREE`, and `sh -c` runs it through the `shell.Runner` given to `check.Run`.
  Close checks are skipped, and the close output says so, when the closed session's worktree no
  longer exists: there is nothing to examine.
  Verify: unit — the `shell.Cmd` the fake runner receives, per event; testscripts log the
  variables; testscript `check_close_gone`.
- **R-CHK-005** The kinds MUST read their files below `Context.Worktree` and run git there; they
  MUST read the `Facts` for the root in `Context.Root`.
  `section-matches` MUST read headings by the same rules as `rule.Headings` (up to three leading
  spaces, trailing `#`s kept, CRLF trimmed), as does the Lessons check (R-SESS-153).
  Verify: unit — kinds over a fixture worktree; unit `TestOneHeadingParser`.

## Results

- **R-CHK-006** A `run` check passes on exit 0. Any other exit, a timeout (`Result.TimedOut`), and
  a command that cannot start (the runner returns an error) MUST fail the check (fail closed). The
  timeout is the declared `timeout`, 30 seconds when undeclared, passed as `Cmd.Timeout`.
  Verify: unit — exits 0 and 3, `TimedOut`, a runner error, the `Cmd.Timeout` for both cases.
- **R-CHK-007** A kind MUST fail, never pass, when it cannot decide: a file that does not exist or
  cannot be read, a heading that is not found, a pattern that matches nothing where a match is
  needed, a git or `Facts` error, a kind core does not know.
  Verify: unit — each of these per kind.
- **R-CHK-008** All checks of the event MUST run, in order (profile order, local last), and every
  failure MUST be returned, in that order, as a `Failure` with the check's id, message, pointer and
  a detail. The detail of a `run` check is its combined output trimmed of surrounding whitespace
  and cut to at most 1500 bytes; with no output it names the exit status (`exit 3`), the timeout
  (`timed out`) or the start error. The detail of a kind names what failed: the file, the heading or `require` pattern not found, the
  offending added line, or the two values that differ.
  Verify: unit — two failing and one passing check give two failures in order; output of 3000 bytes.
- **R-CHK-009** `check.Refusal(event, failures)` MUST render each failure as two lines, and the
  command MUST refuse with them (exit 4):

  ```
  ness open refused: <message> (skill ness-program, section Autonomy)
    program/autonomy-recorded: <detail>
  ```

  A check with no `message` (a `run` check may omit it) renders `<id> failed` in its place. The
  pointer is appended by core; a script never prints its own. No failures render the empty string.
  Verify: unit — `Refusal` over one and two failures; testscripts assert stderr and exit 4.
- **R-CHK-010** A refusing check MUST leave nothing written: at open no session row, worktree,
  branch, tab or record directory; at integrate no throwaway worktree, no gate run and no change
  to the target branch or the store; at close the session, tab, worktree and branch as they were.
  Verify: testscripts `check_open`, `check_integrate`, `check_close`.

## Kinds

- **R-CHK-011** `section-matches` (`file`, `heading`, `require`) MUST pass when the body of the
  heading named `heading` (any level) matches every `require` regexp. The body ends at the next
  heading of equal or higher level, so deeper headings belong to it. A line inside a fenced code
  block (``` or ~~~) is neither a heading nor a boundary. When the heading occurs more than once the
  first one counts.
  Verify: unit — pass, a require that only matches in the next section, a deeper subsection, a
  fenced heading, no heading, no file.
- **R-CHK-012** `branch-forbids` (`path`, `pattern`) MUST read the unified diff of one
  `git diff <into>...<branch>` run in the worktree and fail when a line added in a file whose path
  matches the `path` regexp matches `pattern`, tested on the line without its leading `+`. In
  `path`, `{record}` expands to the quoted `NESS_RECORD`. Removed lines, context lines and added
  lines in other paths never count. The diff MUST NOT depend on the user's git configuration: git
  is run with `-c core.quotePath=false` and `diff --no-color --no-ext-diff --no-textconv
  --src-prefix=a/ --dst-prefix=b/`.
  Verify: unit — an added line, a removed line, another path, a clean diff, a git error; unit with
  a real repository whose config sets `color.ui=always` and `diff.external` and whose record
  path is non-ASCII; testscript `check_integrate` with real git.
- **R-CHK-013** `match-last` (`file`, `pattern`, `against_file`, `against_pattern`) MUST pass when
  group 1 of the first match of `pattern` in `file` equals group 1 of the last match of
  `against_pattern` in `against_file`, and fail when either has no match or either file is missing.
  Verify: unit — equal, unequal, no match in each file, a missing file.
- **R-CHK-014** `checkpoint-since-landing` MUST pass when `Facts.LastLanding(root)` is zero or
  `Facts.LastCheckpoint(root)` is not before it; equal timestamps pass. It MUST fail when a landing
  exists and the latest checkpoint is older or absent. It MUST pass, without reading the facts,
  when `Context.Root` is empty (the operator acts) and when the acting session is not a root
  (`Context.Session` differs from `Context.Root`): only a root can send a checkpoint.
  Verify: unit — no landing, checkpoint before, after and equal, no checkpoint, a `Facts` error, a
  non-root session, an empty root.
- **R-CHK-015** `check.Run` MUST skip a check whose `Size` list is non-empty and does not contain
  `Context.Size`; a check with no `Size` runs for every size, including an empty `Context.Size`
  only when its list is empty.
  Verify: unit `TestRunFiltersOnTheRootsSize`.
