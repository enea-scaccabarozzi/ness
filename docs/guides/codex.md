---
title: Run sessions on Codex
description: Opt in to the experimental Codex client, map the ladder to models your account lists, and know what the worker cannot do there.
---

> [!NOTE]
> Codex support is experimental. The worker does not deliver mail to Codex sessions, so a
> Codex session reads its mail with `ness inbox`.

ness renders its harness for one client per project: `claude` by default, or `codex`. The skills,
`AGENTS.md` and the ladder stay the same. Codex settings go to `.codex/config.toml`, and sync
still writes the `.claude/` files.

## Prerequisites

- Codex installed and signed in. `codex --version` prints a version.
- A project set up with `ness init`.

## Switch the client

1. Set the client and opt in. Without the opt-in, ness refuses the client:

   ```text title="Output (exit 4)"
   ness: .ness/ness.toml: client = "codex" needs experimental = ["codex"]
   ```

   ```toml title=".ness/ness.toml"
   client       = "codex"
   experimental = ["codex"]
   ```

2. Render it.

   ```sh
   ness sync
   ```

   The new file carries the compaction limit and the subagent rung, the `search` kind's
   `economy`:

   ```toml title=".codex/config.toml"
   model_auto_compact_token_limit = 235000

   [agents]
   default_subagent_model = "gpt-6-luna"
   default_subagent_reasoning_effort = "medium"
   ```

3. Run `ness doctor`.

## Map the ladder to your models

`tier-models` asks Codex which models your account offers. A rung whose model is not listed fails:

```text title="Output (one rung of five shown)"
fail	tier-models	tier apex (codex = "astra"): model "astra" is not listed by codex debug models (gpt-reserve, gpt-6-luna, gpt-5.6-terra, gpt-5.6-luna, codex-auto-review); upgrade Codex or check the models your account offers, or set [tiers.NAME] codex in .ness/ness.toml	fix: edit [tiers.<name>] in .ness/ness.toml
```

Set the `codex` value of each failing rung to a listed model:

```toml title=".ness/ness.toml"
[tiers.lean]
codex = "gpt-5.6-terra/low"

[tiers.standard]
codex = "gpt-5.6-terra/medium"

[tiers.deep]
codex = "gpt-5.6-terra/high"

[tiers.frontier]
codex = "gpt-5.6-terra/high"

[tiers.apex]
codex = "gpt-5.6-terra/xhigh"
```

Run `ness sync` again. `ness pick` shows the new Codex column.

### Check that it worked

```sh
ness doctor
```

```text title="Output (excerpt)"
ok	client-version	codex 0.162.1
ok	tier-models	7 tiers resolve for codex
ok	worker-clients	no store yet
```

The first command that opens the store, such as `ness status` or `ness open`, creates it. From
then on, `worker-clients` warns:

```text title="Output (after ness status)"
warn	worker-clients	codex has no worker support	fix: run sessions on claude, or read their mail with ness inbox
```

## What differs on Codex

| Area | Claude | Codex |
|---|---|---|
| Settings | `.claude/settings.json` | `.codex/config.toml` |
| Worker mail delivery | Yes | No: run `ness inbox` |
| Efforts | `low`, `medium`, `high`, `xhigh`, `max` | `minimal`, `low`, `medium`, `high`, `xhigh` |

To go back, remove both keys and run `ness sync`. Sync removes its keys from `.codex/config.toml`;
delete the file once nothing else of yours is in it.

Next: [Diagnose a project](./doctor.md).
