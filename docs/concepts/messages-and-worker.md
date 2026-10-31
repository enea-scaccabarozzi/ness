---
title: Messages and the worker
description: Sessions talk through one store with send, reply and inbox. The worker wakes idle sessions, compacts them, resumes them after limits, reports stalls and tells you when a session needs you.
---

A **message** is one item in the project's store, sent from one session to another or to the
operator. The [worker](../start/quickstart.md#4-install-the-worker) delivers those items: it
types a short hint into an idle session's terminal, so no session has to poll.

```text
 session A ── ness send ──▶ store (SQLite, shared by every worktree) ◀── ness inbox ── session B
                                  │
                               worker, every 5 s
                                  │
                ┌─────────────────┼──────────────────┬────────────────────┐
           hint B when idle   /compact at a     resume after a       stall report,
                              boundary          usage limit          alert to your phone
```

## The store

The store is a SQLite database inside the repository's git directory. The primary checkout and
every worktree share it, so a child in its worktree and its manager in another see the same
items. Concurrent sends from separate processes neither lose nor duplicate an item.

A send is idempotent. The same sender sending the same content twice gets `already sent <id>`
and no second item.

## Send, reply, inbox

Three commands carry every conversation between sessions:

| Command | What it does |
|---|---|
| `ness send --to ID\|manager\|operator --body TEXT\|@FILE` | Creates one item and prints `sent <id> to <to>`. |
| `ness inbox` | Lists your open items, one line each: id, sender, intent, age and the first 80 characters. |
| `ness inbox --show ITEM` or `--unread` | Prints full bodies. |
| `ness reply ITEM --body TEXT --as DISPOSITION` | Answers and settles the item in one step. The sender gets the answer as a `reply` item, unless the sender is the one replying. |

`ness inbox` never prints full bodies, so a long message costs no context until you ask for it.
An item listed by `ness inbox` is marked pulled, and the worker never hints it again.

A session may send to its manager, its children and the operator. Siblings talk through their
manager. Two rules keep the tree informed:

- When a child sends to the operator, its manager gets an `fyi` copy.
- A root may send to another open root of the same project. The operator gets an `fyi` copy.

### Intents

Every item carries an **intent**. It decides whether the item is an obligation, an item that
blocks the recipient's close until it is settled.

| Intent | Obligation | Rules |
|---|---|---|
| `request` (the default) | yes | |
| `decision` | yes | Used for reserved decisions: the options and a recommendation. |
| `handoff` | yes | The body needs `## Done`, `## Now` and `## Next`. |
| `fyi` | no | Stays listed until shown or replied to. |
| `checkpoint` | only with `## Ask` | Root to operator only. Needs `## Done`, `## Now` and `## Next`. `--page URL` appends a page link. |
| `reply` | no | Created by `ness reply` only. |

`ness reply --as` settles an item in one of six ways:

| Disposition | Meaning |
|---|---|
| `resolved` (the default) | Done. |
| `accepted` | Taken on; the answer is still owed. |
| `declined` | Will not do it. |
| `deferred` | Still open and still an obligation; stays in the inbox. |
| `returned` | Handed back to the sender. |
| `withdrawn` | The sender cancels its own item, with a reason. |

### Waiting without polling

To wait for a reply or a child, a session pulls its inbox once and ends its turn. The worker
wakes it when something arrives.

The pre-tool hook refuses polling of `ness inbox` or
`ness status`, from a shell loop or from Monitor, ScheduleWakeup or CronCreate. A loop or Monitor
that reads anything else passes. [Guides and sensors](./guides-and-sensors.md#the-pre-tool-hook)
lists every refusal.

To wait on a date instead of an item, a session parks itself:

```sh
ness inbox --boundary "state in plan.md; integrate after Friday" --until 2026-11-07
```

A bare date means midnight UTC. The worker leaves a parked session alone until the time passes.

## The worker

The worker runs as a systemd user unit, one per project. `ness worker install` writes the unit
under `~/.config/systemd/user/`.

That unit and the secrets file of `ness secret set` are the only
files ness writes outside your repository. `ness worker run` runs the worker in the foreground
when you have no systemd.

```sh
ness worker install
```

```text title="Output"
installed /home/you/.config/systemd/user/ness-acme.service (ExecStart=/home/you/.local/bin/ness worker run); check with ness doctor or systemctl --user status ness-acme
```

The worker writes a heartbeat to the store on every pass. `ness status` shows it a few seconds
after the install:

```text title="ness status"
probes: none (feature off)
worker: 0.1.0+b13684c9f547 heartbeat 0s
```

The worker writes one line per action to `worker.log` in the git directory, next to the store.

Every minute it runs its own executable, the absolute path in the unit, with `--version`. When
the version differs from its own, it logs a `restart` line and exits 0. systemd starts it again
5 seconds later, on the new binary.

`ness worker run` exits the same way and does not come back. The worker reads `.ness/ness.toml`
once at start, so a change to its configuration needs
`systemctl --user restart ness-<project>`.

### Waking

Every 5 seconds the worker looks at each session's terminal through the runtime. When a
session has unread items, it types one short hint, batched for all of them. `--urgent` skips
the batching.

The worker never types into a session that is working, or into an input box that holds text.
It treats the client's dim placeholder as empty. After the box last changed, it waits
`worker.quiet_seconds` (default 60) so it never races your typing.

### Compaction at boundaries

A session that finished a step writes its state down and runs `ness inbox --boundary NOTE`.
The next time the session is idle, the worker types `/compact NOTE`. The compaction happens
between steps, never in the middle of one, and the note tells the compacted session where it
stands.

After every compaction the session-start hook reminds the agent to re-read its brief
and its inbox.

The worker never compacts a session that recorded no boundary.

### Usage limits

When a session hits a usage limit, the worker reads the notice on its screen and types
nothing while the limit holds. It parses the reset time, such as `resets 2:10pm` or
`resets Oct 8`, in your local time zone. At the reset plus 60 seconds it types the client's
resume text, once.

A notice with no reset time keeps the session silent. The worker reads the screen again every
15 minutes and resumes normal delivery once the notice is gone.

### Stalls

A session idle with open obligations for `worker.stall_minutes` (default 30) gets one stall
report to its manager per episode. A root's report goes to the operator's inbox. A child that
has never sent its handoff gets the same report, even with no obligation.

A session is not stalled while it waits: on an item it sent, on a probe it owns, or while
parked. A session with a background shell or agent still running is working, not stalled. A
screen that shows an API error, an overloaded or a capacity error gets a stall report too.

### Telling the operator

You work with sessions in their chat, and only there. Your phone only notifies you and points
at the session that needs you. Set one channel as the operator channel, and the worker posts an
alert there:

```toml title=".ness/ness.toml"
features = ["notify"]

[notify]
operator = "phone"

[notify.channels.phone]
kind    = "telegram"
token   = "secret:telegram-token"
chat_id = "secret:telegram-chat"
```

| Alert | When |
|---|---|
| 🟠 Needs you | A request, decision or handoff to the operator, a checkpoint with `## Ask`, or a session's reply to an item the operator sent |
| 🟢 Checkpoint | A checkpoint without `## Ask` |
| 🔴 Trouble | A session blocked, out of usage, showing an error, stopped for an unavailable model, or stalled twice |
| 🟡 Your move | A root is idle, waits on nothing and has asked you nothing |

Each alert names the project and the session, and links the session when its transcript has a
Remote Control URL. It carries the item's first line or its `## Ask`; a Checkpoint alert carries
the first item under `## Next`.

Alerts for one tree share a thread. With no operator channel set, the worker posts nothing.

## Links

- [Sessions](./sessions.md): the tree that messages run along.
- [Guides and sensors](./guides-and-sensors.md): the hook that refuses polling.
- [Notify](../guides/notify.md): configure a channel and the operator alerts.
- [Configuration](../reference/config.md): `[worker]` and `[notify]`.
- [CLI reference](../reference/cli/index.md): `send`, `reply`, `inbox` and `worker`.

Next: [The session record](./session-record.md).
