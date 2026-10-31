> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# withdraw: decisions

- **W1** Withdraw is `ness reply ITEM --as withdrawn --body REASON`, not a new verb: the CLI's
  shape stays one command per settlement, and the cheat sheet gains one value. The CLI routes it
  to a sender check (`runWithdraw`) instead of the addressee check.
- **W2** `returned` is added: cheap (one value, same migration) and clean (it settles the item and
  the sender gets the answer as a normal reply). `deferred` keeps its meaning, "still mine,
  later"; `returned` means "not mine, back to you".
- **W3** The recipient sees a withdraw as a `reply` item from the sender, body
  `withdrew ITEM: REASON`. A reply carries no obligation and cannot be replied to (R-MSG-050), so
  the notice never starts a new round trip.
- **W4** `close --withdraw` excuses the closer's own items in the close check and withdraws them
  in `finishClose`, after the landed check: a close refused later leaves nothing withdrawn. No
  notice is sent, since the recipient is being closed (its items are retired anyway). Reason is
  the note, `withdrawn at close` when there is none.
- **W5** Migration 6 rebuilds `items` (SQLite cannot alter a CHECK): create, copy, drop, rename,
  re-create the index. No table references `items`, so foreign keys need no special handling.
- **W6** A probe wake (sent by its owner to itself) is never withdrawn: the verdict answers it.
- **W7** Today only a manager sends a child items, so "items from someone else still block a
  `--withdraw` close" cannot be built through the CLI; the predicate is `Sender == closer` and the
  testscript proves the closer's items are the ones withdrawn.
