# Example: a production week after a cutover

This is an online store's checkout service. It moved from a monolith to separate services, and
the cutover went live on a Monday at 09:35 UTC. The session that did the cutover watched
production until the next Monday at 12:00 UTC. This page shows that window written as ness
probes.

## The reasoning that came first
- **What changed**: four order flows (card payments, wallet payments, refunds, a stock
  reservation tracker), each with its own worker and confirmation email, plus new storage, a
  payment-provider rate budget and a customer-account migration.
- **What failure looks like**: confirmations missing or late, duplicate or stale orders, wrong
  totals in the email, a provider webhook going quiet upstream, the rate budget running out,
  memory creeping up, customers losing their saved carts.
- **Window**: until 2026-10-27 12:00 UTC, so the first probes got `window: 169h`.

## The probe set: one concern per file, cadence by how fast it hurts

| probe       | every | what RED means                                                        |
|-------------|-------|-----------------------------------------------------------------------|
| runtime     | 1h    | a service restarted (restart-count delta) or is down                  |
| storage     | 1h    | writes failing, or table growth off the hour-of-week baseline         |
| budget      | 1h    | provider rate burn on course to run out before the budget resets      |
| webhooks    | 1h    | an upstream webhook quiet past its normal gap (input, not output)     |
| delivery    | 1h    | orders placed over 10 min ago still unconfirmed; p99 latency over target two hours running |
| resources   | 3h    | memory slope, DB CPU, disk growth, a secret in the logs               |
| card        | 3h    | card orders dead for 6h, duplicates, stale orders, wrong totals       |
| wallet      | 3h    | quality checks on wallet orders                                       |
| refunds     | 6h    | quality checks on refunds                                             |
| stock       | 6h    | stock-reservation tracker delay                                       |
| accounts    | 6h    | migrated customers missing saved carts or addresses                   |
| audit       | 24h, jitter 6h | floorless: the daily semantic audit wakes the session        |

A probe file, `watch/delivery.md`:

```markdown
---
every: 1h
window: 169h
timeout: 5m
floor: cd docs/sessions/cutover/watch && ./probes/run.py delivery delivery.sh
expect: sent>=0
confirm: 1
---
Confirmations per flow over the last hour: placed, sent, failed, retried, mail-provider
throttles, p50/p99 latency. RED is an unconfirmed order older than 10 minutes, or p99 over
target (card 60 s, wallet 120 s) in both of the last two hours with at least 10 sends each.
Fewer sends than that is UNJUDGED, not GREEN. When woken: open the full output the evidence
points at, check whether one flow or all of them are affected, and look at the worker logs for
that hour.
```

## The floor protocol
Every floor goes through one runner (`probes/run.py NAME SCRIPT`). The runner runs the script
over ssh, saves the full output to `out/NAME-<time>.log`, redacts secrets and prints a short
summary. The script prints lines such as:

```
ANOMALY delivery.unsent.card 3 three card orders placed >10m ago without a confirmation
UNJUDGED delivery.p99.wallet only 4 sends in the hour
CANNOT ssh timeout to prod-db
SUMMARY sent=41 placed=41 unsent=0
```

The runner then exits 0 when there is no ANOMALY, 1 when there is at least one ANOMALY that
is not known, and 2 when it hit CANNOT, so the run is UNCLEAR (a fault in the check, not in
the system). The last line, `SUMMARY key=value ...`, is what `expect` reads.

The known-causes file, `known.tsv`, holds one row per accepted cause:

```
mem\.wallet	60	2026-10-27	O246
```

The row means: an anomaly key matching `mem\.wallet` with a value up to 60 is known
(decision O246) until 2026-10-27. After that date the anomaly wakes the session again. The
first RESOURCES red of the week (wallet worker memory at 52 MB) went exactly this way: the
session judged it as the expected cache warm-up, recorded the decision and added the row.

## The daily audit
The audit is a floorless probe (`every: 24h`, `jitter: 6h`), so it lands at a different hour
each day and catches weekend peaks. When it wakes, the session fans out five subagents, one per
area: orders, accounts, database, queue, runtime. Each subagent reads logs and data end to end
and returns its findings. The session verifies every surprising claim itself before acting on
it. It writes `audit/<date>/summary.md` and disposes every finding: fix now, add a known-causes
row with an expiry, or file a follow-up.

## What the first windows taught (and the skill now says)
Of 48 wakes in the dev week, 7 were true faults, 21 were mixed and 20 were floor
limitations. Most of the noise had three sources: totals that hid a segment going to zero, an
upstream webhook being quiet when the floor read it as our own failure, and flat thresholds in
the night trough. Hour-of-week baselines, separate input and output probes and the
known-causes file removed most of it.

## Hotfix policy
When a fix shipped mid-window, the session wrote down one of three choices: **absorbed** (the
fix is small, so the window and its probes continue), **extended** (the window end moves out
by the time the fix needs to be proven), or **restarted** (the change is big enough that the
week starts over). With ness probes, "absorbed" is a restart of the affected probe with
`--keep-window`, and "restarted" is `ness probe start` without it.
