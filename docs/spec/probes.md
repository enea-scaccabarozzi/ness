> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Probes (PRB)

Package: `internal/probe` (parser, floor runner, states, redaction); scheduling in
`internal/worker`; cli `probe start|run|verdict|stop`. Sources: §2 (features), §4 (probe owner),
§5, §8, §16 (#5, #6), D9, D10, D11, D12, D24.

## File format

- **R-PRB-001** A probe MUST be one Markdown file with YAML frontmatter; the body after the
  frontmatter is the scope.
  Verify: unit — the §8 example parses; scope equals the body text.
- **R-PRB-002** The parser MUST accept the fields `every`, `jitter`, `once`, `window`, `floor`,
  `expect`, `confirm`, `timeout`, `wake`, `sample`, `notify`, `channel`.
  Verify: unit — a file with every field reads each back.
- **R-PRB-003** An unknown frontmatter field MUST be refused, naming file and field.
  Verify: unit — `evry: 30m` → error naming `evry`.
- **R-PRB-004** `wake` MUST accept only `never | change | red | not-green | always`.
  Verify: unit — each accepted; `sometimes` refused.
- **R-PRB-005** `notify` MUST accept only `none | auto | judged`.
  Verify: unit — each accepted; others refused.
- **R-PRB-006** `sample` MUST be a number in [0, 1].
  Verify: unit — `0.1` ok; `1.5`, `-0.1` refused.
- **R-PRB-007** `confirm` MUST be an integer ≥ 1.
  Verify: unit — `0` refused.
- **R-PRB-008** A probe with neither `every` nor `once` MUST be refused.
  Verify: unit — error naming the file.
- **R-PRB-009** The probe parser MUST NOT panic on any input.
  Verify: fuzz — arbitrary bytes seeded with the §8 example.
- **R-PRB-010** `notify: auto` or `judged` without `channel` MUST be refused.
  Verify: unit — error naming `channel`.

## Floor states

- **R-PRB-020** A floor exiting 0 MUST yield GREEN (subject to R-PRB-023).
  Verify: unit — floor `exit 0` → GREEN.
- **R-PRB-021** A floor exiting 1 MUST yield RED.
  Verify: unit — `exit 1` → RED.
- **R-PRB-022** A floor exiting with any other code, or exceeding `timeout`, MUST yield UNCLEAR,
  and a timed-out floor process MUST be killed.
  Verify: unit — `exit 2` → UNCLEAR; `sleep` past timeout → UNCLEAR and the process is gone.
- **R-PRB-023** When `expect` is set and the `key=value` pairs on the floor's last output line do
  not satisfy it, the state MUST be VACUOUS, never GREEN (e.g. `expect: n>=1` with `n=0`).
  Verify: unit — exit 0 + last line `n=0` → VACUOUS; `n=3` → GREEN; no `n` → VACUOUS.
- **R-PRB-025** `expect` MUST be a comma-separated list of `key op number` conditions with `op` in
  `>= <= > < == !=`, all of which must hold; any other form MUST be refused at parse time.
  Verify: unit — `n>=1, lag<50` with `n=2 lag=10` → GREEN, `lag=60` → VACUOUS; `n=>1` refused.
- **R-PRB-024** The `key=value` and `expect` parsers MUST NOT panic on any input.
  Verify: fuzz — arbitrary last lines and expressions.

## Waking

- **R-PRB-030** A floorless probe MUST wake its owner on every scheduled run.
  Verify: unit (synctest) — `every: 30m`, no floor → one wake per 30 min.
- **R-PRB-031** `wake: never` MUST never wake (sample aside, R-PRB-036).
  Verify: unit — RED floor; no wake with `sample: 0`.
- **R-PRB-032** `wake: change` MUST wake when the state differs from the previous run's.
  Verify: unit — GREEN, GREEN, RED → one wake at the third run.
- **R-PRB-033** `wake: red` MUST wake only on RED.
  Verify: unit — UNCLEAR and VACUOUS do not wake; RED does.
- **R-PRB-034** `wake: not-green` MUST wake on RED, UNCLEAR and VACUOUS.
  Verify: unit — each state wakes.
- **R-PRB-035** `wake: always` MUST wake on every run.
  Verify: unit — GREEN wakes.
- **R-PRB-036** `sample: p` MUST also wake on a fraction p of GREEN runs, chosen at random.
  Verify: unit — seeded RNG, 10,000 GREEN runs with `sample: 0.1` → wakes within 10% ± 1%.
- **R-PRB-037** `confirm: k` MUST require k consecutive non-GREEN runs before a non-GREEN wake and
  before a `notify: auto` post of a non-GREEN state.
  Verify: unit — `confirm: 2`, RED, GREEN, RED, RED → one wake and (with auto) one post, both at
  the fourth run.
- **R-PRB-038** A wake MUST be a request item in the owner's inbox carrying the probe's scope and
  the (capped, redacted) evidence of the run.
  Verify: unit — item intent `request`; body contains the scope text and the evidence.
- **R-PRB-039** Evidence MUST be the last 4 KB of the floor's combined stdout and stderr, with the
  values of every environment variable listed in `[probes] redact_env` replaced before it is
  stored, sent or printed (including by `probe run`).
  Verify: unit — floor prints `$SECRET` with `redact_env = ["SECRET"]` → value absent from store,
  item and `probe run` output; 1 MB output → stored evidence = its last 4,096 bytes (redacted).

## Schedule

- **R-PRB-040** `every` MUST schedule runs at that interval, offset by a random delay within
  `jitter` when set.
  Verify: unit (synctest) — `every: 30m`, `jitter: 5m` → each run within [30m, 35m] of the slot.
- **R-PRB-041** `once` MUST run a single check at a random time within `jitter`, then stop the
  probe.
  Verify: unit (synctest) — exactly one run, within the jitter window; probe no longer active.
- **R-PRB-042** With `window`, the probe MUST stop once the window has elapsed since start and its
  owner MUST get one final wake asking for a final verdict (posted per `notify`); without
  `window`, it MUST run until stopped.
  Verify: unit (synctest) — `window: 48h` → no floor runs after 48 h; exactly one final wake;
  verdict on it posts when `judged`.

## Notify policy

- **R-PRB-050** `notify: none` MUST post nothing.
  Verify: unit — fake channel records no posts.
- **R-PRB-051** `notify: auto` on a probe with a floor MUST post floor state changes directly to
  `channel`, with no agent involved.
  Verify: unit — GREEN→RED → one post to the fake channel, no inbox item needed.
- **R-PRB-052** `notify: judged` MUST post only verdicts.
  Verify: unit — state changes post nothing; a verdict posts once.
- **R-PRB-053** A floorless probe with `notify: auto` MUST behave as `judged`.
  Verify: unit — runs post nothing; verdict posts.

## Commands

- **R-PRB-060** `ness probe start PATH…` MUST accept files or directories and register each probe
  with the calling session as owner.
  Verify: testscript — a dir with two probes → status lists two, owner = caller.
- **R-PRB-061** After `start`, edits to a probe file MUST be reloaded by the worker without
  another `start`.
  Verify: unit — change `every`; next schedule uses the new value.
- **R-PRB-062** `start --keep-window` on an already-started probe MUST keep the original window
  start; without it, the window restarts.
  Verify: unit (synctest) — restart at 24h with `--keep-window` → stops at 48h from the first start.
- **R-PRB-063** `ness probe run PATH` MUST run the floor now, print the state and evidence, record
  nothing (no store change, no wake, no post), and exit 0 for GREEN and 1 for any other state.
  Verify: testscript — store dump and fake channel unchanged; stdout has the state; RED → exit 1.
- **R-PRB-064** `ness probe verdict ITEM --state green|red --note TEXT` MUST answer the wake item,
  record the verdict and post it when the probe is `judged`.
  Verify: testscript — item disposed; status shows the verdict; fake channel has one post.
- **R-PRB-065** `verdict --probe-bug` MUST record the verdict as a probe defect, MUST NOT post it,
  and MUST flag the probe `needs-edit` in status until its file content changes.
  Verify: testscript — fake channel records nothing; status shows `needs-edit`; edit the file →
  flag cleared.
- **R-PRB-066** `verdict` on an item that is not a probe wake MUST be refused.
  Verify: testscript — ordinary request item → exit non-zero.
- **R-PRB-067** `ness probe stop PATH --note TEXT` MUST stop the probe and record the note;
  `--note` MUST be required.
  Verify: testscript — no further runs; without `--note` → refused.
- **R-PRB-068** Probe state MUST show in `ness status` (R-SESS-081).
  Verify: testscript.

## Ownership

- **R-PRB-070** When a probe's owner closes, ownership MUST fall to the owner's manager (§16 #5).
  Verify: testscript — close the quick task; status shows the manager as owner; next wake lands
  in the manager's inbox.
- **R-PRB-071** With no live session to own it, the probe MUST pause and be shown red in status
  and reported by doctor; it MUST NOT run headlessly, and it MUST resume only when a live session
  runs `ness probe start` on it (that session becomes owner).
  Verify: testscript — status `paused` red; doctor lists it; no wake items or tabs created; a new
  session opening does not resume it; `probe start` from it does, with it as owner.

## Decided details

- **R-PRB-080** Omitted fields MUST default to: `jitter` 0, `confirm` 1, `timeout` 60s,
  `wake` not-green, `sample` 0, `notify` none (a floorless probe still wakes on every run,
  R-PRB-030).
  Verify: unit — a file with only `every` and `floor` loads with these values.
- **R-PRB-081** Durations (`every`, `jitter`, `window`, `timeout`) MUST accept Go duration syntax
  plus a `d` (24 h) unit.
  Verify: unit — `2d` = 48h, `1d12h` = 36h, `90m` ok; `1w` refused.
- **R-PRB-082** `verdict --state` MUST accept only `green` and `red`.
  Verify: testscript — `--state unclear` → exit 2.
- **R-PRB-083** The floor MUST run as `sh -c <floor>` with the owner's worktree root as cwd and
  the inherited environment plus `NESS_PROBE` (probe path), `NESS_SESSION` (owner) and
  `NESS_PROBE_LABEL` (the probe's label, empty when it has none, R-PRB-087).
  Verify: unit — floor `pwd; env` → cwd = owner worktree; the variables present.
- **R-PRB-084** A probe's identity MUST be its repo-relative path, its owner session and its label
  (R-PRB-085); `start` of the same path by two sessions MUST create two probes, and `stop PATH`
  MUST stop only the caller's. A probe without a label is identified by path and owner alone.
  Verify: testscript — two owners start `probes/x.md`; status lists two; one stop leaves the other.
- **R-PRB-085** `start PATH --label LABEL` MUST give the probe a label; the same path started by
  the same owner under different labels MUST be different probes, each with its own window, schedule,
  streak, final wake and stop. A restart, an adoption of a paused probe and the merge of a closing
  session's probes (R-PRB-070) MUST match the label as well as the path, and ness MUST NOT end
  or replace a probe because a newer label started: the owner stops what a release replaces.
  Probes started before labels existed have the empty label and behave as before.
  Verify: unit — two labels at one path are two live rows; restarting `a` leaves `b`'s window;
  adopt and merge respect labels; a store from before labels keeps its probes. Unit — A's 2h window
  from t0 and B's from t0+1h: A gets one final wake and stops at t0+2h while B runs on.
- **R-PRB-086** A label MUST match `[A-Za-z0-9][A-Za-z0-9._-]{0,31}` (a release sha or a version
  fits); `start --label` MUST refuse any other before registering anything.
  Verify: unit — the accepted and refused labels; testscript — `--label "bad label"` → exit 2.
- **R-PRB-087** A labelled probe MUST be named with its label in brackets in its wake items, its
  window-end wake, its auto-posts and its judged verdict (`probe PATH [LABEL] run N: STATE`), MUST
  post in the thread `probe:PATH:LABEL`, and its floor MUST see `NESS_PROBE_LABEL`. A probe without
  a label keeps the thread `probe:PATH` and the unbracketed texts. Confirm streaks, `wake: change`
  baselines and auto-post de-duplication MUST be kept per probe, so labels never share them.
  Verify: unit — two labelled probes on one file post to two threads, a repeat RED in one posts
  nothing and the other's baseline is its own; each floor run sees its own label; no label, the old
  texts.
- **R-PRB-088** `stop PATH` MUST stop the caller's probe with the given `--label`, or without one
  the caller's unlabelled probe, or the only live one when it is the only one at the path; when
  several are live and none is meant it MUST refuse and list the live labels. `status` MUST show the
  label next to the path and as `label` in its JSON.
  Verify: testscript — start `--label a` and `--label b`; status shows both; a bare stop refuses
  naming `a, b`; `stop --label a` leaves `b` active.
- **R-PRB-089** Due floors MUST run concurrently, at most 8 at a time; when more are due, the
  longest overdue start first and the rest wait for a free slot. A probe whose floor is still running
  MUST NOT be started again, whatever its schedule says. When a floor ends, its run is booked (and
  its wake made, and its post sent after the booking) one booking at a time, and only if the probe is
  still the same active row: a probe stopped, paused or restarted meanwhile, or any floor that ends
  after the worker was told to stop, is not booked and is run again when it is due (a restart of the
  worker loses the floors then running). A transfer to a new owner keeps the row, so its wake goes to
  the new owner. A window that elapses while a floor runs ends after that run is booked. A running
  floor and a slow channel post MUST NOT hold up the rest of the pass.
  Verify: unit — two floors that each wait for the other to start finish; ten due floors, eight
  start, the peak is eight and all ten run once; nine probes: the one overdue longest starts first;
  a probe due again while its floor runs starts no second floor; a probe stopped, paused or
  restarted meanwhile is not booked; a floor ending after stop is not booked and none starts; a
  transfer meanwhile wakes the new owner; a floor that cannot start books nothing and schedules the
  next run; a window elapsing mid-floor ends after the run; a pass returns, and beats, with a floor
  running or a post hanging.
