> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Secrets in configuration — design (CX-22)

**Decision asked:** approve option A (references in config + a per-user secrets file that ness
reads itself, managed with `ness secret set`), or pick B/C.

## The problem
A notify channel today names an environment variable (`token_env = "TELEGRAM_TOKEN"`) and the
process that posts must have that variable. The worker runs as a systemd user unit whose
environment is only PATH, so the Telegram tokens were wired by hand: a 0600 env file under
`~/.config/ness/` plus a systemd drop-in with `EnvironmentFile=`, per project. One-shot commands
(`ness post`, `ness doctor`) see the token only if the shell exported it, so doctor and the
worker can disagree. `chat_id` must be written in the committed `.ness/ness.toml`, which is
personal data in a repo that will become public. Every new secret-bearing key (slack, github,
future forges) would repeat the `*_env` pattern.

## Options
**A. References + per-user secrets file (recommended).** Any value of a channel key may be a
reference instead of a literal:
- `secret:NAME` — read from `~/.config/ness/secrets/<project>.toml` (0600, outside every repo,
  `NAME = "value"` lines), keyed by the `project` of ness.toml.
- `env:NAME` — read from the environment (CI, and the migration path for `token_env`).
- anything else is a literal.
ness reads the file itself at the moment it builds a channel, so the worker, `ness post` and
`ness doctor` resolve the same way with no systemd env, no drop-in, no export.
+ one mechanism for every key and every atom; no systemd edits; no new dependency.
− a plaintext 0600 file at rest (same exposure as today's env file and `~/.ssh`).

**B. ness manages the systemd environment.** Keep `token_env`; `ness secret set` writes the env
file and the drop-in (or `LoadCredential=`). − one-shot commands from a shell still lack the
values (doctor ≠ worker); tied to systemd; `chat_id` stays in the repo.

**C. OS keyring (Secret Service / libsecret).** − this box is headless: no unlocked keyring for a
user service without extra plumbing; adds a dependency and a D-Bus path that fails opaquely.

Later, without redesign, A can grow a `cmd:` source (`cmd:pass show ness/tg`) to cover
keyrings and password managers. Not built now (YAGNI); a `file:` source likewise.

## How A works
**Secret vs plain.** A key is either *secret* or *plain*, fixed by its name in one table in
`internal/config`:
- secret keys (`token`): MUST be a reference; a literal is refused at load ("a token is never
  written in ness.toml; use token = \"secret:NAME\"").
- plain keys (`chat_id`, `channel_id`, `repo`, `command`): literal **or** reference — the
  operator's "both". `chat_id = "123"` and `chat_id = "secret:tg-chat"` are both valid.
Every value that came from a reference is treated as sensitive for redaction, whichever key.

**Resolution.** One package `internal/secret`: `Resolve(value) (string, error)` over a store
(the file read once per command, lookups by name) and `getenv`. `buildChannel` resolves every
key of the channel through it before constructing the atom — no per-atom code; a new atom just
declares which of its keys are secret in the table. A missing reference is an error naming the
key and the reference, never a value: `channel phone: token: secret telegram-token is not set
in ~/.config/ness/secrets/ness.toml`.

**File rules.** Created 0600 in a 0700 directory. If it is group/world readable, ness refuses to
read it and says `chmod 600 …` (like ssh).

**Redaction.** The resolver returns the resolved sensitive values; the existing
`notify.Redact`/`probe.Redact` mask all of them (not just the token) in atom errors, command
channel output, and probe output. Probes additionally redact every value in the project's
secrets file automatically; `redact_env` stays for variables ness does not manage.

**Doctor.** A new `secrets` check walks every reference in ness.toml: ok (`3 references
resolve`), or fail per missing one with the fix `ness secret set NAME`, plus a fail for a
too-open file. It prints names and paths only. `operator-notify` reuses the same message.

**What ness writes vs the user.**
- `ness secret set NAME` — reads the value from stdin (never argv, so it stays out of shell
  history and `ps`), writes the file atomically 0600. Prompts without echo on a TTY.
- `ness secret list` — names only, and which of them ness.toml references.
- `ness secret unset NAME`.
- The user writes the references in ness.toml by hand (`ness init` keeps not asking for
  channels).

**Migration.** `token_env = "X"` becomes `token = "env:X"`, identical behaviour. The old key is
refused with that exact rewrite in the message (pre-public v1: no alias kept). For this lab:
`ness secret set telegram-token` and `ness secret set telegram-chat` per project, ness.toml gets
`token = "secret:telegram-token"`, `chat_id = "secret:telegram-chat"` (or a literal), then the
hand-made drop-ins and env files are deleted and the units restarted — I do the repo side; the
deletion of `~/.config/ness/telegram-*.env` and the drop-ins I will do only on your go (or you).

**Out of scope.** The `command` channel gets no token of its own today and inherits the
worker's environment; it stays so (a command needing a secret reads it itself). Forges will use
the same table when they exist.

## Spec
core-config: R-CFG-080 reference syntax and sources; R-CFG-081 secret vs plain keys and literal
refusal (replaces R-CFG-046); R-CFG-082 file location, mode and refusal; R-CFG-083 `ness secret`
commands; R-CFG-084 `token_env` refused with its rewrite. notify: R-NTF-060 credentials resolved
through the references (replaces R-NTF-042); R-NTF-061 all referenced values redacted. doctor:
the `secrets` check.

## Tests (black-box, written before code)
Resolver table (literal, env, secret, missing, too-open file); config load refuses a literal
token and `token_env`; testscript: `ness secret set` from stdin then `ness post --dry-run` and
`ness doctor` resolve it with an empty environment; a failing command channel whose output
echoes a secret prints `[redacted]`; `ness secret list` never prints a value.
