> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# ness's own repository quality gate (QA)

Scope: this repository. Sources: §1 (principle 8), §11 (ness's own code), roadmap invariants
I1–I6 and lanes, D1, D2, D28–D34.

## Build and layout

- **R-QA-001** The module path MUST be `github.com/enea-scaccabarozzi/ness` (D32, pending
  confirmation) and the binary MUST build with `CGO_ENABLED=0`.
  Verify: CI — `go list -m` equals it; `CGO_ENABLED=0 go build ./cmd/ness` succeeds.
- **R-QA-002** Packages MUST follow the §11 layout (`cmd/ness` main only, `internal/cli`,
  `config`, `sync`, `profile`, `store`, `hook`, `session`, `worker`, `probe`, `stats`, `notify`, `rule`,
  `shell`, `ui`, `check`, `ladder`, `atom/*`, `profiles/`).
  Verify: review + unit — a test listing packages against the allowed set.
- **R-QA-003** `cmd/ness` MUST contain only `main` wiring (composition root).
  Verify: review / go-strict audit.
- **R-QA-004** Releases MUST be built by goreleaser as static binaries.
  Verify: CI — `goreleaser check` and a snapshot build; `file` reports statically linked.
- **R-QA-046** The binary's release version MUST be the `-ldflags` value or the release tag it was
  built from, and the build MUST carry the commit it was built from: `--version`, the worker's
  heartbeat and its restart check (R-WRK-009) show `VERSION+REVISION[.dirty]` (12 hex
  characters), while the pin, the lock and the release notes use the release version alone.
  (Field feedback 3: a binary built from main past the tag reported `0.1.0`.)
  Verify: unit — the version and build from a pseudo-version build, a tag build and a build
  without version control; testscript — `--version` with a build shows `0.1.0+REV`.
- **R-QA-047** The CLI reference under `docs/reference/cli` MUST be generated from the command
  tree, one page per visible command, and MUST equal fresh output; text outside code MUST be
  escaped so a `<word>` or `{name}` cannot break the MDX build.
  Verify: unit — the checked-in pages equal fresh output (`-update` rewrites them); a long text
  with `<name>` and `{id}` renders escaped outside code and verbatim inside it.
- **R-QA-048** No private term of `.private/denylist.txt` MUST appear anywhere in the repository
  except `docs/discovery/`, `.git` and build or dependency output, binaries included; a term
  matches across any run of whitespace, so a term wrapped over a line is caught. The scan is
  skipped where the list is absent.
  Verify: unit — a planted term wrapped over a line break is found; the tree selection keeps
  `.agents/`, `.github/`, testdata and binaries and drops the excluded folders; the scan of the
  repository finds no term.

## The gate

- **R-QA-005** The gate MUST run, and fail on failure of any of: golangci-lint (go-strict config),
  test lint (`.golangci.tests.yml`), the compliance audit, `go test -race ./...`, govulncheck,
  `ness sync --check` on this repository.
  Verify: CI — introduce one violation per step in a throwaway branch; each fails the gate.
- **R-QA-006** `ness sync --check` on this repository MUST pass (ness uses go-strict on itself).
  Verify: CI step.
- **R-QA-007** The gate MUST run in GitHub Actions on every push from W0.
  Verify: review — workflow file; a green run on the skeleton.
- **R-QA-008** govulncheck MUST report no vulnerabilities in called code.
  Verify: CI step exit status.
- **R-QA-009** Tests MUST also pass with `-count=4`, run in CI only (`make repeat`), not in the
  gate integrate runs (D44).
  Verify: unit — `make repeat` holds `-count=4`, no earlier target does, and CI runs it.

## Requirement coverage (I4)

- **R-QA-010** The gate MUST fail when any requirement id defined in `docs/spec/*.md` has no
  `covers:` marker in a Go test, a testscript file, or the review ledger.
  Verify: unit — the coverage checker on a fixture spec with one uncovered id exits non-zero
  naming it.
- **R-QA-011** The coverage checker MUST recognise `// covers: R-…` in Go test files and
  `# covers: R-…` in testscript files, with several ids per marker.
  Verify: unit — fixtures for each form.
- **R-QA-013** The gate MUST fail when a `covers:` marker or a reviews.md entry names an id not
  defined in `docs/spec/`.
  Verify: unit — fixture marker `R-SYNC-999` → exit non-zero naming it.
- **R-QA-012** A `review` requirement MUST count as covered only by an entry in
  `docs/spec/reviews.md` naming the id, reviewer session, date and reviewed commit.
  Verify: unit — entry missing a field → uncovered.

## Tests

- **R-QA-020** CLI behaviour MUST be tested with testscript against throwaway git repositories
  and the `internal/testkit` fakes.
  Verify: review — each CLI requirement's covering test is a testscript.
- **R-QA-021** Tests MUST be hermetic: they clear `GIT_*` environment variables and refuse to run
  git or write files outside a temporary directory.
  Verify: unit — the testkit setup fails a test that targets a path outside `t.TempDir()`; with
  `GIT_DIR` set in the outer env, tests still pass.
- **R-QA-022** Every parser that reads user files MUST have a fuzz target: `ness.toml`,
  `profile.toml`, `lock.json`, managed blocks, settings.json/config.toml editors, probe
  frontmatter and `key=value` lines, hook payloads, transcripts, limit notices, Markdown
  renderers; CI MUST run each target for 30 s per push and for 10 min nightly.
  Verify: review — a `Fuzz*` per parser; CI workflow steps with `-fuzztime=30s` and the nightly
  `-fuzztime=10m` job.
- **R-QA-023** Mutation testing with gremlins MUST run weekly on `internal/sync` and
  `internal/store`, report-only in v1 (it never fails the gate).
  Verify: review — scheduled workflow; report stored as an artifact.
- **R-QA-024** Test fakes for runtime, client, integrators and channels MUST live in
  `internal/testkit` and pass the same contract suites as the real atoms.
  Verify: contract(fake+real).
- **R-QA-025** Acceptance tests MUST be written and frozen by a separate session before
  implementation, and the implementer MUST NOT edit them (D33) (withdrawn: D39 replaces the separate test writer with TDD inside each plan task).
  Verify: review — git history shows test commits by the test-writer session before the
  implementation commits; frozen with `ness contract` once available.
- **R-QA-026** Tests MUST NOT use `time.Sleep`; timers are tested with `testing/synctest`.
  Verify: test lint (`.golangci.tests.yml`).
- **R-QA-027** Real-client e2e tests MUST be behind the build tag `e2e`, run locally on demand
  (never in default CI), and MUST abort when the client's usage file shows weekly usage ≥ 80%
  (D31).
  Verify: unit — the e2e runner with a usage-file fixture at 80% aborts before launching;
  `go test ./...` without the tag compiles no e2e test.

## Process

- **R-QA-030** Commits MUST follow Conventional Commits.
  Verify: CI — commit lint on the PR range.
- **R-QA-031** Releases MUST be cut with release-please, whose notes include a "running sessions"
  section.
  Verify: review — release-please config with that section.
- **R-QA-044** The go-strict pre-push hook MUST run only build, lint, audit and the sync check; the
  test suite runs in the gate integrate uses and in CI (D44).
  Verify: unit — the rendered `lefthook.yml` runs `make build lint audit selfcheck`.
- **R-QA-045** The commit-msg check MUST accept the subject `integrate SOURCE into TARGET` that
  ness writes with `integrate.mode = "merge"`.
  Verify: testscript — `builtin_git_conventional.txtar` commits that subject.
- **R-QA-032** The public repository MUST be published from a fresh history that never contains
  `docs/discovery/` (D30).
  Verify: review — `git log --all -- docs/discovery` empty in the public repo.
- **R-QA-033** After the first version, open-source docs (philosophy, what it is, why it exists)
  MUST exist before publication (D29).
  Verify: review.

## Decided details

- **R-QA-040** There MUST be no line-coverage floor; requirement coverage (R-QA-010) is the floor.
  Verify: review — no coverage-percentage step in CI.
- **R-QA-041** CI MUST install python3 via mise, pinned in the repo's mise config, for the
  compliance audit.
  Verify: review — `mise.toml` pins python; CI step uses `mise install`.
- **R-QA-042** Any locally run real-client work (e2e, skill evals) MUST apply the same usage stop
  rule as R-QA-027.
  Verify: review — eval runner calls the same guard.

- **R-QA-043** The Go module path MUST be `github.com/enea-scaccabarozzi/ness` (D38).
  Verify: unit — a test reads go.mod and asserts the module line.
