---
title: Guides and sensors
description: Guides tell the agent the workflow before it acts. Sensors catch drift as it happens. Every refusal points back to its guide.
---

ness steers agents in two directions. A **guide** reaches the agent before it acts: prose that
says what to do and why. A **sensor** checks what the agent does and refuses work that breaks a
rule.

Each sensor's refusal names the guide it enforces, so the agent that hits it is sent to
the prose.

## Guides

| Guide | Where it lives | When the agent sees it |
|---|---|---|
| Core skills | `.agents/skills/ness-entry`, `.agents/skills/ness-launch` | `ness-entry` at the start of every session, `ness-launch` before it opens a child |
| Feature and profile skills | `.agents/skills/<name>`, rendered by `ness sync` | When the feature or profile is on and the work needs the skill |
| Managed blocks | `AGENTS.md`, between `ness:` markers | At the start of every session. The router block says to load `ness-entry` first. |
| Profile slots | [Slots](./profiles-and-sync.md#slots) in the skills and files a profile renders | Inside the skill or file, wherever the slot sits. A profile adds its text there. |
| Session start | Printed by the `session-start` hook | At every start: the session's identity and a cheat sheet. After a compaction, a reminder to re-read the brief and the inbox. |
| The brief | `brief.md` in the session's record directory | First. Every session's opening prompt points at its brief. |
| Your docs | Your repository | When a skill or the brief sends the agent there |

The skills and slot text come from the core, the features and your
[profiles](./profiles-and-sync.md). `ness sync` renders them and keeps them current.

## Sensors

| Sensor | When it fires | What happens |
|---|---|---|
| Pre-tool hook | Before each tool call of a session | The call does not run. The hook exits 2 with the reason. |
| Profile checks | At `ness open`, `ness integrate` and `ness close` | The command is refused (exit 4) and nothing is written. Every failing check is reported. |
| The gate | In `ness integrate`, in a throwaway checkout. It runs again when the target moved outside the session records meanwhile. | The branch merges only when the gate passes. |
| Contract check | When `ness contract check` runs, for example in your gate. The pre-tool hook guards frozen files too. | It exits 1 and names each frozen file that differs. Needs the [`contracts`](./features.md#contracts) feature. |
| Handoff and checkpoint | `ness send --intent handoff` or `--intent checkpoint` | A body without `## Done`, `## Now` and `## Next` is refused. Only a root sends a checkpoint, and only to the operator. |
| Lessons | `ness close` | A report without a `## Lessons` heading is refused (exit 4). |
| Doctor | When you run `ness doctor` | Read-only. It prints a fix for each failed check and runs none. |
| Stats | When you run `ness stats` | Read-only. Cost and context size, grouped by session, tier, role, rung or tool. |
| Probes | On a schedule or once. Needs the [`probes`](./features.md#probes) feature. | The probe's `wake` setting decides which floor states wake the owner: `never`, `change`, `red`, `not-green` or `always`. |

### The pre-tool hook

The hook guards the tool calls of a ness session. It never guards you, the operator, or a
conversation that is not a session.

Each refusal exits 2 with the reason, and the call does not
run. In order, it refuses:

1. A hook payload that does not parse.
2. Polling: Monitor, ScheduleWakeup or CronCreate whose input runs `ness inbox` or `ness status`.
3. A shell wait loop around `ness inbox` or `ness status`. A wait loop is `while`, `until`,
   `for ((`, `watch`, or any loop with a `sleep` in it.
4. A shell loop with a `sleep` around `ness integrate`, `ness send` or `ness open`.
5. A shell wait loop that runs `pgrep -f` or `pgrep --full`.
6. Backgrounding with a lone `&`, unless the call sets the Bash tool's `run_in_background` or a
   `wait` follows on the same line.
7. The Explore subagent. `ness-sweep` is the search agent.
8. A whole saved tool result: a Read without offset and limit, or `cat`, or `sed` without `-n`.
9. A subagent call that breaks the [model rules](./models.md#hook-enforcement):
   - a model and effort that are not a rung, or none at all for an agent that is not a named ness agent;
   - a rung above the ceiling;
   - no `kind:` line when the selector needs one;
   - a rung above the recommendation, unless `escalate:` asks for exactly one rung more.
10. Running the configured gate by hand.
11. An Edit, Write or MultiEdit of `.ness/ness.toml` whose new text touches `max_children`,
    `[launch]`, `[tiers]` or `[worker]`.
12. A call that matches one of your own `[[guards]]`.
13. With the `contracts` feature on, an edit of a whole-file contract. An edit of a scenario,
    prefix or format contract goes through with a warning.

It lets these through:

- one `ness inbox` call, and a loop or Monitor that does not read `ness inbox` or `ness status`;
- a `for NAME in` loop over a list, without `sleep`;
- a one-shot `pgrep`, and `&&`, `&>` or `>&`;
- `head`, `grep`, `sed -n` or a Read with a range over a saved tool result;
- forked subagents, which keep the session's own rung;
- shell commands that write a file. The edit guards see only Edit, Write, MultiEdit,
  NotebookEdit and Codex `apply_patch`, so `ness contract check` stays the judge of frozen files.

With `models.enforce = "log"`, every subagent refusal of item 9 becomes a `would-refuse` record
and the call runs.

## The loop: refusals point to guides

Every refusal ends with a pointer in one format: `(skill NAME, section HEADING)`. The section is
the guide the sensor enforces. The agent reads it and changes course.

Two real refusals from a session on 2026-10-31. The hook refused a command that ended in `&`:

```text title="pre-tool hook"
ness: backgrounding with & is refused: nothing wakes you when the job ends; run it with the Bash tool's run_in_background option and end your turn; the task notification wakes you (skill ness-entry, section Communicate)
```

It refused a subagent call that named a model without an effort. The ladder listing in the
middle is trimmed:

```text title="pre-tool hook"
ness: model "opus" with effort "" is not a rung of the ladder; pass both, as ness pick --kind NAME prints them … (skill ness-entry, section Model and effort)
```

`ness sync` keeps the pointers honest. It refuses a pointer to a heading that does not exist and
lists the headings that do. Rename a heading, and sync fails before the pointer rots.

## Add your own rule

A rule that lives only in prose is a request an agent can forget. A local rule has three parts:

1. A guide: a section in one of your skills that says what to do and why.
2. A sensor: a check that fails when the rule is broken.
3. The pointer: the check names that skill and section.

Your own profile lives in `.ness/profiles/local/`. It runs only when `ness.toml` lists it in
`profiles`. This check refuses to open a child for a task without a plan file:

```toml title=".ness/profiles/local/profile.toml"
[[checks]]
id      = "plan-linked"
on      = "open"
for     = "child"
run     = "./scripts/check-plan-linked.sh"
timeout = "20s"
skill   = "acme-planning"
section = "Plans"
message = "Open a child only for a task that has a plan file."
```

When the script exits non-zero, `ness open` prints the message, the pointer, and what the script
printed:

```text title="Output"
ness open refused: Open a child only for a task that has a plan file. (skill acme-planning, section Plans)
  local/plan-linked: no plans/<task>.md for scope "retry-backoff"
```

A check can use a built-in kind instead of a script: `section-matches`, `branch-forbids`,
`match-last` or `checkpoint-since-landing`. Some rules need judgement, such as whether a name is
clear. Say so in the skill section, so nobody looks for a check that is not there.

## Links

- [Sessions](./sessions.md): the session tree, sizes and closing.
- [Principles](../why/principles.md): the rules ness is built by.
- [Grow your own harness](../guides/local-profile.md): every mechanism, with examples.
- Specifications: [hooks](../spec/hooks.md) and [checks](../spec/checks.md).

Next: [Atoms](./atoms.md).
