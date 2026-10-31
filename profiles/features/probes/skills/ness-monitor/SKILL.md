---
name: ness-monitor
description: How a session watches a running system over days with ness probes after a release, a cutover, a migration or any big change. It covers designing the mechanical floors, the judging and the daily semantic audit, proposing that plan to the operator, then running the window to its end. Use it whenever the work is "watch prod", "keep an eye on it for a week", a post-release or post-cutover window, a soak, an audit schedule, or anything that will run for hours to days, even if nobody says "monitor" or "probe".
---

# ness monitor

Monitoring over days is a job for the session that made the change, not for a script. You know
what changed and why, so you are the judge. The worker runs the cheap mechanical checks on a
schedule, and it wakes you only when one of them needs a judgment. Everything here uses plain
ness probes (the `ness-probes` skill has the commands and the file format).

## Who does the monitoring
- **This session, kept open or resumed.** Every judgment and every audit runs in the session
  that owns the probes: a wake lands in its inbox, it reads the evidence and answers with
  `ness probe verdict`.
- **No headless runs.** Don't use `claude -p`, cron jobs that start an agent, or systemd timers
  that run an audit prompt. A headless run has no memory of the change or of yesterday's
  findings, nobody acts on its output, and its alerts go nowhere. When you need a periodic
  look, write a probe that wakes you. When the operator asks for a headless job by name, say
  why a probe is better and propose the probe in your plan. The operator decides: build what
  they choose after hearing why.
- **Subagents are fine inside a wake.** A daily audit can fan out to subagents from the woken
  session. They report back to it, and it decides.

## 1. Reason first
Before writing any probe, write down in the session record:
- **What changed**: components, data paths, configs, the things that are new in production.
- **What failure would look like** to a user and to the system: wrong output, missing output,
  late output, cost, resource exhaustion, silent data loss.
- **The window**: an absolute end time, not "a week". Write it as a UTC timestamp and set
  each probe's `window` so that it ends there.
- **What already exists**: dashboards, health endpoints, logs, queries you can reuse as floors.

## 2. Design three layers
**Floors (mechanical, run by the worker).** One concern per probe file, a cheap command, and
exit codes that mean something: 0 GREEN, 1 RED, anything else UNCLEAR. Cadence follows how
fast the concern can hurt: delivery and feeds hourly, resources and correctness every few
hours, slow-moving identity and accounting checks every 6-24h. Add `expect` with a sample
count (`n>=1`) so a floor that saw nothing is VACUOUS instead of a false GREEN. The key must
count what the floor actually examined (rows, requests, signals). `n>=0` and `errors=0` are
always true when nothing was seen, so they guard nothing. Derive the count from the data, never
from the number of queries the script issues.

**Judges (you, on a wake).** The probe body is the scope you check when woken: what to look at
beyond the floor's evidence and what counts as a real fault. Use `confirm: 2` on noisy floors
and `sample` on floors with thin coverage. The worker runs due floors in parallel (eight at a
time), so a slow floor delays no other; a floor still running is not started again.

**Semantic audit (you, once a day).** A floorless probe (`every: 24h`, `jitter: 6h`, no
`floor`) wakes you every day at a different hour. The audit reads what no floor can: logs end
to end, odd but non-failing behaviour, slow drifts, a sample of real outputs checked by hand.
Fan out one subagent per area. Before a finding gates anything, verify the surprising claims
yourself, because audits overclaim. Dispose every finding: fix, accept as known with an expiry,
or file it as a follow-up.

## What makes a floor trustworthy
These rules come from a real production window, where most early wakes were noise:
1. **Separate "dead" from "low".** Zero output for hours is an outage; low volume may be a
   quiet hour. Check them in different probes or report them as different keys.
2. **Check the input separately from the output.** An upstream feed going quiet and your
   pipeline dropping data look the same from the output side.
3. **Compare against the same hour of the week**, not a flat threshold. Weekend peaks and
   night troughs are normal.
4. **Measure the outcome, in absolute terms**: delivered and acknowledged, with absolute
   latency, not only "sent".
5. **Keep one RED from blinding the rest.** One probe per concern, and restart with
   `--keep-window` after a fix so the window does not reset.
6. **Report "cannot judge" as its own state.** If the floor could not reach the data (ssh
   failed, query timed out), exit 2 so the run is UNCLEAR. Don't exit 1, because that is a RED
   about the system.
7. **Break volume down** per segment (per sport, region, cohort, rule) so one segment going to
   zero is not hidden by the total.
8. **Deduplicate known causes.** Keep a known-causes file the floor reads (key pattern, cap,
   expiry date, decision id). A known issue then stops waking you, and its expiry makes sure
   somebody looks at it again. Above the cap the anomaly must wake you again. Test that with a
   value over the cap, because a silent cap check turns the file into a mute button.
9. **Attribute errors to their source** (service, connection name, job), not only count them.
10. **When you compare two systems, fail on a one-sided loss** and name the cause class. Treat
    known semantic differences as KNOWN, not as diffs.
11. **Keep the full output.** Write it to a file the wake points at. The wake evidence is
    capped, so it is a summary.
12. **Use sustained rates, not instants.** Look at "error rate over 15 min", not one bad sample.
13. **Watch the slow killers**: memory slope, DB CPU, disk growth, restart-count deltas.
14. **Give every floor a positive control.** Before you trust a GREEN, run the floor against a
    case you know is RED (`ness probe run` with a lowered threshold, or a fixture).

A floor's last line carries `key=value` pairs (counts, rates, the anomaly keys) so that
`expect` can check them and the wake shows them.

## 3. Propose before starting
Monitoring costs the operator attention for days, so show the plan and wait for a yes. Send it
with `ness send --to operator`. Put a page in it when the plan is long. The plan has:
- the window (absolute UTC end) and what "done" means at the end;
- a table with one row per probe: concern, cadence, floor command (or "judge only"), what RED
  means, `confirm` and `sample`;
- the daily audit's areas and what it disposes;
- the expected wake volume per day, and how a known issue gets silenced (the known-causes
  file);
- the hotfix policy: what happens to the window when a fix ships mid-window.

## 4. Start
1. Put the probe files in one directory under the session record (for example
   `<session>/watch/`), with the floor scripts next to them. A floor runs from the owner's
   worktree root, so write it as `cd <session>/watch && ./probes/...`.
2. Run each probe with `ness probe run PATH` until the floor's output is right, including the
   positive control.
3. Run `ness probe start DIR`. Pass `--label <release>` when the same files also watch another
   release in parallel.
4. Check `ness status`: every probe should be active with a next run.

## 5. Run the window
- **Answer every wake** with `ness probe verdict`. Give a one-line note that says what you
  saw.
- **A wrong floor is a probe bug, not a system fault.** Answer with `--probe-bug`, fix the
  file, and the edit reloads.
- **A known cause** gets a row in the known-causes file with an expiry and the decision id.
  Never relax the floor's threshold for it.
- **Hotfixes.** When a fix ships, restart only the affected probes. Use `--keep-window` when
  the window should hold, and record the choice.
- **Waiting.** Between wakes, end your turn. The worker wakes you, so don't poll.

## 6. Close the window
At the window end each probe sends its final wake. Then:
- write a summary: true faults found, noise, probe bugs, known causes still open, and what the
  floors missed but the audit caught;
- turn what you learned into floor lessons for the next window;
- stop leftover probes with `ness probe stop`, then report to your manager.

For a full worked example, read `references/example-prod-week.md`. It is an eleven-probe week
after a production cutover, with the floor protocol and the daily audit.
