> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Profiles: format, slots, conflicts (PROF)

Package: `internal/profile`. Sources: §2 (blocks, project layer), §7, §16 (#8, #9), D3, D27.
What each built-in profile renders is in [profiles-builtin.md](profiles-builtin.md).

## Format and loading

- **R-PROF-001** A profile MUST be a directory containing `profile.toml` plus the files it
  renders; `profile.toml` carries `name`, `summary`, and optionally `requires`, `conflicts`,
  `requires_tools`, `[slots]`, `[files]`, `[config]`, `[features]`, `[atoms]`, `[[checks]]` (syntax below).
  Verify: unit — the §7 example and the syntax example below load; each field reads back.
- **R-PROF-002** Built-in profiles MUST be embedded in the binary (`embed.FS`) and carry the
  binary's version.
  Verify: testscript — with no `profiles/` directory on disk, `ness init --profile program` works;
  the lock records the profile version = binary version.
- **R-PROF-003** Local profiles MUST be loaded from `.ness/profiles/<name>/`.
  Verify: testscript — a local profile named in config renders its block.
- **R-PROF-005** The `profile.toml` parser MUST NOT panic on any input and MUST refuse invalid
  input naming file, line and field.
  Verify: fuzz — arbitrary bytes seeded with every built-in `profile.toml`.
- **R-PROF-006** A slot target or file a profile references that does not exist in the profile
  directory MUST make sync refuse, naming the profile and the missing file.
  Verify: testscript — `{ append = "missing.md" }`; exit non-zero; tree unchanged.

## Syntax

```toml
name    = "example"
summary = "one line"
requires  = ["go-strict"]
conflicts = ["other"]
requires_tools = ["python3"]

[slots]
"ness-entry#stop-and-ask"  = { append = "stop-and-ask.md" }   # list slot
"ness-program#autonomy" = { set = "autonomy.md" }    # single slot
"AGENTS.md#local"          = { block = "agents-local.md" }    # AGENTS.md block

[files]
"docs/workflow/ops-policy.md" = { seed = "ops-policy.md" }    # created once
".golangci.yml"               = { own = "golangci.yml" }      # owned

[config]                      # defaults for .ness/ness.toml keys, by dotted path
"integrate.push" = true

[features]
enable = ["contracts"]

[atoms]                       # seam = atom
integrator = "squash"         # seams: client, integrator, model_policy
```

- **R-PROF-007** `[files]` entries MUST be `{ own = FILE }` or `{ seed = FILE }`; any other form
  MUST be refused.
  Verify: unit — both accepted; `{ copy = … }` refused naming the entry.
- **R-PROF-008** `[config]` keys MUST be dotted `.ness/ness.toml` paths validated against the
  config schema; a value set in `.ness/ness.toml` MUST win over a profile default.
  Verify: unit — profile `integrate.push = true`, config `push = false` → false; unknown path →
  refused.
- **R-PROF-009** `[features] enable` MUST accept only `probes`, `notify`, `contracts`; `[atoms]`
  MUST accept only known seams and atoms.
  Verify: unit — `enable = ["x"]` and `integrator = "octopus"` refused.

## Capabilities

- **R-PROF-010** A profile MUST be able to add owned files; ness owns them per sync.md rules.
  Verify: testscript — owned file created and listed in the lock with owner = the profile.
- **R-PROF-011** A profile MUST be able to add seeded files (`{ seed = FILE }`), created once if
  missing.
  Verify: testscript — see R-SYNC-070.
- **R-PROF-012** A profile MUST be able to `append` to a list slot.
  Verify: unit — appended text appears in the slot of the rendered skill.
- **R-PROF-013** A profile MUST be able to `set` a single slot.
  Verify: unit — the slot holds exactly the set text.
- **R-PROF-014** A profile MUST be able to contribute a `block` to AGENTS.md.
  Verify: testscript — `"AGENTS.md#local" = { block = … }` renders a managed block.
- **R-PROF-015** A profile MUST be able to set config defaults, enable features and choose atoms.
  Verify: testscript — a profile with `[features] enable = ["probes"]` makes sync render the probes
  skill text.
- **R-PROF-016** Every profile requirement in `requires_tools` MUST be checked at sync and doctor
  by looking the tool up on `PATH`.
  Verify: testscript — see R-SYNC-005, R-DOC-009.
- **R-PROF-017** Slots MUST be addressed as `<skill-or-file>#<slot>` (e.g.
  `ness-entry#stop-and-ask`, `AGENTS.md#local`).
  Verify: unit — slot key parser accepts the §7 examples, rejects keys without `#`.

## Order and the project layer

- **R-PROF-020** Profiles MUST be applied in the order listed in `.ness/ness.toml`.
  Verify: unit — two profiles appending to one list slot render in list order; swap the order and
  the render swaps.
- **R-PROF-021** The project's own profile (`.ness/profiles/local/`) MUST be applied last,
  regardless of its position in the list.
  Verify: unit — `local` listed first; its list-slot text still appears last.
- **R-PROF-022** Only the project's own profile, always named `local` (`.ness/profiles/local/`),
  MAY override a single slot another profile set; the override MUST replace the other profile's
  text.
  Verify: unit — program sets `ness-program#autonomy`, local sets it; render holds local's
  text; no error.
- **R-PROF-023** Seam choices and owned paths MUST NOT be overridable by the `local` profile; a
  seam conflict is resolved only by `.ness/ness.toml`.
  Verify: unit — `local` `[atoms] integrator` against a conflicting profile → still an error;
  `[integrate] mode` in ness.toml → resolved.

## Conflict rules (§7 table)

- **R-PROF-030** Two profiles appending to the same list slot MUST NOT be an error.
  Verify: unit — two appends → exit 0, both present.
- **R-PROF-031** Two non-project profiles setting the same single slot MUST be an error naming
  both profiles and the slot, unless the project's profile sets it.
  Verify: testscript — two local test profiles set `x#y`; sync exits non-zero naming both; add a
  `local` set → exit 0.
- **R-PROF-032** Two profiles choosing different atoms for the same seam MUST be an error naming
  both and the seam, unless `.ness/ness.toml` sets the seam.
  Verify: testscript — integrator `squash` vs `pr`; error; set in config → exit 0.
- **R-PROF-033** Two profiles setting the same config key to different values MUST be an error
  naming both and the key, unless `.ness/ness.toml` sets the key.
  Verify: testscript — as above for one key.
- **R-PROF-034** There MUST be no "last wins" resolution between non-project profiles.
  Verify: unit — every conflicting pair in R-PROF-031..033 errors regardless of list order.
- **R-PROF-035** Two profiles claiming the same owned path MUST be an error naming both and the
  path, with no project override.
  Verify: testscript — two profiles own `docs/x.md`; error even with a `local` profile.
- **R-PROF-036** A profile whose `conflicts` names another applied profile MUST be an error
  naming both.
  Verify: testscript — `conflicts = ["program"]` with program enabled; error.
- **R-PROF-037** Profile conflicts MUST be detected before any file is written.
  Verify: testscript — conflicting profiles; tree unchanged after the failed sync.
- **R-PROF-038** Two profiles that append a size to the sizes list slot MUST both appear (the research lab
  line and program, §16 #8).
  Verify: unit — both sizes rendered, in profile order.

## Decided details

- **R-PROF-040** A profile whose `requires` names a profile that is not applied MUST be an error
  naming both (no implicit enabling).
  Verify: testscript — `behavior-specs` without `go-strict` → exit 4 naming both.
- **R-PROF-041** A slot key naming a slot that does not exist in its target skill or file MUST
  be an error naming the profile and the slot.
  Verify: testscript — `"ness-entry#nope"` → exit 4.
- **R-PROF-042** A local profile with the same name as a built-in MUST be an error.
  Verify: testscript — `.ness/profiles/program/` → exit 4 naming `program`.
- **R-PROF-043** A local profile's version recorded in the lock MUST be the SHA-256 of its
  directory content (sorted paths and bytes).
  Verify: unit — editing any file of the profile changes the recorded version; reordering
  nothing leaves it equal.
- **R-PROF-044** A profile whose `name` differs from its directory name MUST be refused, naming
  both.
  Verify: testscript — dir `a`, name `b`; exit 4 naming `a` and `b`.

- **R-PROF-045** A profile MAY declare `[[checks]]` (the runner is in [checks.md](checks.md)). Each
  entry MUST carry `id` (unique in the profile), `on` (`open`, `integrate` or `close`), `skill` and
  `section`, and exactly one of `kind` or `run`; `message` is required with a `kind` and optional
  with `run`; `for` (`child` or `root`) and, with `run` only, `timeout` (a duration) are optional.
  A `kind` takes flat arguments: `section-matches` takes `file`, `heading`, `require` (a list of
  regexps); `branch-forbids` takes `path` and `pattern` (regexps); `match-last` takes `file`,
  `pattern`, `against_file`, `against_pattern`; `checkpoint-since-landing` takes none. The parser
  MUST refuse, naming `profile.toml` and the field, an unknown field, an unknown kind, a missing or
  unknown kind argument, a regexp that does not compile, a duplicate id, a missing required field,
  both or neither of `kind` and `run`, a bad `on` or `for`, and a `file` or `against_file` that is
  absolute or contains `..`.
  Verify: unit — a good declaration of every kind and of `run` reads back; one refusal case per
  clause above.
- **R-PROF-046** `ness sync` (so `ness sync --check`) MUST refuse a `[[checks]]` entry of any
  applied profile, local included, and a `[[guards]]` entry of `.ness/ness.toml`, whose
  `(skill, section)` pointer does not resolve in the rendered skills, naming the entry
  (`<profile>/<id>`) and listing the section headings that exist; nothing is written.
  Verify: testscript — a local check with a missing section exits 4 naming `local/<id>` and the
  existing headings, the tree is unchanged, `--check` also exits 4; the corrected pointer exits 0.
- **R-PROF-047** `Resolved.Checks` MUST list the checks of the applied profiles in profile order,
  the local profile last, each id prefixed `<profile>/`.
  Verify: unit — three profiles given local first resolve to `a/..`, `b/..`, `local/..` in
  declaration order.

## Autonomy slots

- **R-PROF-080** `ness-entry` MUST provide the single slot `ness-entry#autonomy-default`, the
  level a session uses when its brief states none; core MUST set it to "act within the brief", and
  a project's local profile MUST be able to replace it.
  Verify: testscript — the core text renders "act within the brief"; a local `set` replaces it.
- **R-PROF-081** `ness-program`'s Autonomy section MUST name the modes (gate, bands, to checkpoint X,
  custom) and say the mode is asked per run, whatever text fills the slot.
  Verify: testscript — the section renders with the default text and with a local one.
- **R-PROF-082** `ness-launch` MUST route delegated work to a subagent by default and to a child
  only when the work needs a conversation, its own decomposition, or isolation or duration; size
  alone MUST NOT be a reason for a child, and the child cap MUST NOT be a reason for a subagent.
  `ness-entry` MUST point there.
  Verify: testscript — the section and the pointer render. Eval — `make eval-delegation`: four
  scenarios, the expected route for every item (main's text: 2 of 4 scenarios at 6/6; this text:
  4 of 4).
- **R-PROF-083** `ness-entry` MUST tell sessions to test the packages they touch and leave the
  full gate to `ness integrate`.
  Verify: testscript — the sentence renders.
- **R-PROF-084** A profile check MAY declare `size`, a non-empty list of size names; an empty list,
  a non-list or a non-string element MUST be refused at parse, and a name no profile declares (nor `quick` or `full`)
  MUST be refused at resolve, naming the check.
  Verify: unit `TestParseCheckSizeList` over `testdata/checks/size.toml` and `bad/size_*.toml`.
- **R-PROF-085** A profile MAY declare `sizes`, a list of size names (`[a-z][a-z0-9-]{0,31}`)
  it adds to `quick` and `full`. The resolved set MUST list `quick`, `full`, then each profile's;
  a name repeated by two profiles, or equal to `quick` or `full`, MUST be refused naming both.
  Verify: unit `TestResolveSizes`, `TestResolveSizeConflicts`, `TestParseRefusesABadSizeName`.
