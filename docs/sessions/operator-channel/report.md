> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# operator-channel: report

Asked (brief.md): make clear to agents that the operator works only in the session chat, never opens
`ness inbox` for themselves, and never answers on the phone, which is only a notification and a
pointer; say what the operator-bound item is for. No new mechanism: phone replies, chat-answer
reminders and auto-expiry were proposed and rejected by the operator.

Delivered:
- ness-entry "Ask the operator" rewritten: the chat is the only operator channel; every ask is
  written in full in the chat, then filed with `ness send --to operator` as the harness's record
  (phone notification, the worker's "waiting on the operator", the trail); the answer arrives in
  chat and is recorded with `ness reply`. Checkpoints: report in the chat, file the same text.
- Post-compact line (profiles/core/post-compact.md) says the same, so it survives compaction.
- Reverses field batch B (2026-10-15), which had told roots "the operator reads alerts, not the
  chat. The chat copy is optional." Spec R-BLT-102 and R-BLT-108 updated; core_text and
  hook_session_start testscripts assert the new text and the absence of the old one.
- Evidence: `go test ./internal/cli/ ./profiles/ ./internal/sync/` ok; `ness sync --check` ok.

## Lessons
- Text that tells agents how the operator reads is product policy: a past round inverted it from
  one field observation; check it with the operator before changing it again.
