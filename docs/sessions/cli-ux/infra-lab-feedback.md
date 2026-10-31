> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Field run 1: the infrastructure lab `nightly`, 2026-10-10

One root (`nightly`, promoted with `--here`) and two children (`backups`, `alerts`), from
15:01 to 17:07 UTC. Sources: the infrastructure lab store and worker log, the three transcripts, herdr.
Outcome: both problems diagnosed and fixed on the lab, pushed to the infrastructure lab main (`624d098`);
the operator approved every change after the fact.

## Timeline that matters

| UTC | What happened |
|---|---|
| 15:02-15:06 | Root opens `backups`, then `alerts`; each `ness open` waits the full 120 s herdr timeout. `backups` lands in workspace w6 (another project's workspace). |
| 15:07-15:09 | `backups` needs two numbers from the operator; the root relays them in 70 s (the operator is in the root's tab). |
| 15:08 | Worker: root pane focused, nothing typed. This holds until about 17:02. |
| 15:10 | `alerts` sends its situation report (fyi) to the root and starts fixing at once. |
| 15:12 | The root's Claude Code shows an "away" recap: the operator is not at the terminal. |
| 15:15-15:37 | `alerts` waits 21 min on an AskUserQuestion in its own tab (deploys blocked by the permission classifier). |
| 15:25 | `backups` sends a decision to the root, then tells its own tab "waiting for the manager". |
| 15:37 | The operator finds that question in the `backups` tab and answers it there. The root's copy of the decision stays open. |
| 15:39 | Worker stall notice "root idle with 1 obligation" goes to the operator inbox. Nobody reads it. |
| 15:53, 16:03 | Both children hand off. |
| 17:04 | The worker finally hints the root (4 unread, 1 h 55 min after it went idle). The root acknowledges, closes both children, integrates, pushes. |
| 17:06 | The operator gets the situation report they asked for "first", after the fixes are live. |

## Findings

**F1, bug: a child's tab opens in whichever workspace the operator has focused.** `OpenTab`
runs `herdr tab create` with no `--workspace`, so herdr picks the focused one. Fix: open in the
parent's workspace (the prefix of its tab id); a root opened outside herdr uses
`HERDR_WORKSPACE_ID`, then the default.

**F2, bug: "focused" is taken to mean "the operator is here".** herdr reports a pane as focused
when it is the selected tab in the herdr UI, even with the operator in another app or away. The
worker held every hint to the root for about 1 h 55 min. Fix: a focused pane is held only while
the operator is active there (screen or draft changed within the quiet window). After a few
idle minutes it is treated like any other pane.

**F3, gap: nothing ever reaches the operator.** The stall notice, the children's questions and
the "needs you on the Mac" ask all landed where the operator does not look: the operator
inbox, or the child's own tab. Fix: the worker notifies the operator for every new operator
item and every pane blocked on a prompt (herdr notification, plus a notify channel such as
Telegram for the phone). Also, a stall the worker caused itself (F2) is not reported as the
session's stall.

**F4, friction: `ness open` costs 2 min per child.** The first prompt goes in argv, so Claude
starts working at once and herdr's "ready for input" never arrives before the 120 s timeout.
Fix: start the agent without a prompt (ready in seconds), then `herdr agent prompt` the first
message.

**F5, friction: agents relearn the CLI.** Root and children ran `ness --help`,
`ness send --help`, `ness inbox --help` and `ness reply --help`, and the root guessed at the
`--json` keys. The cheat sheet exists but only after compaction. Fix: the session-start hook
always prints it.

**F6, friction: acknowledgement ritual.** The root sent four "Received; folded into nightly"
replies to children it closed a second later; they sit in closed inboxes. Fix: closing a child
accepts its open items to the manager, and a handoff needs no reply.

**F7, friction: a question answered elsewhere stays open.** The `backups` decision was answered
by the operator in the child's tab, but the item to the root stayed open and was "accepted" 1.5 h
later, after the work was done. Fix: falls out of the escalation policy below (an operator item
answered once, wherever).

**F8, policy: the operator's checkpoint was lost twice.** The request said "report to me the
situation and then if required fix". The root's brief turned "me" into the root, and the child
sent an fyi and kept going. See "Autonomy".

**F9, policy: children talk to the operator off the record.** Children asked questions
(AskUserQuestion) and wrote operator-addressed summaries in their own tabs, which nobody reads
unless the operator happens to visit. See "Escalation".

**F10, the infrastructure lab (not ness): the lab tunnel is shared and unguarded.** `backups` ran
`tunnel down` while `alerts` was using it ("first attempt raced a sibling's teardown").
Parallel children need a reference-counted tunnel (up takes a lease, down releases it).

**F11, fixed by the restart: names.** The root carried the client's own title ("backup
upload drop") under the tab label "Ness Test". After the worker restart it is
`infra-lab/nightly`. This run's children were launched by the old build, without a name.

Still owed by the operator, from the run: the laptop backup fix (check its log, rerun its install
script), the storage provider's "upload flowing" check after the first upload, and
closing the `nightly` root.

## Escalation: who tells the operator what (proposal)

The facts from this run: four things needed the operator (two facts only they knew, one
approval on their tracker account, one action on their Mac). The one relayed through a root that
the operator was looking at took 70 s; the one relayed through a root they were not looking at
never arrived; the two asked in a child's tab took 12 and 21 min and depended on the operator
wandering into that tab.

The ladder is right for **authority** and wrong as a **channel**. A relay adds latency and loses
wording, and adds nothing when the relaying session cannot decide either. Proposal:

1. **Who decides:** the lowest session whose brief grants the decision. The operator's reserved
   decisions are listed per project (irreversible actions, money, accounts and credentials,
   publishing outside the repository, policy changes, scope beyond the brief) and are never
   decided by a session.
2. **Who asks:** the session where the question arises. Any session may send `--to operator`
   for a reserved decision, a fact only the operator has, or an action only the operator can
   take. Its manager gets a copy automatically, so the manager's map stays current. Everything
   else goes to the manager, which either decides or forwards the item unchanged, never
   reworded.
3. **One queue, pushed:** every operator item is listed by `ness inbox` outside a session, and
   the worker notifies the operator when one arrives (F3). The answer goes straight to the
   asker: `ness reply ITEM` from the shell, or in the asker's tab, which records it. One answer
   closes the item everywhere (F7).
4. **Chat is not a channel:** a question or an ask the operator must see is an operator item.
   Text in a child's tab, and AskUserQuestion in a child, are assumed unread. A child's
   permission prompt shows up through F3 (blocked pane).

This replaces R-MSG-017's "only a root sends `--to operator`" and fills the empty "Reserved
decisions" section of ness-entry.

## Autonomy: checkpoints (proposal)

What happened was defensible: every change was reversible, in policy, and verified, and the
operator approved all of it afterwards. What was lost is the chance to steer, at the one point
the operator asked for ("report to me, then fix"). The reason agents skip checkpoints is that,
without F2 and F3, a checkpoint costs hours. Once it costs a notification and a reply from the
phone, it can be honoured cheaply.

Proposal: three levels, written into every brief, defaulting per project.

- **plan first:** the session sends its plan as an operator decision and changes nothing until
  the go.
- **report, then act** (a diagnosis checkpoint): the session diagnoses, sends the situation
  and the proposed fix as an operator decision, and waits for it before any change.
- **act within the brief** (today's behaviour): only reserved decisions stop it; it reports at
  handoff.

Rules around them: the operator's own words set the level ("report to me then fix" is
report-then-act, and "me" stays the operator), a root never lowers it when it writes a child's
brief, and the default for the infrastructure lab stays "act within the brief". A root also sends one short
"what each child will do, what it may change, where it stops" message when it opens children;
it did that here and it was useful.
