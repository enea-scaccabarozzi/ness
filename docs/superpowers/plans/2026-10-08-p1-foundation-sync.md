> [!NOTE]
> Archive: altered for the public release. Names of private projects and local paths were replaced; the content is otherwise as written.

# P1 Foundation and Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A `ness` binary that runs `ness init` and `ness sync` safely in any git repository: it reads `.ness/ness.toml`, enforces the version pin, loads and combines profiles with conflict detection, and renders files, managed blocks, JSON keys and symlinks under a hashed lock. It never overwrites content it did not write.

**Architecture:** `internal/config` parses and validates the configuration. `internal/profile` loads profiles (the embedded core, built-ins and local ones) and resolves them into one `Resolved` value or a conflict error. `internal/sync` turns that value into desired entries (`Render`), compares them with the disk and the lock (`Compute`), and writes the result (`Apply`). `internal/cli` holds thin cobra commands. `internal/testkit` provides hermetic testscript setup. The gate is golangci-lint (go-strict config), `go test -race`, govulncheck, a requirement-coverage test and `ness sync --check` on this repository.

**Tech Stack:** Go 1.25+ (local 1.27.1), cobra, BurntSushi/toml, tidwall/gjson+sjson, aymanbagabas/go-udiff, rogpeppe/go-internal/testscript. Tools come from mise and CI runs on GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-08-ness-v1-scope.md` (split), `docs/design/ness-design.md` §2, §5–7, §11, and the requirements in `docs/spec/core-config.md`, `sync.md`, `profiles.md`, `quality.md`, `models.md` (tier table, subagents).

## Global Constraints

- Module `github.com/enea-scaccabarozzi/ness`, one binary, `CGO_ENABLED=0` (R-QA-001, R-QA-043).
- Go directive `go 1.25`. File APIs go through `os.Root` (Go 1.25 methods), so sync cannot write outside the repository (R-SYNC-072).
- Exit codes: 0 ok, 1 a check found problems, 2 usage, 3 sync conflict, 4 refused (R-CFG-035). A refusal writes nothing (R-CFG-036).
- No global config. The only env var production code reads is none: the test binary reads `NESS_TEST_VERSION` in test code only (R-CFG-001, R-CFG-002).
- Output goes to the injected writers in `cli.Deps`. Only `cmd/ness` touches `os.Stdout` (R-CFG-033).
- Lock hashes are SHA-256 hex and the lock carries `"schema": 1` (R-SYNC-087).
- Block markers: `<!-- ness:begin owner/block -->` in Markdown and `# ness:begin owner/block` elsewhere (R-SYNC-040).
- Hook commands are wrapped as `command -v ness >/dev/null && ness hook <EVENT> || true` (R-SYNC-054).
- `autoCompactWindow` = 265000 (R-SYNC-084). `env.CLAUDE_CODE_SUBAGENT_MODEL` = the economy tier's Claude model (R-SYNC-085).
- Default tiers (R-MOD-001): standard `opus/medium` · `gpt-6.1-sol/medium`; frontier `fable/medium` · `astra/medium`; economy `sonnet/medium` · `gpt-6-luna/low`.
- Subagents (R-MOD-020): ness-sweep = economy model at low effort; ness-review = standard at medium; ness-verify = standard at high; ness-design = frontier at medium.
- Every test names the requirement ids it covers: `// covers: R-…` in Go tests and `# covers: R-…` in scripts.
- Conventional commits. Each commit ends with the session's Co-Authored-By/Claude-Session lines.

## Review Focus

1. **A user's hand-edited `.claude/settings.json` with comments-free but oddly formatted JSON, or invalid JSON.** Sync must preserve every unowned byte, and must refuse an unparsable file by name while other files still sync. Pinned in Task 7 (`TestJSONKeepsUnownedBytes`, `TestJSONInvalid`) and Task 13 (`sync_json.txtar`).
2. **A user keeps their version of a file (`--keep`) and later the profile stops rendering it.** The orphan path must not delete the kept file. Pinned in Task 12 (`TestOrphanOfKeptVersionIsKept`).
3. **AGENTS.md whose text quotes a marker inside a code fence, or has CRLF line endings.** The parser sees fence lines as markers (known ceiling, marked `ponytail:`). CRLF must still be recognised. Pinned in Task 6 (`TestBlocksCRLF`).
4. **Running `ness sync` twice in a row.** The second run must be a no-op with exit 0 and `--check` exit 0 (deterministic render and lock). Pinned in Task 13 (`sync_scope.txtar`).
5. **`ness init` in a repository with a detached HEAD.** Init cannot infer `base`, so it must refuse, naming `--base`. Pinned in Task 14 (`init.txtar`).

## Deferred to later plans

These ids belong to areas P1 delivers, but they need commands or text from a later plan. The coverage test (Task 15) lists them so it stays green while the gap stays visible:

- P2: R-CFG-014 (needs `send`/`inbox`), R-CFG-053 (`hook` and `status` outside a project).
- P3: R-CFG-036 (store rows and tabs), R-SYNC-068, R-SYNC-086 (Codex keys), R-QA-024, R-QA-027, R-QA-042.
- P5: R-CFG-030, R-CFG-031 (the full command set), R-CFG-054 (feature commands), R-PROF-015 (feature skill text).
- P6: R-CFG-032, R-CFG-034, R-CFG-035, R-CFG-037 (every command), R-CFG-051 (TTY prompts), R-SYNC-032, R-SYNC-033, R-SYNC-089 (release notes), R-PROF-002 (needs a built-in profile), R-QA-004, R-QA-005, R-QA-023, R-QA-030, R-QA-031, R-QA-041.
- Publication: R-QA-032, R-QA-033.
- Withdrawn by D39: R-QA-025 (separate test writer).
- Review entries in `docs/spec/reviews.md`, written by the end-of-plan reviewer: R-CFG-002, R-CFG-033, R-SYNC-083, R-QA-002, R-QA-003, R-QA-007, R-QA-008, R-QA-009, R-QA-020, R-QA-022, R-QA-026, R-QA-040.

## File map

```
go.mod, go.sum                      module, tool directive for govulncheck
cmd/ness/main.go                    composition root
internal/cli/cli.go                 Main, Deps, exit codes, root command
internal/cli/project.go             repository root, config load, pin check
internal/cli/sync.go                ness sync, shared plan/apply/report
internal/cli/init.go                ness init
internal/cli/script_test.go         testscript runner
internal/cli/testdata/script/*.txtar
internal/config/config.go           schema, Parse, defaults, validation
internal/config/pin.go              version pin
internal/profile/profile.go         profile.toml, Source.Load
internal/profile/resolve.go         Resolve: order, slots, files, defaults, conflicts
internal/sync/blocks.go             managed blocks
internal/sync/jsonkeys.go           owned JSON keys and hook entries
internal/sync/lock.go               .ness/lock.json
internal/sync/render.go             Resolved + Config -> []Entry
internal/sync/engine.go             Compute, Keep, Apply
internal/testkit/testkit.go         Setup, Cmds for scripts
internal/testkit/coverage.go        requirement coverage scan
profiles/embed.go                   embedded core and built-ins
profiles/core/...                   router block, ness-entry skill
profiles/builtin/README.md          placeholder until P6 adds built-ins
.golangci.yml, .golangci.tests.yml, mise.toml, Makefile, .github/workflows/ci.yml
docs/spec/reviews.md                review ledger (R-QA-012)
```

---

### Task 1: Skeleton, script harness, gate and CI

**Files:**
- Create: `go.mod`, `cmd/ness/main.go`, `internal/cli/cli.go`, `internal/cli/script_test.go`, `internal/cli/testdata/script/version.txtar`, `internal/testkit/testkit.go`, `internal/testkit/repo_test.go`, `.golangci.yml`, `.golangci.tests.yml`, `mise.toml`, `Makefile`, `.github/workflows/ci.yml`

**Interfaces:**
- Produces: `cli.Main(args []string, d cli.Deps) int`. `cli.Deps{Stdin io.Reader; Stdout, Stderr io.Writer; Version string}`. Constants `cli.ExitOK=0, ExitProblems=1, ExitUsage=2, ExitConflict=3, ExitRefused=4`. Unexported helpers `refuse(format, args...) error`, `exitCode(code int) error` (a silent exit), and `testkit.Setup`, `testkit.Cmds()`. Script commands: `exits CODE CMD ARGS...`, `snap NAME [DIR]`, `samesnap NAME [DIR]`, `islink PATH TARGET`.

- [ ] **Step 1: Module and dependencies**

```bash
cd ~/src/ness
go mod init github.com/enea-scaccabarozzi/ness
go mod edit -go=1.25
go get github.com/spf13/cobra@v1.10.2 github.com/BurntSushi/toml@v1.6.0 \
  github.com/tidwall/gjson@v1.20.0 github.com/tidwall/sjson@v1.2.5 \
  github.com/aymanbagabas/go-udiff@v0.4.1 github.com/rogpeppe/go-internal@v1.16.0
go get -tool golang.org/x/vuln/cmd/govulncheck@latest
```

`mise.toml`:
```toml
[tools]
go = "1.27.1"
golangci-lint = "2.13.2"
```

- [ ] **Step 2: Write the failing script and repo test**

`internal/cli/testdata/script/version.txtar`:
```
# The binary reports its version, and an unknown command is a usage error.
# covers: R-QA-001
exec ness --version
stdout '^ness version 0.1.0$'
exits 2 ness nope
stderr 'unknown command'
```

`internal/testkit/repo_test.go`:
```go
package testkit_test

import (
	"os"
	"strings"
	"testing"
)

func TestModulePath(t *testing.T) {
	// covers: R-QA-043, R-QA-001
	data, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(data), "\n")
	if first != "module github.com/enea-scaccabarozzi/ness" {
		t.Fatalf("go.mod starts with %q", first)
	}
}
```

`internal/cli/script_test.go`:
```go
package cli_test

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"ness": func() {
			// Test-only seam: scripts pick the binary version to exercise the pin (R-CFG-011).
			v := os.Getenv("NESS_TEST_VERSION")
			if v == "" {
				v = "0.1.0"
			}
			os.Exit(cli.Main(os.Args[1:], cli.Deps{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Version: v}))
		},
	})
}

func TestScripts(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:   "testdata/script",
		Setup: testkit.Setup,
		Cmds:  testkit.Cmds(),
	})
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/...`
Expected: FAIL to build (`undefined: cli.Main`, `undefined: testkit.Setup`).

- [ ] **Step 4: Implement the CLI root, main and testkit**

`internal/cli/cli.go`:
```go
// Package cli implements the ness command line. Commands are thin: they parse flags, call the
// internal packages and write to the injected writers.
package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Exit codes (R-CFG-035).
const (
	ExitOK       = 0
	ExitProblems = 1
	ExitUsage    = 2
	ExitConflict = 3
	ExitRefused  = 4
)

// Deps carries what a command needs from the process.
type Deps struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Version string
}

// exitError ends a command with a specific exit code; a nil err exits silently.
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string {
	if e.err == nil {
		return fmt.Sprintf("exit %d", e.code)
	}
	return e.err.Error()
}

func (e *exitError) Unwrap() error { return e.err }

// refuse is the error of an unmet precondition: exit 4, nothing written.
func refuse(format string, a ...any) error {
	return &exitError{code: ExitRefused, err: fmt.Errorf(format, a...)}
}

// exitCode ends the command with code and no further message.
func exitCode(code int) error { return &exitError{code: code} }

// Main runs ness with args and returns the process exit code.
func Main(args []string, d Deps) int {
	root := newRoot(d)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return ExitOK
	}
	var ee *exitError
	if errors.As(err, &ee) {
		if ee.err != nil {
			_, _ = fmt.Fprintln(d.Stderr, "ness:", ee.err)
		}
		return ee.code
	}
	_, _ = fmt.Fprintln(d.Stderr, "ness:", err)
	return ExitUsage
}

func newRoot(d Deps) *cobra.Command {
	root := &cobra.Command{
		Use:           "ness",
		Short:         "Install and run an AI coding-agent harness in this project",
		Version:       d.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetIn(d.Stdin)
	root.SetOut(d.Stdout)
	root.SetErr(d.Stderr)
	root.CompletionOptions.DisableDefaultCmd = true
	return root
}
```

`cmd/ness/main.go`:
```go
// Command ness installs and runs an AI coding-agent harness inside one project.
package main

import (
	"os"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
)

// version is set at release time with -ldflags "-X main.version=<version>".
var version = "0.1.0"

func main() {
	os.Exit(cli.Main(os.Args[1:], cli.Deps{
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: version,
	}))
}
```

`internal/testkit/testkit.go`:
```go
// Package testkit is the shared test setup: hermetic git repositories and custom commands for
// CLI scripts. Later plans add the atom fakes here.
package testkit

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"
)

const gitconfig = `[user]
	name = ness test
	email = test@example.invalid
[init]
	defaultBranch = main
[commit]
	gpgsign = false
`

// Setup makes a script hermetic (R-QA-021): it refuses a work dir outside the temp dir, drops
// inherited GIT_* variables, points HOME inside $WORK and creates a git repository at
// $WORK/repo on branch main with one empty commit.
func Setup(e *testscript.Env) error {
	if err := insideTemp(e.WorkDir); err != nil {
		return err
	}
	vars := make([]string, 0, len(e.Vars))
	for _, kv := range e.Vars {
		if !strings.HasPrefix(kv, "GIT_") {
			vars = append(vars, kv)
		}
	}
	e.Vars = vars
	home := filepath.Join(e.WorkDir, ".home")
	e.Setenv("HOME", home)
	e.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if err := os.MkdirAll(home, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfig), 0o600); err != nil {
		return err
	}
	repo := filepath.Join(e.WorkDir, "repo")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		return err
	}
	for _, args := range [][]string{{"init", "-q"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir, cmd.Env = repo, e.Vars
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w\n%s", args, err, out)
		}
	}
	return nil
}

func insideTemp(dir string) error {
	tmp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return err
	}
	abs, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(abs, tmp+string(filepath.Separator)) {
		return fmt.Errorf("work dir %s is outside %s; refusing to run git there", abs, tmp)
	}
	return nil
}

// Cmds returns the custom script commands:
//
//	exits CODE CMD [ARGS...]  run CMD; fail unless it exits with CODE
//	snap NAME [DIR]           record a hash of the tree under DIR (default .), .git excluded
//	samesnap NAME [DIR]       fail unless the tree matches snapshot NAME (negatable)
//	islink PATH TARGET        fail unless PATH is a symlink whose target is TARGET
func Cmds() map[string]func(ts *testscript.TestScript, neg bool, args []string) {
	return map[string]func(ts *testscript.TestScript, neg bool, args []string){
		"exits":    exits,
		"snap":     snap,
		"samesnap": samesnap,
		"islink":   islink,
	}
}

func exits(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) < 2 {
		ts.Fatalf("usage: exits CODE CMD [ARGS...]")
	}
	want, err := strconv.Atoi(args[0])
	ts.Check(err)
	got := 0
	if err := ts.Exec(args[1], args[2:]...); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			ts.Fatalf("%s: %v", args[1], err)
		}
		got = ee.ExitCode()
	}
	if got != want {
		ts.Fatalf("%s exited %d, want %d", args[1], got, want)
	}
}

func snapDir(ts *testscript.TestScript, args []string) (name, dir string) {
	if len(args) < 1 || len(args) > 2 {
		ts.Fatalf("usage: snap|samesnap NAME [DIR]")
	}
	dir = "."
	if len(args) == 2 {
		dir = args[1]
	}
	return args[0], ts.MkAbs(dir)
}

func snap(ts *testscript.TestScript, _ bool, args []string) {
	name, dir := snapDir(ts, args)
	h, err := treeHash(dir)
	ts.Check(err)
	ts.Setenv("SNAP_"+name, h)
}

func samesnap(ts *testscript.TestScript, neg bool, args []string) {
	name, dir := snapDir(ts, args)
	h, err := treeHash(dir)
	ts.Check(err)
	if same := h == ts.Getenv("SNAP_"+name); same == neg {
		ts.Fatalf("tree %s: same as snapshot %s = %v", dir, name, same)
	}
}

func islink(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: islink PATH TARGET")
	}
	got, err := os.Readlink(ts.MkAbs(args[0]))
	ts.Check(err)
	if got != args[1] {
		ts.Fatalf("%s links to %q, want %q", args[0], got, args[1])
	}
}

// treeHash hashes paths, types, file bytes and link targets under dir, skipping .git.
func treeHash(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(h, "%s\x00%v\x00", rel, d.Type())
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			_, _ = h.Write([]byte(target))
		case d.Type().IsRegular():
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			_, _ = h.Write(b)
		}
		_, _ = h.Write([]byte{0})
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}
```

- [ ] **Step 5: Gate files**

`.golangci.yml` (go-strict, cleaned from the production backend; same linters, no copied comments):
```yaml
version: "2"
run:
  timeout: 10m
  tests: false
  modules-download-mode: readonly
linters:
  enable:
    - asasalint
    - asciicheck
    - bidichk
    - bodyclose
    - contextcheck
    - cyclop
    - decorder
    - dogsled
    - dupl
    - dupword
    - durationcheck
    - errcheck
    - errname
    - errorlint
    - forbidigo
    - forcetypeassert
    - funlen
    - gocritic
    - godot
    - godox
    - gomoddirectives
    - gosec
    - govet
    - ineffassign
    - lll
    - misspell
    - nakedret
    - nilerr
    - nilnil
    - noctx
    - prealloc
    - predeclared
    - reassign
    - revive
    - staticcheck
    - tagliatelle
    - unconvert
    - unparam
    - unused
    - usestdlibvars
    - wastedassign
  settings:
    gocritic:
      enabled-tags: [diagnostic, style, performance, experimental, opinionated]
      disabled-checks: [hugeParam, rangeExprCopy, rangeValCopy]
    errcheck:
      check-type-assertions: true
      check-blank: true
    govet:
      enable-all: true
      disable: [fieldalignment]
    nakedret:
      max-func-lines: 1
    tagliatelle:
      case:
        rules:
          json: snake
          toml: snake
    dupl:
      threshold: 75
    lll:
      line-length: 120
      tab-width: 4
    misspell:
      locale: US
    forbidigo:
      forbid:
        - pattern: ^fmt\.Print.*$
        - pattern: ^log\.Print.*$
formatters:
  enable: [gofmt, goimports, gofumpt]
  settings:
    gofmt:
      simplify: true
    goimports:
      local-prefixes: [github.com/enea-scaccabarozzi/ness]
```

`.golangci.tests.yml`:
```yaml
version: "2"
# Test-code linters (test-discipline): no wall-clock sleep, helpers marked, modern testing APIs.
run:
  timeout: 10m
  tests: true
linters:
  default: none
  enable: [forbidigo, thelper, tparallel, usetesting]
  settings:
    forbidigo:
      analyze-types: true
      forbid:
        - pattern: ^time\.Sleep$
          msg: no wall-clock sleep in tests; use testing/synctest or pass now as a value
formatters:
  enable: []
```

`Makefile`:
```make
FUZZTIME ?= 30s

.PHONY: validate lint test vuln selfcheck fuzz

validate: lint test vuln selfcheck

lint:
	golangci-lint run ./...
	golangci-lint run -c .golangci.tests.yml ./...

test:
	go test -race -count=1 ./...
	go test -count=4 ./...

vuln:
	go tool govulncheck ./...

selfcheck:
	@echo "selfcheck: enabled in Task 15"

fuzz:
	@for pkg in $$(go list ./...); do \
	  for f in $$(go test -list '^Fuzz' $$pkg | grep '^Fuzz'); do \
	    go test -run '^$$' -fuzz "^$$f$$" -fuzztime $(FUZZTIME) $$pkg || exit 1; \
	  done; \
	done
```

`.github/workflows/ci.yml`:
```yaml
name: ci
on:
  push:
  pull_request:
  schedule:
    - cron: "17 3 * * *"
jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: jdx/mise-action@v2
      - run: make validate
  fuzz:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: jdx/mise-action@v2
      - run: make fuzz FUZZTIME=${{ github.event_name == 'schedule' && '10m' || '30s' }}
```

- [ ] **Step 6: Run the tests and the gate**

Run: `go test ./... && make lint vuln`
Expected: PASS. Both tests pass, and the linters and govulncheck report nothing. Fix any lint finding in the code (not by disabling a linter).

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum mise.toml Makefile .golangci.yml .golangci.tests.yml .github cmd internal
git commit -m "build: module skeleton, script harness, go-strict gate and CI"
```

---

### Task 2: Requirement coverage check

**Files:**
- Create: `internal/testkit/coverage.go`, `internal/testkit/coverage_test.go`, `docs/spec/reviews.md`

**Interfaces:**
- Produces: `testkit.ScanCoverage(specDir, root string) (*Coverage, error)` and `(*Coverage).Problems(areas []string, deferred map[string]string) []string`. Task 15 calls both from `TestRequirementCoverage`.

- [ ] **Step 1: Write the failing test**

`internal/testkit/coverage_test.go`:
```go
package testkit_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) (specDir, root string) {
	t.Helper()
	root = t.TempDir()
	specDir = filepath.Join(root, "docs", "spec")
	write(t, filepath.Join(specDir, "a.md"), "- **R-AAA-001** x MUST y.\n- **R-AAA-002** x MUST z.\n"+
		"- **R-AAA-003** x MUST w.\n- **R-BBB-001** other area.\n")
	write(t, filepath.Join(specDir, "reviews.md"),
		"- R-AAA-003 | s-123 | 2026-10-09 | abc1234\n- R-AAA-002 | s-123 | 2026-10-09\n")
	// The markers are split so the repository's own scan does not read them as real markers.
	marker := "cov" + "ers:"
	write(t, filepath.Join(root, "pkg", "x_test.go"), "package x\n// "+marker+" R-AAA-001, R-ZZZ-999\n")
	write(t, filepath.Join(root, "pkg", "testdata", "script", "s.txtar"), "# "+marker+" R-AAA-001\n")
	return specDir, root
}

func TestCoverageProblems(t *testing.T) {
	// covers: R-QA-010, R-QA-011, R-QA-012, R-QA-013
	specDir, root := fixture(t)
	c, err := testkit.ScanCoverage(specDir, root)
	if err != nil {
		t.Fatal(err)
	}
	got := c.Problems([]string{"AAA"}, nil)
	want := []string{
		"R-AAA-002 has no covering test or review entry",
		"R-ZZZ-999 is named by a covers marker or review entry but not defined in docs/spec",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("problems:\n%q\nwant:\n%q", got, want)
	}
}

func TestCoverageDeferred(t *testing.T) {
	// covers: R-QA-010
	specDir, root := fixture(t)
	c, err := testkit.ScanCoverage(specDir, root)
	if err != nil {
		t.Fatal(err)
	}
	got := c.Problems([]string{"AAA"}, map[string]string{"R-AAA-002": "P2"})
	if len(got) != 1 {
		t.Fatalf("problems: %q", got)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/testkit/`
Expected: FAIL to build (`undefined: testkit.ScanCoverage`).

- [ ] **Step 3: Implement**

`internal/testkit/coverage.go`:
```go
package testkit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	defRe    = regexp.MustCompile(`(?m)^- \*\*(R-[A-Z]+-\d{3})\*\*`)
	coverRe  = regexp.MustCompile(`(?m)(?://|#)\s*covers:\s*(.+)$`)
	idRe     = regexp.MustCompile(`R-[A-Z]+-\d{3}`)
	reviewRe = regexp.MustCompile(`(?m)^- (R-[A-Z]+-\d{3}) \| ([^|\n]+) \| (\d{4}-\d{2}-\d{2}) \| ([0-9a-f]{7,40})\s*$`)
)

// Coverage maps requirement ids defined in the spec to what covers them.
type Coverage struct {
	defined map[string]bool
	covered map[string]bool
	named   []string // every id a marker or review entry names
}

// ScanCoverage reads definitions from specDir/*.md, review entries from specDir/reviews.md, and
// covers markers from every *_test.go and *.txtar under root (R-QA-010, R-QA-011, R-QA-012).
func ScanCoverage(specDir, root string) (*Coverage, error) {
	c := &Coverage{defined: map[string]bool{}, covered: map[string]bool{}}
	specs, err := filepath.Glob(filepath.Join(specDir, "*.md"))
	if err != nil {
		return nil, err
	}
	for _, p := range specs {
		b, err := os.ReadFile(p) //nolint:gosec // paths come from a glob of the spec dir
		if err != nil {
			return nil, err
		}
		for _, m := range defRe.FindAllStringSubmatch(string(b), -1) {
			c.defined[m[1]] = true
		}
		for _, m := range reviewRe.FindAllStringSubmatch(string(b), -1) {
			c.mark(m[1])
		}
	}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !(strings.HasSuffix(p, "_test.go") || strings.HasSuffix(p, ".txtar")) {
			return err
		}
		b, err := os.ReadFile(p) //nolint:gosec // walking the repository
		if err != nil {
			return err
		}
		for _, m := range coverRe.FindAllStringSubmatch(string(b), -1) {
			for _, id := range idRe.FindAllString(m[1], -1) {
				c.mark(id)
			}
		}
		return nil
	})
	return c, err
}

func (c *Coverage) mark(id string) {
	c.covered[id] = true
	c.named = append(c.named, id)
}

// Problems lists defined ids of the given areas with no cover (unless deferred) and ids named
// anywhere but not defined (R-QA-013), sorted.
func (c *Coverage) Problems(areas []string, deferred map[string]string) []string {
	var out []string
	for id := range c.defined {
		area := strings.Split(id, "-")[1]
		if slices.Contains(areas, area) && !c.covered[id] && deferred[id] == "" {
			out = append(out, id+" has no covering test or review entry")
		}
	}
	for _, id := range c.named {
		if !c.defined[id] {
			out = append(out, id+" is named by a covers marker or review entry but not defined in docs/spec")
		}
	}
	for id, plan := range deferred {
		if c.covered[id] {
			out = append(out, fmt.Sprintf("%s is deferred to %s but is covered; remove it from deferred", id, plan))
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
```

`docs/spec/reviews.md`:
```markdown
# Review ledger

A `review` requirement counts as covered only by an entry here (R-QA-012), one per line:

`- <id> | <reviewer session> | <YYYY-MM-DD> | <reviewed commit>`

Entries are appended by the reviewer at the end of each plan.
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/testkit/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/testkit docs/spec/reviews.md
git commit -m "test: requirement coverage scan over spec, markers and review ledger"
```

### Task 3: Configuration schema, defaults and validation

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces:
  - `config.Path = ".ness/ness.toml"`
  - `config.Parse(data []byte) (*config.File, error)`
  - `(*File).Pin() string`
  - `(*File).Defined(dotted string) bool`
  - `(*File).Config(defaults map[string]any) (*config.Config, error)`
  - `config.ValidKey(dotted string, value any) error`
  - `config.DefaultTiers() map[string]config.Tier`
  - `config.SplitTier(spec string) (model, effort string)`
  - Types: `Config` and its sub-structs (fields below), `Tier{Claude, Codex string}`, `Duration{time.Duration}`.

- [ ] **Step 1: Write the failing tests**

`internal/config/config_test.go`:
```go
package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

const minimal = `ness = "0.1"
project = "p"
base = "main"
gate = "make validate"
`

func load(t *testing.T, body string, defaults map[string]any) (*config.Config, error) {
	t.Helper()
	f, err := config.Parse([]byte(body))
	if err != nil {
		return nil, err
	}
	return f.Config(defaults)
}

func mustFail(t *testing.T, body string, wants ...string) {
	t.Helper()
	_, err := load(t, body, nil)
	if err == nil {
		t.Fatalf("no error for:\n%s", body)
	}
	for _, w := range wants {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not contain %q", err, w)
		}
	}
}

func TestDefaults(t *testing.T) {
	// covers: R-CFG-042, R-MOD-001, R-MOD-003
	c, err := load(t, minimal, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Sessions != "docs/sessions" || c.Client != "claude" || c.Integrate.Mode != "merge" || c.Integrate.Push {
		t.Errorf("defaults: %+v", c)
	}
	if c.Models.Policy != "text" || c.Models.RouterThreshold != 0.8 || c.Launch.ReadyTimeout.Duration != time.Minute {
		t.Errorf("model/launch defaults: %+v %+v", c.Models, c.Launch)
	}
	if c.Worker.StallMinutes != 30 || c.Worker.QuietSeconds != 60 {
		t.Errorf("worker defaults: %+v", c.Worker)
	}
	want := map[string]config.Tier{
		"standard": {Claude: "opus/medium", Codex: "gpt-6.1-sol/medium"},
		"frontier": {Claude: "fable/medium", Codex: "astra/medium"},
		"economy":  {Claude: "sonnet/medium", Codex: "gpt-6-luna/low"},
	}
	for name, tier := range want {
		if c.Tiers[name] != tier {
			t.Errorf("tier %s = %+v, want %+v", name, c.Tiers[name], tier)
		}
	}
}

func TestTierRowEditKeepsOtherClient(t *testing.T) {
	// covers: R-MOD-003
	c, err := load(t, minimal+"[tiers.economy]\nclaude = \"haiku/low\"\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Tiers["economy"] != (config.Tier{Claude: "haiku/low", Codex: "gpt-6-luna/low"}) {
		t.Fatalf("economy = %+v", c.Tiers["economy"])
	}
}

func TestSyntaxErrorNamesLineAndColumn(t *testing.T) {
	// covers: R-CFG-003
	mustFail(t, minimal+"x = ]\n", ".ness/ness.toml:5:")
}

func TestUnknownKeyNamed(t *testing.T) {
	// covers: R-CFG-043, R-CFG-046
	mustFail(t, minimal+"[worker]\nstal_minutes = 5\n", "worker.stal_minutes")
	mustFail(t, minimal+"[notify.channels.r]\nkind = \"slack\"\ntoken = \"xoxb\"\nchannel_id = \"C1\"\n",
		"notify.channels.r.token")
}

func TestRequiredKeys(t *testing.T) {
	// covers: R-CFG-040
	for _, key := range []string{"ness", "project", "base", "gate"} {
		var kept []string
		for _, line := range strings.Split(strings.TrimSpace(minimal), "\n") {
			if !strings.HasPrefix(line, key+" ") {
				kept = append(kept, line)
			}
		}
		mustFail(t, strings.Join(kept, "\n")+"\n", key+" is required")
	}
}

func TestEnumValues(t *testing.T) {
	// covers: R-CFG-004
	mustFail(t, minimal+"client = \"gpt\"\n", "client", "claude", "codex")
	mustFail(t, minimal+"[integrate]\nmode = \"rebase\"\n", "integrate.mode", "merge, squash, pr")
	mustFail(t, minimal+"features = [\"x\"]\n", "features", "probes")
	mustFail(t, minimal+"[tiers.standard]\nclaude = \"opus\"\n", "tiers.standard.claude")
	mustFail(t, minimal+"[tiers.standard]\nclaude = \"opus/huge\"\n", "tiers.standard.claude", "medium")
}

func TestCodexNeedsExperimental(t *testing.T) {
	// covers: R-CFG-044
	mustFail(t, minimal+"client = \"codex\"\n", "experimental")
	if _, err := load(t, minimal+"client = \"codex\"\nexperimental = [\"codex\"]\n", nil); err != nil {
		t.Fatal(err)
	}
}

func TestChannels(t *testing.T) {
	// covers: R-CFG-045
	mustFail(t, minimal+"[notify.channels.r]\nkind = \"discord\"\n", "notify.channels.r.kind")
	mustFail(t, minimal+"[notify.channels.r]\nkind = \"slack\"\ntoken_env = \"T\"\n", "notify.channels.r.channel_id")
	ok := minimal + "[notify.channels.r]\nkind = \"command\"\ncommand = \"cat\"\n"
	if _, err := load(t, ok, nil); err != nil {
		t.Fatal(err)
	}
}

func TestProfileDefaultsLoseToFile(t *testing.T) {
	// covers: R-PROF-008
	c, err := load(t, minimal+"[integrate]\npush = false\n",
		map[string]any{"integrate.push": true, "integrate.mode": "squash"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Integrate.Push || c.Integrate.Mode != "squash" {
		t.Fatalf("integrate = %+v", c.Integrate)
	}
	if err := config.ValidKey("integrate.pushh", true); err == nil {
		t.Fatal("unknown key accepted")
	}
	if err := config.ValidKey("worker.stall_minutes", "ten"); err == nil {
		t.Fatal("wrong type accepted")
	}
}

func TestReadsNoUserConfig(t *testing.T) {
	// covers: R-CFG-001
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	a, errA := load(t, minimal, nil)
	b, errB := load(t, minimal, nil)
	if errA != nil || errB != nil || a.Client != b.Client {
		t.Fatal("config depends on the environment")
	}
}

func FuzzParse(f *testing.F) {
	// covers: R-CFG-005
	f.Add([]byte(minimal))
	f.Add([]byte(minimal + "[tiers.standard]\nclaude = \"opus/medium\"\n[notify.channels.a]\nkind = \"slack\"\n"))
	f.Add([]byte("ness = \n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		file, err := config.Parse(data)
		if err != nil {
			return
		}
		_, _ = file.Config(map[string]any{"integrate.push": true})
	})
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/config/`
Expected: FAIL to build (package has no non-test files).

- [ ] **Step 3: Implement**

`internal/config/config.go`:
```go
// Package config loads and validates .ness/ness.toml, the only place ness reads project
// settings from (R-CFG-001).
package config

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Path is the configuration file, relative to the repository root.
const Path = ".ness/ness.toml"

// Config is the validated content of .ness/ness.toml with defaults applied.
type Config struct {
	Ness         string          `toml:"ness"`
	Project      string          `toml:"project"`
	Base         string          `toml:"base"`
	Gate         string          `toml:"gate"`
	Sessions     string          `toml:"sessions"`
	Client       string          `toml:"client"`
	Profiles     []string        `toml:"profiles"`
	Features     []string        `toml:"features"`
	Experimental []string        `toml:"experimental"`
	Worktree     Worktree        `toml:"worktree"`
	Integrate    Integrate       `toml:"integrate"`
	Contracts    Contracts       `toml:"contracts"`
	Probes       Probes          `toml:"probes"`
	Notify       Notify          `toml:"notify"`
	Tiers        map[string]Tier `toml:"tiers"`
	Models       Models          `toml:"models"`
	Launch       Launch          `toml:"launch"`
	Worker       Worker          `toml:"worker"`
}

// Worktree lists files copied or linked from the primary checkout into each session worktree.
type Worktree struct {
	Copy []string `toml:"copy"`
	Link []string `toml:"link"`
}

// Integrate configures how ness integrate folds a branch.
type Integrate struct {
	Mode string `toml:"mode"`
	Push bool   `toml:"push"`
}

// Contracts lists project-wide frozen paths.
type Contracts struct {
	Frozen []string `toml:"frozen"`
}

// Probes configures the probes feature.
type Probes struct {
	RedactEnv []string `toml:"redact_env"`
}

// Notify holds the named notification channels.
type Notify struct {
	Channels map[string]Channel `toml:"channels"`
}

// Channel is one notification channel; which keys are required depends on Kind.
type Channel struct {
	Kind      string `toml:"kind"`
	TokenEnv  string `toml:"token_env"`
	ChannelID string `toml:"channel_id"`
	ChatID    string `toml:"chat_id"`
	Repo      string `toml:"repo"`
	Command   string `toml:"command"`
}

// Tier is one row of the tier table: "<model>/<effort>" per client.
type Tier struct {
	Claude string `toml:"claude"`
	Codex  string `toml:"codex"`
}

// Models configures the model policy.
type Models struct {
	Policy          string  `toml:"policy"`
	RouterThreshold float64 `toml:"router_threshold"`
	FallbackModel   string  `toml:"fallback_model"`
}

// Launch configures session launches.
type Launch struct {
	ReadyTimeout Duration `toml:"ready_timeout"`
}

// Worker configures the worker.
type Worker struct {
	StallMinutes int `toml:"stall_minutes"`
	QuietSeconds int `toml:"quiet_seconds"`
}

// Duration is a time.Duration written as a Go duration string such as "60s".
type Duration struct{ time.Duration }

// UnmarshalText parses a Go duration string.
func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	d.Duration = v
	return nil
}

// DefaultTiers is the tier table for every row or client the file leaves out (R-MOD-001).
func DefaultTiers() map[string]Tier {
	return map[string]Tier{
		"standard": {Claude: "opus/medium", Codex: "gpt-6.1-sol/medium"},
		"frontier": {Claude: "fable/medium", Codex: "astra/medium"},
		"economy":  {Claude: "sonnet/medium", Codex: "gpt-6-luna/low"},
	}
}

// SplitTier splits "<model>/<effort>".
func SplitTier(spec string) (model, effort string) {
	model, effort, _ = strings.Cut(spec, "/")
	return model, effort
}

func defaults() Config {
	return Config{
		Sessions:  "docs/sessions",
		Client:    "claude",
		Integrate: Integrate{Mode: "merge"},
		Models:    Models{Policy: "text", RouterThreshold: 0.8},
		Launch:    Launch{ReadyTimeout: Duration{time.Minute}},
		Worker:    Worker{StallMinutes: 30, QuietSeconds: 60},
	}
}

// File is a parsed .ness/ness.toml: its text and which keys it sets.
type File struct {
	data string
	meta toml.MetaData
	raw  map[string]any
}

// Parse checks syntax (errors carry line and column, R-CFG-003) and validates the file with no
// profile defaults, so a broken file is refused before anything else runs.
func Parse(data []byte) (*File, error) {
	f := &File{data: string(data)}
	meta, err := toml.Decode(f.data, &f.raw)
	if err != nil {
		return nil, posError(err)
	}
	f.meta = meta
	if _, err := f.Config(nil); err != nil {
		return nil, err
	}
	return f, nil
}

// Pin returns the version pin, the value of the ness key.
func (f *File) Pin() string {
	s, _ := f.raw["ness"].(string)
	return s
}

// Defined reports whether the file itself sets the dotted key, e.g. "integrate.mode".
func (f *File) Defined(key string) bool { return f.meta.IsDefined(strings.Split(key, ".")...) }

// Config decodes the file over the built-in defaults and the given profile defaults (dotted key
// to value; the file's own values win, R-PROF-008), then validates the result.
func (f *File) Config(profileDefaults map[string]any) (*Config, error) {
	c := defaults()
	extra := map[string]any{}
	for _, k := range slices.Sorted(maps.Keys(profileDefaults)) {
		if !f.Defined(k) {
			if err := setDotted(extra, k, profileDefaults[k]); err != nil {
				return nil, err
			}
		}
	}
	if err := decodeMap(extra, &c); err != nil {
		return nil, fmt.Errorf("%s: profile default: %w", Path, err)
	}
	meta, err := toml.Decode(f.data, &c)
	if err != nil {
		return nil, posError(err)
	}
	if und := meta.Undecoded(); len(und) > 0 {
		return nil, fmt.Errorf("%s: unknown key %s", Path, und[0])
	}
	fillTiers(&c)
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", Path, err)
	}
	return &c, nil
}

// ValidKey reports whether a profile may set the dotted key to value (R-PROF-008).
func ValidKey(key string, value any) error {
	m := map[string]any{}
	if err := setDotted(m, key, value); err != nil {
		return err
	}
	c := defaults()
	return decodeMap(m, &c)
}

func decodeMap(m map[string]any, c *Config) error {
	if len(m) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return err
	}
	meta, err := toml.Decode(buf.String(), c)
	if err != nil {
		return err
	}
	if und := meta.Undecoded(); len(und) > 0 {
		return fmt.Errorf("unknown key %s", und[0])
	}
	return nil
}

func setDotted(m map[string]any, key string, v any) error {
	parts := strings.Split(key, ".")
	for _, p := range parts[:len(parts)-1] {
		next, ok := m[p]
		if !ok {
			next = map[string]any{}
			m[p] = next
		}
		sub, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("key %s: %s is not a table", key, p)
		}
		m = sub
	}
	m[parts[len(parts)-1]] = v
	return nil
}

func posError(err error) error {
	var pe toml.ParseError
	if errors.As(err, &pe) {
		return fmt.Errorf("%s:%d:%d: %s", Path, pe.Position.Line, pe.Position.Col, pe.Message)
	}
	return fmt.Errorf("%s: %w", Path, err)
}

func fillTiers(c *Config) {
	if c.Tiers == nil {
		c.Tiers = map[string]Tier{}
	}
	for name, d := range DefaultTiers() {
		t := c.Tiers[name]
		if t.Claude == "" {
			t.Claude = d.Claude
		}
		if t.Codex == "" {
			t.Codex = d.Codex
		}
		c.Tiers[name] = t
	}
}
```

Add the validation in the same file:
```go
var (
	claudeEfforts = []string{"low", "medium", "high", "xhigh", "max"}
	codexEfforts  = []string{"minimal", "low", "medium", "high", "xhigh"}
	channelKeys   = map[string][]string{
		"slack":    {"token_env", "channel_id"},
		"telegram": {"token_env", "chat_id"},
		"github":   {"token_env", "repo"},
		"command":  {"command"},
	}
)

func oneOf(key, v string, allowed ...string) error {
	if slices.Contains(allowed, v) {
		return nil
	}
	return fmt.Errorf("%s = %q: accepted values are %s", key, v, strings.Join(allowed, ", "))
}

func (c *Config) validate() error {
	var errs []error
	for _, r := range [][2]string{{"ness", c.Ness}, {"project", c.Project}, {"base", c.Base}, {"gate", c.Gate}} {
		if r[1] == "" {
			errs = append(errs, fmt.Errorf("%s is required", r[0]))
		}
	}
	errs = append(errs,
		oneOf("client", c.Client, "claude", "codex"),
		oneOf("integrate.mode", c.Integrate.Mode, "merge", "squash", "pr"),
		oneOf("models.policy", c.Models.Policy, "text"))
	if c.Client == "codex" && !slices.Contains(c.Experimental, "codex") {
		errs = append(errs, errors.New(`client = "codex" needs experimental = ["codex"]`))
	}
	for _, f := range c.Features {
		errs = append(errs, oneOf("features", f, "probes", "notify", "contracts"))
	}
	for _, e := range c.Experimental {
		errs = append(errs, oneOf("experimental", e, "codex"))
	}
	for _, name := range slices.Sorted(maps.Keys(c.Tiers)) {
		t := c.Tiers[name]
		errs = append(errs,
			tierSpec("tiers."+name+".claude", t.Claude, claudeEfforts),
			tierSpec("tiers."+name+".codex", t.Codex, codexEfforts))
	}
	for _, name := range slices.Sorted(maps.Keys(c.Notify.Channels)) {
		errs = append(errs, c.Notify.Channels[name].validate("notify.channels."+name))
	}
	errs = append(errs, c.validateNumbers())
	return errors.Join(errs...)
}

func (c *Config) validateNumbers() error {
	var errs []error
	if c.Models.RouterThreshold < 0 || c.Models.RouterThreshold > 1 {
		errs = append(errs, errors.New("models.router_threshold must be between 0 and 1"))
	}
	if c.Launch.ReadyTimeout.Duration <= 0 {
		errs = append(errs, errors.New("launch.ready_timeout must be positive"))
	}
	if c.Worker.StallMinutes <= 0 || c.Worker.QuietSeconds < 0 {
		errs = append(errs, errors.New("worker.stall_minutes must be positive and worker.quiet_seconds not negative"))
	}
	return errors.Join(errs...)
}

func tierSpec(key, spec string, efforts []string) error {
	model, effort, ok := strings.Cut(spec, "/")
	if !ok || model == "" {
		return fmt.Errorf("%s = %q: want \"<model>/<effort>\"", key, spec)
	}
	return oneOf(key+" effort", effort, efforts...)
}

func (ch Channel) validate(key string) error {
	need, ok := channelKeys[ch.Kind]
	if !ok {
		return oneOf(key+".kind", ch.Kind, slices.Sorted(maps.Keys(channelKeys))...)
	}
	have := map[string]string{
		"token_env": ch.TokenEnv, "channel_id": ch.ChannelID, "chat_id": ch.ChatID,
		"repo": ch.Repo, "command": ch.Command,
	}
	var errs []error
	for _, k := range need {
		if have[k] == "" {
			errs = append(errs, fmt.Errorf("%s.%s is required for kind %s", key, k, ch.Kind))
		}
	}
	return errors.Join(errs...)
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/config/ && go test -run '^$' -fuzz FuzzParse -fuzztime 20s ./internal/config/`
Expected: PASS, and the fuzzer finds no crash.

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat(config): ness.toml schema, defaults, validation and profile defaults"
```

---

### Task 4: Version pin

**Files:**
- Create: `internal/config/pin.go`, `internal/config/pin_test.go`

**Interfaces:**
- Produces: `config.CheckPin(pin, version string) error` and `config.PinOf(version string) string` (major.minor, used by init).

- [ ] **Step 1: Write the failing test**

`internal/config/pin_test.go`:
```go
package config_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

func TestCheckPin(t *testing.T) {
	// covers: R-CFG-011, R-CFG-012, R-CFG-013
	cases := []struct {
		pin, version string
		ok           bool
	}{
		{"0.3", "0.3.0", true}, {"0.3", "0.3.7", true}, {"0.3", "0.2.9", false}, {"0.3", "0.4.0", false},
		{"1.2", "1.2.0", true}, {"1.2", "1.5.1", true}, {"1.2", "1.1.9", false}, {"1.2", "2.0.0", false},
		{"0.1", "0.1.0-dev", true}, {"x", "0.1.0", false}, {"0.1", "garbage", false},
	}
	for _, c := range cases {
		err := config.CheckPin(c.pin, c.version)
		if (err == nil) != c.ok {
			t.Errorf("CheckPin(%q, %q) = %v, want ok=%v", c.pin, c.version, err, c.ok)
		}
		if err != nil && c.pin != "x" && c.version != "garbage" &&
			(!strings.Contains(err.Error(), c.pin) || !strings.Contains(err.Error(), c.version)) {
			t.Errorf("refusal %q must name the pin and the binary version", err)
		}
	}
}

func TestPinOf(t *testing.T) {
	// covers: R-CFG-010
	if got := config.PinOf("0.3.7-rc1"); got != "0.3" {
		t.Fatalf("PinOf = %q", got)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/config/ -run 'Pin'`
Expected: FAIL to build (`undefined: config.CheckPin`).

- [ ] **Step 3: Implement**

`internal/config/pin.go`:
```go
package config

import (
	"fmt"
	"strconv"
	"strings"
)

// PinOf returns the "<major>.<minor>" pin ness init writes for a binary version (R-CFG-010).
func PinOf(version string) string {
	maj, minor, err := majorMinor(version)
	if err != nil {
		return version
	}
	return fmt.Sprintf("%d.%d", maj, minor)
}

// CheckPin refuses a binary outside the project's pin: for 0.N exactly that minor; for M.N the
// same major and a minor of at least N (R-CFG-011, R-CFG-012).
func CheckPin(pin, version string) error {
	pm, pn, err := majorMinor(pin)
	if err != nil || strings.Count(pin, ".") != 1 {
		return fmt.Errorf(`version pin ness = %q in %s: want "<major>.<minor>"`, pin, Path)
	}
	vm, vn, err := majorMinor(version)
	if err != nil {
		return fmt.Errorf("binary version %q is not a release version", version)
	}
	if vm == pm && (vn == pn || (pm >= 1 && vn > pn)) {
		return nil
	}
	want := fmt.Sprintf("%d.%d.x", pm, pn)
	if pm >= 1 {
		want = fmt.Sprintf("%d.%d or a later %d.x", pm, pn, pm)
	}
	return fmt.Errorf("this project pins ness %s (ness = %q); the running binary is %s", want, pin, version)
}

func majorMinor(v string) (major, minor int, err error) {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, fmt.Errorf("version %q", v)
	}
	if major, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, err
	}
	if minor, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat(config): version pin check"
```

---

### Task 5: Project loading in the CLI

**Files:**
- Create: `internal/cli/project.go`, `internal/cli/testdata/script/config_errors.txtar`, `internal/cli/testdata/script/pin.txtar`
- Modify: `internal/cli/cli.go` (register a hidden `check-config` command only until Task 13 replaces it with `sync`. Delete it in Task 13.)

**Interfaces:**
- Consumes: `config.Parse`, `config.CheckPin`.
- Produces: `type project struct { root string; file *config.File }`, `repoRoot(dir string) (string, error)`, `loadProject(d Deps) (*project, error)`.

- [ ] **Step 1: Write the failing scripts**

`internal/cli/testdata/script/config_errors.txtar`:
```
# A broken or invalid ness.toml is refused (exit 4), naming the position or key, and nothing is written.
# covers: R-CFG-003, R-CFG-004, R-CFG-040, R-CFG-043
cd repo
snap before
exits 4 ness check-config
stderr '\.ness/ness\.toml:5:\d+:'
samesnap before

cp ../bad-enum.toml .ness/ness.toml
exits 4 ness check-config
stderr 'client = "gpt": accepted values are claude, codex'

cp ../unknown.toml .ness/ness.toml
exits 4 ness check-config
stderr 'unknown key worker\.stal_minutes'

cp ../no-gate.toml .ness/ness.toml
exits 4 ness check-config
stderr 'gate is required'

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "make validate"
x = ]
-- bad-enum.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "make validate"
client = "gpt"
-- unknown.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "make validate"
[worker]
stal_minutes = 5
-- no-gate.toml --
ness = "0.1"
project = "p"
base = "main"
```

`internal/cli/testdata/script/pin.txtar`:
```
# The binary refuses to run outside the project's pin, naming the range, its version and its path.
# covers: R-CFG-011, R-CFG-012, R-CFG-013
cd repo
env NESS_TEST_VERSION=0.3.0
exec ness check-config
env NESS_TEST_VERSION=0.3.7
exec ness check-config
env NESS_TEST_VERSION=0.2.9
exits 4 ness check-config
stderr 'pins ness 0\.3\.x'
stderr '0\.2\.9'
stderr 'binary: /'
env NESS_TEST_VERSION=0.4.0
exits 4 ness check-config

cp ../one.toml .ness/ness.toml
env NESS_TEST_VERSION=1.5.1
exec ness check-config
env NESS_TEST_VERSION=2.0.0
exits 4 ness check-config
env NESS_TEST_VERSION=1.1.9
exits 4 ness check-config

-- repo/.ness/ness.toml --
ness = "0.3"
project = "p"
base = "main"
gate = "true"
-- one.toml --
ness = "1.2"
project = "p"
base = "main"
gate = "true"
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/`
Expected: FAIL. `ness check-config` is an unknown command (exit 2, want 4).

- [ ] **Step 3: Implement**

`internal/cli/project.go`:
```go
package cli

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// project is a loaded ness project: the repository root and its parsed configuration.
type project struct {
	root string
	file *config.File
}

// repoRoot returns the top of the git checkout containing dir (a worktree is its own top).
func repoRoot(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", refuse("%s is not inside a git repository; run ness from a project checkout", dir)
	}
	return strings.TrimSpace(string(out)), nil
}

// loadProject finds the repository, parses .ness/ness.toml and checks the version pin. Every
// failure is a refusal.
func loadProject(d Deps) (*project, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := repoRoot(wd)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, config.Path)) //nolint:gosec // fixed path under the repo
	if errors.Is(err, fs.ErrNotExist) {
		return nil, refuse("no %s in %s; run ness init first", config.Path, root)
	}
	if err != nil {
		return nil, err
	}
	f, err := config.Parse(data)
	if err != nil {
		return nil, refuse("%v", err)
	}
	if err := config.CheckPin(f.Pin(), d.Version); err != nil {
		return nil, refuse("%v (binary: %s)", err, executable())
	}
	return &project{root: root, file: f}, nil
}

func executable() string {
	p, err := os.Executable()
	if err != nil {
		return "unknown path"
	}
	return p
}
```

In `cli.go` `newRoot`, before `return root`, add the temporary command:
```go
	root.AddCommand(&cobra.Command{
		Use:    "check-config",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			_, err := loadProject(d)
			return err
		},
	})
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/cli/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): load the project, refuse broken config and foreign pins"
```

### Task 6: Managed blocks

**Files:**
- Create: `internal/sync/blocks.go`, `internal/sync/blocks_test.go`

**Interfaces:**
- Produces (package-internal, used by the engine in Task 12):
  - `type Style`, `markdownStyle`, `hashStyle`, `styleFor(path string) Style`
  - `getBlock(data []byte, s Style, name string) ([]byte, bool, error)`
  - `setBlock(data []byte, s Style, name string, inner []byte) ([]byte, error)`
  - `removeBlock(data []byte, s Style, name string) ([]byte, error)`
  - `parseBlocks(data []byte, s Style) ([]block, error)`

- [ ] **Step 1: Write the failing tests**

`internal/sync/blocks_test.go`:
```go
package sync

import (
	"bytes"
	"strings"
	"testing"
)

func TestBlockMarkers(t *testing.T) {
	// covers: R-SYNC-040, R-SYNC-042
	out, err := setBlock([]byte("# Title\n"), markdownStyle, "core/router", []byte("hello\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := "# Title\n\n<!-- ness:begin core/router -->\nhello\n<!-- ness:end core/router -->\n"
	if string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
	out, err = setBlock(nil, styleFor(".gitignore"), "core/ignore", []byte(".worktrees/\n*.ness-new"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "# ness:begin core/ignore\n.worktrees/\n*.ness-new\n# ness:end core/ignore\n" {
		t.Fatalf("gitignore block %q", out)
	}
}

func TestBlockMovedIsUpdatedInPlace(t *testing.T) {
	// covers: R-SYNC-041, R-SYNC-043
	in := "<!-- ness:begin core/router -->\nold\n<!-- ness:end core/router -->\n# Mine\ntext\n"
	got, ok, err := getBlock([]byte(in), markdownStyle, "core/router")
	if err != nil || !ok || string(got) != "old\n" {
		t.Fatalf("getBlock = %q %v %v", got, ok, err)
	}
	out, err := setBlock([]byte(in), markdownStyle, "core/router", []byte("new\n"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != strings.Replace(in, "old", "new", 1) {
		t.Fatalf("got %q", out)
	}
}

func TestRemoveBlock(t *testing.T) {
	in := "a\n<!-- ness:begin x/y -->\nz\n<!-- ness:end x/y -->\nb\n"
	out, err := removeBlock([]byte(in), markdownStyle, "x/y")
	if err != nil || string(out) != "a\nb\n" {
		t.Fatalf("removeBlock = %q %v", out, err)
	}
	added, err := setBlock([]byte("intro\n"), markdownStyle, "x/y", []byte("z\n"))
	if err != nil {
		t.Fatal(err)
	}
	if out, err := removeBlock(added, markdownStyle, "x/y"); err != nil || string(out) != "intro\n" {
		t.Fatalf("append then remove = %q %v", out, err)
	}
}

func TestBlocksMalformed(t *testing.T) {
	// covers: R-SYNC-046
	cases := map[string]string{
		"no end":     "a\n<!-- ness:begin x/y -->\nb\n",
		"nested":     "<!-- ness:begin x/y -->\n<!-- ness:begin x/z -->\n<!-- ness:end x/z -->\n<!-- ness:end x/y -->\n",
		"duplicate":  "<!-- ness:begin x/y -->\n<!-- ness:end x/y -->\n<!-- ness:begin x/y -->\n<!-- ness:end x/y -->\n",
		"stray end":  "<!-- ness:end x/y -->\n",
		"bad name":   "<!-- ness:begin Router -->\n<!-- ness:end Router -->\n",
		"mismatched": "<!-- ness:begin x/y -->\n<!-- ness:end x/z -->\n",
	}
	for name, in := range cases {
		if _, err := parseBlocks([]byte(in), markdownStyle); err == nil || !strings.Contains(err.Error(), "line ") {
			t.Errorf("%s: err = %v, want an error naming the line", name, err)
		}
	}
}

func TestBlocksCRLF(t *testing.T) {
	in := "a\r\n<!-- ness:begin x/y -->\r\nold\r\n<!-- ness:end x/y -->\r\nb\r\n"
	got, ok, err := getBlock([]byte(in), markdownStyle, "x/y")
	if err != nil || !ok || string(got) != "old\r\n" {
		t.Fatalf("getBlock = %q %v %v", got, ok, err)
	}
}

func FuzzBlocks(f *testing.F) {
	// covers: R-SYNC-044, R-SYNC-047
	f.Add([]byte("intro\n<!-- ness:begin core/router -->\nold\n<!-- ness:end core/router -->\noutro\n"), []byte("new\n"))
	f.Add([]byte("<!-- ness:begin a/b -->\n"), []byte("x"))
	f.Add([]byte("no blocks"), []byte(""))
	f.Fuzz(func(t *testing.T, data, inner []byte) {
		if bytes.Contains(inner, []byte("ness:")) {
			return
		}
		blocks, err := parseBlocks(data, markdownStyle)
		if err != nil {
			return
		}
		out, err := setBlock(data, markdownStyle, "core/router", inner)
		if err != nil {
			t.Fatal(err)
		}
		got, ok, err := getBlock(out, markdownStyle, "core/router")
		want := normInner(inner)
		if err != nil || !ok || !bytes.Equal(got, want) {
			t.Fatalf("round trip: got %q %v %v, want %q", got, ok, err, want)
		}
		for _, b := range blocks {
			if b.name == "core/router" {
				if !bytes.Equal(out[:b.inner], data[:b.inner]) || !bytes.HasSuffix(out, data[b.innerEnd:]) {
					t.Fatal("bytes outside the block changed")
				}
				return
			}
		}
		if !bytes.HasPrefix(out, data) {
			t.Fatal("appending changed existing bytes")
		}
	})
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/sync/`
Expected: FAIL to build (`undefined: setBlock`).

- [ ] **Step 3: Implement**

`internal/sync/blocks.go`:
```go
// Package sync renders core, features and profiles into a repository: whole files, managed
// blocks in shared files, owned JSON keys and symlinks, all recorded in .ness/lock.json. It
// never overwrites or deletes content it did not record (D4).
package sync

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Style is the comment syntax of block markers in one kind of file (R-SYNC-040).
type Style struct {
	begin, end, close string
}

var (
	markdownStyle = Style{begin: "<!-- ness:begin ", end: "<!-- ness:end ", close: " -->"}
	hashStyle     = Style{begin: "# ness:begin ", end: "# ness:end ", close: ""}
	blockNameRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$`)
)

// styleFor picks Markdown markers for .md files and # markers for everything else.
func styleFor(path string) Style {
	if strings.HasSuffix(path, ".md") {
		return markdownStyle
	}
	return hashStyle
}

func (s Style) beginLine(name string) string { return s.begin + name + s.close + "\n" }
func (s Style) endLine(name string) string   { return s.end + name + s.close + "\n" }

// marker classifies one line as "begin", "end" or "" and returns the name it carries.
// ponytail: a marker line inside a fenced code block still counts; quote markers indented in prose.
func (s Style) marker(line string) (kind, name string) {
	t := strings.TrimSpace(line)
	if rest, ok := strings.CutPrefix(t, s.begin); ok {
		name, _ = strings.CutSuffix(rest, s.close)
		return "begin", strings.TrimSpace(name)
	}
	if rest, ok := strings.CutPrefix(t, s.end); ok {
		name, _ = strings.CutSuffix(rest, s.close)
		return "end", strings.TrimSpace(name)
	}
	return "", ""
}

// block is one managed block; offsets are byte positions in the file.
type block struct {
	name                        string
	line                        int
	start, inner, innerEnd, end int
}

// parseBlocks finds every managed block. Malformed markers are an error naming the line.
func parseBlocks(data []byte, s Style) ([]block, error) {
	var (
		out  []block
		open *block
		off  int
	)
	seen := map[string]bool{}
	for i, raw := range bytes.SplitAfter(data, []byte("\n")) {
		kind, name := s.marker(string(raw))
		switch kind {
		case "begin":
			if err := checkBegin(name, open, seen, i+1); err != nil {
				return nil, err
			}
			open = &block{name: name, line: i + 1, start: off, inner: off + len(raw)}
		case "end":
			if open == nil || open.name != name {
				return nil, fmt.Errorf("line %d: end marker for %q without its begin", i+1, name)
			}
			open.innerEnd, open.end = off, off+len(raw)
			out = append(out, *open)
			seen[name] = true
			open = nil
		}
		off += len(raw)
	}
	if open != nil {
		return nil, fmt.Errorf("line %d: block %q has no end marker", open.line, open.name)
	}
	return out, nil
}

func checkBegin(name string, open *block, seen map[string]bool, line int) error {
	switch {
	case !blockNameRe.MatchString(name):
		return fmt.Errorf("line %d: malformed block name %q", line, name)
	case open != nil:
		return fmt.Errorf("line %d: block %q begins inside block %q", line, name, open.name)
	case seen[name]:
		return fmt.Errorf("line %d: duplicate block %q", line, name)
	}
	return nil
}

func findBlock(data []byte, s Style, name string) (block, bool, error) {
	blocks, err := parseBlocks(data, s)
	if err != nil {
		return block{}, false, err
	}
	for _, b := range blocks {
		if b.name == name {
			return b, true, nil
		}
	}
	return block{}, false, nil
}

// normInner makes non-empty block content end with a newline.
func normInner(inner []byte) []byte {
	if len(inner) > 0 && !bytes.HasSuffix(inner, []byte("\n")) {
		return slices.Concat(inner, []byte("\n"))
	}
	return inner
}

// getBlock returns the content between block name's markers and whether the block exists.
func getBlock(data []byte, s Style, name string) ([]byte, bool, error) {
	b, ok, err := findBlock(data, s, name)
	if err != nil || !ok {
		return nil, false, err
	}
	return data[b.inner:b.innerEnd], true, nil
}

// setBlock replaces block name's content in place wherever the block sits (R-SYNC-041), or
// appends the block at the end of the file (R-SYNC-042).
func setBlock(data []byte, s Style, name string, inner []byte) ([]byte, error) {
	b, ok, err := findBlock(data, s, name)
	if err != nil {
		return nil, err
	}
	inner = normInner(inner)
	if ok {
		return slices.Concat(data[:b.inner], inner, data[b.innerEnd:]), nil
	}
	var out bytes.Buffer
	out.Write(data)
	if len(data) > 0 {
		if !bytes.HasSuffix(data, []byte("\n")) {
			out.WriteByte('\n')
		}
		out.WriteByte('\n')
	}
	out.WriteString(s.beginLine(name))
	out.Write(inner)
	out.WriteString(s.endLine(name))
	return out.Bytes(), nil
}

// removeBlock deletes block name with its markers, and the blank line setBlock put before it.
func removeBlock(data []byte, s Style, name string) ([]byte, error) {
	b, ok, err := findBlock(data, s, name)
	if err != nil || !ok {
		return data, err
	}
	head := data[:b.start]
	if bytes.HasSuffix(head, []byte("\n\n")) {
		head = head[:len(head)-1]
	}
	return slices.Concat(head, data[b.end:]), nil
}
```

- [ ] **Step 4: Run the tests and fuzz briefly**

Run: `go test ./internal/sync/ && go test -run '^$' -fuzz FuzzBlocks -fuzztime 30s ./internal/sync/`
Expected: PASS. If the fuzzer finds an input, add it as a seed, fix the code (not the test), and rerun.

- [ ] **Step 5: Commit**

```bash
git add internal/sync
git commit -m "feat(sync): managed blocks found by marker, updated in place"
```

---

### Task 7: Owned JSON keys and hook entries

**Files:**
- Create: `internal/sync/jsonkeys.go`, `internal/sync/jsonkeys_test.go`

**Interfaces:**
- Produces (package-internal):
  - `jsonGet(data []byte, key string) ([]byte, bool, error)`, which returns the compact value
  - `jsonSet(data []byte, key string, value []byte) ([]byte, error)`
  - `jsonDelete(data []byte, key string) ([]byte, error)`
  - `indentJSON(data []byte) []byte`, `compact(raw []byte) []byte`
  - `hookKeyFor(event, match string) string`
- Key forms: a plain gjson path (`autoCompactWindow`, `env.CLAUDE_CODE_SUBAGENT_MODEL`), or a hook entry `hooks.<Event>[<match>]`. A hook entry is the element of `hooks.<Event>` whose `hooks[].command` contains `<match>`.

- [ ] **Step 1: Write the failing tests**

`internal/sync/jsonkeys_test.go`:
```go
package sync

import (
	"bytes"
	"testing"

	"github.com/tidwall/gjson"
)

const nessHook = `{"hooks":[{"type":"command","command":"command -v ness >/dev/null && ness hook session-start || true"}]}`

func TestJSONKeepsUnownedBytes(t *testing.T) {
	// covers: R-SYNC-050, R-SYNC-053
	user := `{"hooks": [{"type": "command", "command": "echo mine"}]}`
	in := []byte("{\n    \"model\":   \"x\",\n  \"hooks\": {\"SessionStart\": [" + user + "]}\n}\n")
	key := hookKeyFor("SessionStart", "ness hook session-start")
	out := in
	for range 2 {
		var err error
		if out, err = jsonSet(out, "autoCompactWindow", []byte("265000")); err != nil {
			t.Fatal(err)
		}
		if out, err = jsonSet(out, key, []byte(nessHook)); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.HasPrefix(out, []byte("{\n    \"model\":   \"x\",\n  \"hooks\": {\"SessionStart\": ["+user)) {
		t.Fatalf("unowned bytes changed:\n%s", out)
	}
	if n := gjson.GetBytes(out, "hooks.SessionStart.#").Int(); n != 2 {
		t.Fatalf("%d SessionStart entries, want the user's and one ness entry:\n%s", n, out)
	}
	got, ok, err := jsonGet(out, key)
	if err != nil || !ok || string(got) != nessHook {
		t.Fatalf("jsonGet hook = %s %v %v", got, ok, err)
	}
}

func TestJSONDeleteHookKeepsUsers(t *testing.T) {
	in := []byte(`{"hooks":{"SessionStart":[{"hooks":[{"command":"echo mine"}]},` + nessHook + `]}}`)
	out, err := jsonDelete(in, hookKeyFor("SessionStart", "ness hook session-start"))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"hooks":{"SessionStart":[{"hooks":[{"command":"echo mine"}]}]}}` {
		t.Fatalf("got %s", out)
	}
}

func TestJSONInvalid(t *testing.T) {
	// covers: R-SYNC-055
	for _, in := range []string{"{nope", "[1]", `"s"`} {
		if _, _, err := jsonGet([]byte(in), "a"); err == nil {
			t.Errorf("jsonGet(%q) accepted", in)
		}
		if _, err := jsonSet([]byte(in), "a", []byte("1")); err == nil {
			t.Errorf("jsonSet(%q) accepted", in)
		}
	}
}

func TestJSONCreatedFileIsIndented(t *testing.T) {
	out, err := jsonSet(nil, "env.CLAUDE_CODE_SUBAGENT_MODEL", []byte(`"sonnet"`))
	if err != nil {
		t.Fatal(err)
	}
	if string(indentJSON(out)) != "{\n  \"env\": {\n    \"CLAUDE_CODE_SUBAGENT_MODEL\": \"sonnet\"\n  }\n}\n" {
		t.Fatalf("got %q", indentJSON(out))
	}
}

func FuzzJSONKeys(f *testing.F) {
	// covers: R-SYNC-055, R-SYNC-050
	f.Add([]byte(`{"a": 1, "hooks": {"SessionStart": []}}`))
	f.Add([]byte(`{"hooks": 3}`))
	f.Add([]byte(`{`))
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, key := range []string{"autoCompactWindow", hookKeyFor("PreToolUse", "ness hook pre-tool")} {
			out, err := jsonSet(data, key, []byte(`[1,2]`))
			if err != nil {
				continue
			}
			got, ok, err := jsonGet(out, key)
			if key == "autoCompactWindow" && (err != nil || !ok || string(got) != "[1,2]") {
				t.Fatalf("round trip %q: %s %v %v", data, got, ok, err)
			}
		}
	})
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/sync/ -run JSON`
Expected: FAIL to build (`undefined: jsonSet`).

- [ ] **Step 3: Implement**

`internal/sync/jsonkeys.go`:
```go
package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const hookPrefix = "hooks."

var errNotObject = errors.New("not a valid JSON object")

// hookKeyFor names the hook entry of event whose command contains match.
func hookKeyFor(event, match string) string { return hookPrefix + event + "[" + match + "]" }

func hookKey(key string) (event, match string, ok bool) {
	rest, ok := strings.CutPrefix(key, hookPrefix)
	if !ok || !strings.HasSuffix(rest, "]") {
		return "", "", false
	}
	return strings.Cut(strings.TrimSuffix(rest, "]"), "[")
}

// hookIndex finds the entry recognised by its command, not its position (R-SYNC-053).
func hookIndex(data []byte, event, match string) int {
	idx := -1
	gjson.GetBytes(data, hookPrefix+event).ForEach(func(k, v gjson.Result) bool {
		for _, h := range v.Get("hooks").Array() {
			if strings.Contains(h.Get("command").String(), match) {
				idx = int(k.Int())
				return false
			}
		}
		return true
	})
	return idx
}

// jsonPath resolves key to a gjson/sjson path; found is false for an absent hook entry, whose
// path then appends to the event's array.
func jsonPath(data []byte, key string) (path string, found bool) {
	event, match, ok := hookKey(key)
	if !ok {
		return key, true
	}
	if i := hookIndex(data, event, match); i >= 0 {
		return fmt.Sprintf("%s%s.%d", hookPrefix, event, i), true
	}
	return hookPrefix + event + ".-1", false
}

func checkObject(data []byte) error {
	t := bytes.TrimSpace(data)
	if len(t) == 0 {
		return nil
	}
	if t[0] != '{' || !gjson.ValidBytes(t) {
		return errNotObject
	}
	return nil
}

// jsonGet returns the compact JSON value at key and whether it exists.
func jsonGet(data []byte, key string) ([]byte, bool, error) {
	if err := checkObject(data); err != nil {
		return nil, false, err
	}
	path, found := jsonPath(data, key)
	if !found {
		return nil, false, nil
	}
	r := gjson.GetBytes(data, path)
	if !r.Exists() {
		return nil, false, nil
	}
	return compact([]byte(r.Raw)), true, nil
}

// jsonSet sets key to the raw JSON value, leaving every other byte as it was (R-SYNC-050).
func jsonSet(data []byte, key string, value []byte) ([]byte, error) {
	if err := checkObject(data); err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	path, _ := jsonPath(data, key)
	return sjson.SetRawBytes(slices.Clone(data), path, value)
}

// jsonDelete removes key if present.
func jsonDelete(data []byte, key string) ([]byte, error) {
	if err := checkObject(data); err != nil {
		return nil, err
	}
	path, found := jsonPath(data, key)
	if !found || !gjson.GetBytes(data, path).Exists() {
		return data, nil
	}
	return sjson.DeleteBytes(slices.Clone(data), path)
}

func compact(raw []byte) []byte {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return raw
	}
	return b.Bytes()
}

// indentJSON pretty-prints a file ness created from nothing.
func indentJSON(data []byte) []byte {
	var b bytes.Buffer
	if err := json.Indent(&b, data, "", "  "); err != nil {
		return data
	}
	b.WriteByte('\n')
	return b.Bytes()
}
```

- [ ] **Step 4: Run the tests and fuzz briefly**

Run: `go test ./internal/sync/ && go test -run '^$' -fuzz FuzzJSONKeys -fuzztime 30s ./internal/sync/`
Expected: PASS. A fuzzer crash inside gjson/sjson on valid input is a real finding: add the seed and guard it in `checkObject`.

- [ ] **Step 5: Commit**

```bash
git add internal/sync
git commit -m "feat(sync): owned JSON keys and hook entries recognised by command"
```

---

### Task 8: The lock

**Files:**
- Create: `internal/sync/lock.go`, `internal/sync/lock_test.go`

**Interfaces:**
- Produces:
  - `sync.LockPath = ".ness/lock.json"`
  - `type Kind string` with `KindFile`, `KindSeed`, `KindBlock`, `KindKey`, `KindLink`
  - `type Lock struct{ Entries []LockEntry; Ness string; Profiles map[string]string; Schema int }`
  - `type LockEntry struct{ Hash, ID string; Kind Kind; Owner, Path, Upstream string }`
  - `sync.ParseLock(data []byte) (*Lock, error)`, `(*Lock).Marshal() ([]byte, error)`
  - Package-internal: `readLock(root *os.Root) (*Lock, error)`, `sum(b []byte) string`

- [ ] **Step 1: Write the failing tests**

`internal/sync/lock_test.go`:
```go
package sync_test

import (
	"bytes"
	"strings"
	"testing"

	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

func TestLockDeterministic(t *testing.T) {
	// covers: R-SYNC-071, R-SYNC-087
	a := nsync.LockEntry{ID: "AGENTS.md#core/router", Path: "AGENTS.md", Kind: nsync.KindBlock, Owner: "core", Hash: "h1", Upstream: "h1"}
	b := nsync.LockEntry{ID: ".gitignore#core/ignore", Path: ".gitignore", Kind: nsync.KindBlock, Owner: "core", Hash: "h2", Upstream: "h2"}
	l1 := nsync.Lock{Schema: 1, Ness: "0.1.0", Profiles: map[string]string{"z": "1", "a": "2"}, Entries: []nsync.LockEntry{a, b}}
	l2 := nsync.Lock{Schema: 1, Ness: "0.1.0", Profiles: map[string]string{"a": "2", "z": "1"}, Entries: []nsync.LockEntry{b, a}}
	m1, err1 := l1.Marshal()
	m2, err2 := l2.Marshal()
	if err1 != nil || err2 != nil || !bytes.Equal(m1, m2) {
		t.Fatalf("not deterministic:\n%s\n%s", m1, m2)
	}
	if !strings.Contains(string(m1), `"schema": 1`) || strings.Index(string(m1), ".gitignore") > strings.Index(string(m1), "AGENTS.md") {
		t.Fatalf("lock layout:\n%s", m1)
	}
	back, err := nsync.ParseLock(m1)
	if err != nil || len(back.Entries) != 2 {
		t.Fatalf("ParseLock = %+v %v", back, err)
	}
}

func TestLockSchema(t *testing.T) {
	// covers: R-SYNC-088
	if _, err := nsync.ParseLock([]byte(`{"schema": 2}`)); err == nil || !strings.Contains(err.Error(), "schema 2") {
		t.Fatalf("schema 2 accepted: %v", err)
	}
}

func FuzzParseLock(f *testing.F) {
	// covers: R-SYNC-073
	f.Add([]byte(`{"schema":1,"entries":[{"id":"a","kind":"file"}]}`))
	f.Add([]byte(`{`))
	f.Fuzz(func(t *testing.T, data []byte) {
		l, err := nsync.ParseLock(data)
		if err != nil {
			if !strings.Contains(err.Error(), nsync.LockPath) {
				t.Fatalf("error does not name the lock: %v", err)
			}
			return
		}
		if _, err := l.Marshal(); err != nil {
			t.Fatal(err)
		}
	})
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/sync/ -run Lock`
Expected: FAIL to build (`undefined: nsync.LockEntry`).

- [ ] **Step 3: Implement**

`internal/sync/lock.go`:
```go
package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// LockPath is the lock file, relative to the repository root.
const LockPath = ".ness/lock.json"

// Kind is what a lock entry tracks.
type Kind string

// Kinds of target.
const (
	KindFile  Kind = "file"  // a whole file ness owns
	KindSeed  Kind = "seed"  // a file created once, then the user's
	KindBlock Kind = "block" // a managed block inside a shared file
	KindKey   Kind = "key"   // a JSON key ness owns
	KindLink  Kind = "link"  // a symlink
)

// Lock records what ness rendered (R-SYNC-071). Field order is alphabetical so the JSON keys
// come out sorted.
type Lock struct {
	Entries  []LockEntry       `json:"entries"`
	Ness     string            `json:"ness"`
	Profiles map[string]string `json:"profiles"`
	Schema   int               `json:"schema"`
}

// LockEntry is one recorded target. Hash is the SHA-256 of what is on disk as ness wrote it or
// as the user kept it; Upstream is the render it was recorded against. They differ only when
// the user kept their own version (sync --keep).
type LockEntry struct {
	Hash     string `json:"hash"`
	ID       string `json:"id"`
	Kind     Kind   `json:"kind"`
	Owner    string `json:"owner"`
	Path     string `json:"path"`
	Upstream string `json:"upstream"`
}

// ParseLock decodes a lock, refusing a schema this binary does not know (R-SYNC-088).
func ParseLock(data []byte) (*Lock, error) {
	var l Lock
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("%s: %w", LockPath, err)
	}
	if l.Schema != 1 {
		return nil, fmt.Errorf("%s: schema %d is not supported by this binary (want 1)", LockPath, l.Schema)
	}
	return &l, nil
}

// readLock reads the repository's lock; a missing lock is an empty one.
func readLock(root *os.Root) (*Lock, error) {
	data, err := root.ReadFile(LockPath)
	if errors.Is(err, fs.ErrNotExist) {
		return &Lock{Schema: 1, Profiles: map[string]string{}}, nil
	}
	if err != nil {
		return nil, err
	}
	return ParseLock(data)
}

// Marshal renders the lock deterministically: entries sorted by id, map keys sorted.
func (l *Lock) Marshal() ([]byte, error) {
	c := *l
	c.Entries = slices.Clone(l.Entries)
	slices.SortFunc(c.Entries, func(a, b LockEntry) int { return strings.Compare(a.ID, b.ID) })
	if c.Entries == nil {
		c.Entries = []LockEntry{}
	}
	if c.Profiles == nil {
		c.Profiles = map[string]string{}
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// sum is the lock's content hash: SHA-256, hex (R-SYNC-087).
func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/sync/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/sync
git commit -m "feat(sync): deterministic lock with schema check"
```

### Task 9: Profile format and loading

**Files:**
- Create: `internal/profile/profile.go`, `internal/profile/profile_test.go`

**Interfaces:**
- Consumes: `config.ValidKey`.
- Produces:
  - `profile.Parse(dirName string, data []byte, dir fs.FS) (*Profile, error)`
  - `profile.Source{Builtin, Local fs.FS; Version string}` with `(Source).Load(name string) (*Profile, error)`
  - `profile.LoadCore(core fs.FS, version string) (*Profile, error)`
  - Types `Profile{Name, Summary string; Requires, Conflicts, RequiresTools []string; Slots map[string]Slot; Files map[string]File; Config map[string]any; Enable []string; Atoms map[string]string; Version string; Local bool; Dir fs.FS}`, `Slot{Append, Set, Block string}` and `File{Own, Seed string}`

- [ ] **Step 1: Write the failing tests**

`internal/profile/profile_test.go`:
```go
package profile_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/enea-scaccabarozzi/ness/internal/profile"
)

const example = `name    = "local"
summary = "project policy"
requires_tools = ["python3"]

[slots]
"ness-entry#stop-and-ask"   = { append = "stop-and-ask.md" }
"program-roadmap#autonomy"  = { set = "autonomy-grant.md" }
"AGENTS.md#local"           = { block = "agents-local.md" }

[files]
"docs/workflow/ops-policy.md" = { seed = "ops-policy.md" }
".golangci.yml"               = { own = "golangci.yml" }

[config]
"integrate.push" = true

[features]
enable = ["contracts"]

[atoms]
integrator = "squash"
`

func exampleFS() fstest.MapFS {
	return fstest.MapFS{
		"stop-and-ask.md": {Data: []byte("ask\n")}, "autonomy-grant.md": {Data: []byte("grant\n")},
		"agents-local.md": {Data: []byte("local\n")}, "ops-policy.md": {Data: []byte("ops\n")},
		"golangci.yml": {Data: []byte("version: 2\n")},
	}
}

func TestParseExample(t *testing.T) {
	// covers: R-PROF-001, R-PROF-017
	p, err := profile.Parse("local", []byte(example), exampleFS())
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "local" || p.RequiresTools[0] != "python3" || p.Slots["program-roadmap#autonomy"].Set != "autonomy-grant.md" ||
		p.Files["docs/workflow/ops-policy.md"].Seed != "ops-policy.md" || p.Config["integrate.push"] != true ||
		p.Enable[0] != "contracts" || p.Atoms["integrator"] != "squash" {
		t.Fatalf("parsed %+v", p)
	}
}

func TestParseRejects(t *testing.T) {
	// covers: R-PROF-005, R-PROF-006, R-PROF-007, R-PROF-008, R-PROF-009, R-PROF-017, R-PROF-044
	cases := map[string]struct{ body, want string }{
		"syntax":        {"name = \n", "profile.toml:1:"},
		"name mismatch": {"name = \"b\"\n", `name "b" differs from its directory "local"`},
		"slot no hash":  {"name = \"local\"\n[slots]\n\"ness-entry\" = { append = \"stop-and-ask.md\" }\n", "<skill-or-file>#<slot>"},
		"slot two":      {"name = \"local\"\n[slots]\n\"a#b\" = { append = \"stop-and-ask.md\", set = \"stop-and-ask.md\" }\n", "exactly one"},
		"missing file":  {"name = \"local\"\n[slots]\n\"a#b\" = { append = \"missing.md\" }\n", "missing.md"},
		"copy file":     {"name = \"local\"\n[files]\n\"x\" = { copy = \"ops-policy.md\" }\n", "copy"},
		"escape":        {"name = \"local\"\n[files]\n\"../x\" = { own = \"ops-policy.md\" }\n", "clean relative path"},
		"feature":       {"name = \"local\"\n[features]\nenable = [\"x\"]\n", `"x" is not a feature`},
		"atom":          {"name = \"local\"\n[atoms]\nintegrator = \"rebase\"\n", "merge, squash, pr"},
		"seam":          {"name = \"local\"\n[atoms]\nrouter = \"x\"\n", `unknown seam "router"`},
		"config key":    {"name = \"local\"\n[config]\n\"integrate.pushh\" = true\n", "integrate.pushh"},
		"block target":  {"name = \"local\"\n[slots]\n\"x#y\" = { block = \"agents-local.md\" }\n", "block"},
	}
	for name, c := range cases {
		_, err := profile.Parse("local", []byte(c.body), exampleFS())
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "profile local") {
			t.Errorf("%s: err = %v, want it to name the profile and contain %q", name, err, c.want)
		}
	}
}

func TestLoadSources(t *testing.T) {
	// covers: R-PROF-003, R-PROF-042
	builtin := fstest.MapFS{"program/profile.toml": {Data: []byte("name = \"program\"\n")}}
	local := fstest.MapFS{
		"mine/profile.toml":    {Data: []byte("name = \"mine\"\n")},
		"program/profile.toml": {Data: []byte("name = \"program\"\n")},
	}
	src := profile.Source{Builtin: builtin, Local: local, Version: "0.1.0"}
	p, err := src.Load("mine")
	if err != nil || !p.Local || p.Version == "" || p.Version == "0.1.0" {
		t.Fatalf("local load = %+v %v", p, err)
	}
	if _, err := src.Load("program"); err == nil || !strings.Contains(err.Error(), "program") {
		t.Fatalf("shadowing a built-in accepted: %v", err)
	}
	if _, err := src.Load("nope"); err == nil || !strings.Contains(err.Error(), `unknown profile "nope"`) {
		t.Fatalf("unknown profile: %v", err)
	}
	src.Local = nil
	p, err = src.Load("program")
	if err != nil || p.Local || p.Version != "0.1.0" {
		t.Fatalf("built-in load = %+v %v", p, err)
	}
}

func TestLocalVersionIsContentHash(t *testing.T) {
	// covers: R-PROF-043
	fsys := fstest.MapFS{"mine/profile.toml": {Data: []byte("name = \"mine\"\n")}, "mine/a.md": {Data: []byte("a")}}
	src := profile.Source{Builtin: fstest.MapFS{}, Local: fsys}
	p1, _ := src.Load("mine")
	p2, _ := src.Load("mine")
	fsys["mine/a.md"] = &fstest.MapFile{Data: []byte("b")}
	p3, _ := src.Load("mine")
	if p1.Version != p2.Version || p1.Version == p3.Version {
		t.Fatalf("versions %s %s %s", p1.Version, p2.Version, p3.Version)
	}
}

func FuzzParse(f *testing.F) {
	// covers: R-PROF-005
	f.Add([]byte(example))
	f.Add([]byte("name = \"local\"\n[slots]\n\"a#b\" = 3\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, err := profile.Parse("local", data, exampleFS())
		if err != nil && !strings.Contains(err.Error(), "profile local") {
			t.Fatalf("error does not name the profile: %v", err)
		}
	})
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/profile/`
Expected: FAIL to build.

- [ ] **Step 3: Implement**

`internal/profile/profile.go`:
```go
// Package profile loads profiles (the embedded core, built-ins and the project's local ones),
// validates them and resolves how they combine, refusing conflicts before anything is written.
package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// Profile is one loaded profile.
type Profile struct {
	Name          string
	Summary       string
	Requires      []string
	Conflicts     []string
	RequiresTools []string
	Slots         map[string]Slot
	Files         map[string]File
	Config        map[string]any
	Enable        []string
	Atoms         map[string]string
	Version       string
	Local         bool
	Dir           fs.FS
}

// Slot is one contribution: exactly one of Append (list slot), Set (single slot) or Block (a
// block in AGENTS.md), each naming a file in the profile directory.
type Slot struct {
	Append string `toml:"append"`
	Set    string `toml:"set"`
	Block  string `toml:"block"`
}

// File is a file the profile renders: Own (kept in sync) or Seed (created once, then yours).
type File struct {
	Own  string `toml:"own"`
	Seed string `toml:"seed"`
}

type manifest struct {
	Name          string            `toml:"name"`
	Summary       string            `toml:"summary"`
	Requires      []string          `toml:"requires"`
	Conflicts     []string          `toml:"conflicts"`
	RequiresTools []string          `toml:"requires_tools"`
	Slots         map[string]Slot   `toml:"slots"`
	Files         map[string]File   `toml:"files"`
	Config        map[string]any    `toml:"config"`
	Features      features          `toml:"features"`
	Atoms         map[string]string `toml:"atoms"`
}

type features struct {
	Enable []string `toml:"enable"`
}

type seam struct {
	key   string
	atoms []string
}

var (
	featureNames = []string{"probes", "notify", "contracts"}
	// atomSeams maps each seam a profile may choose to its config key and atoms (R-PROF-009).
	atomSeams = map[string]seam{
		"client":       {"client", []string{"claude", "codex"}},
		"integrator":   {"integrate.mode", []string{"merge", "squash", "pr"}},
		"model_policy": {"models.policy", []string{"text"}},
	}
)

// Parse decodes and validates the profile.toml of directory dirName (R-PROF-001, R-PROF-005).
func Parse(dirName string, data []byte, dir fs.FS) (*Profile, error) {
	where := "profile " + dirName + ": profile.toml"
	var m manifest
	md, err := toml.Decode(string(data), &m)
	if err != nil {
		var pe toml.ParseError
		if errors.As(err, &pe) {
			return nil, fmt.Errorf("%s:%d:%d: %s", where, pe.Position.Line, pe.Position.Col, pe.Message)
		}
		return nil, fmt.Errorf("%s: %w", where, err)
	}
	if und := md.Undecoded(); len(und) > 0 {
		return nil, fmt.Errorf("%s: unknown field %s", where, und[0])
	}
	p := &Profile{
		Name: m.Name, Summary: m.Summary, Requires: m.Requires, Conflicts: m.Conflicts,
		RequiresTools: m.RequiresTools, Slots: m.Slots, Files: m.Files, Config: m.Config,
		Enable: m.Features.Enable, Atoms: m.Atoms, Dir: dir,
	}
	if err := p.validate(dirName); err != nil {
		return nil, fmt.Errorf("%s: %w", where, err)
	}
	return p, nil
}

func (p *Profile) validate(dirName string) error {
	var errs []error
	if p.Name != dirName {
		errs = append(errs, fmt.Errorf("name %q differs from its directory %q", p.Name, dirName))
	}
	for _, key := range slices.Sorted(maps.Keys(p.Slots)) {
		errs = append(errs, p.checkSlot(key, p.Slots[key]))
	}
	for _, path := range slices.Sorted(maps.Keys(p.Files)) {
		errs = append(errs, p.checkFile(path, p.Files[path]))
	}
	for _, f := range p.Enable {
		if !slices.Contains(featureNames, f) {
			errs = append(errs, fmt.Errorf("features.enable: %q is not a feature (%s)", f, strings.Join(featureNames, ", ")))
		}
	}
	for _, name := range slices.Sorted(maps.Keys(p.Atoms)) {
		s, ok := atomSeams[name]
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("atoms: unknown seam %q", name))
		case !slices.Contains(s.atoms, p.Atoms[name]):
			errs = append(errs, fmt.Errorf("atoms.%s = %q: accepted values are %s", name, p.Atoms[name], strings.Join(s.atoms, ", ")))
		}
	}
	for _, key := range slices.Sorted(maps.Keys(p.Config)) {
		if err := config.ValidKey(key, p.Config[key]); err != nil {
			errs = append(errs, fmt.Errorf("config %q: %w", key, err))
		}
	}
	return errors.Join(errs...)
}

func (p *Profile) checkSlot(key string, s Slot) error {
	target, name, ok := strings.Cut(key, "#")
	if !ok || target == "" || name == "" {
		return fmt.Errorf("slot %q: want \"<skill-or-file>#<slot>\"", key)
	}
	n, file := 0, ""
	for _, v := range []string{s.Append, s.Set, s.Block} {
		if v != "" {
			n, file = n+1, v
		}
	}
	if n != 1 {
		return fmt.Errorf("slot %q: set exactly one of append, set, block", key)
	}
	if (s.Block != "") != (target == "AGENTS.md") {
		return fmt.Errorf("slot %q: AGENTS.md slots take a block, and a block goes only in AGENTS.md", key)
	}
	return p.exists(file)
}

func (p *Profile) checkFile(path string, f File) error {
	if (f.Own == "") == (f.Seed == "") {
		return fmt.Errorf("file %q: set exactly one of own, seed", path)
	}
	if !fs.ValidPath(path) || path == "." {
		return fmt.Errorf("file %q: want a clean relative path inside the repository", path)
	}
	return p.exists(f.Own + f.Seed)
}

func (p *Profile) exists(name string) error {
	if _, err := fs.Stat(p.Dir, name); err != nil {
		return fmt.Errorf("references %s, which is missing from the profile directory", name)
	}
	return nil
}

// Source finds profiles by name: built-ins embedded in the binary and local ones under
// .ness/profiles (R-PROF-003).
type Source struct {
	Builtin fs.FS  // one directory per built-in profile
	Local   fs.FS  // the repository's .ness/profiles; nil when absent
	Version string // the binary version, recorded for built-ins
}

// Load loads one profile by name. A local profile may not reuse a built-in name (R-PROF-042).
func (s Source) Load(name string) (*Profile, error) {
	builtin := hasDir(s.Builtin, name)
	local := s.Local != nil && hasDir(s.Local, name)
	switch {
	case builtin && local:
		return nil, fmt.Errorf("local profile .ness/profiles/%s has the name of a built-in profile; rename it", name)
	case builtin:
		return load(s.Builtin, name, false, s.Version)
	case local:
		return load(s.Local, name, true, "")
	}
	return nil, fmt.Errorf("unknown profile %q (not built in, and no .ness/profiles/%s)", name, name)
}

// LoadCore loads the embedded core profile.
func LoadCore(core fs.FS, version string) (*Profile, error) {
	data, err := fs.ReadFile(core, "profile.toml")
	if err != nil {
		return nil, fmt.Errorf("core profile: %w", err)
	}
	p, err := Parse("core", data, core)
	if err != nil {
		return nil, err
	}
	p.Version = version
	return p, nil
}

func hasDir(fsys fs.FS, name string) bool {
	if fsys == nil || !fs.ValidPath(name) {
		return false
	}
	info, err := fs.Stat(fsys, name)
	return err == nil && info.IsDir()
}

func load(fsys fs.FS, name string, local bool, version string) (*Profile, error) {
	dir, err := fs.Sub(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("profile %s: %w", name, err)
	}
	data, err := fs.ReadFile(dir, "profile.toml")
	if err != nil {
		return nil, fmt.Errorf("profile %s: %w", name, err)
	}
	p, err := Parse(name, data, dir)
	if err != nil {
		return nil, err
	}
	p.Local, p.Version = local, version
	if local {
		if p.Version, err = dirHash(dir); err != nil {
			return nil, fmt.Errorf("profile %s: %w", name, err)
		}
	}
	return p, nil
}

// dirHash is a local profile's version: SHA-256 over its sorted paths and bytes (R-PROF-043).
func dirHash(dir fs.FS) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(dir, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(dir, p)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00", p, len(b))
		_, _ = h.Write(b)
		return nil
	})
	return hex.EncodeToString(h.Sum(nil)), err
}
```

- [ ] **Step 4: Run the tests and fuzz briefly**

Run: `go test ./internal/profile/ && go test -run '^$' -fuzz FuzzParse -fuzztime 20s ./internal/profile/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/profile
git commit -m "feat(profile): profile.toml format, validation and loading"
```

---

### Task 10: Resolving profiles and their conflicts

**Files:**
- Create: `internal/profile/resolve.go`, `internal/profile/resolve_test.go`

**Interfaces:**
- Consumes: `Profile` from Task 9.
- Produces:
  - `profile.Resolve(ps []*Profile, defined func(string) bool) (*Resolved, error)`
  - `type Resolved struct{ Profiles []*Profile; Lists map[string][]Part; Singles map[string]Part; Blocks []Block; Files []OwnedFile; Defaults map[string]any; Features []string; Tools map[string]string }`
  - `(*Resolved).Versions() map[string]string`
  - `Part{Owner string; Text []byte}`, `Block{Name, Owner string; Text []byte}`, `OwnedFile{Path, Owner string; Content []byte; Seed bool}`

- [ ] **Step 1: Write the failing tests**

`internal/profile/resolve_test.go`:
```go
package profile_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/enea-scaccabarozzi/ness/internal/profile"
)

// mk builds a profile from a manifest and files; every file named in the manifest must be given.
func mk(t *testing.T, name, manifest string, files map[string]string, local bool) *profile.Profile {
	t.Helper()
	fsys := fstest.MapFS{}
	for k, v := range files {
		fsys[k] = &fstest.MapFile{Data: []byte(v)}
	}
	p, err := profile.Parse(name, []byte("name = \""+name+"\"\n"+manifest), fsys)
	if err != nil {
		t.Fatal(err)
	}
	p.Local = local
	return p
}

func none(string) bool { return false }

func resolveErr(t *testing.T, ps []*profile.Profile, defined func(string) bool, wants ...string) {
	t.Helper()
	_, err := profile.Resolve(ps, defined)
	if err == nil {
		t.Fatal("no conflict reported")
	}
	for _, w := range wants {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not contain %q", err, w)
		}
	}
}

func appender(t *testing.T, name, text string, local bool) *profile.Profile {
	t.Helper()
	return mk(t, name, "[slots]\n\"ness-entry#sizes\" = { append = \"s.md\" }\n", map[string]string{"s.md": text}, local)
}

func TestListSlotOrder(t *testing.T) {
	// covers: R-PROF-012, R-PROF-020, R-PROF-030, R-PROF-038, R-SYNC-001
	a, b := appender(t, "a", "A\n", false), appender(t, "b", "B\n", false)
	for _, order := range [][]*profile.Profile{{a, b}, {b, a}} {
		r, err := profile.Resolve(order, none)
		if err != nil {
			t.Fatal(err)
		}
		parts := r.Lists["ness-entry#sizes"]
		if len(parts) != 2 || parts[0].Owner != order[0].Name || parts[1].Owner != order[1].Name {
			t.Fatalf("parts %+v for order %s,%s", parts, order[0].Name, order[1].Name)
		}
	}
}

func TestLocalAppliedLast(t *testing.T) {
	// covers: R-PROF-021
	local, a := appender(t, "local", "L\n", true), appender(t, "a", "A\n", false)
	r, err := profile.Resolve([]*profile.Profile{local, a}, none)
	if err != nil {
		t.Fatal(err)
	}
	if parts := r.Lists["ness-entry#sizes"]; parts[1].Owner != "local" || r.Profiles[1].Name != "local" {
		t.Fatalf("local not last: %+v", parts)
	}
}

func setter(t *testing.T, name string, local bool) *profile.Profile {
	t.Helper()
	return mk(t, name, "[slots]\n\"x#y\" = { set = \"v.md\" }\n", map[string]string{"v.md": name + "\n"}, local)
}

func TestSingleSlot(t *testing.T) {
	// covers: R-PROF-013, R-PROF-022, R-PROF-031, R-PROF-034
	r, err := profile.Resolve([]*profile.Profile{setter(t, "a", false)}, none)
	if err != nil || string(r.Singles["x#y"].Text) != "a\n" {
		t.Fatalf("single = %+v %v", r.Singles, err)
	}
	resolveErr(t, []*profile.Profile{setter(t, "a", false), setter(t, "b", false)}, none, "a", "b", "x#y")
	resolveErr(t, []*profile.Profile{setter(t, "b", false), setter(t, "a", false)}, none, "a", "b", "x#y")
	r, err = profile.Resolve([]*profile.Profile{setter(t, "a", false), setter(t, "b", false), setter(t, "local", true)}, none)
	if err != nil || string(r.Singles["x#y"].Text) != "local\n" {
		t.Fatalf("local override = %+v %v", r.Singles, err)
	}
}

func TestSeamConflict(t *testing.T) {
	// covers: R-PROF-032, R-PROF-023, R-PROF-034
	squash := mk(t, "a", "[atoms]\nintegrator = \"squash\"\n", nil, false)
	pr := mk(t, "local", "[atoms]\nintegrator = \"pr\"\n", nil, true)
	resolveErr(t, []*profile.Profile{squash, pr}, none, "a", "local", "integrator")
	r, err := profile.Resolve([]*profile.Profile{squash, pr}, func(k string) bool { return k == "integrate.mode" })
	if err != nil {
		t.Fatalf("ness.toml did not settle the seam: %v", err)
	}
	if _, set := r.Defaults["integrate.mode"]; set {
		t.Fatal("a key the file sets must not get a profile default")
	}
}

func TestConfigConflict(t *testing.T) {
	// covers: R-PROF-033
	a := mk(t, "a", "[config]\n\"integrate.push\" = true\n", nil, false)
	b := mk(t, "b", "[config]\n\"integrate.push\" = false\n", nil, false)
	resolveErr(t, []*profile.Profile{a, b}, none, "a", "b", "integrate.push")
	same := mk(t, "c", "[config]\n\"integrate.push\" = true\n", nil, false)
	r, err := profile.Resolve([]*profile.Profile{a, same}, none)
	if err != nil || r.Defaults["integrate.push"] != true {
		t.Fatalf("equal values = %+v %v", r.Defaults, err)
	}
}

func TestOwnedPathConflict(t *testing.T) {
	// covers: R-PROF-035, R-PROF-010
	own := func(name string, local bool) *profile.Profile {
		return mk(t, name, "[files]\n\"docs/x.md\" = { own = \"x.md\" }\n", map[string]string{"x.md": "x\n"}, local)
	}
	resolveErr(t, []*profile.Profile{own("a", false), own("local", true)}, none, "a", "local", "docs/x.md")
	r, err := profile.Resolve([]*profile.Profile{own("a", false)}, none)
	if err != nil || len(r.Files) != 1 || r.Files[0].Owner != "a" || r.Files[0].Seed {
		t.Fatalf("files = %+v %v", r.Files, err)
	}
}

func TestRelations(t *testing.T) {
	// covers: R-PROF-036, R-PROF-040
	needs := mk(t, "behavior", "requires = [\"strict\"]\n", nil, false)
	resolveErr(t, []*profile.Profile{needs}, none, "behavior", "strict")
	hates := mk(t, "a", "conflicts = [\"b\"]\n", nil, false)
	resolveErr(t, []*profile.Profile{hates, mk(t, "b", "", nil, false)}, none, "a", "b")
	resolveErr(t, []*profile.Profile{mk(t, "b", "", nil, false), mk(t, "b", "", nil, false)}, none, "twice")
}

func TestBlocksNamedByProfile(t *testing.T) {
	// covers: R-PROF-014, R-SYNC-082
	p := mk(t, "local", "[slots]\n\"AGENTS.md#rules\" = { block = \"r.md\" }\n", map[string]string{"r.md": "r\n"}, true)
	r, err := profile.Resolve([]*profile.Profile{p}, none)
	if err != nil || len(r.Blocks) != 1 || r.Blocks[0].Name != "local/rules" {
		t.Fatalf("blocks = %+v %v", r.Blocks, err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/profile/ -run 'Slot|Local|Seam|Config|Owned|Relations|Blocks'`
Expected: FAIL to build (`undefined: profile.Resolve`).

- [ ] **Step 3: Implement**

`internal/profile/resolve.go`:
```go
package profile

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"reflect"
	"slices"
	"strings"
)

// Part is one profile's text for a slot.
type Part struct {
	Owner string
	Text  []byte
}

// Block is a profile's managed block in AGENTS.md, named "<profile>/<block>" (R-SYNC-082).
type Block struct {
	Name  string
	Owner string
	Text  []byte
}

// OwnedFile is a file a profile renders into the repository.
type OwnedFile struct {
	Path    string
	Owner   string
	Content []byte
	Seed    bool
}

// Resolved is what sync renders: the profiles combined, with conflicts rejected.
type Resolved struct {
	Profiles []*Profile        // application order: as given, local last
	Lists    map[string][]Part // list slot -> parts in profile order
	Singles  map[string]Part   // single slot -> the winning part
	Blocks   []Block           // AGENTS.md blocks in profile order
	Files    []OwnedFile       // sorted by path
	Defaults map[string]any    // config defaults from [config] and [atoms]
	Features []string          // features the profiles enable
	Tools    map[string]string // required tool -> first profile requiring it
}

type setter struct {
	part  Part
	local bool
}

type valueBy struct {
	owner, label string
	value        any
}

// Resolve combines profiles in order with the local profile last (R-PROF-020, R-PROF-021).
// defined reports the dotted keys .ness/ness.toml sets; they settle config and seam conflicts.
// There is no "last wins" between other profiles (R-PROF-034).
func Resolve(ps []*Profile, defined func(string) bool) (*Resolved, error) {
	ordered := localLast(ps)
	r := &Resolved{
		Profiles: ordered, Lists: map[string][]Part{}, Singles: map[string]Part{},
		Defaults: map[string]any{}, Tools: map[string]string{},
	}
	errs := []error{checkRelations(ordered)}
	setters := map[string][]setter{}
	owners := map[string]string{}
	values := map[string][]valueBy{}
	for _, p := range ordered {
		errs = append(errs, r.addSlots(p, setters), r.addFiles(p, owners))
		addDefaults(p, values)
		for _, f := range p.Enable {
			if !slices.Contains(r.Features, f) {
				r.Features = append(r.Features, f)
			}
		}
		for _, tool := range p.RequiresTools {
			if _, ok := r.Tools[tool]; !ok {
				r.Tools[tool] = p.Name
			}
		}
	}
	errs = append(errs, r.pickSingles(setters), r.pickDefaults(values, defined))
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	slices.SortFunc(r.Files, func(a, b OwnedFile) int { return strings.Compare(a.Path, b.Path) })
	return r, nil
}

// Versions maps each applied profile to its recorded version.
func (r *Resolved) Versions() map[string]string {
	out := map[string]string{}
	for _, p := range r.Profiles {
		out[p.Name] = p.Version
	}
	return out
}

func isLocal(p *Profile) bool { return p.Local && p.Name == "local" }

func localLast(ps []*Profile) []*Profile {
	out := make([]*Profile, 0, len(ps))
	var local *Profile
	for _, p := range ps {
		if isLocal(p) {
			local = p
			continue
		}
		out = append(out, p)
	}
	if local != nil {
		out = append(out, local)
	}
	return out
}

func checkRelations(ps []*Profile) error {
	var errs []error
	names := map[string]bool{}
	for _, p := range ps {
		if names[p.Name] {
			errs = append(errs, fmt.Errorf("profile %s is applied twice", p.Name))
		}
		names[p.Name] = true
	}
	for _, p := range ps {
		for _, req := range p.Requires {
			if !names[req] {
				errs = append(errs, fmt.Errorf("profile %s requires profile %s; add it to profiles in .ness/ness.toml", p.Name, req))
			}
		}
		for _, c := range p.Conflicts {
			if names[c] {
				errs = append(errs, fmt.Errorf("profiles %s and %s conflict; apply only one", p.Name, c))
			}
		}
	}
	return errors.Join(errs...)
}

func (r *Resolved) addSlots(p *Profile, setters map[string][]setter) error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(p.Slots)) {
		s := p.Slots[key]
		text, err := fs.ReadFile(p.Dir, s.Append+s.Set+s.Block)
		if err != nil {
			errs = append(errs, fmt.Errorf("profile %s: slot %s: %w", p.Name, key, err))
			continue
		}
		part := Part{Owner: p.Name, Text: text}
		switch {
		case s.Append != "":
			r.Lists[key] = append(r.Lists[key], part)
		case s.Set != "":
			setters[key] = append(setters[key], setter{part: part, local: isLocal(p)})
		default:
			_, name, _ := strings.Cut(key, "#")
			r.Blocks = append(r.Blocks, Block{Name: p.Name + "/" + name, Owner: p.Name, Text: text})
		}
	}
	return errors.Join(errs...)
}

func (r *Resolved) addFiles(p *Profile, owners map[string]string) error {
	var errs []error
	for _, path := range slices.Sorted(maps.Keys(p.Files)) {
		if prev, ok := owners[path]; ok {
			errs = append(errs, fmt.Errorf("profiles %s and %s both own %s", prev, p.Name, path))
			continue
		}
		owners[path] = p.Name
		f := p.Files[path]
		content, err := fs.ReadFile(p.Dir, f.Own+f.Seed)
		if err != nil {
			errs = append(errs, fmt.Errorf("profile %s: file %s: %w", p.Name, path, err))
			continue
		}
		r.Files = append(r.Files, OwnedFile{Path: path, Owner: p.Name, Content: content, Seed: f.Seed != ""})
	}
	return errors.Join(errs...)
}

func addDefaults(p *Profile, values map[string][]valueBy) {
	for _, key := range slices.Sorted(maps.Keys(p.Config)) {
		values[key] = append(values[key], valueBy{owner: p.Name, label: key, value: p.Config[key]})
	}
	for _, name := range slices.Sorted(maps.Keys(p.Atoms)) {
		key := atomSeams[name].key
		values[key] = append(values[key], valueBy{owner: p.Name, label: "seam " + name, value: p.Atoms[name]})
	}
}

// pickSingles lets the local profile override a single slot; otherwise two setters conflict.
func (r *Resolved) pickSingles(setters map[string][]setter) error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(setters)) {
		ss := setters[key]
		if i := slices.IndexFunc(ss, func(s setter) bool { return s.local }); i >= 0 {
			r.Singles[key] = ss[i].part
			continue
		}
		if len(ss) > 1 {
			names := make([]string, len(ss))
			for i, s := range ss {
				names[i] = s.part.Owner
			}
			errs = append(errs, fmt.Errorf("profiles %s all set single slot %s; only the local profile may override it",
				strings.Join(names, ", "), key))
			continue
		}
		r.Singles[key] = ss[0].part
	}
	return errors.Join(errs...)
}

// pickDefaults keeps one value per config key; keys the file sets need no default.
func (r *Resolved) pickDefaults(values map[string][]valueBy, defined func(string) bool) error {
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(values)) {
		if defined(key) {
			continue
		}
		vs := values[key]
		for _, v := range vs[1:] {
			if !reflect.DeepEqual(v.value, vs[0].value) {
				errs = append(errs, fmt.Errorf("profiles %s and %s set %s differently (%v, %v); set %s in .ness/ness.toml",
					vs[0].owner, v.owner, v.label, vs[0].value, v.value, key))
			}
		}
		r.Defaults[key] = vs[0].value
	}
	return errors.Join(errs...)
}
```

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/profile/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/profile
git commit -m "feat(profile): resolve order, slots, files and defaults with conflict detection"
```

### Task 11: Core profile and rendering the desired state

**Files:**
- Create: `profiles/embed.go`, `profiles/core/profile.toml`, `profiles/core/router.md`, `profiles/core/skills/ness-entry/SKILL.md`, `profiles/builtin/README.md`, `internal/sync/render.go`, `internal/sync/render_test.go`

**Interfaces:**
- Consumes: `profile.Resolved`, `config.Config`, `config.SplitTier`, and `hookKeyFor` and `normInner` from Tasks 6–7.
- Produces:
  - `profiles.Core() fs.FS`, `profiles.Builtin() fs.FS`
  - `type sync.Entry struct{ ID, Path string; Kind Kind; Owner, Name string; Content []byte }`
  - `sync.Render(res *profile.Resolved, cfg *config.Config, root *os.Root) ([]Entry, error)`
  - Package-internal `entryID(e Entry) string`
- Slot markers in owned Markdown files: a line `<!-- ness:slot list NAME -->` or `<!-- ness:slot single NAME -->`. Contributions are inserted above the marker, and the marker line is removed from the rendered file. Slot key `T#NAME` targets the owned file at path `T`, or else `.agents/skills/T/SKILL.md`.

- [ ] **Step 1: Core profile files**

`profiles/core/profile.toml`:
```toml
name    = "core"
summary = "ness core: the AGENTS.md router and the lifecycle skills"

[slots]
"AGENTS.md#router" = { block = "router.md" }

[files]
".agents/skills/ness-entry/SKILL.md" = { own = "skills/ness-entry/SKILL.md" }
```

`profiles/core/router.md`:
```markdown
## Sessions (ness)
Applies only when the `ness` command exists in this checkout. Otherwise ignore this block.

This repository runs the ness harness. Start every session by loading the `ness-entry` skill.
```

`profiles/core/skills/ness-entry/SKILL.md` (minimal; P6 writes the full core text):
```markdown
---
name: ness-entry
description: Start here in a ness project. Covers who you are in the session tree, the work sizes, and when to stop and ask.
---

# ness entry

You are one session in a tree of accountable sessions. Your brief names your scope, your
manager, and the result you owe.

## Work sizes
- **answer**: reply in chat, with no session record.
- **quick task**: one branch, a few commits, and a short report.
- **full session**: a brief, plan, decisions and report under the session directory.
<!-- ness:slot list sizes -->

## Stop and ask
Stop and ask your manager before anything you cannot undo.
<!-- ness:slot list stop-and-ask -->

## After compaction
Re-read your brief and your inbox before continuing.
<!-- ness:slot list post-compact -->
```

`profiles/builtin/README.md`:
```markdown
One directory per built-in profile, each with a `profile.toml`. Built-ins arrive in plan P6.
```

`profiles/embed.go`:
```go
// Package profiles embeds ness's core text and built-in profiles into the binary (R-PROF-002).
package profiles

import (
	"embed"
	"io/fs"
)

//go:embed all:core all:builtin
var embedded embed.FS

// Core returns the core profile's directory.
func Core() fs.FS { return sub("core") }

// Builtin returns the directory holding one subdirectory per built-in profile.
func Builtin() fs.FS { return sub("builtin") }

func sub(dir string) fs.FS {
	s, err := fs.Sub(embedded, dir)
	if err != nil {
		panic(err) // the directory is embedded at build time; failure is a build defect
	}
	return s
}
```

- [ ] **Step 2: Write the failing tests**

`internal/sync/render_test.go`:
```go
package sync_test

import (
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
	"github.com/enea-scaccabarozzi/ness/profiles"
)

func testConfig(t *testing.T, extra string) *config.Config {
	t.Helper()
	f, err := config.Parse([]byte("ness = \"0.1\"\nproject = \"p\"\nbase = \"main\"\ngate = \"true\"\n" + extra))
	if err != nil {
		t.Fatal(err)
	}
	c, err := f.Config(nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func openRoot(t *testing.T) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func render(t *testing.T, root *os.Root, extra string, ps ...*profile.Profile) ([]nsync.Entry, error) {
	t.Helper()
	core, err := profile.LoadCore(profiles.Core(), "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	res, err := profile.Resolve(append([]*profile.Profile{core}, ps...), func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	return nsync.Render(res, testConfig(t, extra), root)
}

func find(entries []nsync.Entry, id string) *nsync.Entry {
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i]
		}
	}
	return nil
}

func TestRenderCore(t *testing.T) {
	// covers: R-SYNC-060, R-SYNC-061, R-SYNC-066, R-SYNC-067, R-SYNC-069, R-SYNC-084, R-SYNC-085, R-MOD-020
	entries, err := render(t, openRoot(t), "[tiers.economy]\nclaude = \"haiku/medium\"\n")
	if err != nil {
		t.Fatal(err)
	}
	router := find(entries, "AGENTS.md#core/router")
	if router == nil || !strings.HasPrefix(string(router.Content),
		"## Sessions (ness)\nApplies only when the `ness` command exists in this checkout. Otherwise ignore this block.\n") {
		t.Fatalf("router block: %+v", router)
	}
	var keys []string
	for _, e := range entries {
		if e.Path == ".claude/settings.json" {
			keys = append(keys, e.Name)
		}
	}
	wantKeys := []string{"autoCompactWindow", "env.CLAUDE_CODE_SUBAGENT_MODEL",
		"hooks.PreToolUse[ness hook pre-tool]", "hooks.SessionStart[ness hook session-start]"}
	slices.Sort(keys)
	if !slices.Equal(keys, wantKeys) {
		t.Fatalf("settings keys %q", keys)
	}
	if e := find(entries, ".claude/settings.json:autoCompactWindow"); string(e.Content) != "265000" {
		t.Fatalf("autoCompactWindow = %s", e.Content)
	}
	if e := find(entries, ".claude/settings.json:env.CLAUDE_CODE_SUBAGENT_MODEL"); string(e.Content) != `"haiku"` {
		t.Fatalf("subagent model = %s", e.Content)
	}
	hook := find(entries, ".claude/settings.json:hooks.SessionStart[ness hook session-start]")
	if !strings.Contains(string(hook.Content), `"command -v ness >/dev/null && ness hook session-start || true"`) {
		t.Fatalf("hook entry %s", hook.Content)
	}
	if e := find(entries, ".gitignore#core/ignore"); string(e.Content) != ".worktrees/\n*.ness-new\n" {
		t.Fatalf("gitignore block %q", e.Content)
	}
	agents := map[string]string{"ness-sweep": "haiku/low", "ness-review": "opus/medium",
		"ness-verify": "opus/high", "ness-design": "fable/medium"}
	for name, me := range agents {
		model, effort, _ := strings.Cut(me, "/")
		e := find(entries, ".claude/agents/"+name+".md")
		want := "---\nname: " + name + "\n"
		if e == nil || e.Owner != "core" || !strings.HasPrefix(string(e.Content), want) ||
			!strings.Contains(string(e.Content), "\nmodel: "+model+"\neffort: "+effort+"\n---\n") {
			t.Errorf("subagent %s: %+v", name, e)
		}
	}
}

func TestRenderClaudeMD(t *testing.T) {
	// covers: R-SYNC-062, R-SYNC-063, R-SYNC-065
	root := openRoot(t)
	entries, err := render(t, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if e := find(entries, "CLAUDE.md"); e == nil || e.Kind != nsync.KindLink || string(e.Content) != "AGENTS.md" {
		t.Fatalf("CLAUDE.md entry %+v", e)
	}
	if e := find(entries, ".claude/skills"); e == nil || e.Kind != nsync.KindLink || string(e.Content) != "../.agents/skills" {
		t.Fatalf(".claude/skills entry %+v", e)
	}
	if err := root.WriteFile("CLAUDE.md", []byte("# mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err = render(t, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if e := find(entries, "CLAUDE.md#core/import"); e == nil || string(e.Content) != "@AGENTS.md\n" {
		t.Fatalf("import block %+v", e)
	}
}

func local(t *testing.T, manifest string, files map[string]string) *profile.Profile {
	t.Helper()
	fsys := fstest.MapFS{}
	for k, v := range files {
		fsys[k] = &fstest.MapFile{Data: []byte(v)}
	}
	p, err := profile.Parse("local", []byte("name = \"local\"\n"+manifest), fsys)
	if err != nil {
		t.Fatal(err)
	}
	p.Local = true
	return p
}

func TestRenderSlots(t *testing.T) {
	// covers: R-PROF-041, R-PROF-012
	p := local(t, "[slots]\n\"ness-entry#stop-and-ask\" = { append = \"a.md\" }\n", map[string]string{"a.md": "Ask before deploys.\n"})
	entries, err := render(t, openRoot(t), "", p)
	if err != nil {
		t.Fatal(err)
	}
	skill := string(find(entries, ".agents/skills/ness-entry/SKILL.md").Content)
	if !strings.Contains(skill, "anything you cannot undo.\nAsk before deploys.\n\n## After compaction") ||
		strings.Contains(skill, "ness:slot") {
		t.Fatalf("rendered skill:\n%s", skill)
	}
	for _, bad := range []string{
		"[slots]\n\"ness-entry#nope\" = { append = \"a.md\" }\n",
		"[slots]\n\"ness-entry#sizes\" = { set = \"a.md\" }\n",
		"[slots]\n\"ness-nothing#sizes\" = { append = \"a.md\" }\n",
	} {
		if _, err := render(t, openRoot(t), "", local(t, bad, map[string]string{"a.md": "x\n"})); err == nil ||
			!strings.Contains(err.Error(), "local") {
			t.Errorf("slot %q: err = %v", bad, err)
		}
	}
}

func TestRenderRejectsDoubleTargets(t *testing.T) {
	p := local(t, "[files]\n\".claude/agents/ness-sweep.md\" = { own = \"a.md\" }\n", map[string]string{"a.md": "x\n"})
	if _, err := render(t, openRoot(t), "", p); err == nil || !strings.Contains(err.Error(), ".claude/agents/ness-sweep.md") {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/sync/ -run Render`
Expected: FAIL to build (`undefined: nsync.Render`).

- [ ] **Step 4: Implement**

`internal/sync/render.go`:
```go
package sync

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
)

const (
	agentsPath   = "AGENTS.md"
	settingsPath = ".claude/settings.json"
	skillsDir    = ".agents/skills/"
	coreOwner    = "core"
)

// Entry is one desired target: a whole file, a seed, a block, a JSON key or a link.
type Entry struct {
	ID      string
	Path    string
	Kind    Kind
	Owner   string
	Name    string // block name or JSON key; empty for whole files and links
	Content []byte // file bytes, block content, compact JSON value or link target
}

var (
	slotRe     = regexp.MustCompile(`(?m)^<!-- ness:slot (?:list|single) [a-z0-9-]+ -->\n?`)
	hookEvents = []struct{ event, name string }{{"SessionStart", "session-start"}, {"PreToolUse", "pre-tool"}}
)

// Render turns the resolved profiles and the final config into the desired targets, sorted by
// id. It reads the repository only to decide how CLAUDE.md is managed.
func Render(res *profile.Resolved, cfg *config.Config, root *os.Root) ([]Entry, error) {
	files, err := fillSlots(res)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(files)+len(res.Blocks)+12)
	for _, f := range files {
		kind := KindFile
		if f.Seed {
			kind = KindSeed
		}
		out = append(out, Entry{Path: f.Path, Kind: kind, Owner: f.Owner, Content: f.Content})
	}
	for _, b := range res.Blocks {
		out = append(out, Entry{Path: agentsPath, Kind: KindBlock, Owner: b.Owner, Name: b.Name, Content: b.Text})
	}
	core, err := coreEntries(cfg, root)
	if err != nil {
		return nil, err
	}
	out = append(out, core...)
	for i := range out {
		out[i].ID = entryID(out[i])
	}
	slices.SortFunc(out, func(a, b Entry) int { return strings.Compare(a.ID, b.ID) })
	return out, checkTargets(out)
}

func entryID(e Entry) string {
	switch e.Kind {
	case KindBlock:
		return e.Path + "#" + e.Name
	case KindKey:
		return e.Path + ":" + e.Name
	default:
		return e.Path
	}
}

// checkTargets refuses two entries for one id, and a whole file sharing a path with parts.
func checkTargets(entries []Entry) error {
	var errs []error
	whole := map[string]string{}
	parts := map[string]string{}
	for i, e := range entries {
		if i > 0 && entries[i-1].ID == e.ID {
			errs = append(errs, fmt.Errorf("%s is rendered by both %s and %s", e.ID, entries[i-1].Owner, e.Owner))
		}
		if e.Kind == KindBlock || e.Kind == KindKey {
			parts[e.Path] = e.Owner
		} else {
			whole[e.Path] = e.Owner
		}
	}
	for _, p := range slices.Sorted(maps.Keys(whole)) {
		if owner, ok := parts[p]; ok {
			errs = append(errs, fmt.Errorf("%s is owned whole by %s and edited in part by %s", p, whole[p], owner))
		}
	}
	return errors.Join(errs...)
}

// fillSlots inserts slot contributions above their markers, then drops every marker line.
func fillSlots(res *profile.Resolved) ([]profile.OwnedFile, error) {
	files := slices.Clone(res.Files)
	index := map[string]int{}
	for i, f := range files {
		index[f.Path] = i
	}
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(res.Lists)) {
		errs = append(errs, fill(files, index, key, "list", res.Lists[key]))
	}
	for _, key := range slices.Sorted(maps.Keys(res.Singles)) {
		errs = append(errs, fill(files, index, key, "single", []profile.Part{res.Singles[key]}))
	}
	for i := range files {
		files[i].Content = slotRe.ReplaceAll(files[i].Content, nil)
	}
	return files, errors.Join(errs...)
}

func fill(files []profile.OwnedFile, index map[string]int, key, kind string, parts []profile.Part) error {
	target, name, _ := strings.Cut(key, "#")
	path := target
	if _, ok := index[path]; !ok {
		path = skillsDir + target + "/SKILL.md"
	}
	i, ok := index[path]
	if !ok {
		return fmt.Errorf("profile %s fills slot %s, but no rendered skill or file is named %s", parts[0].Owner, key, target)
	}
	marker := []byte(fmt.Sprintf("<!-- ness:slot %s %s -->", kind, name))
	c := files[i].Content
	at := bytes.Index(c, marker)
	if at < 0 {
		return fmt.Errorf("profile %s fills %s slot %s, which %s does not have", parts[0].Owner, kind, key, path)
	}
	var text []byte
	for _, p := range parts {
		text = append(text, normInner(p.Text)...)
	}
	files[i].Content = slices.Concat(c[:at], text, c[at:])
	return nil
}

func coreEntries(cfg *config.Config, root *os.Root) ([]Entry, error) {
	economy, _ := config.SplitTier(cfg.Tiers["economy"].Claude)
	model, err := rawJSON(economy)
	if err != nil {
		return nil, err
	}
	out := []Entry{
		{Path: ".gitignore", Kind: KindBlock, Owner: coreOwner, Name: "core/ignore", Content: []byte(".worktrees/\n*.ness-new\n")},
		{Path: ".claude/skills", Kind: KindLink, Owner: coreOwner, Content: []byte("../.agents/skills")},
		{Path: settingsPath, Kind: KindKey, Owner: coreOwner, Name: "autoCompactWindow", Content: []byte("265000")},
		{Path: settingsPath, Kind: KindKey, Owner: coreOwner, Name: "env.CLAUDE_CODE_SUBAGENT_MODEL", Content: model},
		claudeMD(root),
	}
	for _, ev := range hookEvents {
		cmd := "command -v ness >/dev/null && ness hook " + ev.name + " || true"
		entry, err := rawJSON(map[string]any{"hooks": []map[string]string{{"type": "command", "command": cmd}}})
		if err != nil {
			return nil, err
		}
		out = append(out, Entry{Path: settingsPath, Kind: KindKey, Owner: coreOwner,
			Name: hookKeyFor(ev.event, "ness hook "+ev.name), Content: entry})
	}
	return append(out, subagents(cfg)...), nil
}

// claudeMD links CLAUDE.md to AGENTS.md unless it is the user's file, which gets an import
// block instead (R-SYNC-062, R-SYNC-063).
func claudeMD(root *os.Root) Entry {
	if info, err := root.Lstat("CLAUDE.md"); err == nil && info.Mode().IsRegular() {
		return Entry{Path: "CLAUDE.md", Kind: KindBlock, Owner: coreOwner, Name: "core/import", Content: []byte("@AGENTS.md\n")}
	}
	return Entry{Path: "CLAUDE.md", Kind: KindLink, Owner: coreOwner, Content: []byte(agentsPath)}
}

// subagents generates the named subagents from the tier table (R-MOD-020).
func subagents(cfg *config.Config) []Entry {
	model := func(tier string) string {
		m, _ := config.SplitTier(cfg.Tiers[tier].Claude)
		return m
	}
	agents := []struct{ name, model, effort, desc, body string }{
		{"ness-sweep", model("economy"), "low",
			"Read-only search. Finds files and facts across the repository and returns paths with one-line findings.",
			"Search for what the caller asks and do not edit files. Write long results to a file, then return its path and a summary of at most 8,000 characters."},
		{"ness-review", model("standard"), "medium",
			"Reviews a change against its plan and the project's rules, and reports findings ranked by severity.",
			"Review the change you are given. Report each finding with its file and line, ranked by severity."},
		{"ness-verify", model("standard"), "high",
			"Adversarially verifies one claim or finding and returns a verdict with evidence.",
			"Try to prove the claim wrong. Return CONFIRMED or REJECTED with the evidence that decides it."},
		{"ness-design", model("frontier"), "medium",
			"Designs an interface or plan for a hard problem and returns the options with a recommendation.",
			"Lay out the viable designs, their costs and the one you recommend, with the reason."},
	}
	out := make([]Entry, 0, len(agents))
	for _, a := range agents {
		content := fmt.Sprintf("---\nname: %s\ndescription: %s\nmodel: %s\neffort: %s\n---\n\n%s\n",
			a.name, a.desc, a.model, a.effort, a.body)
		out = append(out, Entry{Path: ".claude/agents/" + a.name + ".md", Kind: KindFile, Owner: coreOwner, Content: []byte(content)})
	}
	return out
}

// rawJSON encodes v compactly without HTML escaping, so "&&" and ">" stay readable.
func rawJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/sync/ ./profiles/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add profiles internal/sync
git commit -m "feat(sync): render core, profile files, slots, blocks, settings keys and subagents"
```

---

### Task 12: The sync engine

**Files:**
- Create: `internal/sync/engine.go`, `internal/sync/engine_test.go`

**Interfaces:**
- Consumes: `Entry`, `readLock`, `sum`, and the block and JSON helpers.
- Produces:
  - `type Action string` with `ActNoop`, `ActCreate`, `ActAdopt`, `ActUpdate`, `ActKeep`, `ActConflict`, `ActDelete`, `ActOrphan`, `ActDrop`
  - `type Step struct{ ID string; Action Action; ... }`
  - `type FilePlan struct{ Path string; Steps []Step; Before, After []byte; Exists, Gone, Link, Conflict, Kept bool; Err error }`
  - `type LockMeta struct{ Ness string; Profiles map[string]string }`
  - `sync.Compute(root *os.Root, entries []Entry, meta LockMeta) (*Plan, error)`
  - `(*Plan).Keep(path string) error`, `(*Plan).Conflicts() []string`, `(*Plan).Changed() (bool, error)`, `(*Plan).LockBytes() ([]byte, error)`
  - `sync.Apply(root *os.Root, p *Plan) error`
  - `Plan.Files []FilePlan` is exported for reporting.

Rules (design §6), applied per target. `w` is the hash of the desired content, `c` the hash of the current content (or `absent`), and `rec` the lock entry with `Hash` and `Upstream`:

| Situation | Action |
|---|---|
| c == w | noop if rec already says w/w, else adopt (record) |
| no rec, absent | create |
| no rec, present and different | conflict (rule 2) |
| rec ness-written (Hash == Upstream) and c == Hash | update (rule 3) |
| rec kept (Hash != Upstream) and c == Hash | noop if Upstream == w, else conflict (rule 4) |
| rec, absent | create (restore) |
| rec, user changed, Upstream == w | keep (rule 4) |
| rec, user changed, upstream changed | conflict (rule 4) |
| no longer rendered | delete if ness-written and unchanged; drop if absent or seed; else orphan (rule 5) |

Seeds: create if unrecorded and absent, adopt (record) if unrecorded and present, otherwise noop forever (R-SYNC-070, R-SYNC-081).

- [ ] **Step 1: Write the failing tests**

`internal/sync/engine_test.go`:
```go
package sync_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

func file(path, content string) nsync.Entry {
	return nsync.Entry{Path: path, Kind: nsync.KindFile, Owner: "p", Content: []byte(content)}
}

func blockEntry(content string) nsync.Entry {
	return nsync.Entry{Path: "AGENTS.md", Kind: nsync.KindBlock, Owner: "p", Name: "p/x", Content: []byte(content)}
}

func syncOnce(t *testing.T, root *os.Root, entries ...nsync.Entry) *nsync.Plan {
	t.Helper()
	p, err := nsync.Compute(root, entries, nsync.LockMeta{Ness: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := nsync.Apply(root, p); err != nil {
		t.Fatal(err)
	}
	return p
}

func read(t *testing.T, root *os.Root, path string) (string, bool) {
	t.Helper()
	b, err := root.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b), true
}

func lockEntry(t *testing.T, root *os.Root, id string) *nsync.LockEntry {
	t.Helper()
	b, ok := read(t, root, nsync.LockPath)
	if !ok {
		return nil
	}
	l, err := nsync.ParseLock([]byte(b))
	if err != nil {
		t.Fatal(err)
	}
	for i := range l.Entries {
		if l.Entries[i].ID == id {
			return &l.Entries[i]
		}
	}
	return nil
}

func TestLockHashIsSHA256(t *testing.T) {
	// covers: R-SYNC-087, R-SYNC-010
	root := openRoot(t)
	syncOnce(t, root, file("a.txt", "hello\n"), blockEntry("inside"))
	h := sha256.Sum256([]byte("hello\n"))
	if e := lockEntry(t, root, "a.txt"); e == nil || e.Hash != hex.EncodeToString(h[:]) || e.Owner != "p" {
		t.Fatalf("lock entry %+v", e)
	}
	hb := sha256.Sum256([]byte("inside\n"))
	if e := lockEntry(t, root, "AGENTS.md#p/x"); e == nil || e.Hash != hex.EncodeToString(hb[:]) {
		t.Fatalf("block entry %+v", e)
	}
}

func TestOrphanOfKeptVersionIsKept(t *testing.T) {
	// covers: R-SYNC-020
	root := openRoot(t)
	if err := root.WriteFile("a.txt", []byte("mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := nsync.Compute(root, []nsync.Entry{file("a.txt", "theirs\n")}, nsync.LockMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Keep("a.txt"); err != nil {
		t.Fatal(err)
	}
	if err := nsync.Apply(root, p); err != nil {
		t.Fatal(err)
	}
	syncOnce(t, root) // a.txt no longer rendered
	if got, ok := read(t, root, "a.txt"); !ok || got != "mine\n" {
		t.Fatalf("kept file deleted or changed: %q %v", got, ok)
	}
}

func TestKeepRefusesCleanPath(t *testing.T) {
	// covers: R-SYNC-018
	root := openRoot(t)
	p, err := nsync.Compute(root, []nsync.Entry{file("a.txt", "x\n")}, nsync.LockMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Keep("a.txt"); err == nil || !strings.Contains(err.Error(), "not in conflict") {
		t.Fatalf("Keep on a clean path: %v", err)
	}
}

// FuzzSyncNeverClobbers runs random sequences of user edits, upstream changes and syncs, and
// checks after each sync that ness changed only bytes it wrote and nobody touched since.
func FuzzSyncNeverClobbers(f *testing.F) {
	// covers: R-SYNC-021, R-SYNC-004, R-SYNC-044
	f.Add([]byte{3, 0, 3, 1, 3, 4, 3, 2, 3})
	f.Add([]byte{5, 3, 6, 1, 3, 4, 1, 3, 2, 3})
	f.Fuzz(func(t *testing.T, ops []byte) {
		root := openRoot(t)
		version, rendered := 0, true
		for i, op := range ops {
			switch op % 7 {
			case 0:
				_ = root.WriteFile("a.txt", []byte(fmt.Sprintf("user %d\n", i)), 0o600)
			case 1:
				version++
			case 2:
				rendered = !rendered
			case 3:
				checkedSync(t, root, version, rendered)
			case 4:
				if p, err := nsync.Compute(root, desired(version, rendered), nsync.LockMeta{}); err == nil && p.Keep("a.txt") == nil {
					_ = nsync.Apply(root, p)
				}
			case 5:
				old, _ := read(t, root, "AGENTS.md")
				_ = root.WriteFile("AGENTS.md", []byte(fmt.Sprintf("line %d\n%s", i, old)), 0o600)
			case 6:
				old, _ := read(t, root, "AGENTS.md")
				_ = root.WriteFile("AGENTS.md", []byte(strings.Replace(old, "\nb", "\nuser b", 1)), 0o600)
			}
		}
	})
}

func desired(version int, rendered bool) []nsync.Entry {
	if !rendered {
		return nil
	}
	return []nsync.Entry{file("a.txt", fmt.Sprintf("v%d\n", version)), blockEntry(fmt.Sprintf("b%d\n", version))}
}

func checkedSync(t *testing.T, root *os.Root, version int, rendered bool) {
	t.Helper()
	preA, hadA := read(t, root, "a.txt")
	preAgents, _ := read(t, root, "AGENTS.md")
	recA := lockEntry(t, root, "a.txt")
	syncOnce(t, root, desired(version, rendered)...)
	postA, hasA := read(t, root, "a.txt")
	if hadA && (!hasA || postA != preA) && !nessWrote(recA, preA) {
		t.Fatalf("a.txt %q was changed to %q (present %v) although ness had not written it; lock %+v", preA, postA, hasA, recA)
	}
	postAgents, _ := read(t, root, "AGENTS.md")
	if outside(preAgents) != outside(postAgents) {
		t.Fatalf("text outside the block changed:\n%q\n%q", preAgents, postAgents)
	}
}

func nessWrote(rec *nsync.LockEntry, content string) bool {
	h := sha256.Sum256([]byte(content))
	return rec != nil && rec.Hash == rec.Upstream && rec.Hash == hex.EncodeToString(h[:])
}

// outside strips the p/x block (markers included) and the blank line sync puts before it.
func outside(s string) string {
	const endMarker = "<!-- ness:end p/x -->\n"
	begin, end := strings.Index(s, "<!-- ness:begin p/x -->"), strings.Index(s, endMarker)
	if begin >= 0 && end > begin {
		head := strings.TrimRight(s[:begin], "\n")
		if head != "" {
			head += "\n"
		}
		s = head + s[end+len(endMarker):]
	}
	return strings.TrimRight(s, "\n")
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/sync/ -run 'Lock|Orphan|Keep'`
Expected: FAIL to build (`undefined: nsync.Compute`).

- [ ] **Step 3: Implement**

`internal/sync/engine.go`:
```go
package sync

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
)

// Action is what sync does to one target.
type Action string

// Actions.
const (
	ActNoop     Action = "noop"
	ActCreate   Action = "create"
	ActAdopt    Action = "adopt"
	ActUpdate   Action = "update"
	ActKeep     Action = "keep"     // the user's version stays; upstream is unchanged
	ActConflict Action = "conflict" // held back; the render goes to <path>.ness-new
	ActDelete   Action = "delete"
	ActOrphan   Action = "orphan" // no longer rendered and changed by the user: kept, released
	ActDrop     Action = "drop"   // no longer rendered, nothing to remove: released
)

const (
	absent   = "absent"
	notALink = "\x00not a symlink"
	newExt   = ".ness-new"
)

// Step is the planned action for one target.
type Step struct {
	ID      string
	Action  Action
	want    *Entry
	rec     *LockEntry
	cur     []byte
	present bool
}

// FilePlan is the plan for one path. A conflict in any step holds back the whole file (R-SYNC-045,
// R-SYNC-051); an unreadable or unparsable file is left alone (R-SYNC-046, R-SYNC-055).
type FilePlan struct {
	Path     string
	Steps    []Step
	Before   []byte // current bytes, or the link target for links
	After    []byte // bytes after sync; for a conflict, the render written to <path>.ness-new
	Exists   bool
	Gone     bool
	Link     bool
	Conflict bool
	Kept     bool
	Err      error
}

// LockMeta is what the lock records besides its entries.
type LockMeta struct {
	Ness     string
	Profiles map[string]string
}

// Plan is a full sync plan; nothing is written until Apply.
type Plan struct {
	Files   []FilePlan
	meta    LockMeta
	oldLock []byte
}

// Compute plans a sync of entries against the repository and its lock without writing.
func Compute(root *os.Root, entries []Entry, meta LockMeta) (*Plan, error) {
	lock, err := readLock(root)
	if err != nil {
		return nil, err
	}
	oldLock, err := root.ReadFile(LockPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	want := map[string][]Entry{}
	for _, e := range entries {
		e.ID = entryID(e)
		if e.Kind == KindBlock {
			e.Content = normInner(e.Content)
		}
		want[e.Path] = append(want[e.Path], e)
	}
	recs := map[string][]LockEntry{}
	for _, r := range lock.Entries {
		recs[r.Path] = append(recs[r.Path], r)
	}
	paths := slices.Sorted(maps.Keys(want))
	for p := range recs {
		if _, ok := want[p]; !ok {
			paths = append(paths, p)
		}
	}
	slices.Sort(paths)
	plan := &Plan{meta: meta, oldLock: oldLock}
	for _, p := range paths {
		plan.Files = append(plan.Files, planFile(root, p, want[p], recs[p]))
	}
	return plan, nil
}

func planFile(root *os.Root, p string, want []Entry, recs []LockEntry) FilePlan {
	fp := FilePlan{Path: p}
	fp.Link = slices.ContainsFunc(want, func(e Entry) bool { return e.Kind == KindLink }) ||
		(len(want) == 0 && slices.ContainsFunc(recs, func(r LockEntry) bool { return r.Kind == KindLink }))
	if err := fp.read(root); err != nil {
		fp.fail(err, recs)
		return fp
	}
	whole := slices.ContainsFunc(want, isWholeEntry) ||
		(len(want) == 0 && slices.ContainsFunc(recs, func(r LockEntry) bool { return isWhole(r.Kind) }))
	if whole {
		fp.planWhole(want, recs)
	} else {
		fp.planParts(want, recs)
	}
	return fp
}

func isWhole(k Kind) bool          { return k == KindFile || k == KindSeed || k == KindLink }
func isWholeEntry(e Entry) bool    { return isWhole(e.Kind) }
func (fp *FilePlan) held() bool    { return fp.Conflict || fp.Err != nil }
func state(cur []byte, ok bool) string {
	if !ok {
		return absent
	}
	return sum(cur)
}

func (fp *FilePlan) fail(err error, recs []LockEntry) {
	fp.Err = fmt.Errorf("%s: %w", fp.Path, err)
	for i := range recs {
		fp.Steps = append(fp.Steps, Step{ID: recs[i].ID, Action: ActNoop, rec: &recs[i]})
	}
}

func (fp *FilePlan) read(root *os.Root) error {
	info, err := root.Lstat(fp.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	fp.Exists = true
	isLink := info.Mode()&fs.ModeSymlink != 0
	switch {
	case fp.Link && isLink:
		target, err := root.Readlink(fp.Path)
		fp.Before = []byte(target)
		return err
	case fp.Link:
		fp.Before = []byte(notALink) // a real file or directory where ness wants a link (R-SYNC-080)
		return nil
	case info.IsDir():
		return errors.New("is a directory")
	}
	fp.Before, err = root.ReadFile(fp.Path)
	return err
}
```

Continue `engine.go` with the decisions:
```go
// decide applies sync rules 2–4 (design §6) to one rendered target.
func decide(cur []byte, present bool, rec *LockEntry, want []byte) Action {
	w, c := sum(want), state(cur, present)
	switch {
	case c == w:
		if rec != nil && rec.Hash == w && rec.Upstream == w {
			return ActNoop
		}
		return ActAdopt
	case rec == nil && !present:
		return ActCreate
	case rec == nil:
		return ActConflict
	}
	return decideRecorded(c, present, rec, w)
}

func decideRecorded(c string, present bool, rec *LockEntry, w string) Action {
	switch {
	case c == rec.Hash && rec.Hash == rec.Upstream:
		return ActUpdate
	case c == rec.Hash && rec.Upstream == w:
		return ActNoop // the user's kept version; upstream unchanged since
	case c == rec.Hash:
		return ActConflict
	case !present:
		return ActCreate
	case rec.Upstream == w:
		return ActKeep
	default:
		return ActConflict
	}
}

func decideSeed(present bool, rec *LockEntry) Action {
	switch {
	case rec != nil:
		return ActNoop
	case present:
		return ActAdopt
	default:
		return ActCreate
	}
}

// orphanAction applies rule 5: only bytes ness wrote and nobody changed are deleted.
func orphanAction(rec *LockEntry, cur []byte, present bool) Action {
	switch {
	case rec.Kind == KindSeed || !present:
		return ActDrop
	case rec.Hash == rec.Upstream && sum(cur) == rec.Hash:
		return ActDelete
	default:
		return ActOrphan
	}
}

func (fp *FilePlan) planWhole(want []Entry, recs []LockEntry) {
	var rec *LockEntry
	for i := range recs {
		if isWhole(recs[i].Kind) && rec == nil {
			rec = &recs[i]
			continue
		}
		fp.Steps = append(fp.Steps, Step{ID: recs[i].ID, Action: ActDrop, rec: &recs[i]})
	}
	fp.After = fp.Before
	if len(want) == 0 {
		a := orphanAction(rec, fp.Before, fp.Exists)
		fp.Steps = append(fp.Steps, Step{ID: rec.ID, Action: a, rec: rec, cur: fp.Before, present: fp.Exists})
		fp.Gone = a == ActDelete
		return
	}
	w := &want[0]
	a := decide(fp.Before, fp.Exists, rec, w.Content)
	if w.Kind == KindSeed {
		a = decideSeed(fp.Exists, rec)
	}
	fp.Steps = append(fp.Steps, Step{ID: w.ID, Action: a, want: w, rec: rec, cur: fp.Before, present: fp.Exists})
	switch a {
	case ActCreate, ActUpdate:
		fp.After = w.Content
	case ActConflict:
		fp.Conflict, fp.After = true, w.Content
	}
}
```

Then the parts (blocks and keys):
```go
type partOps struct {
	get    func(data []byte, name string) ([]byte, bool, error)
	set    func(data []byte, name string, v []byte) ([]byte, error)
	remove func(data []byte, name string) ([]byte, error)
}

func opsFor(p string) partOps {
	if strings.HasSuffix(p, ".json") {
		return partOps{get: jsonGet, set: jsonSet, remove: jsonDelete}
	}
	s := styleFor(p)
	return partOps{
		get:    func(d []byte, n string) ([]byte, bool, error) { return getBlock(d, s, n) },
		set:    func(d []byte, n string, v []byte) ([]byte, error) { return setBlock(d, s, n, v) },
		remove: func(d []byte, n string) ([]byte, error) { return removeBlock(d, s, n) },
	}
}

// partName recovers a part's block name or key from its lock id.
func partName(r LockEntry) string {
	if r.Kind == KindBlock {
		return strings.TrimPrefix(r.ID, r.Path+"#")
	}
	return strings.TrimPrefix(r.ID, r.Path+":")
}

func (fp *FilePlan) planParts(want []Entry, recs []LockEntry) {
	ops := opsFor(fp.Path)
	data, forced := fp.Before, fp.Before
	byID := map[string]*LockEntry{}
	for i := range recs {
		byID[recs[i].ID] = &recs[i]
	}
	var err error
	for i := range want {
		w := &want[i]
		cur, present, gerr := ops.get(fp.Before, w.Name)
		if gerr != nil {
			fp.fail(gerr, recs)
			return
		}
		a := decide(cur, present, byID[w.ID], w.Content)
		fp.Steps = append(fp.Steps, Step{ID: w.ID, Action: a, want: w, rec: byID[w.ID], cur: cur, present: present})
		delete(byID, w.ID)
		fp.Conflict = fp.Conflict || a == ActConflict
		if forced, err = ops.set(forced, w.Name, w.Content); err == nil && (a == ActCreate || a == ActUpdate) {
			data, err = ops.set(data, w.Name, w.Content)
		}
		if err != nil {
			fp.fail(err, recs)
			return
		}
	}
	for _, id := range slices.Sorted(maps.Keys(byID)) {
		if data, forced, err = fp.planOrphanPart(ops, byID[id], data, forced); err != nil {
			fp.fail(err, recs)
			return
		}
	}
	if len(bytes.TrimSpace(fp.Before)) == 0 && strings.HasSuffix(fp.Path, ".json") {
		data, forced = indentJSON(data), indentJSON(forced)
	}
	fp.After = data
	if fp.Conflict {
		fp.After = forced
	}
}

func (fp *FilePlan) planOrphanPart(ops partOps, rec *LockEntry, data, forced []byte) ([]byte, []byte, error) {
	name := partName(*rec)
	cur, present, err := ops.get(fp.Before, name)
	if err != nil {
		return nil, nil, err
	}
	a := orphanAction(rec, cur, present)
	fp.Steps = append(fp.Steps, Step{ID: rec.ID, Action: a, rec: rec, cur: cur, present: present})
	if a != ActDelete {
		return data, forced, nil
	}
	if data, err = ops.remove(data, name); err != nil {
		return nil, nil, err
	}
	forced, err = ops.remove(forced, name)
	return data, forced, err
}
```

Then the plan API, the lock it produces, and `Apply`:
```go
// Keep records the user's current version of a conflicted path (sync --keep, R-SYNC-017).
func (p *Plan) Keep(target string) error {
	for i := range p.Files {
		fp := &p.Files[i]
		if fp.Path == target && fp.Conflict {
			fp.Conflict, fp.Kept, fp.After, fp.Gone = false, true, fp.Before, false
			return nil
		}
	}
	return fmt.Errorf("%s is not in conflict; --keep takes a path ness sync reported as a conflict", target)
}

// Conflicts lists the conflicted paths.
func (p *Plan) Conflicts() []string {
	var out []string
	for _, fp := range p.Files {
		if fp.Conflict {
			out = append(out, fp.Path)
		}
	}
	return out
}

// Errors lists the files sync leaves alone because it cannot read or parse them.
func (p *Plan) Errors() []error {
	var out []error
	for _, fp := range p.Files {
		if fp.Err != nil {
			out = append(out, fp.Err)
		}
	}
	return out
}

// Changed reports whether Apply would write any file or the lock (for sync --check).
func (p *Plan) Changed() (bool, error) {
	for _, fp := range p.Files {
		if fp.Writes() {
			return true, nil
		}
	}
	lock, err := p.LockBytes()
	return !bytes.Equal(lock, p.oldLock), err
}

// Writes reports whether Apply changes this path (the .ness-new file of a conflict aside).
func (fp *FilePlan) Writes() bool {
	switch {
	case fp.held():
		return false
	case fp.Gone:
		return true
	case !fp.Exists:
		return fp.After != nil
	default:
		return !bytes.Equal(fp.After, fp.Before)
	}
}

// LockBytes renders the lock Apply writes.
func (p *Plan) LockBytes() ([]byte, error) {
	l := Lock{Schema: 1, Ness: p.meta.Ness, Profiles: p.meta.Profiles}
	for i := range p.Files {
		for _, s := range p.Files[i].Steps {
			if e, ok := p.Files[i].record(s); ok {
				l.Entries = append(l.Entries, e)
			}
		}
	}
	return l.Marshal()
}

func (fp *FilePlan) record(s Step) (LockEntry, bool) {
	switch {
	case fp.Kept && s.want != nil:
		return entryFor(*s.want, state(s.cur, s.present), sum(s.want.Content)), true
	case fp.held() || s.Action == ActNoop || s.Action == ActKeep:
		if s.rec == nil {
			return LockEntry{}, false
		}
		return *s.rec, true
	case s.want == nil:
		return LockEntry{}, false
	case s.want.Kind == KindSeed && s.Action == ActAdopt:
		return entryFor(*s.want, sum(s.cur), sum(s.want.Content)), true
	default:
		w := sum(s.want.Content)
		return entryFor(*s.want, w, w), true
	}
}

func entryFor(e Entry, hash, upstream string) LockEntry {
	return LockEntry{ID: e.ID, Path: e.Path, Kind: e.Kind, Owner: e.Owner, Hash: hash, Upstream: upstream}
}

// Apply writes the plan: each file without conflict or error, <path>.ness-new for each
// conflict, and the lock.
func Apply(root *os.Root, p *Plan) error {
	for i := range p.Files {
		if err := p.Files[i].apply(root); err != nil {
			return fmt.Errorf("%s: %w", p.Files[i].Path, err)
		}
	}
	lock, err := p.LockBytes()
	if err != nil || bytes.Equal(lock, p.oldLock) {
		return err
	}
	return writeFile(root, LockPath, lock)
}

func (fp *FilePlan) apply(root *os.Root) error {
	switch {
	case fp.Err != nil:
		return nil
	case fp.Conflict && fp.Link:
		return nil
	case fp.Conflict:
		return writeFile(root, fp.Path+newExt, fp.After)
	}
	if err := root.Remove(fp.Path + newExt); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	switch {
	case !fp.Writes():
		return nil
	case fp.Gone:
		return root.Remove(fp.Path)
	case fp.Link:
		if fp.Exists {
			if err := root.Remove(fp.Path); err != nil {
				return err
			}
		}
		if err := root.MkdirAll(path.Dir(fp.Path), 0o750); err != nil {
			return err
		}
		return root.Symlink(string(fp.After), fp.Path)
	}
	return writeFile(root, fp.Path, fp.After)
}

func writeFile(root *os.Root, p string, data []byte) error {
	if err := root.MkdirAll(path.Dir(p), 0o750); err != nil {
		return err
	}
	return root.WriteFile(p, data, 0o644) //nolint:gosec // repository files are world-readable by convention
}
```

- [ ] **Step 4: Run the tests and the state-machine fuzzer**

Run: `go test ./internal/sync/ && go test -run '^$' -fuzz FuzzSyncNeverClobbers -fuzztime 60s ./internal/sync/`
Expected: PASS. A fuzzer failure is a real violation of D4. Keep the failing input as a seed and fix the engine. Never weaken the invariant.

- [ ] **Step 5: Commit**

```bash
git add internal/sync
git commit -m "feat(sync): plan and apply with the lock, conflicts held per file, keep and orphans"
```

### Task 13: `ness sync`

**Files:**
- Create: `internal/cli/sync.go` and these scripts under `internal/cli/testdata/script/`: `sync_create.txtar`, `sync_adopt_conflict.txtar`, `sync_upgrade.txtar`, `sync_orphan.txtar`, `sync_blocks.txtar`, `sync_json.txtar`, `sync_json_conflict.txtar`, `sync_links.txtar`, `sync_seed.txtar`, `sync_flags.txtar`, `sync_scope.txtar`, `sync_tools.txtar`, `profiles_conflicts.txtar`
- Modify: `internal/cli/cli.go` (register `newSyncCmd(d)` and delete the temporary `check-config` command); `config_errors.txtar` and `pin.txtar` (replace `check-config` with `sync --check`. A valid pinned project with no lock gives exit 1 there, so those lines use `exits 1 ness sync --check` where they expect success)

**Interfaces:**
- Consumes: everything above.
- Produces:
  - `planSync(d Deps, p *project, root *os.Root) (*nsync.Plan, error)`
  - `report(d Deps, plan *nsync.Plan)`
  - `syncExit(plan *nsync.Plan, check bool) error`
  - `init` (Task 14) uses all three.

- [ ] **Step 1: Write the failing scripts**

`sync_create.txtar`:
```
# A first sync creates and records the core targets; repeating it changes nothing, and so does
# re-rendering from scratch.
# covers: R-SYNC-010, R-SYNC-002, R-SYNC-062, R-SYNC-065, R-SYNC-069, R-SYNC-030
cd repo
exec ness sync
stdout '^create AGENTS.md#core/router$'
stdout '^create CLAUDE.md$'
exists .agents/skills/ness-entry/SKILL.md
exists .claude/agents/ness-sweep.md
islink CLAUDE.md AGENTS.md
islink .claude/skills ../.agents/skills
grep '"autoCompactWindow": 265000' .claude/settings.json
grep 'command -v ness >/dev/null && ness hook session-start \|\| true' .claude/settings.json
exec git check-ignore -q .worktrees/x
exec git check-ignore -q foo.ness-new
grep '"schema": 1' .ness/lock.json
exec ness sync --check
! stdout .
snap first
exec ness sync
! stdout .
samesnap first
rm AGENTS.md CLAUDE.md .claude .agents .gitignore .ness/lock.json
exec ness sync
samesnap first

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
```

`sync_adopt_conflict.txtar`:
```
# Unrecorded targets: identical content is adopted; different content is a conflict that keeps
# the user's bytes, writes the render beside them, and does not stop the rest of the sync.
# covers: R-SYNC-011, R-SYNC-012, R-SYNC-016, R-SYNC-022, R-SYNC-024
cd repo
exits 3 ness sync
stdout '^adopt docs/x.md$'
stdout '^conflict docs/y.md: merge docs/y.md.ness-new'
stdout '^conflict docs/z.md: merge docs/z.md.ness-new'
stdout '^create AGENTS.md#core/router$'
cmp docs/y.md ../mine.txt
cmp docs/y.md.ness-new .ness/profiles/local/y.md
grep '"id": "docs/x.md"' .ness/lock.json
! grep '"id": "docs/y.md"' .ness/lock.json
# A pending conflict keeps failing until it is resolved.
exits 3 ness sync
exits 3 ness sync --check

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
profiles = ["local"]
-- repo/.ness/profiles/local/profile.toml --
name = "local"
[files]
"docs/x.md" = { own = "x.md" }
"docs/y.md" = { own = "y.md" }
"docs/z.md" = { own = "z.md" }
-- repo/.ness/profiles/local/x.md --
x
-- repo/.ness/profiles/local/y.md --
y
-- repo/.ness/profiles/local/z.md --
z
-- repo/docs/x.md --
x
-- repo/docs/y.md --
mine
-- repo/docs/z.md --
mine
-- mine.txt --
mine
```

`sync_upgrade.txtar`:
```
# Rule 3 updates untouched files; rule 4 never overwrites a user's edit; --keep settles a
# conflict until upstream changes again.
# covers: R-SYNC-013, R-SYNC-014, R-SYNC-015, R-SYNC-017, R-SYNC-018, R-PROF-003
cd repo
exec ness sync
cmp docs/x.md ../x1.md
cp ../x2.md .ness/profiles/local/x.md
exec ness sync
stdout '^update docs/x.md$'
cmp docs/x.md ../x2.md
# The user edits; upstream unchanged: left alone.
cp ../mine.md docs/x.md
exec ness sync
! stdout .
cmp docs/x.md ../mine.md
# Upstream changes too: conflict, the user's bytes stay.
cp ../x3.md .ness/profiles/local/x.md
exits 3 ness sync
stdout '^conflict docs/x.md'
cmp docs/x.md ../mine.md
cmp docs/x.md.ness-new ../x3.md
# --keep on a clean path is refused and changes nothing.
cp .ness/lock.json ../lock.before
exits 4 ness sync --keep docs/other.md
stderr 'not in conflict'
cmp .ness/lock.json ../lock.before
exec ness sync --keep docs/x.md
! exists docs/x.md.ness-new
exec ness sync
! stdout .
cmp docs/x.md ../mine.md
cp ../x2.md .ness/profiles/local/x.md
exits 3 ness sync
exists docs/x.md.ness-new

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
profiles = ["local"]
-- repo/.ness/profiles/local/profile.toml --
name = "local"
[files]
"docs/x.md" = { own = "x.md" }
"docs/other.md" = { own = "other.md" }
-- repo/.ness/profiles/local/x.md --
one
-- repo/.ness/profiles/local/other.md --
other
-- x1.md --
one
-- x2.md --
two
-- x3.md --
three
-- mine.md --
mine
```

`sync_orphan.txtar`:
```
# Rule 5: a target no longer rendered is deleted if untouched, kept and released if edited.
# covers: R-SYNC-019, R-SYNC-020, R-PROF-010
cd repo
exec ness sync
grep '"owner": "local"' .ness/lock.json
cp ../mine.md docs/b.md
cp ../none.toml .ness/ness.toml
exec ness sync
stdout '^delete docs/a.md$'
stdout '^orphaned docs/b.md'
! exists docs/a.md
cmp docs/b.md ../mine.md
! grep 'docs/' .ness/lock.json
cp ../later.md docs/b.md
exec ness sync
! stdout .
cmp docs/b.md ../later.md

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
profiles = ["local"]
-- none.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
-- repo/.ness/profiles/local/profile.toml --
name = "local"
[files]
"docs/a.md" = { own = "a.md" }
"docs/b.md" = { own = "b.md" }
-- repo/.ness/profiles/local/a.md --
a
-- repo/.ness/profiles/local/b.md --
b
-- mine.md --
mine
-- later.md --
later
```

`sync_blocks.txtar`:
```
# Blocks are appended after the user's text, found and updated wherever moved, conflict at file
# granularity, and malformed markers leave the file alone while other files still sync.
# covers: R-SYNC-041, R-SYNC-042, R-SYNC-045, R-SYNC-046, R-SYNC-060, R-PROF-014
cd repo
exec ness sync
grep '\A# Mine\n\nhello\n\n<!-- ness:begin core/router -->\n' AGENTS.md
grep 'Rules v1\.\n<!-- ness:end local/rules -->\n\z' AGENTS.md
grep -count=2 'ness:begin' AGENTS.md
# Moved to the top and updated in place.
cp ../moved.md AGENTS.md
cp ../rules2.md .ness/profiles/local/rules.md
exec ness sync
stdout '^update AGENTS.md#local/rules$'
cmp AGENTS.md ../moved-v2.md
# An edit inside a block whose upstream changed: the file is untouched, the render goes beside it.
cp ../edited.md AGENTS.md
cp ../rules3.md .ness/profiles/local/rules.md
exits 3 ness sync
cmp AGENTS.md ../edited.md
exists AGENTS.md.ness-new
grep 'Rules v3\.' AGENTS.md.ness-new
exec ness sync --keep AGENTS.md
# Malformed markers: AGENTS.md is left alone, docs/x.md still syncs.
cp ../broken.md AGENTS.md
cp ../x2.md .ness/profiles/local/x.md
exits 1 ness sync
stderr 'AGENTS.md: line 1: block "core/router" has no end marker'
cmp AGENTS.md ../broken.md
cmp docs/x.md ../x2.md

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
profiles = ["local"]
-- repo/.ness/profiles/local/profile.toml --
name = "local"
[slots]
"AGENTS.md#rules" = { block = "rules.md" }
[files]
"docs/x.md" = { own = "x.md" }
-- repo/.ness/profiles/local/rules.md --
Rules v1.
-- repo/.ness/profiles/local/x.md --
x1
-- repo/AGENTS.md --
# Mine

hello
-- rules2.md --
Rules v2.
-- rules3.md --
Rules v3.
-- x2.md --
x2
-- moved.md --
<!-- ness:begin core/router -->
## Sessions (ness)
Applies only when the `ness` command exists in this checkout. Otherwise ignore this block.

This repository runs the ness harness. Start every session by loading the `ness-entry` skill.
<!-- ness:end core/router -->
# Mine

hello

<!-- ness:begin local/rules -->
Rules v1.
<!-- ness:end local/rules -->
-- moved-v2.md --
<!-- ness:begin core/router -->
## Sessions (ness)
Applies only when the `ness` command exists in this checkout. Otherwise ignore this block.

This repository runs the ness harness. Start every session by loading the `ness-entry` skill.
<!-- ness:end core/router -->
# Mine

hello

<!-- ness:begin local/rules -->
Rules v2.
<!-- ness:end local/rules -->
-- edited.md --
<!-- ness:begin core/router -->
## Sessions (ness)
Applies only when the `ness` command exists in this checkout. Otherwise ignore this block.

This repository runs the ness harness. Start every session by loading the `ness-entry` skill.
<!-- ness:end core/router -->
# Mine

hello

<!-- ness:begin local/rules -->
Rules v2, with my own line.
<!-- ness:end local/rules -->
-- broken.md --
<!-- ness:begin core/router -->
text
```

`sync_json.txtar`:
```
# settings.json keeps every unowned byte; an equal unrecorded key is adopted; ness hook entries
# are found by command, never duplicated; the lock lists exactly the owned key paths; the
# wrapped hook command is silent without ness.
# covers: R-SYNC-050, R-SYNC-051, R-SYNC-053, R-SYNC-054, R-SYNC-067
cd repo
exec ness sync
stdout '^adopt \.claude/settings\.json:autoCompactWindow$'
grep '\A\{\n    "model":   "x",\n    "autoCompactWindow": 265000,' .claude/settings.json
grep 'echo mine' .claude/settings.json
grep -count=1 'ness hook session-start' .claude/settings.json
exec ness sync
grep -count=1 'ness hook session-start' .claude/settings.json
grep -count=4 '"id": "\.claude/settings\.json:' .ness/lock.json
grep '"id": "\.claude/settings\.json:env\.CLAUDE_CODE_SUBAGENT_MODEL"' .ness/lock.json
mkdir ../emptybin
env PATH=$WORK/emptybin
exec /bin/sh -c 'command -v ness >/dev/null && ness hook session-start || true'
! stdout .
! stderr .

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
-- repo/.claude/settings.json --
{
    "model":   "x",
    "autoCompactWindow": 265000,
  "hooks": {"SessionStart": [{"hooks": [{"type": "command", "command": "echo mine"}]}]}
}
```

`sync_json_conflict.txtar`:
```
# A differing unrecorded key is a conflict for the whole file; other files still sync.
# covers: R-SYNC-051, R-SYNC-052, R-SYNC-055
cd repo
exits 3 ness sync
stdout '^conflict \.claude/settings\.json'
grep '"autoCompactWindow": 100000' .claude/settings.json
grep '"autoCompactWindow":265000' .claude/settings.json.ness-new
exists AGENTS.md
cp ../invalid.json .claude/settings.json
exits 1 ness sync
stderr '\.claude/settings\.json: not a valid JSON object'
cmp .claude/settings.json ../invalid.json

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
-- repo/.claude/settings.json --
{"autoCompactWindow": 100000}
-- invalid.json --
{"autoCompactWindow":
```

`sync_links.txtar`:
```
# A user's CLAUDE.md gets an import block; a real .claude/skills directory is a conflict left
# untouched; project skills beside ness-* are never touched.
# covers: R-SYNC-063, R-SYNC-064, R-SYNC-080
cd repo
exits 3 ness sync
stdout '^conflict \.claude/skills'
cmp .claude/skills/mine.txt ../mine.txt
! exists .claude/skills.ness-new
grep '\A# My claude notes\n' CLAUDE.md
grep '<!-- ness:begin core/import -->\n@AGENTS.md\n' CLAUDE.md
cmp .agents/skills/deploy/SKILL.md ../deploy.md

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
-- repo/CLAUDE.md --
# My claude notes
-- repo/.claude/skills/mine.txt --
mine
-- repo/.agents/skills/deploy/SKILL.md --
deploy
-- mine.txt --
mine
-- deploy.md --
deploy
```

`sync_seed.txtar`:
```
# A seed is created once, then belongs to the user: edits and deletion are respected.
# covers: R-SYNC-070, R-SYNC-081, R-PROF-011
cd repo
exec ness sync
cmp docs/ops.md .ness/profiles/local/ops.md
cp ../mine.md docs/ops.md
cp ../ops2.md .ness/profiles/local/ops.md
exec ness sync
cmp docs/ops.md ../mine.md
rm docs/ops.md
exec ness sync
! exists docs/ops.md

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
profiles = ["local"]
-- repo/.ness/profiles/local/profile.toml --
name = "local"
[files]
"docs/ops.md" = { seed = "ops.md" }
-- repo/.ness/profiles/local/ops.md --
ops v1
-- ops2.md --
ops v2
-- mine.md --
mine
```

`sync_flags.txtar`:
```
# --check and --dry-run write nothing; their exit codes are 0 clean, 1 changes, 3 conflict.
# covers: R-SYNC-030, R-SYNC-031, R-SYNC-034, R-CFG-038
cd repo
snap before
exits 1 ness sync --check
stdout '^create AGENTS.md#core/router$'
samesnap before
exec ness sync --dry-run
stdout '^create AGENTS.md#core/router$'
stdout '^@@ '
samesnap before
exec ness sync --dry-run --json
stdout '"schema": 1'
stdout '"action": "create"'
samesnap before
exits 2 ness sync --json
exec ness sync
exec ness sync --check
cp ../x2.md .ness/profiles/local/x.md
exits 1 ness sync --check
cp ../mine.md docs/x.md
exits 3 ness sync --check
exits 3 ness sync --dry-run
! exists docs/x.md.ness-new

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
profiles = ["local"]
-- repo/.ness/profiles/local/profile.toml --
name = "local"
[files]
"docs/x.md" = { own = "x.md" }
-- repo/.ness/profiles/local/x.md --
x1
-- x2.md --
x2
-- mine.md --
mine
```

`sync_scope.txtar`:
```
# Sync writes exactly the planned paths and the lock, nothing outside the repository, no feature
# text unless enabled, and ignores per-user config trees.
# covers: R-SYNC-003, R-SYNC-006, R-SYNC-072, R-CFG-001, R-CFG-006
cd repo
mkdir $HOME/.config/ness $WORK/xdg/ness
cp ../evil.toml $HOME/.config/ness/ness.toml
cp ../evil.toml $WORK/xdg/ness/ness.toml
env XDG_CONFIG_HOME=$WORK/xdg
snap home $HOME
exec ness sync
samesnap home $HOME
exec git status --porcelain --untracked-files=all
cmp stdout ../status.txt
grep '\A# ness:begin core/ignore\n\.worktrees/\n\*\.ness-new\n# ness:end core/ignore\n\z' .gitignore

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
-- evil.toml --
profiles = ["nope"]
client = "codex"
-- status.txt --
?? .agents/skills/ness-entry/SKILL.md
?? .claude/agents/ness-design.md
?? .claude/agents/ness-review.md
?? .claude/agents/ness-sweep.md
?? .claude/agents/ness-verify.md
?? .claude/settings.json
?? .claude/skills
?? .gitignore
?? .ness/lock.json
?? .ness/ness.toml
?? AGENTS.md
?? CLAUDE.md
```

`sync_tools.txtar`:
```
# A tool a profile requires must be on PATH before anything is written.
# covers: R-SYNC-005, R-PROF-016, R-CFG-036
cd repo
snap before
exits 4 ness sync
stderr 'profile local requires nonexistent-tool-xyz, which is not on PATH'
samesnap before

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
profiles = ["local"]
-- repo/.ness/profiles/local/profile.toml --
name = "local"
requires_tools = ["nonexistent-tool-xyz"]
```

`profiles_conflicts.txtar`:
```
# Profile conflicts are refused, all named, before any file is written.
# covers: R-PROF-031, R-PROF-032, R-PROF-033, R-PROF-035, R-PROF-036, R-PROF-037, R-PROF-040, R-PROF-044, R-PROF-006
cd repo
snap before
exits 4 ness sync
stderr 'profiles a, b all set single slot ness-entry#x'
stderr 'profiles a and b set seam integrator differently'
stderr 'profiles a and b set integrate.push differently'
stderr 'profiles a and b both own docs/z.md'
stderr 'profiles c and a conflict'
stderr 'profile c requires profile missing'
samesnap before
cp ../mismatch.toml .ness/ness.toml
exits 4 ness sync
stderr 'name "f" differs from its directory "e"'
cp ../missingfile.toml .ness/ness.toml
exits 4 ness sync
stderr 'profile g: .*nope\.md, which is missing'
samesnap before

-- repo/.ness/ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
profiles = ["a", "b", "c"]
-- mismatch.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
profiles = ["e"]
-- missingfile.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
profiles = ["g"]
-- repo/.ness/profiles/a/profile.toml --
name = "a"
[slots]
"ness-entry#x" = { set = "t.md" }
[files]
"docs/z.md" = { own = "t.md" }
[config]
"integrate.push" = true
[atoms]
integrator = "squash"
-- repo/.ness/profiles/a/t.md --
a
-- repo/.ness/profiles/b/profile.toml --
name = "b"
[slots]
"ness-entry#x" = { set = "t.md" }
[files]
"docs/z.md" = { own = "t.md" }
[config]
"integrate.push" = false
[atoms]
integrator = "pr"
-- repo/.ness/profiles/b/t.md --
b
-- repo/.ness/profiles/c/profile.toml --
name = "c"
conflicts = ["a"]
requires = ["missing"]
-- repo/.ness/profiles/e/profile.toml --
name = "f"
-- repo/.ness/profiles/g/profile.toml --
name = "g"
[files]
"docs/g.md" = { own = "nope.md" }
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/`
Expected: FAIL. `ness sync` is an unknown command (exit 2).

- [ ] **Step 3: Implement**

`internal/cli/sync.go`:
```go
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/aymanbagabas/go-udiff"
	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/profile"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
	"github.com/enea-scaccabarozzi/ness/profiles"
)

type syncOpts struct {
	check, dryRun, asJSON bool
	keep                  []string
}

func newSyncCmd(d Deps) *cobra.Command {
	var o syncOpts
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Render core, features and profiles into this repository",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if o.asJSON && !o.dryRun {
				return &exitError{code: ExitUsage, err: errors.New("--json needs --dry-run")}
			}
			p, err := loadProject(d)
			if err != nil {
				return err
			}
			return runSync(d, p, o)
		},
	}
	f := cmd.Flags()
	f.BoolVar(&o.check, "check", false, "write nothing; exit 1 if anything would change, 3 on conflict")
	f.BoolVar(&o.dryRun, "dry-run", false, "write nothing; print the plan and a diff per changed file")
	f.BoolVar(&o.asJSON, "json", false, "with --dry-run, print the plan as JSON")
	f.StringArrayVar(&o.keep, "keep", nil, "keep your version of conflicted `PATH` until upstream changes again")
	cmd.MarkFlagsMutuallyExclusive("check", "dry-run")
	return cmd
}

func runSync(d Deps, p *project, o syncOpts) error {
	root, err := os.OpenRoot(p.root)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	plan, err := planSync(d, p, root)
	if err != nil {
		return err
	}
	for _, path := range o.keep {
		if err := plan.Keep(filepath.ToSlash(path)); err != nil {
			return refuse("%v", err)
		}
	}
	switch {
	case o.check:
		report(d, plan)
		return syncExit(plan, true)
	case o.dryRun && o.asJSON:
		if err := printPlanJSON(d, plan); err != nil {
			return err
		}
		return syncExit(plan, false)
	case o.dryRun:
		report(d, plan)
		printDiffs(d, plan)
		return syncExit(plan, false)
	}
	if err := nsync.Apply(root, plan); err != nil {
		return err
	}
	report(d, plan)
	return syncExit(plan, false)
}

// planSync loads core and the configured profiles, resolves them, checks required tools and
// plans the sync. Every failure here is a refusal: nothing has been written yet.
func planSync(d Deps, p *project, root *os.Root) (*nsync.Plan, error) {
	base, err := p.file.Config(nil)
	if err != nil {
		return nil, refuse("%v", err)
	}
	core, err := profile.LoadCore(profiles.Core(), d.Version)
	if err != nil {
		return nil, err
	}
	src := profile.Source{Builtin: profiles.Builtin(), Version: d.Version}
	if dir := filepath.Join(p.root, ".ness", "profiles"); isDir(dir) {
		src.Local = os.DirFS(dir)
	}
	list := []*profile.Profile{core}
	for _, name := range base.Profiles {
		pr, err := src.Load(name)
		if err != nil {
			return nil, refuse("%v", err)
		}
		list = append(list, pr)
	}
	res, err := profile.Resolve(list, p.file.Defined)
	if err != nil {
		return nil, refuse("%v", err)
	}
	cfg, err := p.file.Config(res.Defaults)
	if err != nil {
		return nil, refuse("%v", err)
	}
	if err := checkTools(res.Tools); err != nil {
		return nil, err
	}
	entries, err := nsync.Render(res, cfg, root)
	if err != nil {
		return nil, refuse("%v", err)
	}
	plan, err := nsync.Compute(root, entries, nsync.LockMeta{Ness: d.Version, Profiles: res.Versions()})
	if err != nil {
		return nil, refuse("%v", err)
	}
	return plan, nil
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// checkTools refuses when a profile's required tool is not on PATH (R-SYNC-005).
func checkTools(tools map[string]string) error {
	for _, t := range slices.Sorted(maps.Keys(tools)) {
		if _, err := exec.LookPath(t); err != nil {
			return refuse("profile %s requires %s, which is not on PATH; install it, then run ness sync", tools[t], t)
		}
	}
	return nil
}

var verbs = map[nsync.Action]string{
	nsync.ActCreate: "create", nsync.ActUpdate: "update", nsync.ActAdopt: "adopt",
	nsync.ActDelete: "delete", nsync.ActOrphan: "orphaned",
}

// report prints one line per changed target and per conflicted path (R-SYNC-022); unreadable
// files go to stderr.
func report(d Deps, plan *nsync.Plan) {
	for _, fp := range plan.Files {
		switch {
		case fp.Err != nil:
			_, _ = fmt.Fprintf(d.Stderr, "ness: %v; left unchanged\n", fp.Err)
		case fp.Conflict && fp.Link:
			_, _ = fmt.Fprintf(d.Stdout, "conflict %s: something else is where ness keeps a symlink; move it, or run ness sync --keep %s\n", fp.Path, fp.Path)
		case fp.Conflict:
			_, _ = fmt.Fprintf(d.Stdout, "conflict %s: merge %s.ness-new into it, then run ness sync --keep %s\n", fp.Path, fp.Path, fp.Path)
		case fp.Kept:
			_, _ = fmt.Fprintf(d.Stdout, "kept %s\n", fp.Path)
		default:
			for _, s := range fp.Steps {
				if verb := verbs[s.Action]; verb == "orphaned" {
					_, _ = fmt.Fprintf(d.Stdout, "orphaned %s (no longer rendered; it is yours now)\n", s.ID)
				} else if verb != "" {
					_, _ = fmt.Fprintf(d.Stdout, "%s %s\n", verb, s.ID)
				}
			}
		}
	}
}

func printDiffs(d Deps, plan *nsync.Plan) {
	for _, fp := range plan.Files {
		if fp.Err != nil || !(fp.Conflict || fp.Writes()) {
			continue
		}
		after, label := string(fp.After), fp.Path
		if fp.Gone {
			after = ""
		}
		if fp.Conflict {
			label += ".ness-new"
		}
		_, _ = fmt.Fprint(d.Stdout, udiff.Unified(fp.Path, label, string(fp.Before), after))
	}
}

type planJSON struct {
	Files  []fileJSON `json:"files"`
	Schema int        `json:"schema"`
}

type fileJSON struct {
	Conflict bool       `json:"conflict"`
	Error    string     `json:"error,omitempty"`
	Path     string     `json:"path"`
	Steps    []stepJSON `json:"steps"`
}

type stepJSON struct {
	Action string `json:"action"`
	ID     string `json:"id"`
}

func printPlanJSON(d Deps, plan *nsync.Plan) error {
	out := planJSON{Schema: 1, Files: []fileJSON{}}
	for _, fp := range plan.Files {
		f := fileJSON{Path: fp.Path, Conflict: fp.Conflict, Steps: []stepJSON{}}
		if fp.Err != nil {
			f.Error = fp.Err.Error()
		}
		for _, s := range fp.Steps {
			f.Steps = append(f.Steps, stepJSON{ID: s.ID, Action: string(s.Action)})
		}
		out.Files = append(out.Files, f)
	}
	enc := json.NewEncoder(d.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// syncExit maps a plan to the exit code: 3 on conflict, 1 on an unreadable file or (for
// --check) any change, else 0 (R-SYNC-030, R-CFG-038).
func syncExit(plan *nsync.Plan, check bool) error {
	switch {
	case len(plan.Conflicts()) > 0:
		return exitCode(ExitConflict)
	case len(plan.Errors()) > 0:
		return exitCode(ExitProblems)
	}
	if check {
		changed, err := plan.Changed()
		if err != nil {
			return err
		}
		if changed {
			return exitCode(ExitProblems)
		}
	}
	return nil
}
```

In `cli.go`, replace the temporary `check-config` command with `root.AddCommand(newSyncCmd(d))`. In `config_errors.txtar` and `pin.txtar`, replace every `ness check-config` with `ness sync --check`. Where a line expects success, use `exits 1 ness sync --check` (nothing is synced yet, so the check reports changes).

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/cli/`
Expected: PASS. When a script fails, read the testscript log. Fix the code where the code is wrong, and fix the script only when the script misstates the requirement (then say so in the commit message).

- [ ] **Step 5: Run the whole suite and lint**

Run: `go test -race ./... && make lint`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): ness sync with --check, --dry-run, --json and --keep"
```

---

### Task 14: `ness init`

**Files:**
- Create: `internal/cli/init.go`, `internal/cli/testdata/script/init.txtar`, `internal/cli/testdata/script/init_profiles.txtar`
- Modify: `internal/cli/cli.go` (register `newInitCmd(d)`)

**Interfaces:**
- Consumes: `planSync`, `report`, `syncExit`, `repoRoot`, `config.PinOf`, `config.Parse`.

- [ ] **Step 1: Write the failing scripts**

`init.txtar`:
```
# init refuses what it cannot decide, then writes ness.toml and syncs; a second init is refused.
# covers: R-CFG-020, R-CFG-021, R-CFG-023, R-CFG-024, R-CFG-025, R-CFG-050, R-CFG-052, R-CFG-010
cd repo
exec git checkout -q --detach
exits 4 ness init --gate 'make validate' --yes
stderr '--base'
exec git checkout -q main
exits 4 ness init --gate 'make validate' --profile nope --yes
stderr 'unknown profile "nope"'
! exists .ness
exits 4 ness init --yes
stderr '--gate'
! exists .ness
exec ness init --gate 'make validate' --client claude --yes
cmp .ness/ness.toml ../want.toml
exists .ness/lock.json
exists AGENTS.md
exec ness sync --check
exits 4 ness init --gate x --yes
stderr 'already exists'
cmp .ness/ness.toml ../want.toml

-- want.toml --
ness = "0.1"
project = "repo"
base = "main"
gate = "make validate"
client = "claude"
```

`init_profiles.txtar`:
```
# --profile is repeatable and recorded in the order given.
# covers: R-CFG-022
cd repo
exec ness init --gate true --base main --profile b --profile a --yes
grep '^profiles = \["b", "a"\]$' .ness/ness.toml
exists docs/a.md
exists docs/b.md

-- repo/.ness/profiles/a/profile.toml --
name = "a"
[files]
"docs/a.md" = { own = "a.md" }
-- repo/.ness/profiles/a/a.md --
a
-- repo/.ness/profiles/b/profile.toml --
name = "b"
[files]
"docs/b.md" = { own = "b.md" }
-- repo/.ness/profiles/b/b.md --
b
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run TestScripts/init`
Expected: FAIL. `init` is an unknown command.

- [ ] **Step 3: Implement**

`internal/cli/init.go`:
```go
package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

type initOpts struct {
	profiles                    []string
	client, base, gate, project string
	yes                         bool
}

func newInitCmd(d Deps) *cobra.Command {
	var o initOpts
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create .ness/ness.toml and render the harness into this repository",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return runInit(d, o) },
	}
	f := cmd.Flags()
	f.StringArrayVar(&o.profiles, "profile", nil, "apply profile `NAME` (repeatable, applied in order)")
	f.StringVar(&o.client, "client", "", "client: claude or codex (default claude)")
	f.StringVar(&o.base, "base", "", "base branch (default: the current branch)")
	f.StringVar(&o.gate, "gate", "", "gate `CMD` that ness integrate runs")
	f.StringVar(&o.project, "project", "", "project name (default: the repository directory name)")
	f.BoolVar(&o.yes, "yes", false, "never prompt")
	return cmd
}

// runInit writes .ness/ness.toml and syncs. It plans the sync from the new file before writing
// anything, so an unknown profile or a conflict between profiles leaves the repository as it
// was (R-CFG-025). Prompting on a TTY arrives in P6 (R-CFG-051); until then every missing value
// is refused.
func runInit(d Deps, o initOpts) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := repoRoot(wd)
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(root, config.Path)
	if _, err := os.Lstat(cfgPath); !errors.Is(err, fs.ErrNotExist) {
		return refuse("%s already exists; edit it and run ness sync", config.Path)
	}
	if err := fillInitDefaults(&o, root); err != nil {
		return err
	}
	data := initFile(o, config.PinOf(d.Version))
	file, err := config.Parse(data)
	if err != nil {
		return refuse("%v", err)
	}
	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer func() { _ = rootFS.Close() }()
	plan, err := planSync(d, &project{root: root, file: file}, rootFS)
	if err != nil {
		return err
	}
	if err := rootFS.MkdirAll(".ness", 0o750); err != nil {
		return err
	}
	if err := rootFS.WriteFile(config.Path, data, 0o644); err != nil { //nolint:gosec // committed config
		return err
	}
	if err := nsync.Apply(rootFS, plan); err != nil {
		return err
	}
	report(d, plan)
	return syncExit(plan, false)
}

func fillInitDefaults(o *initOpts, root string) error {
	if o.project == "" {
		o.project = filepath.Base(root)
	}
	if o.base == "" {
		o.base = currentBranch(root)
	}
	var missing []string
	if o.gate == "" {
		missing = append(missing, `--gate (the command ness integrate runs, e.g. --gate "make validate")`)
	}
	if o.base == "" {
		missing = append(missing, "--base (HEAD is detached, so the base branch cannot be inferred)")
	}
	if len(missing) > 0 {
		return refuse("missing %s", strings.Join(missing, " and "))
	}
	return nil
}

func currentBranch(root string) string {
	out, err := exec.Command("git", "-C", root, "symbolic-ref", "--quiet", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// initFile writes the keys init knows, in a fixed order.
// ponytail: strconv.Quote equals a TOML basic string for printable text; config.Parse rejects
// anything else before it is written.
func initFile(o initOpts, pin string) []byte {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "ness = %s\nproject = %s\nbase = %s\ngate = %s\n",
		strconv.Quote(pin), strconv.Quote(o.project), strconv.Quote(o.base), strconv.Quote(o.gate))
	if o.client != "" {
		_, _ = fmt.Fprintf(&b, "client = %s\n", strconv.Quote(o.client))
	}
	if len(o.profiles) > 0 {
		quoted := make([]string, len(o.profiles))
		for i, p := range o.profiles {
			quoted[i] = strconv.Quote(p)
		}
		_, _ = fmt.Fprintf(&b, "profiles = [%s]\n", strings.Join(quoted, ", "))
	}
	return []byte(b.String())
}
```

Register in `cli.go`: `root.AddCommand(newInitCmd(d), newSyncCmd(d))`.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/cli/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): ness init writes the pinned config and syncs"
```

---

### Task 15: Dogfood, coverage gate, hermetic test, wrap-up

**Files:**
- Create: `internal/testkit/coverage_repo_test.go`, `internal/testkit/testkit_test.go`
- Generated by `ness init` in this repository: `.ness/ness.toml`, `.ness/lock.json`, `AGENTS.md`, `CLAUDE.md`, `.agents/skills/ness-entry/`, `.claude/`, `.gitignore`
- Modify: `Makefile` (`selfcheck`), `README.md` (status), `docs/spec/quality.md` (mark R-QA-025 withdrawn)

- [ ] **Step 1: Hermetic setup test**

`internal/testkit/testkit_test.go`:
```go
package testkit_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestSetupIsHermetic(t *testing.T) {
	// covers: R-QA-021
	work := t.TempDir()
	env := &testscript.Env{WorkDir: work, Vars: []string{"PATH=" + os.Getenv("PATH"), "GIT_DIR=/nonexistent"}}
	if err := testkit.Setup(env); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(env.Vars, func(kv string) bool { return strings.HasPrefix(kv, "GIT_DIR=") }) {
		t.Fatal("GIT_DIR leaked into the script environment")
	}
	if _, err := os.Stat(filepath.Join(work, "repo", ".git")); err != nil {
		t.Fatalf("no repository: %v", err)
	}
	if err := testkit.Setup(&testscript.Env{WorkDir: "/"}); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("a work dir outside the temp dir was accepted: %v", err)
	}
}
```

Run: `go test ./internal/testkit/`
Expected: PASS. If `Env.Setenv` panics on a bare `Env`, add `Values: map[any]any{}` or construct the env the way testscript does, and note why in a comment.

- [ ] **Step 2: The repository coverage test**

`internal/testkit/coverage_repo_test.go`:
```go
package testkit_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// doneAreas are the spec areas whose plans have landed (P1).
var doneAreas = []string{"CFG", "SYNC", "PROF", "QA"}

// deferred names ids of done areas that a later plan, a review entry or publication delivers.
var deferred = map[string]string{
	"R-CFG-014": "P2", "R-CFG-053": "P2",
	"R-CFG-036": "P3", "R-CFG-041": "P3", "R-SYNC-068": "P3", "R-SYNC-086": "P3",
	"R-QA-024": "P3", "R-QA-027": "P3", "R-QA-042": "P3",
	"R-CFG-030": "P5", "R-CFG-031": "P5", "R-CFG-054": "P5", "R-PROF-015": "P5",
	"R-CFG-032": "P6", "R-CFG-034": "P6", "R-CFG-035": "P6", "R-CFG-037": "P6", "R-CFG-051": "P6",
	"R-SYNC-032": "P6", "R-SYNC-033": "P6", "R-SYNC-089": "P6", "R-PROF-002": "P6",
	"R-QA-004": "P6", "R-QA-005": "P6", "R-QA-023": "P6", "R-QA-030": "P6", "R-QA-031": "P6", "R-QA-041": "P6",
	"R-QA-032": "publication", "R-QA-033": "publication",
	"R-QA-025": "withdrawn (D39)",
	// Review entries, written to docs/spec/reviews.md by the end-of-plan reviewer.
	"R-CFG-002": "review", "R-CFG-033": "review", "R-SYNC-074": "review", "R-SYNC-083": "review",
	"R-QA-002": "review", "R-QA-003": "review", "R-QA-006": "review", "R-QA-007": "review",
	"R-QA-008": "review", "R-QA-009": "review", "R-QA-020": "review", "R-QA-022": "review",
	"R-QA-026": "review", "R-QA-040": "review",
}

func TestRequirementCoverage(t *testing.T) {
	// The gate for I4: every requirement of a done area has a covering test.
	c, err := testkit.ScanCoverage("../../docs/spec", "../..")
	if err != nil {
		t.Fatal(err)
	}
	if problems := c.Problems(doneAreas, deferred); len(problems) > 0 {
		t.Fatalf("requirement coverage:\n%s", strings.Join(problems, "\n"))
	}
}
```

Run: `go test ./internal/testkit/ -run TestRequirementCoverage`
Expected: PASS. If the test names an uncovered id, it is a real gap: find the task whose code delivers it, add the test there with its covers marker, and rerun. Defer an id only when its behaviour needs a later plan's command, and write that plan's name.

- [ ] **Step 3: Mark R-QA-025 withdrawn**

In `docs/spec/quality.md`, append to the R-QA-025 statement: ` (withdrawn: D39 replaces the separate test writer with TDD inside each plan task)`.

- [ ] **Step 4: Dogfood**

```bash
go run ./cmd/ness init --gate "make validate" --base main --yes
git status --short
go run ./cmd/ness sync --check
```
Expected: init prints the created targets, `git status` shows exactly the core files, and `sync --check` exits 0.

Set `Makefile` `selfcheck` to:
```make
selfcheck:
	go run ./cmd/ness sync --check
```

- [ ] **Step 5: The full gate**

Run: `make validate && make fuzz FUZZTIME=10s`
Expected: PASS end to end.

- [ ] **Step 6: Update README status**

In `README.md`, replace the status line with: `Status: P1 (foundation and sync) implemented: ness init and ness sync. Plans: docs/superpowers/specs/2026-10-08-ness-v1-scope.md.`

- [ ] **Step 7: Commit and push, check CI**

```bash
git add -A
git commit -m "build: dogfood ness on itself, enforce requirement coverage in the gate"
git push
gh run watch --exit-status
```
Expected: the CI run is green. If it fails only in CI, fix the cause (often the environment: mise, PATH, git identity) and push again.

---

## End-of-plan review

After Task 15, launch one fresh reviewer on the most capable model with: the spec files for CFG, SYNC and PROF, this plan, and `git diff <first P1 commit>^..HEAD`. The reviewer checks:
1. Each covered requirement's test would fail without its behaviour (spot-check by breaking the code).
2. The Review Focus items hold.
3. The review-kind ids listed under `review` in `deferred`.

For each review-kind id that holds, the reviewer appends a line to `docs/spec/reviews.md` (`- <id> | <reviewer session> | <date> | <commit>`), and that id is removed from `deferred`. Then fix each finding, rerun `make validate`, and report to the operator: what shipped, the evidence (test counts, fuzz time, CI run link), and what is deferred.
