---
title: Post a note to a channel
description: Configure a channel and post one Markdown note to it, rendered for Slack, Telegram, GitHub or your own command.
---

`ness post` sends one Markdown note to a configured channel. It is separate from `ness send`,
which stays inside ness: a post leaves the machine and cannot be taken back. This guide sets up a
`command` channel, which runs a program of yours, so you can test every step without posting to
a real service.

## Prerequisites

- The `notify` feature on, which renders the `ness-notify` skill:

  ```toml title=".ness/ness.toml"
  features = ["notify"]
  ```

## Add a command channel

1. Declare the channel. The command gets the note on stdin and the thread key in
   `NESS_THREAD_KEY`, and runs in the project root.

   ```toml title=".ness/ness.toml"
   [notify.channels.log]
   kind    = "command"
   command = "{ echo \"thread=$NESS_THREAD_KEY\"; cat; } >> notes.log"
   ```

2. Run `ness sync` and commit.

3. Write the note.

   ```markdown title="note.md"
   # Release 1.4

   Shipped **retry backoff**. See `src/retry.sh`.

   | check | state |
   |---|---|
   | disk | GREEN |
   ```

4. Print the payload without sending it.

   ```sh
   ness post --channel log --file note.md --dry-run
   ```

   A command channel gets the Markdown as written:

   ```text title="Output"
   # Release 1.4

   Shipped **retry backoff**. See `src/retry.sh`.

   | check | state |
   |---|---|
   | disk | GREEN |
   ```

5. Post it in a thread.

   ```sh
   ness post --channel log --file note.md --thread release-1.4
   ```

   ```text title="Output"
   posted 1 part to log
   ```

### Check that it worked

`notes.log` starts with `thread=release-1.4`, then the note. A later post with the same
`--thread` key replies in the same thread. A channel name that is not configured is refused:

```text title="Output (exit 4)"
ness: no such channel nope; configured: log
```

## Add a Slack, Telegram or GitHub channel

Tokens are references, never values. Write `secret:NAME` for a secret stored with
`ness secret set NAME`, or `env:VAR` for an environment variable.

```toml title=".ness/ness.toml"
[notify.channels.team]
kind       = "slack"
token      = "secret:slack-token"
channel_id = "C0123"

[notify.channels.phone]
kind    = "telegram"
token   = "env:TELEGRAM_TOKEN"
chat_id = "42"

[notify.channels.tracker]
kind  = "github"
token = "secret:github-token"
repo  = "acme/shop"
```

Store the Slack and GitHub tokens. `ness secret set` reads each from stdin, so it never appears
in your shell history:

```sh
ness secret set slack-token < slack-token.txt
ness secret set github-token < github-token.txt
ness secret list
```

```text title="Output"
github-token	referenced
slack-token	referenced
```

`--dry-run` shows what each platform would receive, and sends nothing. For the note above, Slack
gets markdown blocks, with the table in a code block:

```json title="Output (team)"
{"channel":"C0123","blocks":[{"type":"markdown","text":"# Release 1.4"},{"type":"markdown","text":"Shipped **retry backoff**. See `src/retry.sh`."},{"type":"markdown","text":"```\n| check | state |\n|---|---|\n| disk | GREEN |\n```"}]}
```

Telegram gets HTML, with the table in `<pre>`. The JSON escapes `<` and `>` as `\u003c` and
`\u003e`:

```json title="Output (phone)"
{"chat_id":"42","text":"\u003cb\u003eRelease 1.4\u003c/b\u003e\n\nShipped \u003cb\u003eretry backoff\u003c/b\u003e. See \u003ccode\u003esrc/retry.sh\u003c/code\u003e.\n\n\u003cpre\u003e| check | state |\n|---|---|\n| disk | GREEN |\u003c/pre\u003e","parse_mode":"HTML"}
```

A GitHub channel posts into an issue or pull request, so it needs `--thread` with its number:

```sh
ness post --channel tracker --file note.md --dry-run
```

```text title="Output (exit 2)"
ness: channel tracker posts into a thread; pass --thread KEY
```

```sh
ness post --channel tracker --file note.md --thread 12 --dry-run
```

```json title="Output (tracker)"
{"body":"# Release 1.4\n\nShipped **retry backoff**. See `src/retry.sh`.\n\n| check | state |\n|---|---|\n| disk | GREEN |"}
```

| Kind | Needs | Renders | Part limit | Thread |
|---|---|---|---|---|
| `slack` | `token`, `channel_id` | markdown blocks; tables as code blocks | 12,000 characters and 50 blocks | Replies under the first post |
| `telegram` | `token`, `chat_id` | HTML parse mode; tables as `<pre>` | 4,096 UTF-16 code units | Replies to the first post |
| `github` | `token`, `repo` | GitHub Markdown as is | 65,536 characters | Required: the issue or pull request number |
| `command` | `command`, optional `token` | Raw Markdown on stdin | none | The key, in `NESS_THREAD_KEY` |

A longer note is split into parts at paragraph and code-fence boundaries. A command channel's `token` is used only to
mask the secret in its error text, and a post that runs past 30 seconds is killed.

## Write a note for one channel

Put a file named `note.<channel>.md` in the directory of the `--file` you post, whatever that
file is called. When it is missing, ness looks for `note.<kind>.md`, such as `note.slack.md`. ness
sends the file verbatim: no rendering and no splitting.

Write it in the platform's own format, because a verbatim text keeps its Markdown marks. For the
`phone` channel, `note.phone.md` holds `Release 1.4 is out: **retry backoff**.`, and Telegram
receives the asterisks as they are:

```json title="Output (phone, dry run)"
{"chat_id":"42","text":"Release 1.4 is out: **retry backoff**.\n","parse_mode":"HTML"}
```

Telegram reads that text as HTML, so write `<b>retry backoff</b>` there instead.

## Tell the operator when a session needs them

`[notify] operator` names the channel the worker uses to alert you. The alert says which session
needs you; you answer in that session's chat.

```toml title=".ness/ness.toml"
[notify]
operator = "phone"
```

`ness doctor` checks the channel and every token reference. With `TELEGRAM_TOKEN` exported and
both secrets stored, the three references resolve:

```text title="Output"
ok	operator-notify	phone (telegram)
ok	secrets	3 references resolve
```

With `TELEGRAM_TOKEN` unset, both checks fail, and `secrets` names the fix:

```text title="Output (exit 1)"
fail	operator-notify	phone: channel phone: token: TELEGRAM_TOKEN is not set	fix: see the secrets check
fail	secrets	notify.channels.phone.token: env:TELEGRAM_TOKEN missing	fix: export TELEGRAM_TOKEN
```

Remove a stored secret with `ness secret unset NAME`.

Next: [Freeze acceptance files](./contracts.md).
