---
title: Choose how work lands
description: Pick the integrate mode (squash, merge, rebase or pr), push after landing, and keep release notes readable.
---

`ness integrate` folds a branch into its target through the gate. A session runs it with no
`--from` or `--into`: its branch goes into its manager's branch. You, the operator, can fold any
pair.

The mode decides what the target's history looks like.

## Prerequisites

- A `gate` in `.ness/ness.toml` that passes on the base branch.
- For `push` and `pr`: a remote named `origin`. For `pr`: the `gh` CLI, signed in.

## Fold a branch with the default mode

The default mode is `squash`: one commit on the target, with a Conventional Commit subject.

```sh
ness integrate --from feat/greet --into main --subject 'feat(greet): add greet.sh'
```

```text title="Output"
integrated feat/greet into main at d664a15eeed99a30d5d50fabe2d5cd303c552b01
```

The line names the new commit on the target by its full hash. Anything the gate prints comes
before it; the gate here, `true`, prints nothing.

### Check that it worked

```sh
git log -1 --format=%B main
```

```text title="Output"
feat(greet): add greet.sh

BEGIN_NESTED_COMMIT
feat(greet): add greet.sh
END_NESTED_COMMIT
BEGIN_NESTED_COMMIT
fix(greet): greet by name
END_NESTED_COMMIT

Ness-Integrates: 28159536bc5beb1e1e05c224c12ecb8c0250c3c7
```

Each folded commit stays in the body as a nested commit, and the trailer records the source
commit. A squash without a valid subject is refused:

```text title="Output (exit 4)"
ness: a squash lands one commit and needs its subject: --subject 'type(scope): summary', a Conventional Commit subject such as 'feat(cli): add --subject' (got "") (skill ness-launch, section Wait, then fold)
```

## Pick a mode

```toml title=".ness/ness.toml"
[integrate]
mode  = "squash"
push  = true
setup = "npm ci"
```

| Mode | Lands | `--subject` |
|---|---|---|
| `squash` (default) | One commit, folded commits nested in its body | Required |
| `merge` | A merge commit `integrate BRANCH into TARGET` | Ignored, with a warning |
| `rebase` | The branch's commits replayed on the target, linear | Ignored, with a warning |
| `pr` | A pull request opened with `gh`. The local target never moves. | Ignored, with a warning |

In `merge`, `rebase` and `pr` mode, a subject prints a warning and the command still succeeds.
With `rebase`:

```text title="Output"
ness: --subject ignored: integrate.mode rebase writes its own message
integrated feat/r into main at 0ff466d93a7c7d7ef434e49c5e700e3a2f1af8a3
```

A branch the target already holds is refused in every mode:

```text title="Output (exit 4)"
ness: nothing to integrate: feat/wave is already in main
```

With `pr`, the gate runs on a local merge preview first. Only then does ness push the branch and
open the request. `ness close` refuses while the request is not merged.

## Run the gate

ness folds the branch in a throwaway checkout, runs `setup` there (for dependencies), then runs
`gate` once with `NESS_BASE` set. The target moves only when the gate passes. A failing gate
prints the gate's output and exits 1:

```text title="Output (exit 1)"
ness: the gate failed: sh -c export NESS_BASE='d4301e107076f34fde1be8a6a1e076d780675502'
./scripts/gate.sh: exit status 1: src/broken.sh: 1: Syntax error: "then" unexpected
```

Integrations into one target run one at a time, under a lock in `.git/ness/`.

## Push after landing

`push = true` pushes the target to `origin` after it moves. The default is `false`.

## Keep release notes readable

release-please reads each nested commit of a squash as its own change, so every folded `feat:` and `fix:` gets a line in the changelog. To
route session work to its own section, give it a type, such as `sessions:`, and map it in
`release-please-config.json`:

```json title="release-please-config.json"
{
  "release-type": "go",
  "packages": {
    ".": {
      "changelog-sections": [
        { "type": "feat", "section": "Features" },
        { "type": "fix", "section": "Bug Fixes" },
        { "type": "sessions", "section": "Running sessions" }
      ]
    }
  }
}
```

The built-in `git-conventional` profile adds a `commit-msg` hook that refuses a subject outside
the Conventional Commit form. The hook does nothing until you
[turn it on in each clone](../reference/builtin-profiles.md#git-conventional).

Next: [Pick the model for each job](./models.md).
