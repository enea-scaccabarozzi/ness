# ness

There is no universal harness. Build yours.

ness is the machinery under an AI coding-agent harness. You build the harness: the process,
rules and checks your project needs. ness ships the parts every harness needs:

- a tree of sessions, and the messages between them;
- a worker that wakes idle sessions;
- a committed record of every session;
- profiles that change the process without forking ness.

It is one Go binary. It works with Claude Code today, and with Codex as experimental.

Documentation: [enea-scaccabarozzi.github.io/ness](https://enea-scaccabarozzi.github.io/ness),
built from the [`docs/`](docs/index.md) folder of this repository.

## Requirements

- Linux with systemd (the worker runs as a systemd user unit).
- [herdr](https://herdr.dev), the terminal multiplexer that holds one tab per session.
- Claude Code. Codex works as experimental.

ness is tested solo and in small teams. See [Limits](docs/why/limits.md) for every limit.

## Quickstart

Install ness:

```sh
curl -fsSL https://raw.githubusercontent.com/enea-scaccabarozzi/ness/main/install.sh | sh
```

[Install](docs/start/install.md) has the other ways: mise, `go install` and the archive. Then follow
[Quickstart](docs/start/quickstart.md) to run a first session. Every command there was run by
hand, and its output is shown as the plain renderer prints it.

> [!WARNING]
> Sessions that ness launches skip the client's permission prompts (Claude Code
> `--dangerously-skip-permissions`, Codex's bypass flag). ness's hooks still guard them. Set
> `[launch] permissions = "client"` in `.ness/ness.toml` to keep the client's own settings.

## Read the docs in order

1. [Introduction](docs/index.md): what ness is, and the argument for it.
2. [Why ness](docs/why/philosophy.md): the story, the [principles](docs/why/principles.md), the
   [defaults](docs/why/defaults.md) and the [limits](docs/why/limits.md).
3. [Get started](docs/start/install.md): install, the first quick task, and the first week.
4. [Concepts](docs/concepts/sessions.md): sessions, messages, records, profiles, atoms, models.
5. Guides and reference: build your own harness on top of ness, then look up the exact facts.
6. Built with ness: ness has been developed with ness. The [design](docs/design/ness-design.md),
   [spec](docs/spec/README.md) and [session records](docs/sessions/) are public, anonymised.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the dev setup, the gate and the writing style.
Report a vulnerability privately as described in [SECURITY.md](SECURITY.md).

## License

[Apache-2.0](LICENSE).
