> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Core: configuration, version pin, init, CLI conventions (CFG)

Packages: `internal/config`, `internal/cli`, `cmd/ness`. Sources: §1 (principles 6, 7), §3
(distribution), §5 (command list, `init`), §6 (`.ness/ness.toml`), D5, D15, D22, D28.

## Configuration

- **R-CFG-001** ness MUST read project configuration only from `.ness/ness.toml` at the
  repository root (and profiles under `.ness/profiles/`); it MUST NOT read any per-user or
  system-wide ness configuration file.
  Verify: testscript — with a `$HOME/.config/ness/` and `$XDG_CONFIG_HOME/ness/` tree holding
  conflicting settings, `ness sync --dry-run` and `ness doctor --json` output is byte-identical
  to a run with those trees absent.
- **R-CFG-002** ness MUST NOT change behaviour based on environment variables other than those
  the client, runtime and git define and those the design names (`[probes] redact_env` values,
  the variables named by `env:` references (R-CFG-080), and the terminal variables of R-UI-001 to R-UI-013:
  `NESS_UI`, `NESS_THEME`, `NO_COLOR`, `CLICOLOR_FORCE`, `TERM`, `COLUMNS`, `CI`, `ACCESSIBLE`,
  `CLAUDECODE`; the terminal libraries read their own, such as `COLORTERM`).
  Verify: review — grep of `os.Getenv`/`os.LookupEnv` call sites against the allowed list.
- **R-CFG-003** When `.ness/ness.toml` fails to parse, every command that reads it MUST refuse,
  naming the file, line and column of the error.
  Verify: testscript — malformed TOML; `ness sync`, `ness status`, `ness doctor` exit non-zero
  and stderr contains `.ness/ness.toml:<line>:<col>`.
- **R-CFG-004** When a value in `.ness/ness.toml` fails schema validation (wrong type, value
  outside its enumeration), every command that reads it MUST refuse, naming the key path and the
  accepted values.
  Verify: testscript — `client = "gpt"`; exit 4; stderr names `client` and lists `claude`,
  `codex`.
- **R-CFG-005** The `.ness/ness.toml` parser MUST NOT panic or hang on any input.
  Verify: fuzz — target over arbitrary bytes, seeded with the init output and every example in
  the spec; no panic, returns within 1 s per input.
- **R-CFG-006** Features (`probes`, `notify`, `contracts`) MUST be off unless enabled in
  `.ness/ness.toml` or by an applied profile.
  Verify: testscript — fresh `ness init`; `ness sync --dry-run` plans no feature skill files.

## Version pin

- **R-CFG-010** `.ness/ness.toml` MUST carry a version pin of the form `ness = "<major>.<minor>"`
  (e.g. `ness = "0.3"`), written by `ness init` as the running binary's major.minor.
  Verify: testscript — after `ness init --yes`, the file contains `ness = "<binary major.minor>"`.
- **R-CFG-011** For a 0.x pin (`ness = "0.N"`), every command (except `--version` and `--help`)
  MUST run only when the binary is exactly version 0.N (any patch), and refuse otherwise.
  Verify: testscript — pin `0.3`: binary 0.3.0 and 0.3.7 run; 0.2.9 and 0.4.0 exit 4.
- **R-CFG-012** For a pin with major ≥ 1 (`ness = "M.N"`), every command (except `--version`
  and `--help`) MUST run only when the binary's major equals M and its minor is ≥ N; a binary
  older than the pin, or of another major, MUST refuse.
  Verify: testscript — pin `1.2`: binary 1.2.0 and 1.5.1 run; 1.1.9 and 2.0.0 exit 4.
- **R-CFG-013** A version refusal MUST print the pinned range, the running binary's version and
  the running binary's absolute path.
  Verify: testscript — stderr contains `0.4`, the binary version string and `$(which ness)`.
- **R-CFG-014** Within the pinned minor, the binary MUST accept generated files whose lock
  records an older patch version, without requiring `ness sync`.
  Verify: testscript — lock records 0.3.0, binary 0.3.2; `ness send`/`ness inbox` succeed.

## `ness init`

- **R-CFG-020** `ness init` MUST create `.ness/ness.toml` and then perform `ness sync`.
  Verify: testscript — after `ness init --yes` the file and `.ness/lock.json` exist and
  `ness sync --check` exits 0.
- **R-CFG-021** `ness init` MUST refuse when `.ness/ness.toml` already exists, leaving it
  byte-identical.
  Verify: testscript — pre-existing file; exit non-zero; file hash unchanged; no lock written.
- **R-CFG-022** `ness init --profile NAME` MUST be repeatable and record the profiles in
  `.ness/ness.toml` in the order given.
  Verify: testscript — `--profile go-strict --profile behavior-specs` → the profile list in that
  order.
- **R-CFG-023** `ness init --client`, `--base` and `--gate "CMD"` MUST record the client, the
  base branch and the gate command in `.ness/ness.toml`.
  Verify: testscript — values read back from the file match the flags.
- **R-CFG-024** `ness init --yes` MUST NOT prompt and MUST complete without reading stdin.
  Verify: testscript — stdin closed; exit 0.
- **R-CFG-025** `ness init` MUST refuse an unknown profile name before writing any file.
  Verify: testscript — `--profile nope`; exit non-zero; `.ness/` absent.

## CLI surface and output

- **R-CFG-030** The binary MUST expose exactly these 17 top-level commands: `init`, `sync`,
  `doctor`, `stats`, `open`, `close`, `status`, `send`, `reply`, `inbox`, `integrate`, `probe`,
  `post`, `contract`, `worker`, `hook`, `secret` (CX-22).
  Verify: testscript — `ness --help` command list equals the set; an unknown command exits
  non-zero.
- **R-CFG-031** Flags removed from the old harness (§5 "Cut") MUST NOT exist: `--reconcile`,
  `--phase`, `--evidence`, `--pane`, `--workspace`, `--request-key`, `--hold`, `--review-at`,
  `--transfer-to`, `--via`, `--record-operator-response`, `--reap`, `--retain-work`, `--model`;
  nor commands `wait`, `ship`, `verify-release`, `watch`, `coordination-authorize`.
  Verify: testscript — each flag on its former command and each command exits non-zero with
  "unknown".
- **R-CFG-032** `status`, `inbox`, `doctor`, `stats`, `probe run`, `sync --dry-run` and `pick` MUST accept
  `--json` and then write a single JSON object to stdout and nothing else.
  Verify: testscript — for each command stdout parses as one JSON object; no non-JSON bytes.
- **R-CFG-037** Every JSON document ness prints MUST carry `"schema": 1` at the top level; the
  field-level schema of each command MUST be frozen by golden tests.
  Verify: testscript — each `--json` output has `schema` = 1 and matches its golden file.
- **R-CFG-033** Command output MUST go to injected writers (no direct `os.Stdout` use outside
  `cmd/ness`).
  Verify: review — grep for `os.Stdout`/`fmt.Print` outside `cmd/ness`.
- **R-CFG-034** A command that succeeds MUST exit 0.
  Verify: testscript — across every command's happy-path script.
- **R-CFG-035** Exit codes MUST be: 0 success; 1 a check found problems (`sync --check`,
  `doctor`, `contract check`, `probe run` with a non-GREEN state); 2 usage error (unknown
  command or flag, missing or invalid argument); 3 sync conflict; 4 refused (unsupported
  environment or unmet precondition). The one exception is `ness hook pre-tool`, whose block
  response is exit 2 as the client requires (R-HOOK-014).
  Verify: testscript — one script per code per command family asserting the exact code.
- **R-CFG-038** When a sync conflict and a check failure coincide, the exit code MUST be 3.
  Verify: testscript — `sync --check` with a pending conflict exits 3.
- **R-CFG-036** A refusal MUST exit 4, MUST NOT create or modify any file, store row or runtime
  tab, and MUST print to stderr what is missing and a fix.
  Verify: testscript — for each refusal script, exit 4 and tree hash and store dump before/after
  are equal.

## Schema of `.ness/ness.toml`

```toml
ness     = "0.3"                 # required: version pin
project  = "infra-lab"            # required: project name (init fills from the directory name)
base     = "main"                # required: base branch for roots and root integration
gate     = "make validate"       # required: gate command run by integrate
sessions = "docs/sessions"       # session-record directory (default "docs/sessions")
client   = "claude"              # claude | codex (default claude)
profiles = ["program", "go-strict"]
features = ["probes", "notify", "contracts"]
experimental = ["codex"]         # opt-in to experimental atoms

[worktree]
copy = [".env.example"]          # copied from the primary checkout into each new worktree
link = ["data/"]                 # symlinked from the primary checkout

[integrate]
mode = "squash"                  # squash | merge | rebase | pr (default squash)
push = false                     # push the target after a successful fold (default false)
setup = "just data-link"         # runs in the gate worktree before the gate (optional)

[contracts]
frozen = ["api/schema.json", "features/*.feature", "experiments/*/spec.md"]
                                 # project-wide frozen paths or path.Match globs (no **), checked
                                 # against the base branch: .feature scenario-locked, prefix .md
                                 # append-only, the rest whole-file (R-CTR-040)
prefix = ["experiments/*/spec.md"]  # path.Match globs of files frozen as an append-only prefix

[contracts.formats.pytest]       # a project-defined format, tried in file order before the builtins
match = ["tests/test_*.py"]      # path.Match globs (no **)
units = "tools/units.py"         # run in an export of the merge-base: text on stdin, path as last
                                 # argument (a label), prints {"id": ..., "body": ...} lines
                                 # (R-CTR-047, -048)

[probes]
redact_env = ["DEPLOY_TOKEN"]

[notify]
operator = "phone"               # the channel the worker tells the operator on (CX-16); optional

[notify.channels.releases]
kind = "slack"                   # slack | telegram | github | command, plus that kind's keys
token      = "secret:slack-token"  # a reference, never the token itself (R-CFG-080)
channel_id = "C0123"

[tiers.standard]
claude = "opus/medium"
codex  = "gpt-6.1-sol/medium"

[models]
policy = "declared"              # or "command" or "jev": the selector that picks a rung
base = "standard"                # the rung offsets count from, and every fallback
ceiling = ""                     # highest rung reachable; "" is the top of the ladder
ladder = ["bulk", "economy", "lean", "standard", "deep", "frontier", "apex"]  # cheapest first
command = "./pick"               # the command selector's program (policy = "command")
command_timeout = "10s"
router_threshold = 0.8
enforce = "refuse"               # or "log": the hook records a subagent call above its rung and allows it

[models.kinds]                   # offset from base per work kind; entries merge with the defaults
review = 2
fallback_model = "sonnet"        # optional; passed as --fallback-model only when set

[models.jev]                     # the jev selector (policy = "jev"); falls back to declared
model = "jev-1.14.0"             # a pinned version; aliases (jev-latest, jev-preview) are refused
key = "env:TYPESAFE_API_KEY"     # a reference (env:NAME or secret:NAME), never a literal (R-CFG-081)
endpoint = "https://api.typesafe.ai"  # https, or http to a loopback host only (R-MOD-069)
timeout = "5s"                   # bound on one request, retry included; > 0 and at most 10s

[launch]
ready_timeout = "60s"
permissions = "skip"             # or "client": the client's own permission settings
max_children = 0                 # children open at once across the project; 0: half the cores (R-SESS-152)

[worker]
stall_minutes = 30
quiet_seconds = 60

[[guards]]                       # project guards on pre-tool (R-CFG-086, R-HOOK-022)
tool    = "Bash"                 # regexp on the tool name
match   = '^pytest\s*$'          # regexp on the Bash command, or on the edited file path
message = "Run the one test you changed, not the suite."
skill   = "ness-entry"           # the pointer every refusal carries (R-RULE-001)
section = "Tests"
```

Kind keys for `[notify.channels.NAME]`: slack `token`, `channel_id`; telegram `token`,
`chat_id`; github `token`, `repo` (`owner/name`); command `command` (and an optional `token`,
masked in its output). Any channel value may be a reference instead of a literal (R-CFG-080).

- **R-CFG-040** `.ness/ness.toml` MUST be validated against the schema above; `ness`, `project`,
  `base` and `gate` MUST be present.
  Verify: testscript — each required key removed in turn → exit 4 naming the key.
- **R-CFG-041** `sessions` MUST default to `docs/sessions` and MUST be used as `<sessions>` by
  every command.
  Verify: testscript — unset → records under `docs/sessions/`; set to `rec` → under `rec/`.
- **R-CFG-042** `client` MUST default to `claude`; `integrate.mode` to `squash` (R-CFG-060); `integrate.push`
  to false; `models.policy` to `declared` (`text` is refused, replaced by `declared`); `models.base` to
`standard`; `models.ceiling` to empty (top rung); `models.ladder` and `models.kinds` to R-MOD-005
and R-MOD-008; `models.command_timeout` to 10 s; `models.router_threshold` to 0.8; `models.enforce` to `refuse`;
  `models.jev` to model `jev-1.14.0`, key `env:TYPESAFE_API_KEY`, endpoint `https://api.typesafe.ai`, timeout 5 s;
  `launch.ready_timeout` to 60 s; `launch.permissions` to `skip`; `launch.max_children` to 0 (half the cores); `worker.stall_minutes` to 30; `worker.quiet_seconds` to 60.
  Verify: unit — a minimal valid file loads with exactly these defaults.
- **R-CFG-043** An unknown key anywhere in `.ness/ness.toml` MUST be refused, naming its path.
  Verify: testscript — `[worker] stal_minutes = 5` → exit 4 naming `worker.stal_minutes`.
- **R-CFG-044** `client = "codex"` (or `--client codex`) MUST be refused unless `experimental`
  contains `"codex"`.
  Verify: testscript — without the opt-in → exit 4 naming `experimental`; with it → accepted.
- **R-CFG-045** A `[notify.channels.NAME]` entry with an unknown `kind`, or missing a key its
  kind requires, MUST be refused at config validation.
  Verify: testscript — `kind = "discord"` → exit 4; slack without `channel_id` → exit 4.
- **R-CFG-046** Superseded by R-CFG-081 (CX-22): a channel token is a reference, never a literal.
- **R-CFG-055** `[notify] operator = NAME` MUST name a key of `[notify.channels]` and MUST be
  refused when it does not, or when the `notify` feature is off, naming `notify.operator`.
  Verify: testscript — `operator = "nope"` → exit 4 naming `notify.operator`; set with
  `features = []` → exit 4 naming the feature; set with the feature on and the channel defined →
  accepted.
- **R-CFG-070** Each set of names (clients, integrate modes, model policies, features,
  experiments, hook events) MUST be written once; validation, type errors, profile seams, the
  atoms and the hook command read that list.
  Verify: unit — every client name builds the atom of that name; the existing refusal tests pass.

- **R-CFG-086** `[[guards]]` MAY repeat. Each entry has `tool` and `match` (regexps), an optional
  `content` (regexp on the new text of an edit, R-HOOK-022), and `message`, `skill`, `section`
  (all required, non-empty). Guards default to none.
  Verify: unit — two entries read back in file order with every key; a file without `[[guards]]`
  has none.
- **R-CFG-087** Config parse MUST refuse a guard whose `tool`, `match` or `content` is not a valid
  regexp, or whose `tool`, `match`, `message`, `skill` or `section` is missing or empty, naming the
  entry (`guards[N]`, zero-based) and the key. Whether `(skill, section)` resolves is checked by
  `ness sync` (R-PROF-046).
  Verify: unit — one refusal case per clause, each naming `guards[N]` and the key; testscript — see
  sync_guard_pointer.

## Secrets (CX-22)

- **R-CFG-080** A value of a `[notify.channels.NAME]` key MAY be a reference instead of a literal:
  `secret:NAME` reads NAME from the project's secrets file (R-CFG-082), `env:NAME` reads the
  environment variable NAME; any other value is the literal itself. NAME MUST match
  `[A-Za-z0-9_-]+`; a reference with another name MUST be refused at config validation. A
  reference whose value is missing or empty MUST fail the command that needs it, naming the key
  and the reference and never a value.
  Verify: unit — literal, `env:`, `secret:` and a missing reference resolve or fail as stated;
  testscript — `chat_id = "secret:bad name"` → exit 4.
- **R-CFG-081** A secret key (`token`) MUST be a reference: a literal MUST be refused at config
  validation, saying to use `secret:NAME` or `env:NAME`. A plain key (`chat_id`, `channel_id`,
  `repo`, `command`) MAY be a literal or a reference.
  Verify: testscript — `token = "xoxb-1"` → exit 4 naming `secret:`; `chat_id = "42"` and
  `chat_id = "secret:chat"` both load.
- **R-CFG-082** The secrets file of a project MUST be `<user config dir>/ness/secrets/<project>.toml`
  (`$XDG_CONFIG_HOME`, else `~/.config`), one `NAME = "value"` line per secret, outside every
  repository. ness MUST create it with mode 0600 in a 0700 directory and MUST refuse to read it
  when it is readable by group or others, saying `chmod 600 PATH`. A missing file holds no
  secrets.
  Verify: unit — a 0644 file → refused naming chmod; a missing file → every `secret:` reference
  missing.
- **R-CFG-083** `ness secret set NAME` MUST read the value from stdin (without echo on a TTY;
  trailing newline dropped) and never from the command line, and write it atomically;
  `ness secret list` MUST print the names only, each marked when the project's ness.toml
  references it; `ness secret unset NAME` MUST remove it. No command prints a secret value.
  Verify: testscript — set from stdin, then list shows the name and not the value; unset removes
  it; an empty value refuses.
- **R-CFG-084** The former key `token_env` MUST be refused naming its replacement
  `token = "env:NAME"`.
  Verify: unit — `token_env = "T"` → error naming `token = "env:T"`.
- **R-CFG-085** Doctor MUST report a `secrets` check: `ok` with the number of references when
  every reference of ness.toml resolves (none: `ok` saying so), else `fail` naming each missing
  one (key and reference) with the fix `ness secret set NAME` or `export NAME`; a too-open secrets
  file is a `fail` with the `chmod` fix. It MUST NOT print a value.
  Verify: testscript — a missing `secret:` → fail with the fix; after `ness secret set` → ok;
  stdout never holds the value.

## Init, project detection and features

- **R-CFG-050** `ness init` MUST fill `project` from the repository directory name unless given.
  Verify: testscript — repo dir `infra-lab` → `project = "infra-lab"`.
- **R-CFG-051** `ness init` on a TTY without `--yes` MUST prompt for each value not given by a
  flag.
  Verify: testscript with a pseudo-TTY — prompts appear for gate and base when the flags are
  absent.
- **R-CFG-052** `ness init` off a TTY without `--yes` MUST refuse when a required value is
  missing, listing the missing flags.
  Verify: testscript — no TTY, no `--gate` → exit 4; stderr names `--gate`.
- **R-CFG-053** Outside a ness project (no `.ness/ness.toml` from the cwd up to the repo root)
  only `init`, `--version` and `--help` MUST run; every other command MUST refuse, except
  `ness hook`, which MUST exit 0 silently, and `doctor` (R-DOC-023).
  Verify: testscript — `ness status` outside → exit 4; `ness hook pre-tool` → exit 0, no output.
- **R-CFG-054** A command of a disabled feature (`probe` without `probes`, `post` without
  `notify`, `contract` without `contracts`) MUST refuse, naming the feature to enable.
  Verify: testscript — `ness post …` with notify off → exit 4; stderr names `notify`.
- **R-CFG-060** `integrate.mode` MUST default to `squash`; a project keeps merge history only by
  setting `mode = "merge"` (D44).
  Verify: unit — a file without `[integrate]` loads with mode `squash`.
- **R-CFG-061** `[integrate] setup` is an optional shell command (default empty).
  Verify: unit — a file with `setup = "just data-link"` loads it.
