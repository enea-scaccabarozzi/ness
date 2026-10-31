> [!NOTE]
> Archive: altered for the public release. Local paths were replaced; the content is otherwise as written.

# Review: ness 69d173e (parallel-probes) — probe floors in parallel

Worktree: ~/src/ness/.claude/worktrees/agent-afbc1766c9faadd7d (read-only, not edited).
Repro tests were run in a scratch copy:
/tmp/<scratchpad>/scratchpad/copy/internal/worker/review_repro_test.go

Race run (`go test -race -count=3 ./internal/worker/`): see the RACE section at the end.

No High findings. The core mechanics hold up: the in-flight set, the cap, the release-after-booking
order, re-reading the row under `book`, streaks read from the fresh row, Rand and Post used only
under `book`, and a single SQLite connection. There are no lost or double bookings among
concurrent probes, and no same-probe overlap.

## Medium

### M1. A restart (same row) while a floor runs gets the stale floor's booking; a once probe gets stopped
internal/worker/probes.go:136-155 (floor), internal/store/probes.go:72-107 (StartProbe reuses the id)

`ness probe start` on a probe the owner already has active, or adopting a paused one, keeps the
same id and state `active`. It resets `started_at`, `next_run_at=now` and `final_wake`. The
re-check in `floor()` looks only at `cur.State == active`, so the old floor (old `pf`, old file
hash, started before the restart) books into the restarted row:
- once probe: `decide` calls `stopProbe("once: ran")`, so the restarted probe is stopped before its own run.
- every probe: the restart's `next_run_at=now` is overwritten with `oldStart+every(old pf)`, so the
  edited probe waits a full period. Its first run is the old file's result, under the old hash.
- adopt-after-pause: the floor ran in the old owner's worktree, and its result is booked and woken
  to the new owner.

Repro (fails): TestReviewRestartDuringFloorIsStoppedByOldRun in the repro file. Output:
`state=stopped runs=1 note="once: ran"`.
The doc comment at probes.go:133-135 also claims "run by hand" is handled. `ness probe run` never
touches the store (cli/probe.go:341), and nothing compares anything except state.

Fix: capture `p.StartedAt` (and `p.Owner`) at launch. Under `book`, drop the result when
`cur.StartedAt != p.StartedAt || cur.Owner != p.Owner`, logging "floor dropped: probe restarted".
For the TOCTOU with a CLI stop in another process (M1b below), make RecordRun conditional instead:
`... WHERE id = ? AND state = 'active' AND started_at = ?` and treat 0 rows as "dropped". That
also closes the stop/record window atomically.

### M2. Worker stop under systemd books every in-flight floor as UNCLEAR and wakes or posts it; the spec documents a "loss" that is not what happens
internal/cli/worker.go:139 (`defer w.Wait()`), internal/probe/floor.go:58-61, internal/atom/systemd/systemd.go:62-66

The unit has the default KillMode=control-group. `systemctl --user stop/restart` sends SIGTERM to
every process in the cgroup, and Setpgid does not leave the cgroup. The worker catches SIGTERM
(NotifyContext), the floors' `sh` die with exit 143, and `State()` maps that to UNCLEAR.
`w.Wait()` then lets each floor book: streak+1, and with the defaults (confirm 1, wake not-green)
a wake lands in the owner's inbox and `notify: auto` posts "GREEN → UNCLEAR". With up to 8 floors
in flight, one restart can produce 8 false alarms. The serial code had the same exposure for one
floor. The build notes say "a floor's output is lost if the worker exits mid-floor", which is
wrong under systemd. A real loss happens only on SIGKILL after TimeoutStopSec.
R-PRB-089 says nothing about worker exit.

Repro (fails): TestReviewSigtermedFloorIsBookedAndWakes. A runner returning Exit 143 gives
`state=UNCLEAR runs=1 wakes=1`. On a box: start a probe with `floor: sleep 50`, then run
`systemctl --user restart ness-<project>` within 50 s. The owner's inbox gets an UNCLEAR wake.

Fix: give Probes a `stopping atomic.Bool`. Add `Worker.Stop()` that sets it and then calls
`Wait()`, and have runWorker's loop path call it (keep `Wait()` for `--once`). In `floor()` under
`book`, drop the result when `stopping` is set ("floor dropped: worker stopping"). The row stays
due and reruns after restart. Spec this in R-PRB-089, and write the loss down there: "a floor
running when the worker stops is not booked; the probe runs again after restart".

### M3. A slow channel post holds `book` and stalls the pass through floorless bookings; the "post keeps its beat" now hides a hung loop
internal/worker/probes.go:121-125, 142, 380-392

`Post` (HTTP timeout 30 s, cli/exec.go:65, possibly more than one request) runs under `book`.
Every other booking waits, including the floorless booking the pass makes inline at
probes.go:121. So one stuck Slack call blocks the pass, which goes against "a running floor MUST
NOT hold up the rest of the pass". Separately, `post()` is now called only from floor goroutines
(autoPost is floored-only). Its `w.beat()` refreshes the heartbeat from off the loop, so a hung
pass (the documented hung-herdr case, cli/worker.go:126) looks alive while floors finish and
post.

Repro: a floored probe with notify auto and a Post fake that blocks on a channel, plus a
floorless due probe. `b.launch()` twice: the second launch never returns until the post is
released.
Fix: take the network call out of the critical section. Under `book`, compute and record
(including SetNotifiedState as "pending" or after), then post outside the lock. Simplest: under
`book`, decide `text` and whether to post, unlock, call Post, and re-lock only to
SetNotifiedState. Per-probe order is still safe because the same probe never overlaps. Delete
the `w.beat()` in `post()`, since the pass no longer waits on posts.

## Low

### L1. Starvation past the cap: slots go to probes in path order, not to the longest-overdue
internal/worker/probes.go:52-66 (Store.Probes orders by path, label, owner)
Repro: 9 probes whose floor runtime is at least `every`. When a slot frees, each pass hands it to
the first due probe in path order, so `z.md` never runs. No status, log line or alert shows that
a probe is waiting on the cap, and the heartbeat is healthy.
Fix: in `probes()`, sort the due probes by NextRunAt before claiming. Log "floor deferred: cap"
once per id (Log.Error dedups).

### L2. Shutdown via version change waits for floors with no heartbeat or passes
internal/cli/worker.go:139, loop.go:44-47
After `Loop.Run` returns nil (binary changed), `Wait()` blocks for up to the longest floor
timeout, which has no upper bound (probe.go accepts any positive duration). During that time the
lock is held, there are no deliveries, and doctor sees a stale heartbeat. With M2's Stop() this
becomes "drop at once". Otherwise document it.

### L3. Floor that hangs past its timeout: bounded, but invisible
shell.Run kills the process group on timeout. WaitDelay is 2 s, so Run returns at
timeout+2s and the slot frees (defer release). That part is fine. A `setsid` grandchild escapes
(already a ponytail). Nothing reports a floor that runs long, and the old per-floor beat was the
only signal tied to floors. Consider showing "running since" in `ness probe status`. Only add it
if operators ask.

### L4. `TestSlowFloorHoldsNeitherThePassNorTheHeartbeat` does not discriminate
internal/worker/probes_test.go:282-303
Against the serial code it still passes, only about 5 s slower: the gate times out with an error,
floorFailed books, the pass returns, and the second launch beats. The build notes admit this. To
make it bite, assert that the first `b.launch()` returns while `g` is still closed:
`b.w.InFlight()==1` right after launch, before `waitStarted`. Then the heartbeat check is
meaningful. Removing `TestFloorRunRefreshesTheHeartbeat` lost no live behavior: per-floor beat
is gone by design. The post-beat (P-20 for slow channels) has no test, and M3 says to delete it
anyway.

### L5. Test hygiene: a failing test leaks blocked floor goroutines into later tests
probes_parallel_test.go:31-35. A `t.Fatal` before `g.open()` leaves floors blocked for up to 5 s.
They then call floorFailed on a closed store and write to a TempDir that is already removed. Fix:
in newGate, `t.Cleanup(func(){ g.open(); b.w.Wait() })` (pass the bench). This needs to run before
the store-close cleanup, and cleanups run LIFO, so registering after newBench is enough.
Otherwise the barrier tests are deterministic: channels only, the 5 s selects only bound a red
run, and there are no sleeps.

### L6. Missing tests for the interleavings the spec line claims or implies
Covered: concurrency, cap, no restart while running, stopped meanwhile, pass returns.
Not covered: paused (owner closed) meanwhile, transferred to the manager meanwhile (books and
wakes the new owner, which is the intended SendWake semantics but untested here), restarted
meanwhile (M1), floor error path through the goroutine (floorFailed under `book`), and the
window end while running (correctly deferred until the floor books; untested).

### L7. Doc and comment accuracy
- R-PRB-089 (docs/spec/probes.md:190-197) is accurate for what is built. It is silent on worker
  exit (M2), on restart (M1), and on fairness past the cap (L1). Its "only if the probe is still
  active" check is not atomic with the write (M1b: a CLI stop between the re-read and RecordRun
  still books a stopped row; SendWake guards the wake, but autoPost does not).
- probes.go:133-135 "stopped, paused or run by hand": "run by hand" is not a store event (see M1).
- worker.go:138 `now = w.Now() // floors took time` is stale (acknowledged in the notes).
- probes.go:386 `w.beat() // a slow channel must not read as a dead worker`: no longer true (M3).
- SKILL.md line: accurate.

## Checked and fine
- Lost or double bookings: release happens after book.Unlock and before Done (defer LIFO), so the
  row is never due and unclaimed between booking and release.
- Confirm streaks: same-probe floors never overlap, the streak comes from the re-read row under
  `book`, and labels are separate rows.
- Notify auto: NotifiedState is re-read per booking and per thread (`probe.Thread(path,label)`);
  the post order across probes is completion order, which is acceptable.
- Pause, owner close, handover: pause gives `floor dropped`; move-to-session books to the new owner.
  SendWake re-reads the owner in-statement.
- Window end: a running probe is skipped before `endWindow`, so the final wake comes after the
  late floor books.
- SQLite: SetMaxOpenConns(1) serializes in-process access, with no nested store calls, so there is
  no in-process SQLITE_BUSY. Cross-process access relies on busy_timeout and immediate txlock, as
  before.
- Wait(): Add happens only on the pass goroutine and Wait on the same goroutine after the loop, so
  WaitGroup reuse rules hold. Floors are bounded by timeout+2s, so there is no goroutine leak beyond that.
- Log: all three public methods are locked, and `write` and `rotate` are only reached under the lock.

## RACE
- `mise exec -- go test -race -count=3 ./internal/worker/` as given: FAIL after hitting Go's default
  10m timeout (panic: test timed out, during TestLogRotatesAt10MB on the 3rd count). No test failed
  and no DATA RACE before that. One count takes about 4 min under -race.
- Rerun with `-timeout 40m`: `ok ... 731.802s`, 0 DATA RACE reports.
- Note: anyone running the gate command as written gets a timeout, not a race. Add `-timeout` or
  use -count=1 in the docs and gate.
