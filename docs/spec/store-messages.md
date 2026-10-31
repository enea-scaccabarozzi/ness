> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Store and messages (MSG)

Packages: `internal/store`, cli `send`, `reply`, `inbox`. Sources: §3 (messages), §5 (Messages,
Cut table), §6 (runtime files), §13 (items 1–2), §14 (wake rules), §15 (no import), D7.

## Store

- **R-MSG-001** The store MUST be a SQLite database at `<git-common-dir>/ness/state.sqlite3`,
  shared by the primary checkout and every worktree.
  Verify: testscript — `ness send` from a worktree, `ness inbox` from the primary checkout as
  the recipient, sees the item; the file exists at that path.
- **R-MSG-002** The store MUST use a pure-Go SQLite driver (`modernc.org/sqlite`) so the binary
  builds with `CGO_ENABLED=0`.
  Verify: unit/CI — `CGO_ENABLED=0 go build ./...` succeeds; `go list -deps` contains no cgo
  SQLite package.
- **R-MSG-003** Concurrent writes from separate processes MUST neither lose nor duplicate items.
  Verify: unit (`-race`) + testscript — 20 concurrent `ness send` processes with distinct bodies
  produce exactly 20 items.
- **R-MSG-004** ness MUST NOT read or import the old harness stores (§15 step 2).
  Verify: review — no code path opens files other than its own store.
- **R-MSG-005** Store invariants (one disposition per item; items reference existing senders and
  recipients or are retired) MUST hold under concurrent `send`, `reply`, `close` and worker
  passes.
  Verify: unit (`-race`, `-count=4`) — randomized concurrent operations, invariant checked after.

## `ness send`

- **R-MSG-010** `ness send` MUST accept `--to ID|manager|operator`, `--body TEXT|@FILE`,
  `--intent request|fyi|decision|handoff|checkpoint`, `--page URL` and `--urgent`.
  Verify: testscript — each value accepted; other intents refused.
- **R-MSG-011** `--intent` MUST default to `request`.
  Verify: testscript — `ness inbox` of the recipient shows `request`.
- **R-MSG-012** `--body @FILE` MUST read the body from FILE; `--body TEXT` MUST use TEXT
  literally.
  Verify: testscript — `--show` returns the file's exact bytes.
- **R-MSG-013** `--to manager` MUST resolve to the sender's manager; for a root, the manager is
  the operator.
  Verify: testscript — child sends to `manager` → the root's inbox; root sends to `manager` →
  an operator item.
- **R-MSG-014** A send whose idempotency key (derived from sender + content) matches any existing
  item, open or disposed, MUST NOT create a second item (the key is permanent).
  Verify: testscript — the same send twice, and again after the first is resolved → one item.
- **R-MSG-015** Different content from the same sender MUST create distinct items.
  Verify: testscript — two bodies → two items.
- **R-MSG-016** `ness send` MUST print exactly one line, `sent <id> to <to>`, or
  `already sent <id>` for an idempotent repeat, and MUST NOT print the body.
  Verify: testscript — stdout matches `^sent m-[a-z2-7]{8} to \S+$` then
  `^already sent m-…$`; the body string is absent from stdout and stderr.
- **R-MSG-017** A send to a session that is not the sender's manager or one of its children MUST
  be refused (siblings talk through the manager), except a root to another root (R-MSG-073); `--to operator` MUST be accepted from any
  session, for what its manager cannot settle (CX-17; amended from "only a root").
  Verify: testscript — sibling-to-sibling send exits 4; child and root `--to operator` succeed;
  no item created by the refusals.
- **R-MSG-051** A child's send `--to operator` MUST also create one `fyi` to the child's manager
  whose body is `[copy of <id> to operator]`, a blank line, then the body; a root's send MUST
  create no copy, and an idempotent repeat (R-MSG-014) MUST create no second copy.
  Verify: testscript — child sends to the operator → the manager's inbox lists one fyi with that
  header; root sends → the operator's inbox gains one item and nothing else; the same child send
  again → `already sent`, still one copy.
- **R-MSG-073** A root's send to another open root of the same project MUST be accepted, and MUST
  also create one `fyi` to the operator whose body is `[copy of <id> from <sender> to <recipient>]`,
  a blank line, then the body; an idempotent repeat (R-MSG-014) MUST create no second copy, and
  the copy MUST NOT alert (fyi is outside `UnalertedToOperator`, R-MSG-067). A root's send to
  itself, a root's send to another root's child and a child's send outside its tree stay refused
  (R-MSG-017). Replies run between the two roots as between any sender and recipient (R-MSG-022,
  R-MSG-050). (FF3-D10: roots relaying through the operator's paste was refused 5 times.)
  Verify: testscript `send_peer` — root A to root B → B's inbox and one operator fyi with that
  header; the repeat → `already sent`, no second copy; B replies, only A settles the reply; a child
  to the other root and a root to the other tree's child exit 4; unit — `routeRefusal` table;
  `TestUnalertedToOperatorSelectsTheOperatorsOpenAsks` keeps fyi out of the alerts.
- **R-MSG-018** A send to a closed or unknown session MUST be refused.
  Verify: testscript — `--to` a closed id exits non-zero.
- **R-MSG-019** `ness send` MUST NOT accept a `--request-key` flag (key is derived).
  Verify: testscript — see R-CFG-031.

## `ness reply`

- **R-MSG-020** `ness reply ITEM` MUST answer and dispose the item in one step, with `--body`
  and `--as resolved|declined|deferred|accepted|returned|withdrawn` (the last two CX-21).
  Verify: testscript — after reply the item has that disposition and the answer body is stored.
- **R-MSG-021** `--as` MUST default to `resolved`.
  Verify: testscript — reply without `--as` → disposition `resolved`.
- **R-MSG-022** The reply's answer MUST reach the original sender as a new item with intent
  `reply`, which carries no obligation.
  Verify: testscript — child's request answered by the root; the child's inbox lists a `reply`
  item with the body; the child's close is not blocked by it.
- **R-MSG-023** `ness reply` MUST refuse an item not addressed to the replying session; operator
  items MAY be replied to by the operator (a command run with no `NESS_SESSION`), by the item's
  sender or by the sender's manager, and by nobody else (CX-17; amended from "any root").
  Verify: testscript — a child replies to an item addressed to its sibling → exit 4; operator
  replies outside a session to an operator item → ok; the sender's manager → ok; a sibling root
  → exit 4.
- **R-MSG-024** The sender of an item addressed to the operator, root or child, MUST be able to
  record the operator's chat answer on it with `ness reply` (CX-17).
  Verify: testscript — child sends `--to operator`; child runs `ness reply ITEM --body …`; the
  item is disposed with that answer.
- **R-MSG-052** When the replier is the item's sender, `ness reply` MUST create no reply item (the
  asker already knows the answer); any other replier's answer reaches the sender (R-MSG-022).
  Verify: testscript — after the sender records the answer its inbox is empty; after the
  operator answers, the sender's inbox lists one reply.
- **R-MSG-025** `ness reply` on an already-disposed item MUST be refused.
  Verify: testscript — second reply exits non-zero; first answer unchanged.
- **R-MSG-050** `ness reply` by the recipient of an item of intent `reply` MUST settle it with
  the given disposition and answer and MUST deliver nothing back, so two sessions can never bounce
  replies; the conversation continues with a new `send`. (Amended, field feedback 3: the refusal
  was hit about 15 times by agents clearing their inbox; settling keeps the no-bounce guarantee.)
  Verify: testscript — the asker settles the reply it received → ok, disposition `resolved`, the
  replier's inbox stays empty; unit — `Reply` on a reply disposes it and returns no reply id.
- **R-MSG-060** The sender of an open item MUST be able to settle it with
  `ness reply ITEM --as withdrawn --body REASON`: the item takes disposition `withdrawn` with
  REASON as its answer, is no longer open and no longer an obligation. Only the sender may
  withdraw; the recipient, a sibling or anyone else is refused (exit 4). (CX-21, option D: a
  manager whose picture changed no longer has to ask the recipient to decline.)
  Verify: testscript — the recipient and a sibling are refused; the sender's withdraw sets
  `withdrawn` and the answer; a deferred item can be withdrawn; unit — `Withdraw` settles it and
  `Obligations` drops it.
- **R-MSG-061** A withdraw MUST deliver the reason to the recipient as one item of intent `reply`
  from the sender, body `withdrew ITEM: REASON`; the operator is a recipient like any other.
  Verify: testscript — the recipient's inbox lists the notice; a child's withdrawn request to the
  operator shows in the operator's inbox; unit — the notice's sender, recipient and body.
- **R-MSG-062** A withdraw of a disposed item, of a reply item or of a probe wake MUST be refused
  (exit 4) and change nothing.
  Verify: testscript — a second withdraw → `already withdrawn`; withdrawing a reply → `is a
  reply`; unit — `ErrDisposed`, `ErrReplyToReply`.
- **R-MSG-063** Migration 6 MUST rebuild `items` with the `returned` and `withdrawn`
  dispositions allowed, keeping every row and column.
  Verify: unit — a version-5 store with a deferred, retired probe item migrates with the row
  intact, and both new dispositions are then accepted.
- **R-MSG-064** `ness reply --as returned` MUST settle the item (the recipient hands the work
  back to the sender, who gets the answer as a reply, R-MSG-022); unlike `deferred` it is no
  longer open. (CX-21 ruling, decision W2.)
  Verify: testscript — a handoff returned → disposition `returned`, the sender's inbox lists the
  answer.
- **R-MSG-065** A `checkpoint` body MUST contain the headings `## Done`, `## Now` and `## Next`;
  `send` MUST refuse (exit 2) a body without any of them, naming the missing ones, and create
  nothing. `--page URL` MUST append the line `Page: URL` to the body, and MUST be refused (exit 2) with any
  other intent and with a body that already has a `Page:` line (the message names both). A heading
  inside a fenced code block (``` or ~~~) is not a heading, for the required ones and for `## Ask`.
  Verify: unit — `CheckpointMissing`, `HasAsk`, `Section`, `PageOf`; testscript `send_checkpoint`.
- **R-MSG-066** Only a root MUST send a `checkpoint`, and only `--to operator`; any other sender or
  recipient MUST be refused (exit 4).
  Verify: testscript `send_checkpoint` — a child, and a root to a child, exit 4.
- **R-MSG-067** Migration 7 MUST rebuild `items` with intent `checkpoint` allowed and the columns
  `ask` (0/1) and `alerted_at`, keep every row, and stamp `alerted_at = created_at` on existing
  rows so an upgrade replays no alert. `UnalertedToOperator` lists the open operator-bound
  request, decision, handoff and checkpoint items, and the unshown replies to operator-sent items
  (a withdrawal's notice is born alerted), not yet alerted, oldest first; `MarkAlerted`
  stamps one durably.
  Verify: unit — a version-6 store migrates with both rows intact and nothing unalerted; the stamp
  survives a reopen.
- **R-MSG-071** `LastLanding(root)` MUST return the latest `integrated_at` of any descendant of
  `root` (children, grandchildren and deeper; closed sessions included), and the zero time when
  none has landed. `root`'s own integration and sessions of other roots do not count.
  Verify: unit — `internal/store/landing_test.go`: grandchild, closed child, other tree, nothing landed.
- **R-MSG-072** `LastCheckpoint(root)` MUST return the creation time of the latest `checkpoint`
  item sent by `root` to the operator, and the zero time when none. Checkpoints of other senders,
  and items of other intents, do not count.
  Verify: unit — `internal/store/landing_test.go`.
- **R-MSG-070** A `handoff` body MUST contain the headings `## Done`, `## Now` and `## Next`, as a
  checkpoint's does (R-MSG-065, fenced headings excluded): `send --intent handoff` MUST refuse
  (exit 2) a body without any of them, naming only the missing ones, ending in
  `(skill ness-entry, section Communicate)`, and create nothing. Other intents need no headings.
  Verify: testscript `send_handoff`.

## `ness inbox`

- **R-MSG-030** `ness inbox` MUST print exactly one line per open item addressed to the calling
  session, with id, from, intent, age and the first 80 characters of the body.
  Verify: testscript — three items → three lines, each with those five fields.
- **R-MSG-031** Newlines and tabs in the body excerpt MUST be replaced so each item stays on one
  line, and the excerpt MUST be at most 80 characters (runes) of body.
  Verify: unit + fuzz — arbitrary bodies; the excerpt has no `\n`, length ≤ 80 runes.
- **R-MSG-032** `ness inbox` MUST NOT print full bodies.
  Verify: testscript — a 10 KB body; stdout under 200 bytes per item.
- **R-MSG-033** `ness inbox --show ITEM` MUST print that item's full body and nothing about
  other items. `--show ITEM ITEM...` MUST print each named item's full body, in the order named,
  each under a header line (`--- ` then id, from, intent and age, tab-separated), and mark each
  shown; it MUST check every id first, so one it refuses prints and marks nothing.
  Verify: testscript — output equals the body (plus a header line if specified);
  `inbox_many.txtar` — two items under their headers, a bad id among good ones shows nothing.
- **R-MSG-034** `ness inbox --boundary NOTE [--until TIME]` MUST record that the session's state is
  written down and that it asks to be compacted when idle (consumed by the worker, R-WRK-030), and
  a parked record (when, NOTE, the until-time) that the compaction does not clear and a new boundary
  replaces (R-WRK-099). `--until` takes RFC 3339, `YYYY-MM-DD` or `YYYY-MM-DDTHH:MM` (UTC); it is
  refused without `--boundary` or not in the future, and a refused call parks nothing. Migration 12
  adds the record; a boundary from before it is not a park.
  Verify: testscript — store shows a pending boundary for the session with NOTE; `inbox_until` —
  each time form, the refusals, a plain boundary with no until-time; unit `waits_test` — migration
  12 keeps rows.
- **R-MSG-035** An empty inbox MUST print nothing and exit 0.
  Verify: testscript — stdout empty, exit 0.
- **R-MSG-036** Items listed by `ness inbox` MUST be marked pulled, so the worker never hints them
  again; a pulled item MUST stay open until disposed.
  Verify: testscript — after `inbox`, the store marks the item pulled; a second `inbox` still
  lists it.
- **R-MSG-037** `ness inbox --unread` MUST print every item `ness inbox` lists that was never
  shown, each in full under the R-MSG-033 header line, and mark each shown; with none, it prints
  nothing. It is the one call that reads everything a wake brought, instead of a shell loop over
  `--show` (field round 3: 54 such loops refused in a week).
  Verify: testscript `inbox_many.txtar` — only the unshown item printed; a second call prints
  nothing.

## Decided details

- **R-MSG-040** `request`, `decision` and `handoff` items, and a `checkpoint` with an `## Ask`
  section, MUST create an obligation for the recipient until disposed; otherwise the intent is a
  label only. An `## Ask` section that is empty, or whose first line is "Nothing", "None" or
  "N/A" (any case, with or without punctuation, or "Nothing blocking/asked/needed" and nothing
  more on the line: "Nothing blocking, but please approve X" is an ask), asks nothing.
  Verify: testscript — one of each blocks the recipient's close; disposing all unblocks it; unit —
  a checkpoint with `## Ask` is an obligation; one without, with an empty one or with "Nothing."
  is not.
- **R-MSG-041** An `fyi` item, and a `checkpoint` without `## Ask`, MUST carry no obligation: it
  MUST NOT block close, and it MUST stay listed in `ness inbox` until shown with `--show` or
  replied to.
  Verify: testscript — fyi present → close allowed; after `--show` it is no longer listed; unit —
  the same for a checkpoint without `## Ask`.
- **R-MSG-042** `send --urgent` MUST make the worker hint that item at the next pass outside
  batching and the quiet window, still never typing into a non-empty box (R-WRK-014).
  Verify: unit (synctest) — the box changed 10 s ago and is empty → urgent hint delivered; box
  with text → not delivered.
- **R-MSG-043** `ness reply --as deferred` MUST leave the item listed in `ness inbox`, marked
  deferred, and still an obligation.
  Verify: testscript — inbox line shows `deferred`; close still blocked.
- **R-MSG-044** Item ids MUST be `m-` followed by 8 lowercase base32 characters.
  Verify: unit + fuzz — generated ids match `^m-[a-z2-7]{8}$`; unique across 10,000
  (40 random bits collide at 100,000 draws in about 0.45% of runs, which would flake the gate;
  at 10,000 the odds are 4.5e-5; the primary key guards uniqueness in the store).
- **R-MSG-045** Ages in `inbox` MUST be printed as `<n>m`, `<n>h` or `<n>d` (largest unit, floor);
  the line layout MUST be frozen by golden tests.
  Verify: unit — 59 s → `0m`, 3 h 10 m → `3h`, 50 h → `2d`; golden inbox output.
- **R-MSG-046** The store MUST carry a schema version and apply forward-only versioned migrations
  automatically on open, after the backup of R-UPD-010; a store newer than the binary MUST be
  refused.
  Verify: unit — a v1 store fixture opens with a v2 binary and is migrated; a newer store → exit 4.
- **R-MSG-047** `ness inbox` run outside a session (no `NESS_SESSION`) MUST list operator items.
  Verify: testscript — root sends `--to operator`; `inbox` with no session env lists it.
- **R-MSG-048** `ness inbox --json` MUST list the same items as the text output.
  Verify: testscript — ids equal.
- **R-MSG-049** `ness send --intent reply` MUST be refused (reply items are created only by
  `ness reply`).
  Verify: testscript — exit 2.
