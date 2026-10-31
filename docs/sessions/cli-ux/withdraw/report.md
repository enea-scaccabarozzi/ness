> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# withdraw: report

CX-21, option D: the sender of an open item can withdraw it, and a manager can close a child
while withdrawing what it sent the child.

## Delivered
1. `ness reply ITEM --as withdrawn --body REASON`: only the sender may use it. The item becomes
   `withdrawn` with the reason as its answer; it is no longer open and no longer an obligation.
   The recipient gets a `reply` item from the sender reading `withdrew ITEM: REASON`. A disposed
   item, a reply item and a probe wake are refused (R-MSG-060..062).
2. `ness close ID --withdraw [--note REASON]`: the closer's own open items to the session
   (deferred ones too) are left out of the close check, and after the landed check they are
   settled as `withdrawn` with the note as the reason. Without the flag they still block, and the
   refusal now names `--withdraw` (R-SESS-130, R-SESS-131).
3. `returned` disposition, added (W2): it settles the item and the sender gets the answer. This
   is how a child hands work back; `deferred` keeps meaning "mine, later" (R-MSG-064).
4. Migration 6 rebuilds `items` to allow the two new values, keeping every row (R-MSG-063).
5. Spec ids R-MSG-060..064 and R-SESS-130..131; the cheat sheet; one line each in ness-entry and
   ness-launch; `ness sync` re-rendered with this checkout's binary.

## Evidence
- Unit: `internal/store/withdraw_test.go` (a v5 store migrates with a deferred, retired probe
  item intact, after which both new values are accepted; `Withdraw` settles the item, sends the
  notice and refuses a second withdraw and a reply).
- Testscripts: `reply_withdraw.txtar`, `close_withdraw.txtar`, plus one more check each in
  `probe_verdict.txtar` (a wake is not withdrawn), `reply.txtar` (the list of accepted values)
  and `hook_session_start.txtar` (the cheat sheet).
- Gate: `mise exec -- make validate` (result recorded at commit).

## Decisions
W1..W7 in `decisions.md`.

## Lessons
- `ness sync` through the installed `ness` re-renders from the profiles embedded in *that* build.
  After editing `profiles/`, run `go run ./cmd/ness sync`.
- Today a child's inbox can only receive from its manager, so a `--withdraw` close cannot leave
  anyone else's item blocking; if siblings or the operator ever reach children directly, the
  `Sender == closer` predicate already covers that case.
- With four sibling gates running at once the load reached about 48, and testscripts timed out
  (`context deadline exceeded`) without any assertion failing. Re-run the gate rather than
  chasing these.
