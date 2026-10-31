---
title: Models
description: One ladder of model and effort pairs, a base rung, selectors that answer an offset from it, and a hook that holds subagent calls to the recommendation.
---

The **ladder** is the project's list of (model, effort) pairs, cheapest first. Each entry is a
[rung](../why/defaults.md#the-model-ladder). In configuration a rung is a `[tiers.NAME]` row, and
`--tier` names one.

Every session runs at one rung, the **base** rung unless a selector or you
pick another. Every subagent call is judged against the rung its kind of work calls for.

## The ladder

`ness pick` with no arguments prints it. The Claude and Codex columns are the model and effort
each client gets on that rung:

```sh
ness pick
```

```text title="Output"
bulk      haiku/medium   gpt-6-luna/low      
economy   sonnet/medium  gpt-6-luna/medium   
lean      opus/low       gpt-6.1-sol/low     
standard  opus/medium    gpt-6.1-sol/medium    base
deep      opus/high      gpt-6.1-sol/high    
frontier  fable/medium   astra/medium        
apex      fable/high     astra/high            ceiling
```

Three keys of `[models]` shape it:

| Key | Default | Meaning |
|---|---|---|
| `ladder` | the seven rungs above | The order, cheapest first. Each name is a `[tiers.NAME]` row. |
| `base` | `standard` | The rung every offset counts from, and every fallback lands on |
| `ceiling` | the top rung | No selector and no step goes above it |

Each `[tiers.NAME]` row sets the pair for each client, such as `claude = "opus/medium"`. Edit a
row and the next launch and the next sync use it.

Moving `base` moves every new launch, and every
named subagent after the next sync. A running session keeps its rung.

## Kinds

A **kind** names the sort of work a call does. Each kind maps to an offset from base:

| Kind | Offset | At the default base |
|---|---|---|
| `search`, `run` | −2 | economy |
| `implement`, `debug` | 0 | standard |
| `research`, `review`, `verify`, `design` | +1 | deep |

`[models.kinds]` overrides one kind's offset, or adds a kind of your own. `ness pick --kind`
prints the rung for one kind:

```sh
ness pick --kind review
```

```text title="Output"
deep      opus/high      kind review
model: opus  effort: high
```

An unknown kind exits 4 and lists the kinds.

## Selectors

A **selector** is the atom that picks the rung. It reads the task text, the role (root, child or
subagent), the client, the kind and the agent type. It answers an offset with a confidence and a
one-line reason.

ness clamps the answer to the ladder and the ceiling. A selector error, or a confidence below
`models.router_threshold` (default 0.8), gives the base rung.

| `models.policy` | Selector |
|---|---|
| `declared` (default) | The offset of the declared kind |
| `command` | Your program reads the task as JSON on stdin and prints `{"offset", "confidence", "reason"}` |
| `jev` | A hosted judgement model answers narrow questions about the task. Any failure falls back to `declared`. |

A `command` selector can be a few lines of shell. This one sends docs-only work two rungs down:

```sh title="pick"
#!/bin/sh
# Docs-only changes go two rungs down; everything else stays at base.
if grep -qi 'docs' ; then
  echo '{"offset": -2, "confidence": 0.9, "reason": "docs-only change"}'
else
  echo '{"offset": 0, "confidence": 0.9, "reason": "code change"}'
fi
```

```toml title=".ness/ness.toml"
[models]
policy  = "command"
command = "./pick"
```

```sh
echo "fix a typo in the docs" | ness pick --stdin
```

```text title="Output"
economy   sonnet/medium  docs-only change
model: sonnet  effort: medium
```

## Where the rung applies

| Who | How it gets its rung |
|---|---|
| A root or child | `ness open` asks the selector, or takes `--tier`. A rung above the recommendation needs `--reason`. |
| A named subagent | `ness-sweep`, `ness-review`, `ness-verify` and `ness-design` are generated with the rung of their kind (search, review, verify, design) |
| Any other subagent | The prompt starts with `kind: <name>` and passes the model and effort `ness pick --kind <name>` prints |

A session's model and effort never change while it runs. `ness open` prints the rung and the
reason, and `ness stats --by rung` shows what each rung cost.

### Moving a session up a rung

When a session needs a stronger rung, it is relaunched one rung up. `ness open --resume ID
--tier up` does this, but it refuses while the session's client is live, or when it cannot read
the pane's state.

Live means herdr still reports an agent in the session's pane, idle or working. A session that
only ended its turn is still live.

1. The session writes its state down with `ness inbox --boundary NOTE` and ends its turn.
2. Its client exits: the operator quits it in its pane (`/exit` in Claude Code), or closes its
   tab.
3. The manager runs `ness open --resume ID --tier up`. The client starts again in a new run, on
   the next rung, with its transcript.

## Hook enforcement

The pre-tool hook judges every subagent call of a session. It reads the prompt's leading
`key: value` lines for `kind:` and `escalate:`, and compares the requested rung with the
recommended one:

| Requested rung | Verdict |
|---|---|
| Below the recommendation | allowed, recorded as `below` |
| At it | allowed |
| One rung above, with `escalate: <reason>` | allowed, recorded as `escalated` |
| Higher, or one above without a reason | refused |

A subagent prompt that escalates starts like this:

```text
kind: verify
escalate: the migration touches billing; a wrong pass costs real money
Check that ...
```

An escalate buys exactly one rung. A model without an effort takes the effort of its kind's
rung when that rung uses the same model.

Any other half pair, or a pair that is not on the ladder, is refused with the ladder and a
pointer to `ness pick`. A call above the ceiling, or without a `kind:` line when the selector
needs one, is refused too.

Every refusal ends in `(skill ness-entry, section Model and effort)`. Only the refusal above the
recommendation offers `escalate:` as a way out.

With `models.enforce = "log"`, the hook lets every call run and records `would-refuse` for each
one it would have refused, whatever the reason. Every judged call is stored with its kind, the
requested and recommended rungs and the verdict.

## Links

- [Atoms](./atoms.md): the selectors as atoms of the model seam, chosen in a profile's `[atoms]` as `model_policy`.
- [Guides and sensors](./guides-and-sensors.md): a real ladder refusal.
- [Tune the ladder](../guides/models.md): move base, edit rungs, write a selector.
- [Configuration](../reference/config.md): `[tiers]` and `[models]`.
- [CLI reference](../reference/cli/index.md): `pick`, `open` and `stats`.

Next: [Grow your own harness](../guides/local-profile.md).
