> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# operator-loop plan

Each step is its own commit; `make validate` passes at every commit; tests are written from
specs.md before the code they judge.

1. **Spec.** Amend and add the ids in specs.md to `docs/spec/*.md`.
2. **CX-17 send/reply.** Testscripts first (`send_routes`, `reply`, new `send_operator`), then
   `resolveRecipient`, the copy in `runSend`, `caller.addressed`, `Store.Reply`.
3. **CX-16 config and doctor.** `Notify.Operator` with validation; `checkOperatorNotify`;
   testscripts `config_errors`, `doctor`.
4. **CX-16 worker.** `Worker.Notify` port; `Store.OpenToOperator`; idle and blocked episodes in
   `pane`; `notify_operator_test.go` on the bench; wiring in `cli/worker.go`.
5. **CX-19 close.** `claude.OperatorSaid` over a fixture transcript (unit); `--authorized` in
   `close.go`; `CloseTab` keeps a root's tab; testscript `close_authorized`, amend `close_adopted`.
6. **Skill text.** `profiles/core/skills/ness-entry`, `ness-launch`, cheat sheet; `ness sync`;
   `core_text.txtar` greps.
7. **Docs and report.** CHANGELOG, README (Telegram config), report.md with the exact config.
