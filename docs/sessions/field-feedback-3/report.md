> [!NOTE]
> Archive: altered for the public release. Names of private projects and private links were replaced; the content is otherwise as written.

# Report: field feedback round 3

Delivered: a verified field analysis of ness in the production backend and the research lab, window 2026-10-21 → 10-29,
published at <private report link removed>. Evidence (private, untracked):
`docs/discovery/findings/field-3/` — q-quant.md, r1–r8 reader findings, v1–v5 verifier verdicts.

## Verdict
The core holds: no lost messages, refusals teach, ladder cut the research lab's frontier share 70.8% → 21.7%.
The operator-facing layer does not: 58 of 59 stall alerts false, 60% of phone posts worker notices,
14 "where do we stand" returns, 9 lost gate runs, drift passing reviews. One fix cycle and one short
field run, with release prep in parallel to the run.

## Ranked weak points (all verified)
1. False stall alerts flood the phone and wake managers (worker.go:251-255; rule ignores probes, accepted requests, boundary).
2. Returning operator not served (phone shows one 200-char line; open ask kills "Your move", alert.go:166-178).
3. Integrate loses gate runs on a moved target; per-target lock refuses; push has no keepalive/retry.
4. 17 h 37 m prompt block with a single notify, manager never told.
5. Framing/product drift passes code-only reviews; no product/framing reserved decision.
6. Roots cannot message each other; child cap project-wide; config guard misses shell writes.
7. No promotion/amend path; added files under frozen globs unchecked; close deadlock needs --discard.
8. Hook false positives (54/54 inbox reads), ness pick stdin hang (10/11 heredoc calls), effort/kind friction.
9. Reply items not disposable; wakes held by an unsent operator draft.
10. transcriptDir wrong for a root adopted inside its worktree (≤2 alerts); version string 0.1.0.

## Next steps
Fix cycle (liveness, operator loop, integrate/contracts, multi-root, hook/CLI precision); a one-week
field run on both projects after the operator's go, measured against the round's numbers; release
prep: fresh public repo, scrub 36 files, README/docs, install/update path, Codex test or label, version.

## Fix round (second ask)
Reasoned from five causes (specs.md "Fix round"), not ten symptoms; two children plus a root batch.
- liveness (child, 0ce5dd5): waits derived from the store (asks, open/accepted requests, probes,
  boundary --until) drive stalls, Your move, status and the cap; stalls to the manager, phone on a
  second miss; blocked prompt escalates 2 min manager, then 10 min / 1 h / 4 h phone; phone text is
  the whole Ask, 400 runes at a sentence end; parked children hold no slot. Replay: accepted-request
  child 0 reports (was one per 30 min); 17.6 h prompt 6 phone alerts (was 1).
- integrate (child, cbdcdd7): per-target lock queues; a moved target is refolded (records-only lands
  without a gate, code re-gates once); push keepalive + one retry; added files under frozen globs
  counted; `contract amend --authorized`; contained branch closes without --discard. Scratch
  reproductions before/after for the race and the push drop.
- root (3e4dfa4, 9b2cea4): inbox --show A B C / --unread; polling guard refuses only unbounded waits;
  pick reads stdin only with --stdin; effort defaults from the kind; sweep = search; recipient-only
  reply settle; transcript path for roots adopted in their worktree; build version; ness-entry text.
  A fresh review found 3 mediums (bash -o pipefail -c loops passing, a sender hiding its own reply,
  an eval missing --stdin), all fixed with tests.
- Pending operator rulings (FF3-D3 root-to-root sends, FF3-D4 product/framing reserved, cli-ux close,
  public repo approach): follow-up batch; nothing shipped depends on them.
- Known limits for the field run: mutual waits are silent; target lock waits without a limit; pr
  integrator push has no keepalive.

## Batch 2 (operator rulings FF3-D10..D12)
- Contracts: R-CTR-057, content equal to the base tip passes (local base at or ahead of the
  merge-base; origin only at or ahead of local base; full ref names). Fixes the squash-dropped
  merge and stale-ref failures behind the production backend's `--no-verify` pushes. `contract amend PATH...`
  takes many paths; a deleted path is proven by testscript. The project-side steps (cutover doc
  line, pre-push skipping the contract check on pushes to main) went to the SODP root.
- Phone: alerts redesigned (kind header, linked bold title, kept Ask structure, named links, no raw
  URLs, safe cut); every claude launch has `--remote-control=<project>/<title>` and the worker reads
  the session's Remote Control URL from the transcript. Telegram's parser now honours backslash
  escapes.
- Policy: non-trivial product decisions reserved (ness-entry), intent check in ness-review; roots
  send to roots with an fyi copy to the operator (R-MSG-073). cli-ux closed on the operator's words.
- Review: one high (base-tip rule trusted an origin behind local base) and four mediums (cut inside
  a link, unescaped title, unchecked page URLs, ref shadowing), all fixed with tests (9127715).
- Left: first alert after a worker restart has no link; root-to-root replies are not copied; the
  intent check has no spec rule; a forged origin ref cannot be caught locally (a record, not a
  boundary).
- Next (another session): public release from this repo: cleanup with rules against private project
  references in docs (FF3-D10 (4)), install/update path, doc design.

## Lessons
- Reader claims shrink or move under verification: 7 of ~15 headline numbers changed (stalls 39→38/39,
  research lab 6→20, asks 55→21, replies 106→14, transcript bug from "cause" to ≤2 alerts). Never publish reader counts unverified.
- Store "OPEN" counts overstate pending work: reads clear replies, fyis expect nothing. Count by intent and inbox state.
- Distilled digests plus subagent readers kept ~780 MB of transcripts out of the root's context; the root read only summaries.
- A full /tmp tmpfs looks like a code failure ("no space left", link errors); testkit refuses work
  dirs outside /tmp, so the box's tmpfs size is a gate dependency. Check `df /tmp` first.
- Two children plus subagents on disjoint files ran without a merge conflict; the one shared file
  (inbox.go) was announced up front and the child rebased.

