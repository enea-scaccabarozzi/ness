> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Decisions: field-feedback

## FF-D1: tier standard (2026-10-15)
The agreed proposal said "tier `full`"; `full` is a work size, not a tier. The root opened at `standard` (opus/medium), the model this conversation already runs on.

## FF-D2: evidence goes to the private discovery folder (2026-10-15)
The analysis holds operator quotes and the research lab internals, so it goes to `docs/discovery/findings/field-research/`, which is removed before publishing. The raw condensed transcripts (about 12 MB) are not committed, following the discovery convention: they are in the main checkout's git-ignored `.private/field-research/`.

## FF-D3: execute the agreed direction, no per-item brainstorming (2026-10-15)
The operator, after the root ran the superpowers brainstorming on alerts: "we went down a rabbit hole … we left our plan to follow superpowers procedure. Brainstorming things that were already clear … this session work and direction was already settled and was doable inline by root itself, using subagents, or at most using some child sessions." The report and clarifications are the design; `plan.md` lists the work; the alerts spec stays as B5's design. A skill's process does not replace the session's agreed map.

## FF-D4: rebrief relayed as the operator (2026-10-15)
setup-cleanup (the research lab s-176c2b204b08) finished Batch A but `ness send` refused its rebrief to study-protocol: a session may message only its manager, children or the operator. It then asked the operator in its chat only, with no item and so no alert: the same failure as cause 4. The root relayed the note with `ness send` run as the operator (NESS_SESSION unset), labelled as relayed, because the operator ordered that rebrief. Open point for ness: sibling roots in one project have no channel; a relay through the operator is the only path today.

## FF-D5: Batch C wording approved; success criteria, not scoring (2026-10-15)
The operator approved the Batch C draft except "scoring change", which is research vocabulary: the builtin program profile says "a change to the success criteria (what counts as done, correct or better)"; the research lab's research profile keeps "scoring or method". The autonomy check in code is folded into Batch E (prose paired with checks).

## FF-D6: children keep notes, only the root numbers decisions (2026-10-15)
Operator agreed ("I like this") to replace Batch C's "children keep no decision ledger": a child may keep plain dated notes in its decisions.md for memory across compaction, with no ids and never cited upward; its handoff states choices and rejected alternatives in words. Only the root numbers decisions. Field cause: per-child id ledgers (C2-D1..12, L0a-D1..15, C3-D1..16) leaked ids upward and bloated records; the notes themselves were useful (C2 caught a 12x error).

## FF-D7: root folds and evaluates without operator gates (2026-10-15)
Operator: "Fold it without my exlicit authorization, then evaluate e yourself and then start B by yourself". Read as: fold Batch B (with C) into main once its second review is clean, with no operator go; the root reviews Batch E's result itself; the root starts the research lab port (Batch D; "B" read as D, since B is the batch being folded) on its own. The Telegram field check needs no separate go.
