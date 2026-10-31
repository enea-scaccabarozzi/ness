> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# operator-loop decisions

- **OL-1** The copy a child's operator item sends its manager is an fyi (no obligation) with the
  header `[copy of <id> to operator]`: the manager's map stays current without a second question
  to answer. A root's send makes no copy (its manager is the operator).
- **OL-2** An operator item may be answered by the operator, its sender or the sender's manager;
  "any root" (the old R-MSG-023) is dropped: a root has no business closing a stranger's question.
  The sender recording an answer gets no reply item back: it already knows.
- **OL-3** The operator channel is `[notify] operator = NAME`, a channel of `[notify.channels]`,
  behind the `notify` feature like every post: one gate, one skill text, one worker wiring.
- **OL-4** "Blocked for a short while" is a 2-minute constant: the quiet window (60 s) is too
  short for a prompt the operator may be reading, and a config knob nobody asked for is YAGNI.
- **OL-5** The authorization check reads the Claude Code transcript, where typed lines carry
  `promptSource: "typed"` and `origin.kind: "human"`; ness's own typed texts (hints, /compact,
  /rename, "continue", the launch prompt) are excluded by their fixed shapes. Codex has no
  transcript: a Codex root cannot close itself and is told to ask the operator.
- **OL-6** Tests are written from specs.md before the code (the judge precedes the implementer
  within one session); `ness-review` checks the result at handoff.

## OL-7 The notification names the newest open item
When a session has several open items to the operator, the line quotes the newest one: it is
what started the idle episode. Alternative (oldest) rejected: the operator would be told about a
question already known.

## OL-8 Blocked threshold is inclusive
R-WRK-081's Verify fixes "2 min → one line", so the check is `now - blockedSince >= 2 min`.

## OL-9 Transcript lines without the new fields count as typed
Claude Code transcripts predating `promptSource` and `origin` have neither; such user lines with a
string content count as the operator's. Any other `promptSource` (`system`) or `origin.kind` is
not. The ness-typed texts are excluded by prefix (`ness: `, `/compact`, `/rename `, `You are ness
session `) and the exact resume text.

## OL-10 A refused landed check on a root says "nothing was touched"
The root's tab is never closed (R-SESS-122), so the "tab is closed, `ness open --resume`" wording
holds only for a child; `CloseTab` reports whether it closed and the message follows.

## OL-11 The transcript check stops mistakes, not a determined agent (review M1)
An agent with a shell can append a typed-looking line to its own transcript, or type into its
own pane through herdr. `--authorized` therefore guards against an agent closing its root by
misreading or inventing an authorization, which is the CX-19 failure seen in the field; a root
that forges evidence is a policy breach the operator reads in the transcript. Not closed in code.

## OL-12 Whole words, at least two, typed-by-human only (review H1, H2)
Substring matching accepted a blank or a common word, and lines without the `promptSource` /
`origin` fields let slash commands and local command output through. The check now wants at
least two whole consecutive words, only in lines the client marks `promptSource: typed`,
`origin.kind: human`, and never a text starting with `<` (how Claude Code records its own
commands). Old transcripts without those fields cannot authorize a close; the operator types the
words again.

## OL-13 The operator closing a running launched root (review M2): open
With a root's tab kept (R-SESS-122), nothing stops its agent between the landed check and the
worktree removal; adopted roots had this already. A close by the operator normally lands on an
idle root; refusing while the pane is `working` is a one-line guard the manager may want in the
delivery sibling's close changes. Left open, reported.
