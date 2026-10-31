---
title: Give a rule a check
description: Pair a rule in prose with a sensor that refuses work breaking it, and point the refusal back at the prose.
---

A rule that lives only in prose is a request an agent can forget. Give it three parts. A skill
section states it, a sensor fails when it is broken, and a pointer
`(skill NAME, section HEADING)` leads from the sensor back to the section.

This guide adds each kind of sensor to the local profile from
[Grow your own harness](./local-profile.md).

## Prerequisites

- A local profile with a skill, here `acme-planning` with a `## Plans` section.
- For integrate checks: a branch to fold and a gate in `.ness/ness.toml`.

## Refuse an open without a plan

1. Write the check script. It runs in the opener's checkout with the context in its environment.
   Print the detail; never print the pointer, ness appends it.

   ```sh title="scripts/check-plan-linked.sh"
   #!/bin/sh
   # Passes when plans/<scope>.md exists in the opener's checkout.
   test -f "plans/$NESS_SCOPE.md" && exit 0
   echo "no plans/$NESS_SCOPE.md for scope \"$NESS_SCOPE\""
   exit 1
   ```

2. Declare it in the profile.

   ```toml title=".ness/profiles/local/profile.toml"
   [[checks]]
   id      = "plan-linked"
   on      = "open"
   run     = "./scripts/check-plan-linked.sh"
   timeout = "20s"
   skill   = "acme-planning"
   section = "Plans"
   message = "Open a session only for a scope that has a plan file."
   ```

   Add `for = "child"` or `for = "root"` to check only one of them. Without `for`, the check runs
   for both, the operator's `ness open --root` included.

3. Run `ness sync`. It resolves the pointer against the rendered skills. A wrong section name is
   refused before anything is written:

   ```text title="Output (section Plan)"
   ness: check local/plan-linked: skill "acme-planning" has no section "Plan"; its sections: Planning at acme, Plans
   ```

   Fix the name and sync again. It exits 0.

### Check that it worked

Open a root for a scope with no plan:

```sh
ness open --root --scope retry-backoff --size quick --tier economy --brief brief.md
```

```text title="Output (exit 4)"
ness: ness open refused: Open a session only for a scope that has a plan file. (skill acme-planning, section Plans)
  local/plan-linked: no plans/retry-backoff.md for scope "retry-backoff"
```

Nothing was written: no session row, worktree, branch or tab. Every failing check of the event is
reported together. `ness open --resume` does not run the checks again.

## Use a built-in kind

A kind needs no script. This one refuses a branch that adds a `TODO` line under `src/`:

```toml title=".ness/profiles/local/profile.toml"
[[checks]]
id      = "no-todo"
on      = "integrate"
kind    = "branch-forbids"
path    = '^src/'
pattern = 'TODO'
skill   = "acme-planning"
section = "Plans"
message = "Finish the work before it lands: no TODO in src/."
```

```sh
ness integrate --from feat/greet --into main --subject 'feat(greet): add greet.sh'
```

```text title="Output (exit 4)"
ness: ness integrate refused: Finish the work before it lands: no TODO in src/. (skill acme-planning, section Plans)
  local/no-todo: src/greet.sh adds "# TODO: greet by name"
```

| Kind | Arguments | Passes when |
|---|---|---|
| `section-matches` | `file`, `heading`, `require` (list of regexps) | The body of `heading` in `file` matches every `require` |
| `branch-forbids` | `path`, `pattern` (regexps) | No line the branch adds in a file matching `path` matches `pattern`. `{record}` in `path` is the session's record directory. |
| `match-last` | `file`, `pattern`, `against_file`, `against_pattern` | Group 1 of the first `pattern` match equals group 1 of the last `against_pattern` match |
| `checkpoint-since-landing` | none | The root sent a checkpoint since its last child landed |

A kind fails, never passes, when it cannot decide: a missing file, a heading not found, a git
error. A `run` check fails on any exit but 0, on a timeout (30 seconds unless `timeout` says
otherwise) and when the command cannot start. Its output, trimmed and cut to 1,500 bytes, is the
detail line.

## The context a script gets

| Variable | Events | Value |
|---|---|---|
| `NESS_EVENT` | all | `open`, `integrate` or `close` |
| `NESS_SESSION` | all | The acting session, empty for the operator |
| `NESS_ROOT` | all | The acting session's root, empty for the operator |
| `NESS_WORKTREE` | all | The checkout examined, also the working directory |
| `NESS_PRIMARY` | all | The primary checkout |
| `NESS_SCOPE` | open | The scope being opened |
| `NESS_BRANCH`, `NESS_INTO`, `NESS_RECORD` | integrate, close | Source branch, target branch, record directory |

## Refuse a tool call with a guard

A guard stops a tool call of a session before it runs. It lives in `.ness/ness.toml`:

```toml title=".ness/ness.toml"
[[guards]]
tool    = "Bash"
match   = '^pytest\s*$'
message = "Run the one test you changed, not the suite."
skill   = "acme-tests"
section = "Scope"
```

`tool` is a regexp on the tool name and `match` a regexp on the Bash command or the edited path.
An optional `content` regexp matches the new text of an edit.

A guard sees only Bash commands and the paths of Edit, Write, MultiEdit, NotebookEdit and Codex
`apply_patch` calls. Guards match text, so they stop conduct, not intent.

`ness sync --check` resolves guard pointers too:

```text title="Output (exit 4)"
ness: guards[0]: skill "acme-tests" does not ship
```

Two guards are built in: running the configured gate by hand inside a session, and an Edit,
Write or MultiEdit of `.ness/ness.toml` that touches the launch limits, `[tiers]` or `[worker]`.
A shell write to `ness.toml` passes them.
[The pre-tool hook](../concepts/guides-and-sensors.md#the-pre-tool-hook) lists every built-in
refusal.

## Put the rule in the gate

`gate` in `.ness/ness.toml` is any command. `ness integrate` runs it once, in a throwaway checkout
of the folded result, with `NESS_BASE` set to the commit the branch is folded onto. Make its
failure messages carry the same pointer:

```sh title="scripts/gate.sh"
#!/bin/sh
# The project's gate: ness integrate runs it once, in a throwaway checkout of the fold.
set -e
for f in src/*.sh; do sh -n "$f" || { echo "syntax error in $f (skill acme-planning, section Plans)"; exit 1; }; done
ness sync --check
echo "gate ok on top of $NESS_BASE"
```

A broken branch leaves the target where it was:

```text title="Output (exit 1)"
syntax error in src/broken.sh (skill acme-planning, section Plans)
sh -c export NESS_BASE='d4301e107076f34fde1be8a6a1e076d780675502'
./scripts/gate.sh: exit status 1: src/broken.sh: 1: Syntax error: "then" unexpected
ness: the gate failed: sh -c export NESS_BASE='d4301e107076f34fde1be8a6a1e076d780675502'
./scripts/gate.sh: exit status 1: src/broken.sh: 1: Syntax error: "then" unexpected
```

`ness sync --check` in the gate is the meta-test: a check or guard whose section vanished fails
the gate. Your own scripts are outside that walk. Give each the same message form and add one test
that scans them for `(skill X, section Y)` and resolves the section.

A `pre-commit` or `pre-push` hook gives the earliest failure, with the gate's message. It is per
clone and can be skipped with `--no-verify`, so keep it in front of the gate, never instead of it.

Next: [Steer a multi-week program](./program.md).
