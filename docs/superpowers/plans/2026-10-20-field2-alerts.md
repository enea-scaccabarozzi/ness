> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Field-2 alert false positives: plan

Evidence: docs/discovery/findings/field-2/alerts.md. One signal, `session.Activity`, read from the
client transcript (cached by size and offset) and fed through `worker.Client.Activity`.

1. Pending background work. Test: claude `Activity` on a trimmed real transcript (agent launched, resumed
   agent, bash launched, completions, "waiting on its own background work" notice) and on the screen
   line "Waiting for N background agent"; worker: idle 31 min with pending work, zero stall reports,
   no Your move, parent's treeBusy true; with completion, one report.
2. Stall clock. `Store.LastInbound(session)`; test: request disposed at 30m03s, no stall until +30 min.
3. Rate limit from the transcript. Test: Fable 429 tail gives Limit; worker: no hint, no stall, one
   Trouble alert per episode; retry after `limitHold`.
4. Episode reset only after a real assistant turn. Test: alert, compaction blip, idle: no second
   alert; alert, real turn, idle: second.
5. Stall fyi loop: child with pending work after a manager hint yields no stall (test over 1).
6. Blocked needs the screen to show a prompt and no compaction. Test: blocked 2 min on a compacting
   screen: no alert.
7. HasAsk: "## Ask / Nothing." is no ask. Test in store and alert.
8. Diagnostics: last 6 screen lines logged on first stall and on a suspected limit.

Spec: R-WRK-050/052/081/093/094 reworded, new R-WRK-096 (activity), R-WRK-097 (transcript limit).
