# Contributing to ness

This guide covers the dev setup, the gate, the test rules, the code layout, releases and the
writing style for the docs. Read [AGENTS.md](AGENTS.md) too: ness develops itself with ness, so
the repository carries its own harness.

## Dev setup

[mise](https://mise.jdx.dev) pins every tool in [`mise.toml`](mise.toml): Go, Python,
golangci-lint, goreleaser and Bun.

1. Install the tools.

   ```sh
   mise install
   ```

2. Run the gate once to see a green baseline.

   ```sh
   make validate
   ```

3. Optional: install the git hooks with [lefthook](https://lefthook.dev). Pre-commit runs the
   linters. Pre-push runs `make build lint audit selfcheck`.

   ```sh
   lefthook install
   ```

The docs site lives in [`site/`](site/README.md) and has its own commands:

```sh
cd site
bun install
bun run dev
```

## The gate

`make validate` is the gate. `ness integrate` runs it before a session's branch lands
(`gate = "make validate"` in [`.ness/ness.toml`](.ness/ness.toml)), and CI runs it on every push.

| Target | What it checks |
|---|---|
| `build` | `CGO_ENABLED=0 go build ./...`, and the opt-in e2e files still compile |
| `lint` | golangci-lint with `.golangci.yml`, then with `.golangci.tests.yml` for test files |
| `test` | `go test -race -count=1 ./...` |
| `audit` | `.ness/audit.py`: one package comment per package, test-file size, no `time.Sleep`, no test hooks in production code |
| `vuln` | govulncheck |
| `selfcheck` | `ness sync --check`: the files ness renders into this repository are current |

CI also runs:

- `make repeat`, every test four times, to catch flaky tests;
- `make fuzz`;
- a Conventional Commits check on every commit subject;
- `goreleaser check`;
- the site build with its link check.

## Test discipline

- Every requirement in [`docs/spec/`](docs/spec/README.md) has an id such as `R-SESS-014`. A test
  names the ids it covers with a `// covers: R-SESS-014` comment. `internal/testkit` fails the
  gate when a requirement of a finished area has no covering test or review entry.
- No `time.Sleep`. Pass the time in as a value, and use `testing/synctest` for timers.
- No permissive mocks and no call-count assertions. Write a small fake by hand.
- Helpers call `t.Helper()`. Fixtures live in `testdata/`.
- A test file stays under 400 lines (`.ness/audit.toml`).
- Tests never reach the network or a real client. `internal/testkit` builds hermetic git
  repositories and fakes for herdr and HTTP.
- Try a change to the store schema in a scratch repository. ness migrates the store of any
  project it runs in, so a test build run in a real project migrates that project's store.
- Real-client runs are opt-in: run `make e2e` inside a herdr session. They drive the real
  client. The target sets `NESS_E2E=1`; a direct `go test -tags e2e` skips them without it.

## Architecture

One binary, `cmd/ness`. Everything else is under `internal/`, plus the embedded text in
`profiles/` and the release notes in the root package.

| Package | Role |
|---|---|
| `cmd/ness` | The `ness` command: installs and runs the harness inside one project. |
| `profiles` | Embeds the core text and the built-in profiles into the binary. |
| `.` (root) | Embeds `CHANGELOG.md`, the release notes `ness sync` prints. |
| `internal/cli` | The command line. |
| `internal/config` | Loads and validates `.ness/ness.toml`, and resolves secret references. |
| `internal/sync` | Renders core, features and profiles into a repository, recorded in `.ness/lock.json`. |
| `internal/profile` | Loads profiles, validates them, and refuses conflicts before anything is written. |
| `internal/session` | The session tree: the seams, the environment checks, the launch state machine. |
| `internal/store` | The SQLite state: sessions and messages, shared by the checkout and its worktrees. |
| `internal/worker` | The per-project worker: wakes, compacts, resumes after limits, reports stalls. |
| `internal/hook` | Parses client hook payloads and decides the pre-tool guards as pure functions. |
| `internal/rule` | The registry of rule pointers: the skill and section each refusal enforces. |
| `internal/check` | Runs the checks a profile declares at `ness open`, `integrate` and `close`. |
| `internal/probe` | Probes: one Markdown file per question about the running world. |
| `internal/notify` | Notify: one Markdown note rendered by a channel atom and posted in a thread. |
| `internal/ladder` | Orders the model tiers into rungs and clamps moves to the base and ceiling. |
| `internal/stats` | Reads the transcripts bound to ness sessions and adds up tokens and cost. |
| `internal/shell` | Runs one command for a probe floor or the command channel. |
| `internal/ui` | Human-facing output: picks the plain or the rich renderer and draws the rich one. |
| `internal/testkit` | Shared test setup: hermetic git repositories and custom commands for CLI scripts. |
| `internal/atom/*` | Atoms, one package each, behind the seams. See below. |

| Seam | Atoms |
|---|---|
| Runtime | `herdr` |
| Client | `claude`, `codex` (experimental) |
| Integrator | `squash`, `merge`, `rebase`, `pr` |
| Channel | `slack`, `telegram`, `github`, `command` |
| Supervisor | `systemd` |
| Model selector | `declared`, `cmdpolicy`, `jev` |

The one-line roles come from each package comment. When a role changes, change the package
comment first; `.ness/audit.py` fails a package without one.

## Commits and releases

- Commit subjects follow [Conventional Commits](https://www.conventionalcommits.org). CI checks
  each subject with `profiles/builtin/git-conventional/commit-msg`.
- [release-please](https://github.com/googleapis/release-please) keeps a release pull request open
  from the commits on `main` (`release-please-config.json`).
- Every stable release goes through a release candidate. Cut one from the release pull request
  with `gh workflow run rc.yml`: it tags `vX.Y.Z-rc.N` and publishes a GitHub pre-release. Run it
  on the field projects, and cut a new rc whenever the pull request moves.
- Merging the pull request releases the version. The release job refuses it without an rc of the
  same tree, the previous stable release's upgrade fixture, and a `sessions:` commit since that
  release.
- A `sessions:` commit becomes the "Running sessions" section of `CHANGELOG.md`: what an open
  session must do after it merges the new version. Write `sessions: nothing to do` when there is
  nothing. Session-record commits use `docs(sessions): …` and do not count.
- After each stable release, commit its upgrade fixture with `scripts/upgrade-fixture.sh vX.Y.Z`.

[Release ness](docs/guides/releasing.md) has every step, the promotion criteria and the recovery
from a refused release.

## Writing style

The docs and the landing page are written for a reader, not to sound impressive. These rules
apply to every page in `docs/`, to the site, and to this file.

### Readers, page by page

| Where | Reader | What they want | Shape |
|---|---|---|---|
| Landing | A developer scanning for 10 seconds | What is it, why should I care, is it real | Headline, one line under it, visuals, proof numbers |
| `why/` | A sceptical senior dev deciding whether to adopt | The argument and the trade-offs | Short claims, each backed by a fact or an example |
| `start/` | A new user at a terminal | Commands that work the first time | Numbered steps, one action each, expected output |
| `concepts/` | An adopter building a mental model | How the parts fit | A definition first, then a diagram or table, then the details |
| `guides/` | A practitioner with a goal | The steps to the goal | A task title, prerequisites, steps, a check that it worked |
| `reference/` | Someone looking one thing up | The exact fact | Tables and lists, no prose arguments |
| `built-with-ness/` | Evaluators, contributors, hiring managers | Evidence | Numbers with their sources, what failed and what was fixed |

### Rules

1. Lead with the point. The first sentence of a page or section says what it is or what to do.
2. One idea per sentence, usually under 25 words. Paragraphs of at most three sentences.
3. Second person, present tense, active voice: "ness opens a worktree", not "a worktree will be
   opened".
4. Use a list for three or more parallel items, a table for comparisons, and numbered steps for
   procedures.
5. Headings: tasks in guides ("Freeze a file"), nouns in concepts and reference ("Probes"). Use
   sentence case.
6. Every command is copy-pasteable and was run by hand. Show the expected output after it.
7. Define a term once, where it first appears, and link to that definition afterwards. Use the
   glossary terms and no synonyms: session, root, child, operator, worker, profile, slot, atom,
   seam, feature, gate, rung, size.
8. Use concrete numbers with their sources. Never "many", "significantly" or "dramatically".
9. State limits plainly ("Linux only", "experimental"). Do not hedge everything else.
10. On the landing page, be punchy but factual: every claim must be one the docs prove.

### Banned

- Words: seamless, robust, powerful, leverage, empower, unlock, elevate, delve, streamline,
  supercharge, game-changer, cutting-edge, effortless, simply, just, easy, crucial,
  comprehensive, ecosystem (unless literal).
- Phrases: "Whether you're X or Y", "In today's fast-paced…", "It's important to note", "Let's
  dive in", "not just X, but Y", "In summary / In conclusion", "Here's the thing", "At its core".
- Patterns: em-dash chains, triads used as padding, rhetorical questions, exclamation marks,
  emoji in prose, closing paragraphs that restate the page, every bullet starting with a bolded
  label plus a colon, sentences that announce what the next sentence will say.

### Code blocks

Give every block a language tag, and a title when it shows a file. Keep a command and its output
in separate blocks or label them clearly. Use no `$` prompt in copyable commands.

### Docs mechanics

- Pages are plain GitHub Markdown with `title` and `description` frontmatter. The site renders
  the same files; nothing is copied into `site/`.
- Each folder orders its pages with a `meta.json`.
- Callouts use GitHub's `> [!NOTE]` syntax.
- `bun run build:pages` then `bun run check:links` in `site/` fails on a link to a missing page,
  file or heading.
- `docs/reference/cli/` is generated from the commands. After changing a command, regenerate it
  with `go test ./internal/cli/ -run TestReferenceIsFresh -update`; the gate fails while it is
  stale.
