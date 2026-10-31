---
name: ness-notify
description: Post one Markdown note to a configured channel (Slack, Telegram, GitHub, command); separate from ness send because an external post is irreversible.
---

# ness notify

- `ness post --channel NAME --file NOTE.md [--thread KEY] [--dry-run]`

Channels are named in `.ness/ness.toml` under `[notify.channels]`. A token is never written
there: `token = "secret:NAME"` reads the user's secrets file (`ness secret set NAME` stores it
from stdin) and `token = "env:NAME"` an environment variable; `chat_id` and the other keys take
a literal or the same references. `ness doctor` names a missing one, never its value. The note is rendered for the platform and split on
paragraph and code-fence boundaries; a `NOTE.NAME.md` or `NOTE.KIND.md` next to it is sent
verbatim instead. `--thread KEY` keeps related posts in one thread: the first post starts it,
later ones reply in it. Always `--dry-run` first: it prints exactly what would be sent. A post
is irreversible.

## Enabling the feature
The worker reads `features` and the channel configuration at its start: after enabling
`notify`, or changing a channel, run `ness sync`, then restart the worker
(`systemctl --user restart ness-<project>`), or probe posts will not go out.
