---
title: Your first week
description: Keep the worker running, get told on your phone, read status and cost, and start shaping your harness.
---

After the [Quickstart](./quickstart.md) you have one repository with ness and a worker. This
page covers what you set up once and what you check every day. As in the Quickstart, the samples
are the plain renderer's output (`NESS_UI=plain`), except where a command passes `--ui rich`.

## Keep the worker running

The worker is a systemd user unit named `ness-<project>`, enabled for your user session, with
`Restart=always`.

```sh
systemctl --user status ness-acme
```

```text title="Output"
● ness-acme.service - ness worker for acme
     Loaded: loaded (/home/you/.config/systemd/user/ness-acme.service; enabled; preset: enabled)
     Active: active (running) since Sat 2026-10-31 14:36:02 CET; 3min 4s ago
…
```

`ness status` prints the worker's heartbeat on its last line. The worker beats every 5 s.
`ness doctor` fails the `worker` check when the last beat is older than 60 s, and prints the fix.

After you upgrade the ness binary, restart the worker with `systemctl --user restart ness-acme`.
Until you do, `ness doctor` fails `worker` with `running <old version>, this binary is <new
version>`. If the binary moved to a new path, run `ness worker install` again.

## Get told on your phone

The worker tells you when a session needs you: a question, a decision, a finished root. It posts
to one channel, and you answer in the session's chat, never on the phone. The phone only points
you to the session.

1. Turn on the `notify` feature and name the channel in `.ness/ness.toml`. This example uses
   Telegram. Slack, GitHub and a local command work the same way.

   ```toml title=".ness/ness.toml"
   features = ["notify"]

   [notify]
   operator = "phone"

   [notify.channels.phone]
   kind = "telegram"
   token = "secret:telegram-token"
   chat_id = "123456789"
   ```

2. Render the change:

   ```sh
   ness sync
   ```

   ```text title="Output"
   create .agents/skills/ness-notify/SKILL.md
   ```

3. Store the bot token. In a terminal, `ness secret set` prompts `value (not echoed): ` and
   reads one hidden line. Piped, it reads the value from stdin.

   ness keeps the token in `~/.config/ness/secrets/<project>.toml`, outside the repository. The
   token never goes into `ness.toml`: a literal there is refused.

   ```sh
   ness secret set telegram-token
   ness secret list
   ```

   ```text title="Output"
   telegram-token	referenced
   ```

4. Check the channel:

   ```sh
   ness doctor
   ```

   ```text title="Output"
   …
   ok	operator-notify	phone (telegram)
   ok	secrets	1 reference resolves
   …
   ```

5. Preview a post without sending it:

   ```sh
   printf '## Done\nThe first note.\n' > note.md
   ness post --channel phone --file note.md --dry-run
   ```

   ```text title="Output"
   {"chat_id":"123456789","text":"\u003cb\u003eDone\u003c/b\u003e\n\nThe first note.","parse_mode":"HTML"}
   ```

   The JSON escapes `<` as `\u003c` and `>` as `\u003e`. Telegram gets `<b>Done</b>` and shows
   it in bold. Drop `--dry-run` to send it.

## Read the session tree

```sh
ness status --all --ui rich
```

The rich view draws the tree with colours. Without the flag, `NESS_UI=plain` prints one
tab-separated line per session, as in the Quickstart.

```text title="Output"
 ness  acme · base main   ● worker 0.x.y · beat 2s

○ s-e852f9ffec34  hello  closed

probes: feature off
```

| Command | Shows |
|---|---|
| `ness status` | The open sessions, the probes and the worker |
| `ness status --all` | Closed sessions too |
| `ness status --watch` | A live view, refreshed every 2 s, in a terminal only |
| `ness status --json` | One JSON object, for scripts |

You never read `ness inbox`. A root reports to you in its chat, and the phone points you at the
session that needs you. The inbox is how sessions read the items sent to them, and the record of
what was sent.

## Read the cost

`ness stats` groups cost by session by default. `--by` takes `session`, `tier`, `role`, `rung`
or `tool`, and `--since` takes a date or a duration.

```sh
ness stats --by rung --since 7d
```

```text title="Output"
rung	sessions	turns	compactions	context	input	output	cache_read	cache_write	cost
economy	1	2	0	53936	12	849	235211	28181	$0.14
total	1	2	0	53936	12	849	235211	28181	$0.14
prices	2026-09-25
```

A week of real work shows which rungs carry the cost. If the base rung is too expensive or too
weak for your project, move it: [Opinions in the defaults](../why/defaults.md#the-model-ladder).

## Start shaping your harness

The shipped defaults are one set of opinions. Read [Opinions in the
defaults](../why/defaults.md) and change the ones that do not fit your project. Each change is a
line in `.ness/ness.toml` or a file in your own profile, `.ness/profiles/local/`, reviewed as a
diff like any other code.

Next: [Sessions](../concepts/sessions.md).
