> [!NOTE]
> Archive: altered for the public release. Local paths were replaced; the content is otherwise as written.

# P5a Notify and Contracts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The feature switch works end to end (`features` in `ness.toml` and a profile's `[features] enable` turn a feature's commands and skill text on, and a command of a feature that is off refuses naming it); `ness contract freeze|amend|check` and the pre-tool contract guard protect frozen files; `ness post` renders one Markdown note for a Slack, Telegram, GitHub or command channel, splits it on block boundaries, threads it by key, records every part it sent and resumes a failed post, and never touches the network under test.

**Architecture:** `internal/cli/project.go` resolves the enabled feature set once (`resolvedConfig`) and `requireFeature` gates the feature commands. Contract logic is `internal/session/contract.go` (lock files in the session record directory, the frozen set, `check`, the `Guard`); the CLI is `internal/cli/contract.go`; `internal/hook` learns which files a tool call edits and the hook's `cwd`. Notify is `internal/notify` (the Markdown block model, the splitter, the `Channel` seam and the `Poster` that records progress in the store) with one atom per platform under `internal/atom/{slack,telegram,github,command}`; HTTP goes through `Deps.Transport` (the fake `testkit.FakeHTTP` records requests and refuses unknown hosts) and commands through the new `internal/shell` runner (`Deps.Shell`). The store gains migration 4 (`threads`, `posts`).

**Tech Stack:** Go 1.27 (mise), `modernc.org/sqlite`, cobra, testscript, `net/http` + `net/http/httptest`, `encoding/json`, `crypto/sha256`. No new module dependency: the Markdown model and the lock format are hand-written.

**Spec:** `docs/superpowers/specs/2026-10-08-ness-v1-scope.md` (P5 row), `docs/design/ness-design.md` §2 (features, the channel seam), §5 "Features", §6 (`contract.lock` under the session record), §9 (notify), and the requirements in `docs/spec/notify.md` (NTF, all 26), `docs/spec/contracts.md` (CTR, all 14), plus the carry-ins `docs/spec/core-config.md` R-CFG-054, `docs/spec/profiles.md` R-PROF-015, `docs/spec/hooks.md` R-HOOK-010. P5b (`2026-10-09-p5b-probes.md`) builds on this plan: probes post through `notify.Poster` and run floors through `shell.Run`.

## Global Constraints

- Everything of P4's Global Constraints holds: external programs run through `Deps.Exec` (`cli.SystemExec` / `testkit.FakeExec`), git discovery variables stripped; exit codes 0 ok, 1 problems, 2 usage, 3 conflict, 4 refused (R-CFG-035); a refusal writes nothing (R-CFG-036); every test names its ids (`// covers:`, `# covers:`); conventional commits with the session's trailers; no push and no CI watch.
- A command of a feature that is off refuses with exit 4 naming the feature to enable (R-CFG-054); the feature set is the union of `features` in `.ness/ness.toml` and every applied profile's `[features] enable` (R-PROF-015).
- No test sends anything to a real service: every HTTP request goes through `Deps.Transport`; `testkit.FakeHTTP` answers only `slack.com`, `api.telegram.org` and `api.github.com` and fails any other host; no real webhook URL, token, chat id or e-mail appears in any fixture (test tokens are literally `test-token`, ids like `C0TEST`, `1001`, `o/r`).
- The channel's credential is read only from the variable named by `token_env` (R-NTF-042, R-CFG-046, R-CFG-002); a missing variable refuses (exit 4) before any request.
- `ness send` never posts to a channel (R-NTF-006); `ness integrate` never calls `post` (R-SESS-100): nothing in `internal/session/integrate.go` or `internal/cli/send.go` imports `internal/notify`.
- The pre-tool guard never inspects Bash commands (R-CTR-021) and never reads `CLAUDE_PROJECT_DIR` (R-CTR-010): the repository is the edited file's own worktree.
- No `time.Sleep` in tests (`.golangci.tests.yml`); nothing in this plan needs a timer.
- Lint limits: `cyclop` and `funlen` defaults, 120 columns, `noctx` (every `http.NewRequestWithContext`), no shadowed `err`.
- Migrations are append-only: this plan appends migration 4 and nothing edits 1–3.

## Review Focus

1. **An edit whose path is outside any repository, or a `Write` of a file whose directory does not exist yet.** The guard must allow it silently (nothing is frozen there) and never fail the tool call on a resolver error. Pinned in Task 4 (`TestGuardAllowsPathsOutsideAnyRepoAndNewFiles`).
2. **A hand-edited `contract.lock` with a malformed line.** `contract check` fails closed naming the lock (exit 1); the guard logs one stderr line and allows the edit (the gate catches it). Pinned in Task 2 (`TestParseLockRejectsMalformedLines`) and Task 3 (`contract.txtar`, the malformed-lock section).
3. **Markdown with an unterminated code fence.** The rest of the note is one code block; the splitter never cuts inside it, and when it alone is over the limit the post fails naming it with nothing sent. Pinned in Task 5 (`TestUnterminatedFenceIsOneBlock`).
4. **Slack answering HTTP 200 with `{"ok":false,"error":"channel_not_found"}`.** That is a failed part: not recorded as sent, the error names Slack's reason. Pinned in Task 6 (`TestSlackOKFalseIsAnError`).
5. **An empty note (`--file` with no blocks).** `post` refuses before any request: there is nothing to send and nothing to record. Pinned in Task 8 (`post.txtar`, the empty-note section).

## Deferred to later plans

The coverage test (Task 10) lists these so the gate stays green while the gap stays visible:

- Review entry in `docs/spec/reviews.md`: R-NTF-030 (golden files per construct × channel) — the end-of-plan reviewer checks `internal/notify/testdata/constructs/` against each atom's `testdata/golden/` and writes the ledger line.
- P5b: R-CFG-030, R-CFG-031 (the full 16-command surface needs `probe`), R-SESS-070, R-SESS-081, R-SESS-083, R-DOC-015, R-WRK-060, R-WRK-061 (all probes).
- Field validation (after P5): the real runs of the channel contract (R-NTF-005, R-NTF-023, R-NTF-027, R-NTF-028 "contract(fake+real)") need the operator's credentials; they live behind the `e2e` tag and skip without `NESS_E2E=1` and the token variables. The fake runs cover the ids now.

## Rulings

- **Enabled features are the union** of `features` in `.ness/ness.toml` and every applied profile's `[features] enable`; `resolvedConfig` computes it once and `doctor`, `status`, `sync`, the hook and the feature commands all read `cfg.Features`. A profile cannot turn a feature off.
- **Feature skill text ships now, minimal.** `profiles/features/<probes|notify|contracts>/SKILL.md` is embedded and rendered to `.agents/skills/ness-<feature>/SKILL.md` only while the feature is on (owner `feature/<name>`); P6 (BLT) rewrites the words, this plan ships the command references so R-PROF-015 and R-BLT-004 have a mechanism to test.
- **`contract.lock` is one line per file: `<blob sha> <path>`, or `amend <path>` while the file is open for amendment.** The amendment state lives in the committed lock, so a diff shows who opened what; `freeze` of an amended path writes the new sha into the lock that holds it (R-CTR-008).
- **`[contracts] frozen` paths are checked against their blob at `base`** (`git rev-parse <base>:<path>`): a project-wide frozen file changes only by a commit on the base branch; `amend` on one is refused ("change it on <base>"); the guard refuses edits to it in every session.
- **The guard fails open on an unreadable lock** (one stderr line, the edit allowed) because a blocked `Edit` of every file would lock the session out; `contract check` fails closed on the same lock (exit 1 naming it), which the gate runs.
- **`freeze` and `amend` need a session**: the operator freezes project-wide paths in `ness.toml`. `amend` and a re-`freeze` act on the lock that holds the path; a fresh `freeze` writes the caller's own lock (`<worktree>/<record>/contract.lock`); the lock's owner is the session whose `Record` is the lock's directory, and only it and its manager may touch it (R-CTR-020).
- **Two seams, not one.** HTTP goes through `Deps.Transport` (`http.RoundTripper`; `testkit.FakeHTTP` records requests in `http.json` and answers canned platform replies). Commands with stdin, an exit code and a timeout (the `command` channel now, probe floors in P5b) go through `shell.Runner` (`shell.Run`: `sh -c`, own process group, killed at the timeout, git env stripped) injected as `Deps.Shell`; the Exec seam returns stdout only and cannot carry those. In scripts the runner is the real one over throwaway commands, as the integrate gate already is.
- **`note.<channel>.md` is looked up by channel name, then by kind** (`note.releases.md`, then `note.slack.md`): a platform-tuned note serves every channel of that kind, a name-specific one wins.
- **GitHub needs `--thread N`**: a post is a comment on issue or PR N; without a thread it refuses (nothing to comment on).
- **Slack renders one `markdown` block per Markdown block** (R-NTF-020); a message carries at most 50 blocks and 12,000 characters of block text (R-NTF-021); tables become fenced code (R-NTF-022).
- **The resume key of a post is sha256(channel, thread key, every rendered part)**: a retry with the same note, channel and thread resumes at the first unsent part; a changed note is a new post.
- **Dry run prints each part's exact request body, one per line**, and the `command` channel's stdin text; a non-dry run's recorded body is byte-identical (`fake http body N FILE`).
- **The notify package never imports the atoms**; the CLI composes `buildChannel(d, cfg, name)` and the P5b worker will do the same through `notify.Poster.Channel`.

## File map

```
profiles/embed.go                           all:features; Features() fs.FS
profiles/features/{probes,notify,contracts}/SKILL.md   minimal feature skill text
internal/config/config.go                   Enable(names)
internal/sync/render.go                     Render(res, cfg, root, features fs.FS); featureEntries
internal/sync/render_test.go                feature skills render only when on
internal/cli/project.go                     resolvedConfig (defaults + features), projectConfig
internal/cli/features.go                    requireFeature
internal/cli/sync.go                        planSync via resolvedConfig
internal/cli/cli.go                         Deps.Transport, Deps.Shell; newPostCmd, newContractCmd registered
internal/cli/testdata/script/features.txtar R-CFG-054, R-PROF-015
internal/session/contract.go                Lock, ParseLock, FindLocks, FrozenFiles, Check, Guard
internal/session/contract_test.go           lock format, frozen set, check, guard spellings
internal/cli/contract.go                    ness contract freeze|amend|check
internal/cli/testdata/script/contract.txtar, contract_rights.txtar, contract_guard.txtar
internal/hook/payload.go                    Cwd
internal/hook/edits.go                      EditedFiles (Edit, Write, MultiEdit, apply_patch)
internal/hook/edits_test.go
internal/cli/hook.go                        preTool runs the contract guard when contracts is on
internal/shell/shell.go                     Cmd, Result, Runner, Run
internal/shell/shell_test.go
internal/notify/markdown.go                 Block, Kind, Parse, Inline, Inlines
internal/notify/split.go                    Renderer, Part, Split, ErrBlockTooLong
internal/notify/channel.go                  Channel
internal/notify/post.go                     Poster, Request, PartialError, ErrNoChannel, PostKey
internal/notify/*_test.go                   parser, splitter, poster, FuzzParse, FuzzSplit
internal/notify/testdata/constructs/*.md    one Markdown file per construct (shared by the atoms)
internal/atom/slack/slack.go                Channel: markdown blocks, 12,000 / 50, thread_ts
internal/atom/telegram/telegram.go          Channel: HTML, 4,096 UTF-16 units, reply_to_message_id
internal/atom/github/github.go              Channel: GFM as is, 65,536, issue comments
internal/atom/command/command.go            Channel: stdin, NESS_THREAD_KEY, no limit
internal/atom/*/testdata/golden/*.txt       golden per construct
internal/atom/*/*_test.go                   golden, limits, payloads, contract over the fake, e2e real
internal/store/migrate.go                   migration 4: threads, posts
internal/store/notify.go                    Thread, SetThread, BeginPost, PostProgress, MarkSent
internal/store/notify_test.go
internal/testkit/http.go                    FakeHTTP RoundTripper, http.json, `fake http` script verbs
internal/testkit/contracts.go               ChannelContract
internal/cli/atoms.go                       buildChannel, buildPoster
internal/cli/post.go                        ness post
internal/cli/testdata/script/post.txtar, post_command.txtar
internal/cli/testdata/script/send.txtar     + fake http requests 0 (R-NTF-006)
internal/testkit/coverage_repo_test.go      NTF, CTR join doneAreas; deferred edited
README.md                                   status line
```

---

### Task 1: The feature switch: union, command gate, skill text

**Files:**
- Modify: `internal/config/config.go`, `internal/cli/project.go`, `internal/cli/sync.go`, `internal/sync/render.go`, `internal/sync/render_test.go`, `profiles/embed.go`
- Create: `internal/cli/features.go`, `profiles/features/probes/SKILL.md`, `profiles/features/notify/SKILL.md`, `profiles/features/contracts/SKILL.md`, `internal/cli/testdata/script/features.txtar`

**Interfaces:**
- Produces:
  - `(*config.Config).Enable(names ...string)`: appends the names not already in `Features`, order kept.
  - `(*config.Config).On(feature string) bool`.
  - `cli.resolvedConfig(d Deps, p *project) (*config.Config, *profile.Resolved, error)`: the config over the profiles' defaults with the profiles' features enabled; `projectConfig` is its first two results.
  - `cli.requireFeature(d Deps, name string) (*project, *config.Config, error)`: the project and config, or a refusal `feature <name> is off: add "<name>" to features in .ness/ness.toml`.
  - `profiles.Features() fs.FS` with `<name>/SKILL.md` per feature.
  - `sync.Render(res *profile.Resolved, cfg *config.Config, root *os.Root, features fs.FS) ([]Entry, error)`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/sync/render_test.go` (use its existing `render` helper after changing its signature in Step 3 to pass `profiles.Features()`):
```go
func TestFeatureSkillsRenderOnlyWhenOn(t *testing.T) {
	// A feature brings its own skill text (design §2); it is a target only while the feature is
	// on, owned by feature/<name>, so sync removes it when the feature goes off.
	// covers: R-PROF-015
	entries := render(t, nil, map[string]any{"features": []string{"probes"}})
	e := find(entries, ".agents/skills/ness-probes/SKILL.md")
	if e == nil || e.Kind != nsync.KindFile || e.Owner != "feature/probes" ||
		!bytes.Contains(e.Content, []byte("ness probe start")) {
		t.Fatalf("probes skill = %+v; want a feature/probes file naming ness probe start", e)
	}
	for _, off := range []string{".agents/skills/ness-notify/SKILL.md", ".agents/skills/ness-contracts/SKILL.md"} {
		if find(entries, off) != nil {
			t.Fatalf("%s rendered while its feature is off", off)
		}
	}
	if find(render(t, nil, nil), ".agents/skills/ness-probes/SKILL.md") != nil {
		t.Fatal("probes skill rendered with no feature on")
	}
}
```

`internal/cli/testdata/script/features.txtar`:
```
# The feature set is the union of ness.toml and the applied profiles' [features] enable: a
# profile enabling probes makes sync render the probes skill text; a command of a feature that
# is off refuses (exit 4) naming the feature, and nothing is written.
# covers: R-CFG-054, R-PROF-015
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
! exists .agents/skills/ness-probes/SKILL.md
# Off: post and contract refuse naming their feature; the tree is untouched.
snap before
exits 4 ness post --channel x --file ../n.md
stderr 'feature notify is off: add "notify" to features'
exits 4 ness contract check
stderr 'feature contracts is off: add "contracts" to features'
samesnap before
# A profile enables probes: sync renders the skill; doctor and status read the union.
exec sh -c 'printf "profiles = [\"p\"]\n" >> .ness/ness.toml'
exec ness sync
exists .agents/skills/ness-probes/SKILL.md
grep 'ness probe start' .agents/skills/ness-probes/SKILL.md
exec ness status
! stdout 'probes: none \(feature off\)'
# ness.toml enables notify: the notify skill appears; the gate opens for post (which then
# refuses for its own reasons: the channel does not exist).
exec sh -c 'printf "features = [\"notify\"]\n" >> .ness/ness.toml'
exec ness sync
exists .agents/skills/ness-notify/SKILL.md
exits 4 ness post --channel x --file ../n.md
stderr 'no such channel x'
# The profile goes: the probes skill goes with it.
exec sh -c 'sed -i "/^profiles = /d" .ness/ness.toml'
exec ness sync
! exists .agents/skills/ness-probes/SKILL.md

-- repo/.ness/profiles/p/profile.toml --
name = "p"
[features]
enable = ["probes"]
-- n.md --
hello
```
The `probes: none (feature off)` line changes in P5b; here `status` with probes on prints `probes: none` (Step 4 edits one line of `printStatus`).

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/sync/ -run TestFeatureSkills; go test ./internal/cli/ -run 'TestScripts/features'`
Expected: FAIL (`render` has three arguments; `ness post` is an unknown command, exit 2 not 4).

- [ ] **Step 3: The union and the skill files**

`internal/config/config.go`, after `SplitTier`:
```go
// Enable adds the features a profile enables (R-PROF-015); a name already on is kept once.
func (c *Config) Enable(names ...string) {
	for _, n := range names {
		if !slices.Contains(c.Features, n) {
			c.Features = append(c.Features, n)
		}
	}
}

// On reports whether the feature is enabled.
func (c *Config) On(feature string) bool { return slices.Contains(c.Features, feature) }
```

`profiles/embed.go`: change the directive to `//go:embed all:core all:builtin all:features` and add:
```go
// Features returns the directory holding one subdirectory per feature with its SKILL.md
// (design §2: a feature brings its own skill text, rendered only while it is on).
func Features() fs.FS { return sub("features") }
```

`profiles/features/probes/SKILL.md`:
```markdown
---
name: ness-probes
description: Probes are scheduled questions about the running world, owned by a session; the worker runs the floor, the woken agent judges within the scope.
---

# ness probes

A probe is one Markdown file with frontmatter (`every` or `once`, optional `floor`, `expect`,
`confirm`, `timeout`, `wake`, `sample`, `notify`, `channel`, `window`, `jitter`); the body is
the scope you check when woken.

- `ness probe start PATH…` — register files or a directory; you own them. Edits reload.
- `ness probe run PATH` — run the floor now and print the state; nothing is recorded.
- `ness probe verdict ITEM --state green|red --note TEXT [--probe-bug]` — answer a wake.
- `ness probe stop PATH --note TEXT` — stop your probe.

A wake is a request in your inbox carrying the scope and the evidence. Judge it, then answer
with `ness probe verdict`. `--probe-bug` records a defect in the probe, posts nothing and flags
it `needs-edit` until the file changes.
```

`profiles/features/notify/SKILL.md`:
```markdown
---
name: ness-notify
description: Post one Markdown note to a configured channel (Slack, Telegram, GitHub, command); separate from ness send because an external post is irreversible.
---

# ness notify

- `ness post --channel NAME --file NOTE.md [--thread KEY] [--dry-run]`

Channels are named in `.ness/ness.toml` under `[notify.channels]`. The note is rendered for the
platform and split on paragraph and code-fence boundaries; a `note.NAME.md` or `note.KIND.md`
next to the note is sent verbatim instead. `--thread KEY` keeps related posts in one thread.
Always `--dry-run` first: it prints exactly what would be sent. A post is irreversible.
```

`profiles/features/contracts/SKILL.md`:
```markdown
---
name: ness-contracts
description: Freeze files you agreed on; the pre-tool guard refuses edits to them until an amendment is opened, and the gate checks them.
---

# ness contracts

- `ness contract freeze PATHS…` — record each file's blob sha in your `contract.lock`.
- `ness contract amend PATH --reason TEXT` — open one frozen file for change; the reason goes
  to the session's `decisions.md`. `freeze` it again when done.
- `ness contract check` — the gate's check: every frozen file matches its sha, or is open.

Edit, Write and MultiEdit on a frozen file are refused by the pre-tool hook. Changes made
through Bash are caught by `ness contract check`.
```

- [ ] **Step 4: Render, resolve, gate**

`internal/sync/render.go`: change the signature and add the feature entries after `core`:
```go
func Render(res *profile.Resolved, cfg *config.Config, root *os.Root, features fs.FS) ([]Entry, error) {
	…
	out = append(out, core...)
	feats, err := featureEntries(cfg, features)
	if err != nil {
		return nil, err
	}
	out = append(out, feats...)
	…
}

// featureEntries renders each enabled feature's skill text (design §2); an unknown feature has
// no text and renders nothing.
func featureEntries(cfg *config.Config, features fs.FS) ([]Entry, error) {
	out := make([]Entry, 0, len(cfg.Features))
	for _, f := range cfg.Features {
		text, err := fs.ReadFile(features, f+"/SKILL.md")
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, Entry{
			Path: skillsDir + "ness-" + f + "/SKILL.md", Kind: KindFile, Owner: "feature/" + f, Content: text,
		})
	}
	return out, nil
}
```
(`io/fs` joins the imports.) In `internal/sync/render_test.go` the `render` helper passes `profiles.Features()` (import `github.com/enea-scaccabarozzi/ness/profiles`).

`internal/cli/project.go`: replace `projectConfig` with:
```go
// resolvedConfig is the project's configuration with the applied profiles' defaults
// (R-PROF-008) and the features they enable (R-PROF-015), and the resolved profiles.
func resolvedConfig(d Deps, p *project) (*config.Config, *profile.Resolved, error) {
	base, err := p.file.Config(nil)
	if err != nil {
		return nil, nil, refuse("%v", err)
	}
	res, err := loadProfiles(d, p, base.Profiles)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := p.file.Config(res.Defaults)
	if err != nil {
		return nil, nil, refuse("%v", err)
	}
	cfg.Enable(res.Features...)
	return cfg, res, nil
}

// projectConfig is resolvedConfig without the profiles.
func projectConfig(d Deps, p *project) (*config.Config, error) {
	cfg, _, err := resolvedConfig(d, p)
	return cfg, err
}
```
`internal/cli/sync.go` `planSync`: replace the `base`/`loadProfiles`/`p.file.Config(res.Defaults)` lines with `cfg, res, err := resolvedConfig(d, p)` and call `nsync.Render(res, cfg, root, profiles.Features())`.

`internal/cli/features.go`:
```go
package cli

import "github.com/enea-scaccabarozzi/ness/internal/config"

// requireFeature loads the project and refuses a command of a feature that is off, naming the
// feature (R-CFG-054). The set is the union of ness.toml and the profiles (R-PROF-015).
func requireFeature(d Deps, name string) (*project, *config.Config, error) {
	p, err := loadProject(d)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return nil, nil, err
	}
	if !cfg.On(name) {
		return nil, nil, refuse("feature %s is off: add %q to features in %s", name, name, config.Path)
	}
	return p, cfg, nil
}
```
`internal/cli/status.go` `printStatus`: replace the fixed line with
```go
	say(d.Stdout, "%s", probesLine)
```
where `runStatus` sets a package-level-free value: add a field `ProbesOff bool` to `statusJSON`? No: keep it simple, pass a string. Change `printStatus(d Deps, out statusJSON)` to `printStatus(d Deps, out statusJSON, probesOn bool)` printing `probes: none (feature off)` when off and `probes: none` when on; `runStatus` passes `cfg.On("probes")`. P5b replaces this line with the probe rows.

Register the gate for the two commands this plan adds (Tasks 3 and 8 fill them): in `internal/cli/cli.go` `newRoot`, add `newPostCmd(d), newContractCmd(d)` to `AddCommand`. Until Task 3 and Task 8 exist, add the two constructors in `features.go` as stubs that only call `requireFeature` — Task 3 moves `newContractCmd` to `contract.go` and Task 8 moves `newPostCmd` to `post.go`:
```go
func newPostCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use: "post", Short: "Post a Markdown note to a configured channel", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			_, _, err := requireFeature(d, "notify")
			if err != nil {
				return err
			}
			return refuse("no such channel x: ness post arrives in Task 8")
		},
	}
}
```
(and the same shape for `contract` with `"contracts"`). The `features.txtar` line `stderr 'no such channel x'` passes against the stub and against the real command.

- [ ] **Step 5: Run the tests, then the whole tree**

Run: `go test ./internal/sync/ ./internal/cli/ -run 'TestFeatureSkills|TestScripts/(features|sync_|init|status|doctor)' && go build ./... && go vet ./... && go run ./cmd/ness sync --check`
Expected: PASS; the selfcheck on this repository still exits 0 (no feature is on here, so no skill is rendered).

- [ ] **Step 6: Commit**

```bash
git add profiles internal/config internal/sync internal/cli
git commit -m "feat(cli): the feature set is the union of ness.toml and the profiles; a command of a feature that is off refuses naming it; feature skill text renders while on"
```

---

### Task 2: Contract locks: format, the frozen set, check, the guard's lookup

**Files:**
- Create: `internal/session/contract.go`, `internal/session/contract_test.go`

**Interfaces:**
- Produces:
  - `session.LockFile = "contract.lock"`, `session.Amended = "amend"`.
  - `session.Lock{Dir string; Files map[string]string}`: `Dir` is the repo-relative record directory, `Files` maps a repo-relative path to its blob sha or `Amended`.
  - `session.ParseLock(data []byte) (map[string]string, error)`, `(Lock).Bytes() []byte` (sorted by path, `<sha-or-amend> <path>\n`).
  - `session.ReadLock(root, dir string) (Lock, error)` (a missing file is an empty lock), `session.WriteLock(root string, l Lock) error`.
  - `session.FindLocks(root, sessions string) ([]Lock, error)`: every `contract.lock` under `<root>/<sessions>`, sorted by `Dir`; a malformed one is `ErrBadLock` naming it.
  - `session.Frozen{Path, Sha, LockDir string}`; `LockDir == ""` is a project-wide path (`[contracts] frozen`).
  - `session.FrozenFiles(locks []Lock, project []string) map[string]Frozen` (a path in several locks: the first lock by `Dir`, an amended entry wins over a sha so an open amendment opens the path).
  - `(Git).BlobSha(dir, path string) (string, error)`: `git hash-object`; `(Git).BaseBlob(dir, base, path string) (string, error)`: `git rev-parse <base>:<path>`.
  - `session.CheckContracts(g Git, root, base string, frozen map[string]Frozen) ([]string, error)`: the paths whose blob differs (sorted), `Amended` skipped; a missing file differs.
  - `session.Guard{Git Git; Sessions string; Project []string; Warn io.Writer}` and `(Guard).Refuse(file, cwd string) string`: the reason to refuse an edit of `file` (relative to `cwd` when relative), `""` to allow.

- [ ] **Step 1: Write the failing tests**

`internal/session/contract_test.go`:
```go
package session_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

const sha1 = "0123456789abcdef0123456789abcdef01234567"

func TestLockFormatRoundTrips(t *testing.T) {
	// One line per file, sorted by path: "<sha> <path>" or "amend <path>" (design §6: the lock
	// is a committed file in the session record, so a diff shows what was frozen or opened).
	// covers: R-CTR-001, R-CTR-005
	l := session.Lock{Dir: "docs/sessions/r", Files: map[string]string{"b/x.feature": session.Amended, "a.json": sha1}}
	got := string(l.Bytes())
	if got != sha1+" a.json\namend b/x.feature\n" {
		t.Fatalf("Bytes = %q", got)
	}
	files, err := session.ParseLock(l.Bytes())
	if err != nil || files["a.json"] != sha1 || files["b/x.feature"] != session.Amended {
		t.Fatalf("ParseLock = %v, %v", files, err)
	}
}

func TestParseLockRejectsMalformedLines(t *testing.T) {
	// Review Focus 2: a hand-edited lock with a line that is not "<sha> <path>" is an error that
	// names the line, never a silently unfrozen file.
	// covers: R-CTR-003
	for _, bad := range []string{"a.json\n", "zz a.json\n", sha1 + "\n", "amend\n", sha1 + " \n"} {
		if _, err := session.ParseLock([]byte(bad)); err == nil || !strings.Contains(err.Error(), "line 1") {
			t.Errorf("ParseLock(%q) = %v; want an error naming line 1", bad, err)
		}
	}
	if files, err := session.ParseLock([]byte("\n# comment\n" + sha1 + " a.json\n")); err != nil || len(files) != 1 {
		t.Fatalf("blank and comment lines: %v, %v", files, err)
	}
}

// repoWithLocks is a repository with two session locks and one project-wide frozen path:
// r freezes a.json, c freezes b.txt and has c.txt open for amendment; p.txt is project-wide.
func repoWithLocks(t *testing.T) (session.Git, string) {
	t.Helper()
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	for _, f := range []string{"a.json", "b.txt", "c.txt", "p.txt"} {
		testkit.Commit(t, g, repo, f, f+"\n", "add "+f)
	}
	sha := func(p string) string {
		s, err := g.BlobSha(repo, p)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	for _, l := range []session.Lock{
		{Dir: "docs/sessions/r", Files: map[string]string{"a.json": sha("a.json")}},
		{Dir: "docs/sessions/r/c", Files: map[string]string{"b.txt": sha("b.txt"), "c.txt": session.Amended}},
	} {
		if err := session.WriteLock(repo, l); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := g.Out(repo, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Out(repo, "commit", "-q", "-m", "locks"); err != nil {
		t.Fatal(err)
	}
	return g, repo
}

func TestFindLocksAndFrozenFiles(t *testing.T) {
	// covers: R-CTR-003
	_, repo := repoWithLocks(t)
	locks, err := session.FindLocks(repo, "docs/sessions")
	if err != nil || len(locks) != 2 || locks[0].Dir != "docs/sessions/r" || locks[1].Dir != "docs/sessions/r/c" {
		t.Fatalf("FindLocks = %+v, %v", locks, err)
	}
	frozen := session.FrozenFiles(locks, []string{"p.txt"})
	if len(frozen) != 4 || frozen["c.txt"].Sha != session.Amended || frozen["b.txt"].LockDir != "docs/sessions/r/c" ||
		frozen["p.txt"].LockDir != "" || frozen["a.json"].LockDir != "docs/sessions/r" {
		t.Fatalf("FrozenFiles = %+v", frozen)
	}
	if err = os.WriteFile(filepath.Join(repo, "docs/sessions/r/contract.lock"), []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = session.FindLocks(repo, "docs/sessions"); err == nil || !strings.Contains(err.Error(), "docs/sessions/r/contract.lock") {
		t.Fatalf("a malformed lock: %v; want ErrBadLock naming it", err)
	}
}

func TestCheckContractsNamesEveryDifferingFile(t *testing.T) {
	// A frozen file whose blob differs from its recorded sha, or from base for a project-wide
	// path, is named; an amended file is not; a missing frozen file differs (R-CTR-003).
	// covers: R-CTR-003, R-CTR-004
	g, repo := repoWithLocks(t)
	locks, _ := session.FindLocks(repo, "docs/sessions")
	frozen := session.FrozenFiles(locks, []string{"p.txt"})
	diff, err := session.CheckContracts(g, repo, "main", frozen)
	if err != nil || len(diff) != 0 {
		t.Fatalf("clean tree: %v, %v", diff, err)
	}
	for _, f := range []string{"a.json", "c.txt", "p.txt"} { // c.txt is amended: allowed
		if werr := os.WriteFile(filepath.Join(repo, f), []byte("changed\n"), 0o600); werr != nil {
			t.Fatal(werr)
		}
	}
	if rerr := os.Remove(filepath.Join(repo, "b.txt")); rerr != nil {
		t.Fatal(rerr)
	}
	diff, err = session.CheckContracts(g, repo, "main", frozen)
	if err != nil || !slices.Equal(diff, []string{"a.json", "b.txt", "p.txt"}) {
		t.Fatalf("CheckContracts = %v, %v; want a.json, b.txt, p.txt", diff, err)
	}
}

func TestGuardRefusesFrozenSpellingsAndAllowsTheRest(t *testing.T) {
	// The three spellings of one frozen file (relative, absolute, through a symlink) are all
	// refused naming the file and ness contract amend; an amended file, an unfrozen file and a
	// project-wide path behave as the lock says (R-CTR-009, R-CTR-011, R-CTR-012).
	// covers: R-CTR-009, R-CTR-011, R-CTR-012
	g, repo := repoWithLocks(t)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Project: []string{"p.txt"}, Warn: os.Stderr}
	for _, spelling := range []string{"a.json", filepath.Join(repo, "a.json"), filepath.Join(link, "a.json")} {
		reason := gd.Refuse(spelling, repo)
		if !strings.Contains(reason, "a.json is frozen") || !strings.Contains(reason, "ness contract amend a.json") {
			t.Errorf("Refuse(%q) = %q", spelling, reason)
		}
	}
	if r := gd.Refuse("p.txt", repo); !strings.Contains(r, "p.txt is frozen") || !strings.Contains(r, ".ness/ness.toml") {
		t.Errorf("project-wide: %q", r)
	}
	for _, open := range []string{"c.txt", "free.txt", "docs/sessions/r/c/contract.lock"} {
		if r := gd.Refuse(open, repo); r != "" {
			t.Errorf("Refuse(%q) = %q; want allowed", open, r)
		}
	}
}

func TestGuardAllowsPathsOutsideAnyRepoAndNewFiles(t *testing.T) {
	// Review Focus 1: a path outside every repository and a new file in a directory that does
	// not exist yet are allowed silently; the resolver never fails the tool call.
	// covers: R-CTR-011
	g, repo := repoWithLocks(t)
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Warn: os.Stderr}
	outside := filepath.Join(t.TempDir(), "notes.md")
	for _, p := range []string{outside, "new/dir/file.go", filepath.Join(repo, "a", "b", "c.txt"), ""} {
		if r := gd.Refuse(p, repo); r != "" {
			t.Errorf("Refuse(%q) = %q; want allowed", p, r)
		}
	}
}

func TestGuardFailsOpenOnAnUnreadableLock(t *testing.T) {
	// Review Focus 2, the guard's half: a malformed lock is reported once on Warn and the edit
	// is allowed; contract check fails closed on it (TestFindLocksAndFrozenFiles).
	// covers: R-CTR-011
	g, repo := repoWithLocks(t)
	if err := os.WriteFile(filepath.Join(repo, "docs/sessions/r/contract.lock"), []byte("nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder
	gd := session.Guard{Git: g, Sessions: "docs/sessions", Warn: &warn}
	if r := gd.Refuse("a.json", repo); r != "" || !strings.Contains(warn.String(), "docs/sessions/r/contract.lock") {
		t.Fatalf("Refuse = %q, warn %q", r, warn.String())
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/session/ -run 'TestLock|TestParseLock|TestFindLocks|TestCheckContracts|TestGuard'`
Expected: FAIL to compile (`session.Lock` and the rest undefined).

- [ ] **Step 3: Implement `internal/session/contract.go`**

```go
package session

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

const (
	// LockFile is the contract lock in a session's record directory (R-CTR-001).
	LockFile = "contract.lock"
	// Amended marks a frozen file open for amendment in the lock (R-CTR-005).
	Amended = "amend"
)

// ErrBadLock is returned for a contract.lock that does not parse; the message names it.
var ErrBadLock = errors.New("malformed contract.lock")

var lockLine = regexp.MustCompile(`^([0-9a-f]{40}|amend) (\S.*)$`)

// Lock is one contract.lock: Dir is its repo-relative record directory, Files maps a
// repo-relative path to its git blob sha, or Amended while it is open for change.
type Lock struct {
	Dir   string
	Files map[string]string
}

// ParseLock reads "<sha> <path>" or "amend <path>" lines; blank and # lines are skipped.
func ParseLock(data []byte) (map[string]string, error) {
	files := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := lockLine.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("%w: line %d: %q is not \"<sha> <path>\" or \"amend <path>\"", ErrBadLock, n, line)
		}
		files[m[2]] = m[1]
	}
	return files, sc.Err()
}

// Bytes renders the lock sorted by path.
func (l Lock) Bytes() []byte {
	var b bytes.Buffer
	for _, p := range slices.Sorted(maps.Keys(l.Files)) {
		fmt.Fprintf(&b, "%s %s\n", l.Files[p], p)
	}
	return b.Bytes()
}

// ReadLock reads <root>/<dir>/contract.lock; a missing file is an empty lock.
func ReadLock(root, dir string) (Lock, error) {
	l := Lock{Dir: dir, Files: map[string]string{}}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dir), LockFile)) //nolint:gosec // under the repo
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return l, err
	}
	l.Files, err = ParseLock(data)
	if err != nil {
		return l, fmt.Errorf("%s/%s: %w", dir, LockFile, err)
	}
	return l, nil
}

// WriteLock writes the lock into its record directory, creating the directory.
func WriteLock(root string, l Lock) error {
	dir := filepath.Join(root, filepath.FromSlash(l.Dir))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, LockFile), l.Bytes(), 0o600)
}

// FindLocks returns every contract.lock under <root>/<sessions>, sorted by directory.
func FindLocks(root, sessions string) ([]Lock, error) {
	var locks []Lock
	base := filepath.Join(root, filepath.FromSlash(sessions))
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			if errors.Is(werr, fs.ErrNotExist) {
				return nil
			}
			return werr
		}
		if d.IsDir() || d.Name() != LockFile {
			return nil
		}
		rel, rerr := filepath.Rel(root, filepath.Dir(p))
		if rerr != nil {
			return rerr
		}
		l, lerr := ReadLock(root, filepath.ToSlash(rel))
		if lerr != nil {
			return lerr
		}
		locks = append(locks, l)
		return nil
	})
	sort.Slice(locks, func(i, j int) bool { return locks[i].Dir < locks[j].Dir })
	return locks, err
}

// Frozen is one frozen path: its recorded sha (or Amended) and the lock holding it; LockDir ""
// is a project-wide path from [contracts] frozen, compared with its blob at base.
type Frozen struct{ Path, Sha, LockDir string }

// FrozenFiles joins the locks and the project-wide paths. A path in several locks takes the
// first lock by Dir, except that an open amendment anywhere opens the path.
func FrozenFiles(locks []Lock, project []string) map[string]Frozen {
	out := map[string]Frozen{}
	for _, l := range locks {
		for p, sha := range l.Files {
			cur, seen := out[p]
			if !seen || (sha == Amended && cur.Sha != Amended) {
				out[p] = Frozen{Path: p, Sha: sha, LockDir: l.Dir}
			}
		}
	}
	for _, p := range project {
		if _, seen := out[p]; !seen {
			out[p] = Frozen{Path: p}
		}
	}
	return out
}

// BlobSha is git hash-object of the working-tree file.
func (g Git) BlobSha(dir, path string) (string, error) { return g.Out(dir, "hash-object", "--", path) }

// BaseBlob is the sha of path's blob at base.
func (g Git) BaseBlob(dir, base, path string) (string, error) {
	return g.Out(dir, "rev-parse", "--verify", "--quiet", base+":"+path)
}

// CheckContracts returns the frozen paths whose working-tree blob differs from the recorded
// sha (amended ones skipped) or, for a project-wide path, from its blob at base; a missing file
// differs (R-CTR-003). It reads only.
func CheckContracts(g Git, root, base string, frozen map[string]Frozen) ([]string, error) {
	var diff []string
	for _, p := range slices.Sorted(maps.Keys(frozen)) {
		f := frozen[p]
		if f.Sha == Amended {
			continue
		}
		want := f.Sha
		if f.LockDir == "" {
			var err error
			if want, err = g.BaseBlob(root, base, p); err != nil {
				return nil, fmt.Errorf("%s: not in %s: %w", p, base, err)
			}
		}
		got, err := g.BlobSha(root, p)
		if err != nil || got != want { // an unreadable file is a changed file
			diff = append(diff, p)
		}
	}
	return diff, nil
}

// Guard is the pre-tool contract guard (R-CTR-009..012): it resolves the edited file through
// its own worktree, never through CLAUDE_PROJECT_DIR (R-CTR-010), and compares the real path.
// An unreadable lock is reported on Warn and the edit allowed: the gate's check fails closed.
type Guard struct {
	Git      Git
	Sessions string
	Project  []string
	Warn     io.Writer
}

// Refuse returns why the edit of file (relative to cwd when relative) is refused, or "".
func (gd Guard) Refuse(file, cwd string) string {
	if file == "" {
		return ""
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(cwd, file)
	}
	real := realFile(file)
	root, err := gd.Git.Out(existingDir(real), "rev-parse", "--show-toplevel")
	if err != nil {
		return "" // not in a repository: nothing is frozen there
	}
	if root, err = filepath.EvalSymlinks(root); err != nil {
		return ""
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	rel = filepath.ToSlash(rel)
	locks, err := FindLocks(root, gd.Sessions)
	if err != nil {
		fmt.Fprintf(gd.Warn, "ness: contract guard: %v (edits allowed until it is fixed)\n", err)
		return ""
	}
	f, frozen := FrozenFiles(locks, gd.Project)[rel]
	switch {
	case !frozen || f.Sha == Amended:
		return ""
	case f.LockDir == "":
		return rel + " is frozen project-wide ([contracts] frozen in .ness/ness.toml); change it on the base branch"
	}
	return fmt.Sprintf("%s is frozen (%s/%s); run ness contract amend %s --reason TEXT to change it",
		rel, f.LockDir, LockFile, rel)
}

// existingDir is the nearest existing ancestor directory of p, where git can run.
func existingDir(p string) string {
	dir := filepath.Dir(p)
	for dir != filepath.Dir(dir) {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return dir
}

// realFile resolves symlinks in file, or in its nearest existing parent for a file that does
// not exist yet.
func realFile(file string) string {
	if r, err := filepath.EvalSymlinks(file); err == nil {
		return r
	}
	dir, rest := filepath.Dir(file), filepath.Base(file)
	for dir != filepath.Dir(dir) {
		if r, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(r, rest)
		}
		dir, rest = filepath.Dir(dir), filepath.Join(filepath.Base(dir), rest)
	}
	return file
}
```
(`maps` joins the imports.) `BaseBlob`'s `--quiet` makes a path absent from base an exit 1 with no stderr; `CheckContracts` reports it as an error naming the path and base: a `[contracts] frozen` path that does not exist on base is a configuration mistake, not a changed file.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/session/ -run 'TestLock|TestParseLock|TestFindLocks|TestCheckContracts|TestGuard' && golangci-lint run ./internal/session/`
Expected: PASS and a clean lint (`Refuse` stays under the cyclop limit with the switch; if `funlen` complains, move the lock lookup into `func (gd Guard) frozenAt(root, rel string) (Frozen, bool)`).

- [ ] **Step 5: Commit**

```bash
git add internal/session/contract.go internal/session/contract_test.go
git commit -m "feat(session): contract locks: the lock format, the frozen set, check against sha or base, the edit guard over the file's own worktree"
```

---

### Task 3: `ness contract freeze|amend|check`

**Files:**
- Create: `internal/cli/contract.go`, `internal/cli/testdata/script/contract.txtar`, `internal/cli/testdata/script/contract_rights.txtar`
- Modify: `internal/cli/features.go` (delete the `newContractCmd` stub), `internal/cli/cli.go` (unchanged registration)

**Interfaces:**
- Consumes: `session.ReadLock`, `WriteLock`, `FindLocks`, `FrozenFiles`, `CheckContracts`, `Git.BlobSha` (Task 2); `requireFeature` (Task 1); `withStore`, `caller`, `refuse`, `usage`, `exitCode` (existing).
- Produces: `ness contract freeze PATHS…`, `ness contract amend PATH --reason TEXT`, `ness contract check`; `lockOwner(s *store.Store, dir string) (store.Session, bool)`: the session whose `Record` is the lock's directory.

- [ ] **Step 1: Write the failing scripts**

`internal/cli/testdata/script/contract.txtar`:
```
# freeze records each file's blob sha in the calling session's contract.lock, refuses a missing
# path before writing anything, and needs a session; amend opens exactly one frozen file, needs
# --reason, appends the decision line and refuses an unfrozen file; freeze again closes the
# amendment; check verifies every lock and the project-wide paths, exits 1 naming each
# differing file and 0 after a revert, writes nothing, and fails closed on a malformed lock.
# covers: R-CTR-001, R-CTR-002, R-CTR-003, R-CTR-004, R-CTR-005, R-CTR-006, R-CTR-007, R-CTR-008
cd repo
exec ness init --gate true --base main --yes
exec sh -c 'printf "features = [\"contracts\"]\n[contracts]\nfrozen = [\"api.json\"]\n" >> .ness/ness.toml'
exec sh -c 'printf "{}\n" > api.json; printf "Feature: a\n" > a.feature; printf "Feature: b\n" > b.feature'
exec git add -A
exec git commit -q -m 'ness init'
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness open --scope c --tier standard --brief ../../../b.md'
store last-session C
# The operator has no lock.
exits 4 ness contract freeze a.feature
stderr 'needs a session'
# A missing path refuses before anything is written.
env NESS_SESSION=$R
snap before .worktrees
exits 4 sh -c 'cd .worktrees/r-* && ness contract freeze a.feature nope.feature'
stderr 'nope\.feature'
samesnap before .worktrees
# freeze records the blob sha in the session's record directory.
exec sh -c 'cd .worktrees/r-* && ness contract freeze a.feature b.feature'
stdout '^frozen a\.feature b\.feature in docs/sessions/r/contract\.lock$'
exec sh -c 'cd .worktrees/r-* && git hash-object a.feature'
cp stdout ../sha-a
exec sh -c 'cd .worktrees/r-* && grep "^$(cat ../sha-a) a.feature$" docs/sessions/r/contract.lock'
exec sh -c 'cd .worktrees/r-* && git add -A && git commit -q -m freeze'
# check: clean is 0 and writes nothing; an edit (sed -i, as Bash would) is named, exit 1.
snap clean .worktrees
exec sh -c 'cd .worktrees/r-* && ness contract check'
stdout '^ok\t3 frozen files$'
samesnap clean .worktrees
exec sh -c 'cd .worktrees/r-* && sed -i "s/a/A/" a.feature'
exits 1 sh -c 'cd .worktrees/r-* && ness contract check'
stdout '^differs\ta\.feature\tdocs/sessions/r/contract\.lock$'
exec sh -c 'cd .worktrees/r-* && git checkout -- a.feature && ness contract check'
# A project-wide path differs from base until committed on base.
exec sh -c 'cd .worktrees/r-* && printf "{\"v\":2}\n" > api.json'
exits 1 sh -c 'cd .worktrees/r-* && ness contract check'
stdout '^differs\tapi\.json\t\.ness/ness\.toml$'
exec sh -c 'cd .worktrees/r-* && git checkout -- api.json'
# amend: --reason required; an unfrozen file refused; one file opened; the decision logged.
exits 2 sh -c 'cd .worktrees/r-* && ness contract amend a.feature'
stderr 'reason is required'
exits 4 sh -c 'cd .worktrees/r-* && ness contract amend free.txt --reason x'
stderr 'not frozen'
exits 4 sh -c 'cd .worktrees/r-* && ness contract amend api.json --reason x'
stderr 'project-wide'
env NESS_TEST_NOW=2026-10-09T10:00:00Z
exec sh -c 'cd .worktrees/r-* && ness contract amend a.feature --reason "scenario 3 was wrong"'
stdout '^amending a\.feature'
exec sh -c 'cd .worktrees/r-* && grep "^amend a.feature$" docs/sessions/r/contract.lock'
exec sh -c 'cd .worktrees/r-* && grep -c "^Amend a.feature: scenario 3 was wrong (2026-10-09)$" docs/sessions/r/decisions.md'
stdout '^1$'
# While amended the edit passes check; b.feature stays frozen.
exec sh -c 'cd .worktrees/r-* && sed -i "s/a/A/" a.feature && ness contract check'
exec sh -c 'cd .worktrees/r-* && sed -i "s/b/B/" b.feature'
exits 1 sh -c 'cd .worktrees/r-* && ness contract check'
stdout '^differs\tb\.feature\t'
exec sh -c 'cd .worktrees/r-* && git checkout -- b.feature'
# freeze again records the new sha and closes the amendment.
exec sh -c 'cd .worktrees/r-* && ness contract freeze a.feature'
exec sh -c 'cd .worktrees/r-* && git hash-object a.feature'
cp stdout ../sha-a2
exec sh -c 'cd .worktrees/r-* && grep "^$(cat ../sha-a2) a.feature$" docs/sessions/r/contract.lock'
! grep 'amend' $WORK/repo/.worktrees/r-*/docs/sessions/r/contract.lock
exec sh -c 'cd .worktrees/r-* && ness contract check'
# A second session's lock on the same branch is checked too.
exec sh -c 'cd .worktrees/r-* && git add -A && git commit -q -m amend'
env NESS_SESSION=$C
exec sh -c 'cd .worktrees/c-* && git merge -q $(cd ../r-* && git rev-parse HEAD) && ness contract freeze api.json'
exec sh -c 'cd .worktrees/c-* && sed -i "s/A/Z/" a.feature'
exits 1 sh -c 'cd .worktrees/c-* && ness contract check'
stdout '^differs\ta\.feature\tdocs/sessions/r/contract\.lock$'
exec sh -c 'cd .worktrees/c-* && git checkout -- a.feature'
# A malformed lock fails closed, naming it.
exec sh -c 'cd .worktrees/c-* && printf "garbage\n" > docs/sessions/r/c/contract.lock'
exits 1 sh -c 'cd .worktrees/c-* && ness contract check'
stderr 'docs/sessions/r/c/contract\.lock.*line 1'

-- b.md --
b
```

`internal/cli/testdata/script/contract_rights.txtar`:
```
# freeze and amend are allowed only to the lock's owner and its manager: a sibling's amend is
# refused (exit 4), the manager's is accepted; a root's lock is amended by the operator.
# covers: R-CTR-020
cd repo
exec ness init --gate true --base main --yes
exec sh -c 'printf "features = [\"contracts\"]\n" >> .ness/ness.toml'
exec sh -c 'printf "Feature: a\n" > a.feature'
exec git add -A
exec git commit -q -m 'ness init'
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness open --scope c --tier standard --brief ../../../b.md'
store last-session C
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness open --scope d --tier standard --brief ../../../b.md'
store last-session D
env NESS_SESSION=$C
exec sh -c 'cd .worktrees/c-* && ness contract freeze a.feature && git add -A && git commit -q -m freeze'
# The lock reaches the siblings' branches as a merge would carry it.
exec sh -c 'cd .worktrees/d-* && git merge -q $(cd ../c-* && git rev-parse HEAD)'
exec sh -c 'cd .worktrees/r-* && git merge -q $(cd ../c-* && git rev-parse HEAD)'
env NESS_SESSION=$D
exits 4 sh -c 'cd .worktrees/d-* && ness contract amend a.feature --reason x'
stderr 'owned by'
env NESS_SESSION=$R
exec sh -c 'cd .worktrees/r-* && ness contract amend a.feature --reason "manager opens it"'
exec sh -c 'cd .worktrees/r-* && grep "^amend a.feature$" docs/sessions/r/c/contract.lock'
# A root's lock: the operator is its manager.
exec sh -c 'cd .worktrees/r-* && ness contract freeze a.feature && git add -A && git commit -q -m root-freeze'
exec git merge -q $(cd .worktrees/r-* && git rev-parse HEAD)
env NESS_SESSION=
exec ness contract amend a.feature --reason 'operator opens it'
exec grep '^amend a.feature$' docs/sessions/r/contract.lock

-- b.md --
b
```
(After the root freezes, `a.feature` is in two locks; `FrozenFiles` picks the first by `Dir`, `docs/sessions/r`, which the operator may amend. The merge into `main` carries both locks; the operator's command runs from the primary checkout.)

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/cli/ -run 'TestScripts/contract'`
Expected: FAIL (`freeze` is an unknown subcommand of the stub).

- [ ] **Step 3: Implement `internal/cli/contract.go`** and delete the stub from `features.go`

```go
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func newContractCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "contract",
		Short: "Freeze files, open one for amendment, or check every frozen file (the gate)",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	var reason string
	freeze := &cobra.Command{
		Use: "freeze PATHS...", Short: "Record each file's blob sha in your contract.lock", Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error { return withContracts(d, func(e contractEnv) error { return runFreeze(d, e, args) }) },
	}
	amend := &cobra.Command{
		Use: "amend PATH", Short: "Open one frozen file for change; the reason goes to decisions.md", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if reason == "" {
				return usage("--reason is required: why the frozen file changes")
			}
			return withContracts(d, func(e contractEnv) error { return runAmend(d, e, args[0], reason) })
		},
	}
	amend.Flags().StringVar(&reason, "reason", "", "why the file changes (recorded in decisions.md)")
	check := &cobra.Command{
		Use: "check", Short: "Verify every frozen file; exit 1 naming each one that differs", Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return withContracts(d, func(e contractEnv) error { return runCheck(d, e) }) },
	}
	cmd.AddCommand(freeze, amend, check)
	return cmd
}

// contractEnv is what the contract commands share: the project (its root is the caller's
// worktree), the config, the store, the caller and git.
type contractEnv struct {
	p   *project
	cfg *config.Config
	s   *store.Store
	c   caller
	g   session.Git
}

func withContracts(d Deps, fn func(contractEnv) error) (err error) {
	p, cfg, err := requireFeature(d, "contracts")
	if err != nil {
		return err
	}
	s, c, err := openStore(d, p)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	return fn(contractEnv{p: p, cfg: cfg, s: s, c: c, g: session.Git{Exec: d.exec()}})
}

// relPath makes an argument repo-relative (slash form) or refuses a path outside the checkout.
func (e contractEnv) relPath(arg string) (string, error) {
	abs, err := filepath.Abs(arg)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(e.p.root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", refuse("%s is outside this checkout", arg)
	}
	return filepath.ToSlash(rel), nil
}

// frozen is the branch's frozen set: every lock under <sessions> and the project-wide paths.
func (e contractEnv) frozen() ([]session.Lock, map[string]session.Frozen, error) {
	locks, err := session.FindLocks(e.p.root, e.cfg.Sessions)
	if err != nil {
		return nil, nil, err
	}
	return locks, session.FrozenFiles(locks, e.cfg.Contracts.Frozen), nil
}

// lockOwner is the open or closed session whose record directory holds the lock.
func lockOwner(s *store.Store, dir string) (store.Session, bool) {
	all, err := s.Sessions(true)
	if err != nil {
		return store.Session{}, false
	}
	i := slices.IndexFunc(all, func(x store.Session) bool { return x.Record == dir })
	if i < 0 {
		return store.Session{}, false
	}
	return all[i], true
}

// mayTouch refuses unless the caller owns the lock or manages its owner (R-CTR-020).
func (e contractEnv) mayTouch(dir string) error {
	owner, ok := lockOwner(e.s, dir)
	if !ok {
		return refuse("%s/%s belongs to no session of this project", dir, session.LockFile)
	}
	if e.c.id == owner.ID || e.c.id == owner.Parent {
		return nil
	}
	return refuse("%s/%s is owned by %s; only it and its manager %s may change it",
		dir, session.LockFile, owner.ID, displayName(owner.Parent))
}

// runFreeze records each path's blob: an amended path into the lock that holds it (R-CTR-008),
// any other into the caller's own lock (R-CTR-001). A missing path refuses first (R-CTR-002).
func runFreeze(d Deps, e contractEnv, args []string) error {
	if e.c.operator() {
		return refuse("freeze needs a session: the operator freezes project-wide paths in [contracts] frozen")
	}
	locks, frozen, err := e.frozen()
	if err != nil {
		return refuse("%v", err)
	}
	rels := make([]string, 0, len(args))
	for _, a := range args {
		rel, rerr := e.relPath(a)
		if rerr != nil {
			return rerr
		}
		if _, serr := os.Stat(filepath.Join(e.p.root, filepath.FromSlash(rel))); serr != nil {
			return refuse("%s does not exist; freeze names existing files", rel)
		}
		rels = append(rels, rel)
	}
	byDir := map[string]session.Lock{}
	for _, l := range locks {
		byDir[l.Dir] = l
	}
	own := e.c.sess.Record
	if _, ok := byDir[own]; !ok {
		byDir[own] = session.Lock{Dir: own, Files: map[string]string{}}
	}
	touched := map[string]bool{}
	for _, rel := range rels {
		dir := own
		if f, ok := frozen[rel]; ok && f.Sha == session.Amended {
			dir = f.LockDir
		}
		if err = e.mayTouch(dir); err != nil && dir != own {
			return err
		}
		sha, herr := e.g.BlobSha(e.p.root, rel)
		if herr != nil {
			return herr
		}
		byDir[dir].Files[rel] = sha
		touched[dir] = true
	}
	for _, dir := range slices.Sorted(maps.Keys(touched)) {
		if werr := session.WriteLock(e.p.root, byDir[dir]); werr != nil {
			return werr
		}
		say(d.Stdout, "frozen %s in %s/%s", strings.Join(rels, " "), dir, session.LockFile)
	}
	return nil
}

// runAmend opens exactly one frozen file and logs the reason (R-CTR-005, R-CTR-007).
func runAmend(d Deps, e contractEnv, arg, reason string) error {
	rel, err := e.relPath(arg)
	if err != nil {
		return err
	}
	locks, frozen, err := e.frozen()
	if err != nil {
		return refuse("%v", err)
	}
	f, ok := frozen[rel]
	switch {
	case !ok:
		return refuse("%s is not frozen; freeze it first, or edit it freely", rel)
	case f.LockDir == "":
		return refuse("%s is frozen project-wide ([contracts] frozen); change it on %s", rel, e.cfg.Base)
	case f.Sha == session.Amended:
		return refuse("%s is already open for amendment", rel)
	}
	if err = e.mayTouch(f.LockDir); err != nil {
		return err
	}
	i := slices.IndexFunc(locks, func(l session.Lock) bool { return l.Dir == f.LockDir })
	locks[i].Files[rel] = session.Amended
	if err = session.WriteLock(e.p.root, locks[i]); err != nil {
		return err
	}
	line := fmt.Sprintf("Amend %s: %s (%s)\n", rel, reason, d.Now().UTC().Format("2006-01-02"))
	dec := filepath.Join(e.p.root, filepath.FromSlash(f.LockDir), "decisions.md")
	fh, err := os.OpenFile(dec, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // the session record
	if err != nil {
		return err
	}
	if _, err = fh.WriteString(line); err != nil {
		return errors.Join(err, fh.Close())
	}
	if err = fh.Close(); err != nil {
		return err
	}
	say(d.Stdout, "amending %s (%s/%s); freeze it again when done", rel, f.LockDir, session.LockFile)
	return nil
}

// runCheck is the gate's check (R-CTR-003, R-CTR-004): read-only, exit 1 naming each differing file.
func runCheck(d Deps, e contractEnv) error {
	_, frozen, err := e.frozen()
	if err != nil {
		printErr(d.Stderr, err)
		return exitCode(ExitProblems)
	}
	diff, err := session.CheckContracts(e.g, e.p.root, e.cfg.Base, frozen)
	if err != nil {
		return err
	}
	for _, p := range diff {
		where := ".ness/ness.toml"
		if f := frozen[p]; f.LockDir != "" {
			where = f.LockDir + "/" + session.LockFile
		}
		say(d.Stdout, "differs\t%s\t%s", p, where)
	}
	if len(diff) > 0 {
		return exitCode(ExitProblems)
	}
	say(d.Stdout, "ok\t%d frozen files", len(frozen))
	return nil
}
```
(`maps` joins the imports.) `runFreeze` exceeds `funlen`: split the validation loop into `func (e contractEnv) existingRels(args []string) ([]string, error)` and the lock write loop into `func writeLocks(d Deps, root string, byDir map[string]session.Lock, touched map[string]bool, rels []string) error`.

- [ ] **Step 4: Run the scripts**

Run: `go test ./internal/cli/ -run 'TestScripts/(contract|features)' && golangci-lint run ./internal/cli/`
Expected: PASS. Note `contract check` with `[contracts] frozen` works from a worktree because `BaseBlob` reads `main:api.json` from the shared object store.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/contract.go internal/cli/features.go internal/cli/testdata/script/contract.txtar internal/cli/testdata/script/contract_rights.txtar
git commit -m "feat(cli): ness contract freeze, amend and check over the session locks and the project-wide paths"
```

---

### Task 4: The pre-tool contract guard

**Files:**
- Modify: `internal/hook/payload.go` (`Cwd`), `internal/cli/hook.go` (`preTool`)
- Create: `internal/hook/edits.go`, `internal/hook/edits_test.go`, `internal/cli/testdata/script/contract_guard.txtar`

**Interfaces:**
- Consumes: `session.Guard` (Task 2); `bind`, `block` (existing).
- Produces: `hook.Payload.Cwd string` (`json:"cwd"`); `hook.EditedFiles(tool string, input json.RawMessage) []string`: the files an Edit, Write or MultiEdit call (`file_path`) or a Codex `apply_patch` call (`*** Update File:` / `*** Delete File:` lines in any string field) touches; empty for every other tool, Bash included (R-CTR-021).

- [ ] **Step 1: Write the failing tests**

`internal/hook/edits_test.go`:
```go
package hook_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
)

func TestEditedFilesPerTool(t *testing.T) {
	// The guard sees the file an edit tool names, the files a Codex patch updates or deletes,
	// and nothing for Bash or Read (R-CTR-009, R-CTR-021).
	// covers: R-CTR-009, R-CTR-021
	patch := "*** Begin Patch\n*** Update File: a/b.go\n@@\n-x\n+y\n*** Delete File: c.txt\n*** Add File: d.txt\n+hi\n*** End Patch"
	for _, tc := range []struct {
		tool  string
		input any
		want  []string
	}{
		{"Edit", map[string]any{"file_path": "x.go", "old_string": "a", "new_string": "b"}, []string{"x.go"}},
		{"Write", map[string]any{"file_path": "/abs/y.md", "content": "..."}, []string{"/abs/y.md"}},
		{"MultiEdit", map[string]any{"file_path": "z.go", "edits": []any{}}, []string{"z.go"}},
		{"apply_patch", map[string]any{"input": patch}, []string{"a/b.go", "c.txt"}},
		{"Bash", map[string]any{"command": "sed -i s/a/b/ x.go"}, nil},
		{"Read", map[string]any{"file_path": "x.go"}, nil},
		{"Edit", "not an object", nil},
	} {
		raw, _ := json.Marshal(tc.input)
		if got := hook.EditedFiles(tc.tool, raw); !slices.Equal(got, tc.want) {
			t.Errorf("EditedFiles(%s, %s) = %v, want %v", tc.tool, raw, got, tc.want)
		}
	}
}

func TestParseReadsCwd(t *testing.T) {
	// covers: R-CTR-010
	p, err := hook.Parse(strings.NewReader(`{"tool_name":"Edit","cwd":"/w/x","tool_input":{"file_path":"a.go"}}`))
	if err != nil || p.Cwd != "/w/x" {
		t.Fatalf("Parse = %+v, %v", p, err)
	}
}
```
(`strings` joins the imports.) Extend the existing `FuzzParse` seed corpus in `payload_test.go` with the apply_patch payload: `f.Add([]byte(`{"tool_name":"apply_patch","cwd":"/w","tool_input":{"input":"*** Update File: a\n"}}`))`, and add a line to its body: `_ = hook.EditedFiles(p.ToolName, p.ToolInput)` so the fuzz target covers the edit parser (R-HOOK-015).

`internal/cli/testdata/script/contract_guard.txtar`:
```
# With contracts on, pre-tool refuses Edit, Write and MultiEdit (Claude Code) and apply_patch
# (Codex) on a frozen file, naming the file and ness contract amend; it allows unfrozen files
# and amended ones; it resolves the repository from the edited file's own worktree even when
# CLAUDE_PROJECT_DIR names the primary checkout; it never inspects Bash, so a sed -i passes the
# hook and fails contract check. With contracts off the same edit passes.
# covers: R-HOOK-010, R-CTR-009, R-CTR-010, R-CTR-011, R-CTR-021
cd repo
exec ness init --gate true --base main --yes
exec sh -c 'printf "Feature: a\n" > a.feature; printf "free\n" > free.txt'
exec git add -A
exec git commit -q -m 'ness init'
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
env NESS_SESSION=$R
# Off: nothing is guarded.
stdin ../edit.json
exec sh -c 'cd .worktrees/r-* && ness hook pre-tool'
! stderr .
# On, frozen in the session's lock.
exec sh -c 'printf "features = [\"contracts\"]\n" >> .ness/ness.toml && git add -A && git commit -q -m on'
exec sh -c 'cd .worktrees/r-* && git merge -q main && ness contract freeze a.feature'
env CLAUDE_PROJECT_DIR=$WORK/repo
stdin ../edit.json
exits 2 sh -c 'cd .worktrees/r-* && ness hook pre-tool'
stderr 'a\.feature is frozen \(docs/sessions/r/contract\.lock\); run ness contract amend a\.feature'
stdin ../write.json
exits 2 sh -c 'cd .worktrees/r-* && ness hook pre-tool'
stderr 'ness contract amend a\.feature'
stdin ../multi.json
exits 2 sh -c 'cd .worktrees/r-* && ness hook pre-tool'
stderr 'ness contract amend a\.feature'
stdin ../patch.json
exits 2 sh -c 'cd .worktrees/r-* && ness hook pre-tool'
stderr 'ness contract amend a\.feature'
# The payload's cwd is the worktree: an absolute path into it is caught from the primary too.
exec sh -c 'cd .worktrees/r-* && printf "{\"tool_name\":\"Edit\",\"cwd\":\"$(pwd)\",\"tool_input\":{\"file_path\":\"$(pwd)/a.feature\"}}" > ../../../abs.json'
stdin ../abs.json
exits 2 ness hook pre-tool
stderr 'ness contract amend a\.feature'
# Unfrozen: silent. Bash: never inspected, caught by check.
stdin ../free.json
exec sh -c 'cd .worktrees/r-* && ness hook pre-tool'
! stdout .
! stderr .
stdin ../bash.json
exec sh -c 'cd .worktrees/r-* && ness hook pre-tool'
! stderr .
exec sh -c 'cd .worktrees/r-* && sed -i "s/a/A/" a.feature'
exits 1 sh -c 'cd .worktrees/r-* && ness contract check'
stdout 'differs\ta\.feature'
exec sh -c 'cd .worktrees/r-* && git checkout -- a.feature'
# Amended: allowed again until re-frozen.
exec sh -c 'cd .worktrees/r-* && ness contract amend a.feature --reason "fix"'
stdin ../edit.json
exec sh -c 'cd .worktrees/r-* && ness hook pre-tool'
! stderr .
exec sh -c 'cd .worktrees/r-* && ness contract freeze a.feature'
stdin ../edit.json
exits 2 sh -c 'cd .worktrees/r-* && ness hook pre-tool'

-- b.md --
b
-- edit.json --
{"tool_name":"Edit","tool_input":{"file_path":"a.feature","old_string":"a","new_string":"b"}}
-- write.json --
{"tool_name":"Write","tool_input":{"file_path":"a.feature","content":"x"}}
-- multi.json --
{"tool_name":"MultiEdit","tool_input":{"file_path":"a.feature","edits":[{"old_string":"a","new_string":"b"}]}}
-- patch.json --
{"tool_name":"apply_patch","tool_input":{"input":"*** Begin Patch\n*** Update File: a.feature\n@@\n-a\n+b\n*** End Patch"}}
-- free.json --
{"tool_name":"Edit","tool_input":{"file_path":"free.txt","old_string":"free","new_string":"x"}}
-- bash.json --
{"tool_name":"Bash","tool_input":{"command":"sed -i s/a/A/ a.feature"}}
```
The `edit.json` payload has no `cwd`: the guard then uses the process working directory, which is the worktree the hook runs in.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/hook/ -run 'TestEditedFiles|TestParseReadsCwd'; go test ./internal/cli/ -run 'TestScripts/contract_guard'`
Expected: FAIL (`hook.EditedFiles` and `Cwd` undefined; the script's frozen edits exit 0).

- [ ] **Step 3: The parser and the wiring**

`internal/hook/payload.go`: add `Cwd string \`json:"cwd"\`` to `Payload` with the comment `// both: the client's working directory, where relative paths resolve`.

`internal/hook/edits.go`:
```go
package hook

import (
	"encoding/json"
	"regexp"
	"slices"
)

var (
	editTools = []string{"Edit", "Write", "MultiEdit"}
	patchFile = regexp.MustCompile(`(?m)^\*\*\* (?:Update|Delete) File: (.+)$`)
)

// EditedFiles lists the files a tool call writes: the file_path of Claude Code's Edit, Write
// and MultiEdit, the updated and deleted files of a Codex apply_patch. Bash is never inspected
// (R-CTR-021); every other tool edits nothing.
func EditedFiles(tool string, input json.RawMessage) []string {
	switch {
	case slices.Contains(editTools, tool):
		var in toolInput
		if json.Unmarshal(input, &in) != nil || in.FilePath == "" {
			return nil
		}
		return []string{in.FilePath}
	case tool == "apply_patch":
		var out []string
		for _, m := range patchFile.FindAllStringSubmatch(inputText(input), -1) {
			out = append(out, m[1])
		}
		return out
	}
	return nil
}
```

`internal/cli/hook.go`: change the dispatch to `return preTool(d, b)` and replace `preTool`:
```go
// preTool runs the guards for a bound session; a payload that does not parse is refused
// (R-HOOK-017). With contracts on, the contract guard refuses an edit of a frozen file
// (R-HOOK-010, R-CTR-009): the file's own worktree decides, never CLAUDE_PROJECT_DIR (R-CTR-010).
func preTool(d Deps, b bound) error {
	p, err := hook.Parse(d.Stdin)
	if err != nil {
		return block(err.Error())
	}
	if reason := hook.PreTool(p); reason != "" {
		return block(reason)
	}
	files := hook.EditedFiles(p.ToolName, p.ToolInput)
	if len(files) == 0 {
		return nil
	}
	cfg, err := projectConfig(d, b.p)
	if err != nil || !cfg.On("contracts") {
		return nil //nolint:nilerr // an unreadable config is reported by the next command, not by a blocked tool
	}
	cwd := p.Cwd
	if cwd == "" {
		if wd, werr := os.Getwd(); werr == nil {
			cwd = wd
		}
	}
	gd := session.Guard{Git: session.Git{Exec: d.exec()}, Sessions: cfg.Sessions, Project: cfg.Contracts.Frozen, Warn: d.Stderr}
	for _, f := range files {
		if reason := gd.Refuse(f, cwd); reason != "" {
			return block(reason)
		}
	}
	return nil
}
```
(`os` joins the imports.)

- [ ] **Step 4: Run the tests, the fuzz seed, and the whole hook suite**

Run: `go test ./internal/hook/ ./internal/cli/ -run 'TestEditedFiles|TestParseReadsCwd|FuzzParse|TestScripts/(hook_|contract)' && go test ./internal/hook/ -run '^$' -fuzz FuzzParse -fuzztime 10s && golangci-lint run ./internal/hook/ ./internal/cli/`
Expected: PASS; `hook_pretool.txtar` and `hook_unbound.txtar` still pass (no contracts feature there).

- [ ] **Step 5: Commit**

```bash
git add internal/hook internal/cli/hook.go internal/cli/testdata/script/contract_guard.txtar
git commit -m "feat(hook): pre-tool refuses Edit, Write, MultiEdit and apply_patch on a frozen file, resolved through the file's own worktree"
```

---

### Task 5: The Markdown model and the splitter

**Files:**
- Create: `internal/notify/markdown.go`, `internal/notify/split.go`, `internal/notify/channel.go`, `internal/notify/markdown_test.go`, `internal/notify/split_test.go`, `internal/notify/testdata/constructs/{heading,paragraph,bold,italic,code,fence,link,list,table,escaping}.md`

**Interfaces:**
- Produces:
  - `notify.Kind` with `Heading`, `Paragraph`, `Fence`, `List`, `Table`; `notify.Block{Kind Kind; Level int; Lang string; Text string; Items []string; Rows [][]string; Raw string}` (`Raw` is the block's source lines, `Text` the heading or paragraph text with inline markup kept, `Items` the list items, `Rows` the table cells, header row first).
  - `notify.Parse(md string) []Block`: never panics (R-NTF-013); blank lines separate blocks; an unterminated fence runs to the end.
  - `notify.InlineKind` with `Text`, `Bold`, `Italic`, `Code`, `Link`; `notify.Inline{Kind InlineKind; Text, URL string}`; `notify.Inlines(s string) []Inline`.
  - `notify.Renderer` interface: `Render(Block) string`, `Measure(string) int`, `Limit() int` (0 = unlimited), `MaxBlocks() int` (0 = unlimited).
  - `notify.Part{Blocks []string}`, `(Part).Text() string` (blocks joined by `"\n\n"`).
  - `notify.Split(r Renderer, blocks []Block) ([]Part, error)`; `notify.ErrBlockTooLong` wraps an error naming the block (its kind and first 40 characters).
  - `notify.Channel` interface: `Renderer` plus `Kind() string`, `NeedsThread() bool`, `Payload(p Part, thread string) []byte`, `Post(p Part, thread string) (id string, err error)`.

- [ ] **Step 1: Write the failing tests**

`internal/notify/testdata/constructs/*.md` (one construct per file, used by every atom's golden test):
```
heading.md:    # Title\n\n## Sub\n
paragraph.md:  One line.\nSecond line of the same paragraph.\n\nSecond paragraph.\n
bold.md:       This is **strong** and __also strong__.\n
italic.md:     This is *em* and _also em_.\n
code.md:       Run `ness sync --check` now.\n
fence.md:      ```sh\nmake validate\necho "a & b < c"\n```\n
link.md:       See [the design](https://example.invalid/design) and <https://example.invalid/raw>.\n
list.md:       - one\n- two with `code`\n\n1. first\n2. second\n
table.md:      | id | state |\n|---|---|\n| a | GREEN |\n| b | RED |\n
escaping.md:   Tom & Jerry <tag> "quotes" and a * star _ underscore.\n
```
(Write them with real newlines; the table above shows the bytes.)

`internal/notify/markdown_test.go`:
```go
package notify_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

func construct(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "constructs", name+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseBlocks(t *testing.T) {
	// Blocks are headings, paragraphs, fences, lists and tables, separated by blank lines; a
	// fence keeps its language and its raw lines (R-NTF-010 is per channel; this is the model).
	// covers: R-NTF-010
	bs := notify.Parse(construct(t, "heading") + "\n" + construct(t, "paragraph") + "\n" +
		construct(t, "fence") + "\n" + construct(t, "list") + "\n" + construct(t, "table"))
	want := []notify.Kind{notify.Heading, notify.Heading, notify.Paragraph, notify.Paragraph, notify.Fence,
		notify.List, notify.List, notify.Table}
	if len(bs) != len(want) {
		t.Fatalf("got %d blocks, want %d: %+v", len(bs), len(want), bs)
	}
	for i, k := range want {
		if bs[i].Kind != k {
			t.Errorf("block %d kind %v, want %v", i, bs[i].Kind, k)
		}
	}
	if bs[0].Level != 1 || bs[0].Text != "Title" || bs[1].Level != 2 {
		t.Errorf("headings: %+v %+v", bs[0], bs[1])
	}
	if bs[2].Text != "One line.\nSecond line of the same paragraph." {
		t.Errorf("paragraph text %q", bs[2].Text)
	}
	if bs[4].Lang != "sh" || bs[4].Text != "make validate\necho \"a & b < c\"" || bs[4].Raw != construct(t, "fence")[:len(construct(t, "fence"))-1] {
		t.Errorf("fence: %+v", bs[4])
	}
	if len(bs[5].Items) != 2 || bs[5].Items[1] != "two with `code`" || len(bs[6].Items) != 2 {
		t.Errorf("lists: %+v %+v", bs[5], bs[6])
	}
	if len(bs[7].Rows) != 3 || bs[7].Rows[0][1] != "state" || bs[7].Rows[2][1] != "RED" {
		t.Errorf("table rows: %v", bs[7].Rows)
	}
}

func TestUnterminatedFenceIsOneBlock(t *testing.T) {
	// Review Focus 3: a fence without its closing line runs to the end of the note as one block,
	// so the splitter can never cut inside it.
	// covers: R-NTF-011
	bs := notify.Parse("intro\n\n```\ncode\n\nmore code\n")
	if len(bs) != 2 || bs[1].Kind != notify.Fence || bs[1].Text != "code\n\nmore code" {
		t.Fatalf("blocks = %+v", bs)
	}
}

func TestInlines(t *testing.T) {
	// covers: R-NTF-010
	in := notify.Inlines("a **b** _c_ `d **x**` [e](http://u) *f*")
	kinds := make([]notify.InlineKind, 0, len(in))
	for _, i := range in {
		kinds = append(kinds, i.Kind)
	}
	want := []notify.InlineKind{notify.Text, notify.Bold, notify.Text, notify.Italic, notify.Text, notify.Code,
		notify.Text, notify.Link, notify.Text, notify.Italic}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	}
	if in[5].Text != "d **x**" || in[7].Text != "e" || in[7].URL != "http://u" {
		t.Errorf("code and link: %+v %+v", in[5], in[7])
	}
	if got := notify.Inlines("a * b _ c"); len(got) != 1 || got[0].Text != "a * b _ c" {
		t.Errorf("unpaired markers stay text: %+v", got)
	}
}

func FuzzParse(f *testing.F) {
	// covers: R-NTF-013
	for _, n := range []string{"heading", "paragraph", "fence", "list", "table", "escaping", "link"} {
		b, err := os.ReadFile(filepath.Join("testdata", "constructs", n+".md"))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(string(b))
	}
	f.Add("```\nopen")
	f.Add("|a|\n|-|")
	f.Fuzz(func(_ *testing.T, md string) {
		for _, b := range notify.Parse(md) {
			_ = notify.Inlines(b.Text)
			for _, it := range b.Items {
				_ = notify.Inlines(it)
			}
		}
	})
}
```

`internal/notify/split_test.go`:
```go
package notify_test

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

// plain renders the raw block, counts runes, and takes limit and maxBlocks from the test.
type plain struct{ limit, maxBlocks int }

func (p plain) Render(b notify.Block) string { return b.Raw }
func (plain) Measure(s string) int             { return utf8.RuneCountInString(s) }
func (p plain) Limit() int                     { return p.limit }
func (p plain) MaxBlocks() int                 { return p.maxBlocks }

func TestSplitOnBlockBoundaries(t *testing.T) {
	// Parts are whole blocks joined by blank lines, each within the limit; concatenated they
	// equal the full render; a block alone over the limit fails naming it, nothing else.
	// covers: R-NTF-011, R-NTF-012
	md := strings.Repeat("para one two three\n\n", 10) + "```\nfence line\nfence line two\n```\n\nlast"
	blocks := notify.Parse(md)
	parts, err := notify.Split(plain{limit: 60}, blocks)
	if err != nil {
		t.Fatal(err)
	}
	full := make([]string, 0, len(blocks))
	for _, b := range blocks {
		full = append(full, b.Raw)
	}
	var texts []string
	for _, p := range parts {
		if n := utf8.RuneCountInString(p.Text()); n > 60 {
			t.Errorf("part %q has %d runes", p.Text(), n)
		}
		for _, b := range p.Blocks {
			if strings.Count(b, "```")%2 != 0 {
				t.Errorf("part cuts a fence: %q", b)
			}
		}
		texts = append(texts, p.Text())
	}
	if strings.Join(texts, "\n\n") != strings.Join(full, "\n\n") {
		t.Fatalf("parts do not add up to the full render")
	}
	if len(parts) < 4 {
		t.Fatalf("only %d parts for a 60-rune limit", len(parts))
	}
	_, err = notify.Split(plain{limit: 10}, notify.Parse("short\n\n"+strings.Repeat("x", 11)))
	if !errors.Is(err, notify.ErrBlockTooLong) || !strings.Contains(err.Error(), "paragraph \"xxxxxxxxxxx\"") {
		t.Fatalf("over-long block: %v", err)
	}
}

func TestSplitHonoursMaxBlocksAndNoLimit(t *testing.T) {
	// covers: R-NTF-021, R-NTF-029
	blocks := notify.Parse(strings.Repeat("p\n\n", 51))
	parts, err := notify.Split(plain{limit: 0, maxBlocks: 50}, blocks)
	if err != nil || len(parts) != 2 || len(parts[0].Blocks) != 50 || len(parts[1].Blocks) != 1 {
		t.Fatalf("51 blocks, 50 per part: %d parts, %v", len(parts), err)
	}
	parts, err = notify.Split(plain{}, blocks)
	if err != nil || len(parts) != 1 {
		t.Fatalf("no limits: %d parts, %v", len(parts), err)
	}
	if parts, err = notify.Split(plain{}, nil); err != nil || len(parts) != 0 {
		t.Fatalf("no blocks: %v, %v", parts, err)
	}
}

func FuzzSplit(f *testing.F) {
	// covers: R-NTF-011, R-NTF-013
	f.Add("a\n\nb\n\n```\nc\n```\n", 8)
	f.Add("| a | b |\n|---|---|\n| 1 | 2 |\n", 3)
	f.Fuzz(func(t *testing.T, md string, limit int) {
		if limit < 0 {
			limit = -limit
		}
		blocks := notify.Parse(md)
		parts, err := notify.Split(plain{limit: limit}, blocks)
		if err != nil {
			if !errors.Is(err, notify.ErrBlockTooLong) {
				t.Fatalf("unexpected error %v", err)
			}
			return
		}
		var all []string
		for _, p := range parts {
			if limit > 0 && utf8.RuneCountInString(p.Text()) > limit {
				t.Fatalf("part over the limit: %q", p.Text())
			}
			all = append(all, p.Text())
		}
		full := make([]string, 0, len(blocks))
		for _, b := range blocks {
			full = append(full, b.Raw)
		}
		if strings.Join(all, "\n\n") != strings.Join(full, "\n\n") {
			t.Fatalf("parts %q != full %q", all, full)
		}
	})
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/notify/`
Expected: FAIL to compile.

- [ ] **Step 3: Implement `markdown.go`**

```go
// Package notify is the notify feature (design §9): one Markdown note rendered by a channel
// atom, split on block boundaries, posted in a thread by key, each part recorded.
package notify

import (
	"regexp"
	"strings"
)

// Kind is a block's kind.
type Kind int

// The block kinds: everything a renderer treats differently.
const (
	Paragraph Kind = iota
	Heading
	Fence
	List
	Table
)

// Block is one block of the note. Raw is its source (no trailing newline), Text the heading or
// paragraph text with inline markup kept (the fence's body for a Fence), Items a list's items,
// Rows a table's cells with the header first.
type Block struct {
	Kind  Kind
	Level int
	Lang  string
	Text  string
	Items []string
	Rows  [][]string
	Raw   string
}

var (
	headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*#*\s*$`)
	itemRe    = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+(.*)$`)
	tableSep  = regexp.MustCompile(`^\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?$`)
)

// Parse splits md into blocks. It never panics (R-NTF-013): every input is some sequence of
// paragraphs. An unterminated fence runs to the end.
func Parse(md string) []Block {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out []Block
	for i := 0; i < len(lines); {
		line := lines[i]
		switch {
		case strings.TrimSpace(line) == "":
			i++
		case strings.HasPrefix(line, "```"):
			var b Block
			b, i = fence(lines, i)
			out = append(out, b)
		case headingRe.MatchString(line):
			m := headingRe.FindStringSubmatch(line)
			out = append(out, Block{Kind: Heading, Level: len(m[1]), Text: m[2], Raw: line})
			i++
		default:
			var b Block
			b, i = span(lines, i)
			out = append(out, b)
		}
	}
	return out
}

// fence reads from the opening line to the closing line or the end.
func fence(lines []string, i int) (Block, int) {
	lang := strings.TrimSpace(strings.TrimPrefix(lines[i], "```"))
	j := i + 1
	for j < len(lines) && !strings.HasPrefix(lines[j], "```") {
		j++
	}
	body := strings.Join(lines[i+1:j], "\n")
	end := j
	if j < len(lines) {
		end = j + 1
	}
	return Block{Kind: Fence, Lang: lang, Text: body, Raw: strings.Join(lines[i:end], "\n")}, end
}

// span reads a run of non-blank lines that are not a fence or a heading: a table, a list or a
// paragraph.
func span(lines []string, i int) (Block, int) {
	j := i
	for j < len(lines) && strings.TrimSpace(lines[j]) != "" && !strings.HasPrefix(lines[j], "```") &&
		!headingRe.MatchString(lines[j]) {
		j++
	}
	run := lines[i:j]
	raw := strings.Join(run, "\n")
	if len(run) >= 2 && strings.Contains(run[0], "|") && tableSep.MatchString(strings.TrimSpace(run[1])) {
		return Block{Kind: Table, Rows: rows(run), Raw: raw}, j
	}
	if itemRe.MatchString(run[0]) {
		items := make([]string, 0, len(run))
		for _, l := range run {
			if m := itemRe.FindStringSubmatch(l); m != nil {
				items = append(items, m[1])
			} else {
				items[len(items)-1] += " " + strings.TrimSpace(l)
			}
		}
		return Block{Kind: List, Items: items, Raw: raw}, j
	}
	return Block{Kind: Paragraph, Text: raw, Raw: raw}, j
}

// rows splits a table's lines into cells, skipping the separator line.
func rows(run []string) [][]string {
	out := make([][]string, 0, len(run)-1)
	for n, l := range run {
		if n == 1 {
			continue
		}
		l = strings.TrimSpace(l)
		l = strings.TrimPrefix(strings.TrimSuffix(l, "|"), "|")
		cells := strings.Split(l, "|")
		for c := range cells {
			cells[c] = strings.TrimSpace(cells[c])
		}
		out = append(out, cells)
	}
	return out
}

// InlineKind is an inline span's kind.
type InlineKind int

// The inline kinds a renderer tells apart.
const (
	Text InlineKind = iota
	Bold
	Italic
	Code
	Link
)

// Inline is one span of a text: Text (or the link's text) and the link's URL.
type Inline struct {
	Kind      InlineKind
	Text, URL string
}

var inlineRe = regexp.MustCompile("`([^`]+)`|\\*\\*(.+?)\\*\\*|__(.+?)__|\\*([^*\\s](?:[^*]*[^*\\s])?)\\*|" +
	"_([^_\\s](?:[^_]*[^_\\s])?)_|\\[([^\\]]+)\\]\\(([^)\\s]+)\\)|<(https?://[^>\\s]+)>")

// Inlines splits s into spans: code first (its content is literal), then bold, italic, links.
// An unpaired marker stays text.
func Inlines(s string) []Inline {
	var out []Inline
	for len(s) > 0 {
		loc := inlineRe.FindStringSubmatchIndex(s)
		if loc == nil {
			return append(out, Inline{Kind: Text, Text: s})
		}
		if loc[0] > 0 {
			out = append(out, Inline{Kind: Text, Text: s[:loc[0]]})
		}
		out = append(out, span2(s, loc))
		s = s[loc[1]:]
	}
	return out
}

// span2 builds the inline for the first group that matched.
func span2(s string, loc []int) Inline {
	group := func(n int) string {
		if loc[2*n] < 0 {
			return ""
		}
		return s[loc[2*n]:loc[2*n+1]]
	}
	switch {
	case loc[2] >= 0:
		return Inline{Kind: Code, Text: group(1)}
	case loc[4] >= 0, loc[6] >= 0:
		return Inline{Kind: Bold, Text: group(2) + group(3)}
	case loc[8] >= 0, loc[10] >= 0:
		return Inline{Kind: Italic, Text: group(4) + group(5)}
	case loc[12] >= 0:
		return Inline{Kind: Link, Text: group(6), URL: group(7)}
	}
	return Inline{Kind: Link, Text: group(8), URL: group(8)}
}
```

- [ ] **Step 4: Implement `split.go` and `channel.go`**

`internal/notify/split.go`:
```go
package notify

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrBlockTooLong is returned when one block alone exceeds the channel's limit (R-NTF-012).
var ErrBlockTooLong = errors.New("a block exceeds the channel's limit")

// Renderer is what the splitter needs from a channel: one block to platform text, the measure
// the platform counts in, the per-message limit (0: none) and the blocks per message (0: none).
type Renderer interface {
	Render(Block) string
	Measure(string) int
	Limit() int
	MaxBlocks() int
}

// Part is one message: its rendered blocks, joined by a blank line.
type Part struct{ Blocks []string }

// Text joins the part's blocks.
func (p Part) Text() string { return strings.Join(p.Blocks, "\n\n") }

// Split renders the blocks and packs them into parts within Limit and MaxBlocks, cutting only
// between blocks (R-NTF-011); a block over the limit by itself is ErrBlockTooLong naming it,
// before anything is sent (R-NTF-012).
func Split(r Renderer, blocks []Block) ([]Part, error) {
	var parts []Part
	var cur Part
	size := 0
	flush := func() {
		if len(cur.Blocks) > 0 {
			parts = append(parts, cur)
			cur, size = Part{}, 0
		}
	}
	for _, b := range blocks {
		text := r.Render(b)
		n := r.Measure(text)
		if r.Limit() > 0 && n > r.Limit() {
			return nil, fmt.Errorf("%w: %s %q (%d > %d)", ErrBlockTooLong, kindName(b.Kind), head(b.Raw), n, r.Limit())
		}
		sep := 0
		if len(cur.Blocks) > 0 {
			sep = r.Measure("\n\n")
		}
		if (r.Limit() > 0 && size+sep+n > r.Limit()) || (r.MaxBlocks() > 0 && len(cur.Blocks) == r.MaxBlocks()) {
			flush()
			sep = 0
		}
		cur.Blocks = append(cur.Blocks, text)
		size += sep + n
	}
	flush()
	return parts, nil
}

func kindName(k Kind) string {
	return [...]string{"paragraph", "heading", "code fence", "list", "table"}[k]
}

// head is the first 40 characters of s on one line, for an error message.
func head(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if utf8.RuneCountInString(s) > 40 {
		s = string([]rune(s)[:40]) + "…"
	}
	return s
}
```

`internal/notify/channel.go`:
```go
package notify

// Channel is the channel seam (design §2): render Markdown, post, reply in a thread by key.
// Payload is the exact body Post sends, for --dry-run (R-NTF-003); Post sends one part in the
// thread (an empty thread starts one) and returns the id a reply addresses. NeedsThread is
// true for a channel that cannot post without one (GitHub: a comment needs its issue).
type Channel interface {
	Renderer
	Kind() string
	NeedsThread() bool
	Payload(p Part, thread string) []byte
	Post(p Part, thread string) (id string, err error)
}
```

- [ ] **Step 5: Run the tests and a short fuzz**

Run: `go test ./internal/notify/ && go test ./internal/notify/ -run '^$' -fuzz FuzzParse -fuzztime 10s && go test ./internal/notify/ -run '^$' -fuzz FuzzSplit -fuzztime 10s && golangci-lint run ./internal/notify/`
Expected: PASS. `Parse` and `span` are near the `cyclop` limit; if lint objects, move the list branch of `span` into `func list(run []string, raw string) Block`.

- [ ] **Step 6: Commit**

```bash
git add internal/notify
git commit -m "feat(notify): the Markdown block model, inline spans, and the splitter that cuts only between blocks"
```

---

### Task 6: The Slack and Telegram atoms

**Files:**
- Create: `internal/atom/slack/slack.go`, `internal/atom/slack/slack_test.go`, `internal/atom/slack/testdata/golden/*.txt`, `internal/atom/telegram/telegram.go`, `internal/atom/telegram/telegram_test.go`, `internal/atom/telegram/testdata/golden/*.txt`, `internal/testkit/golden.go`

**Interfaces:**
- Consumes: `notify.Block`, `notify.Inlines`, `notify.Part`, `notify.Channel` (Task 5).
- Produces:
  - `slack.Channel{Token, ChannelID string; HTTP *http.Client}` implementing `notify.Channel`: `Kind() = "slack"`, `Limit() = 12000`, `MaxBlocks() = 50`, `Measure` = code points; `Payload` = `{"channel":…,"blocks":[{"type":"markdown","text":…},…]}` plus `"thread_ts"` when threaded; `Post` → `POST https://slack.com/api/chat.postMessage`, `Authorization: Bearer <token>`, returns the reply's `ts`; `{"ok":false,"error":E}` is an error naming E.
  - `telegram.Channel{Token, ChatID string; HTTP *http.Client}`: `Kind() = "telegram"`, `Limit() = 4096`, `Measure` = UTF-16 code units of the text with tags removed and entities decoded; `Payload` = `{"chat_id":…,"text":…,"parse_mode":"HTML"}` plus `"reply_to_message_id"` when threaded; `Post` → `POST https://api.telegram.org/bot<token>/sendMessage`, returns `result.message_id` as a string.
  - `testkit.Golden(t, path string, got string)`: compares with the file, rewrites it under `-update`.
  - `testkit.RenderConstructs(t, r notify.Renderer) map[string]string`: each `internal/notify/testdata/constructs/*.md` parsed and rendered (blocks joined by a blank line), by construct name. The constructs dir is found relative to the module root (`testkit.ModuleRoot(t)` walks up to `go.mod`).

- [ ] **Step 1: Write the failing tests**

`internal/testkit/golden.go` (test helper code in the non-test package, like `RuntimeContract`):
```go
package testkit

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

var update = flag.Bool("update", false, "rewrite golden files")

// Golden compares got with the file at path; -update rewrites it (R-NTF-030).
func Golden(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path) //nolint:gosec // a golden file of the test
	if err != nil {
		t.Fatalf("%s: %v (run with -update to write it)", path, err)
	}
	if string(want) != got {
		t.Errorf("%s differs:\n got: %q\nwant: %q", path, got, string(want))
	}
}

// ModuleRoot is the directory holding go.mod above the test's working directory.
func ModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, serr := os.Stat(filepath.Join(dir, "go.mod")); serr == nil {
			return dir
		}
		if dir == filepath.Dir(dir) {
			t.Fatal("no go.mod above the test directory")
		}
		dir = filepath.Dir(dir)
	}
}

// RenderConstructs renders every construct file through r, by construct name.
func RenderConstructs(t *testing.T, r notify.Renderer) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(ModuleRoot(t), "internal", "notify", "testdata", "constructs", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("constructs: %v, %d files", err, len(files))
	}
	out := map[string]string{}
	for _, f := range files {
		b, rerr := os.ReadFile(f) //nolint:gosec // a construct fixture
		if rerr != nil {
			t.Fatal(rerr)
		}
		var parts []string
		for _, blk := range notify.Parse(string(b)) {
			parts = append(parts, r.Render(blk))
		}
		out[strings.TrimSuffix(filepath.Base(f), ".md")] = strings.Join(parts, "\n\n")
	}
	return out
}
```

`internal/atom/slack/slack_test.go`:
```go
package slack_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/slack"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestSlackGoldenPerConstruct(t *testing.T) {
	// Slack renders to markdown block text: headings, emphasis, code, links and lists pass as
	// Markdown, a table becomes a code block (R-NTF-020, R-NTF-022).
	// covers: R-NTF-010, R-NTF-020, R-NTF-022, R-NTF-030
	for name, got := range testkit.RenderConstructs(t, slack.Channel{}) {
		testkit.Golden(t, filepath.Join("testdata", "golden", name+".txt"), got)
	}
	table := notify.Parse("| a | b |\n|---|---|\n| 1 | 2 |\n")[0]
	if got := (slack.Channel{}).Render(table); !strings.HasPrefix(got, "```\n") || !strings.Contains(got, "| 1 | 2 |") {
		t.Fatalf("table = %q; want a code block", got)
	}
}

func TestSlackLimitsAndPayload(t *testing.T) {
	// 12,000 characters of block text fit one message, 12,001 need two, 51 blocks need two;
	// the payload carries only markdown blocks and thread_ts when threaded; characters are code
	// points, so multibyte text at the limit fits (R-NTF-021, R-NTF-041).
	// covers: R-NTF-020, R-NTF-021, R-NTF-023, R-NTF-041
	c := slack.Channel{ChannelID: "C0TEST"}
	one := strings.Repeat("é", 12000)
	parts, err := notify.Split(c, notify.Parse(one))
	if err != nil || len(parts) != 1 {
		t.Fatalf("12,000 code points: %d parts, %v", len(parts), err)
	}
	parts, err = notify.Split(c, notify.Parse(strings.Repeat("x", 6000)+"\n\n"+strings.Repeat("y", 5999)))
	if err != nil || len(parts) != 2 {
		t.Fatalf("12,001 with the blank line: %d parts, %v", len(parts), err)
	}
	parts, err = notify.Split(c, notify.Parse(strings.Repeat("p\n\n", 51)))
	if err != nil || len(parts) != 2 {
		t.Fatalf("51 blocks: %d parts, %v", len(parts), err)
	}
	var body struct {
		Channel  string `json:"channel"`
		ThreadTS string `json:"thread_ts"`
		Blocks   []struct {
			Type, Text string
		} `json:"blocks"`
	}
	if err = json.Unmarshal(c.Payload(parts[0], "1700000000.000100"), &body); err != nil {
		t.Fatal(err)
	}
	if body.Channel != "C0TEST" || body.ThreadTS != "1700000000.000100" || len(body.Blocks) != 50 {
		t.Fatalf("payload = %+v", body)
	}
	for _, b := range body.Blocks {
		if b.Type != "markdown" || b.Text != "p" {
			t.Fatalf("block = %+v; want markdown p", b)
		}
	}
	if strings.Contains(string(c.Payload(parts[1], "")), "thread_ts") {
		t.Fatal("an unthreaded payload carries thread_ts")
	}
}

func TestSlackOKFalseIsAnError(t *testing.T) {
	// Review Focus 4: Slack answers 200 with ok:false; that is a failed part naming the reason.
	// covers: R-NTF-007
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, `{"ok":false,"error":"channel_not_found"}`)
	}))
	defer srv.Close()
	c := slack.Channel{Token: "test-token", ChannelID: "C0TEST", HTTP: srv.Client(), URL: srv.URL}
	if _, err := c.Post(notify.Part{Blocks: []string{"hi"}}, ""); err == nil || !strings.Contains(err.Error(), "channel_not_found") {
		t.Fatalf("Post = %v; want the Slack error", err)
	}
}
```
`slack.Channel` gets a `URL string` field (empty: `https://slack.com/api/chat.postMessage`) so this test can point at `httptest` without a transport; the contract test in Task 9 goes through `testkit.FakeHTTP` instead.

`internal/atom/telegram/telegram_test.go`:
```go
package telegram_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/telegram"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestTelegramGoldenPerConstruct(t *testing.T) {
	// HTML parse mode: <b>, <i>, <code>, <pre>, <a href>; lists as bullet lines; tables as
	// <pre>; & < > escaped in text (R-NTF-024, R-NTF-026).
	// covers: R-NTF-010, R-NTF-024, R-NTF-026, R-NTF-030
	got := testkit.RenderConstructs(t, telegram.Channel{})
	for name, text := range got {
		testkit.Golden(t, filepath.Join("testdata", "golden", name+".txt"), text)
	}
	if !strings.Contains(got["escaping"], "Tom &amp; Jerry &lt;tag&gt;") || !strings.HasPrefix(got["table"], "<pre>") ||
		!strings.Contains(got["bold"], "<b>strong</b>") || !strings.Contains(got["fence"], "<pre><code class=\"language-sh\">") {
		t.Fatalf("renders: %q", got)
	}
}

func TestTelegramMeasureAndPayload(t *testing.T) {
	// The limit is 4,096 UTF-16 units of the visible text: HTML source over 4,096 whose text is
	// under fits one part; 2,049 astral-plane emoji (4,098 units) need two; the payload has
	// parse_mode HTML and reply_to_message_id when threaded (R-NTF-025, R-NTF-027).
	// covers: R-NTF-024, R-NTF-025, R-NTF-027
	c := telegram.Channel{ChatID: "1001"}
	bold := strings.Repeat("**ab** ", 600) // 4,200 source bytes, 1,800 visible units
	parts, err := notify.Split(c, notify.Parse(bold))
	if err != nil || len(parts) != 1 {
		t.Fatalf("source over, text under: %d parts, %v", len(parts), err)
	}
	emoji := strings.Repeat("😀", 2049)
	parts, err = notify.Split(c, notify.Parse(emoji[:len("😀")*1025]+"\n\n"+emoji[len("😀")*1025:]))
	if err != nil || len(parts) != 2 {
		t.Fatalf("4,098 units in two blocks: %d parts, %v", len(parts), err)
	}
	if got := c.Measure("<b>a&amp;b</b>"); got != 3 {
		t.Fatalf("Measure = %d, want 3", got)
	}
	var body map[string]any
	if err = json.Unmarshal(c.Payload(notify.Part{Blocks: []string{"<b>x</b>"}}, "7"), &body); err != nil {
		t.Fatal(err)
	}
	if body["chat_id"] != "1001" || body["parse_mode"] != "HTML" || body["text"] != "<b>x</b>" || body["reply_to_message_id"] != float64(7) {
		t.Fatalf("payload = %v", body)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/atom/slack/ ./internal/atom/telegram/`
Expected: FAIL to compile.

- [ ] **Step 3: Implement `internal/atom/slack/slack.go`**

```go
// Package slack is the Slack channel atom: Markdown rendered to markdown blocks of
// chat.postMessage, threaded by thread_ts (design §9).
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

// API is chat.postMessage.
const API = "https://slack.com/api/chat.postMessage"

// Channel posts to one Slack channel. URL overrides API in tests.
type Channel struct {
	Token, ChannelID, URL string
	HTTP                  *http.Client
}

// Kind implements notify.Channel.
func (Channel) Kind() string { return "slack" }

// NeedsThread implements notify.Channel.
func (Channel) NeedsThread() bool { return false }

// Limit is the markdown block text per message (R-NTF-021).
func (Channel) Limit() int { return 12000 }

// MaxBlocks is the blocks per message (R-NTF-021).
func (Channel) MaxBlocks() int { return 50 }

// Measure counts code points (R-NTF-041).
func (Channel) Measure(s string) int { return utf8.RuneCountInString(s) }

// Render keeps Markdown for a markdown block (R-NTF-020); a table becomes a code block (R-NTF-022).
func (Channel) Render(b notify.Block) string {
	if b.Kind == notify.Table {
		return "```\n" + b.Raw + "\n```"
	}
	return b.Raw
}

type block struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type message struct {
	Channel  string  `json:"channel"`
	Blocks   []block `json:"blocks"`
	ThreadTS string  `json:"thread_ts,omitempty"`
}

// Payload is the exact chat.postMessage body (R-NTF-003): one markdown block per block.
func (c Channel) Payload(p notify.Part, thread string) []byte {
	m := message{Channel: c.ChannelID, ThreadTS: thread, Blocks: make([]block, 0, len(p.Blocks))}
	for _, t := range p.Blocks {
		m.Blocks = append(m.Blocks, block{Type: "markdown", Text: t})
	}
	out, _ := json.Marshal(m) //nolint:errcheck,errchkjson // plain strings never fail to encode
	return out
}

// Post sends one part and returns its ts, the thread id (R-NTF-023).
func (c Channel) Post(p notify.Part, thread string) (string, error) {
	url := c.URL
	if url == "" {
		url = API
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(c.Payload(p, thread)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }() //nolint:errcheck // read fully below
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var reply struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		TS    string `json:"ts"`
	}
	if res.StatusCode != http.StatusOK || json.Unmarshal(body, &reply) != nil || !reply.OK {
		return "", fmt.Errorf("slack: %s: %s", res.Status, strings.TrimSpace(firstOf(reply.Error, string(body))))
	}
	if reply.TS == "" {
		return "", errors.New("slack: no ts in the reply")
	}
	return reply.TS, nil
}

func firstOf(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
```

- [ ] **Step 4: Implement `internal/atom/telegram/telegram.go`**

```go
// Package telegram is the Telegram channel atom: Markdown rendered to HTML parse mode, posted
// with sendMessage, threaded as a reply (design §9).
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

// API is the Bot API base; the token and method follow.
const API = "https://api.telegram.org"

// Channel posts to one chat. URL overrides API in tests.
type Channel struct {
	Token, ChatID, URL string
	HTTP               *http.Client
}

// Kind implements notify.Channel.
func (Channel) Kind() string { return "telegram" }

// NeedsThread implements notify.Channel.
func (Channel) NeedsThread() bool { return false }

// Limit is sendMessage's text limit (R-NTF-025).
func (Channel) Limit() int { return 4096 }

// MaxBlocks implements notify.Renderer: none.
func (Channel) MaxBlocks() int { return 0 }

var tagRe = regexp.MustCompile(`<[^>]*>`)

// Measure counts UTF-16 code units of the text after entity parsing: tags removed, entities
// decoded (R-NTF-025).
func (Channel) Measure(s string) int {
	return len(utf16.Encode([]rune(html.UnescapeString(tagRe.ReplaceAllString(s, "")))))
}

// Render turns one block into HTML (R-NTF-024): headings bold, lists as bullet lines, fences
// and tables in <pre> (R-NTF-026), text escaped.
func (Channel) Render(b notify.Block) string {
	switch b.Kind {
	case notify.Heading:
		return "<b>" + inlines(b.Text) + "</b>"
	case notify.Fence:
		if b.Lang != "" {
			return `<pre><code class="language-` + html.EscapeString(b.Lang) + `">` + html.EscapeString(b.Text) + "</code></pre>"
		}
		return "<pre>" + html.EscapeString(b.Text) + "</pre>"
	case notify.Table:
		return "<pre>" + html.EscapeString(b.Raw) + "</pre>"
	case notify.List:
		lines := make([]string, 0, len(b.Items))
		for _, it := range b.Items {
			lines = append(lines, "• "+inlines(it))
		}
		return strings.Join(lines, "\n")
	case notify.Paragraph:
	}
	return inlines(b.Text)
}

func inlines(s string) string {
	var out strings.Builder
	for _, in := range notify.Inlines(s) {
		t := html.EscapeString(in.Text)
		switch in.Kind {
		case notify.Bold:
			out.WriteString("<b>" + t + "</b>")
		case notify.Italic:
			out.WriteString("<i>" + t + "</i>")
		case notify.Code:
			out.WriteString("<code>" + t + "</code>")
		case notify.Link:
			out.WriteString(`<a href="` + html.EscapeString(in.URL) + `">` + t + "</a>")
		case notify.Text:
			out.WriteString(t)
		}
	}
	return out.String()
}

type message struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
	ReplyTo   int    `json:"reply_to_message_id,omitempty"`
}

// Payload is the exact sendMessage body (R-NTF-003, R-NTF-027).
func (c Channel) Payload(p notify.Part, thread string) []byte {
	reply, _ := strconv.Atoi(thread) // "" or a message id; anything else starts a new thread
	out, _ := json.Marshal(message{ChatID: c.ChatID, Text: p.Text(), ParseMode: "HTML", ReplyTo: reply}) //nolint:errcheck,errchkjson // strings
	return out
}

// Post sends one part and returns its message id, the thread id for replies (R-NTF-027).
func (c Channel) Post(p notify.Part, thread string) (string, error) {
	base := c.URL
	if base == "" {
		base = API
	}
	url := base + "/bot" + c.Token + "/sendMessage"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(c.Payload(p, thread)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }() //nolint:errcheck // read fully below
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var reply struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	if res.StatusCode != http.StatusOK || json.Unmarshal(body, &reply) != nil || !reply.OK {
		return "", fmt.Errorf("telegram: %s: %s", res.Status, strings.TrimSpace(reply.Description+" "+string(body)))
	}
	return strconv.Itoa(reply.Result.MessageID), nil
}
```
The error message must never include the request URL (it carries the token); neither `Post` prints `url`.

- [ ] **Step 5: Write the golden files, then run the tests**

Run: `go test ./internal/atom/slack/ ./internal/atom/telegram/ -run Golden -update && go test ./internal/atom/slack/ ./internal/atom/telegram/ && golangci-lint run ./internal/atom/slack/ ./internal/atom/telegram/ ./internal/testkit/`
Expected: PASS; `git status` shows ten `testdata/golden/*.txt` per atom. Read each golden file once: the Slack `table.txt` is a fenced table, the Telegram `escaping.txt` has `&amp; &lt;tag&gt; &#34;quotes&#34;`, `link.txt` has two `<a href=…>` (the bare `<https://…>` becomes a link with the URL as text).

- [ ] **Step 6: Commit**

```bash
git add internal/atom/slack internal/atom/telegram internal/testkit/golden.go
git commit -m "feat(atom): slack and telegram channels: markdown blocks and HTML parse mode, platform limits, threads by ts and message id, golden per construct"
```

---

### Task 7: The shell runner, the GitHub and command atoms

**Files:**
- Create: `internal/shell/shell.go`, `internal/shell/shell_test.go`, `internal/atom/github/github.go`, `internal/atom/github/github_test.go`, `internal/atom/github/testdata/golden/*.txt`, `internal/atom/command/command.go`, `internal/atom/command/command_test.go`
- Modify: `internal/cli/cli.go` (`Deps.Shell`, `Deps.Transport`), `internal/cli/exec.go` (`shell()`, `transport()`)

**Interfaces:**
- Produces:
  - `shell.Cmd{Command, Dir string; Env []string; Stdin string; Timeout time.Duration}`, `shell.Result{Output []byte; Exit int; TimedOut bool}`, `shell.Runner func(Cmd) (Result, error)` (the error is "could not start"), `shell.Run(Cmd) (Result, error)`: `sh -c Command` in `Dir`, the inherited environment (git discovery variables stripped) plus `Env`, its own process group killed at `Timeout` (0: none), combined stdout and stderr in `Output`.
  - `github.Channel{Token, Repo, URL string; HTTP *http.Client}`: `Kind() = "github"`, `NeedsThread() = true`, `Limit() = 65536`, `MaxBlocks() = 0`, `Measure` = code points, `Render` = `b.Raw` (GFM as is, tables native); `Payload` = `{"body":…}`; `Post` → `POST https://api.github.com/repos/<repo>/issues/<thread>/comments`, returns the thread (the issue number) as the id.
  - `command.Channel{Command, Dir string; Run shell.Runner}`: `Kind() = "command"`, `NeedsThread() = false`, `Limit() = 0`, `Render` = `b.Raw`; `Payload` = the part's text; `Post` runs the command with the text on stdin and `NESS_THREAD_KEY=<thread>`, returns `thread`; a non-zero exit is an error carrying the output.
  - `cli.Deps.Shell shell.Runner` (nil = `shell.Run`), `cli.Deps.Transport http.RoundTripper` (nil = `http.DefaultTransport`), `(Deps).shell()`, `(Deps).transport()`, `(Deps).httpClient() *http.Client` (30 s timeout over the transport).

- [ ] **Step 1: Write the failing tests**

`internal/shell/shell_test.go`:
```go
package shell_test

import (
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

func TestRunReportsExitCodesOutputCwdEnvAndStdin(t *testing.T) {
	// The runner is what floors (P5b) and the command channel need from a process: its exit
	// code, combined output, cwd, added env and stdin (R-NTF-029, R-PRB-083).
	// covers: R-NTF-029
	dir := t.TempDir()
	r, err := shell.Run(shell.Cmd{Command: "pwd; echo $NESS_THREAD_KEY; cat; echo err >&2; exit 3",
		Dir: dir, Env: []string{"NESS_THREAD_KEY=k1"}, Stdin: "in\n"})
	if err != nil {
		t.Fatal(err)
	}
	want := dir + "\nk1\nin\nerr\n"
	if r.Exit != 3 || r.TimedOut || string(r.Output) != want {
		t.Fatalf("Result = %+v (%q), want exit 3 and %q", r, r.Output, want)
	}
	if r, err = shell.Run(shell.Cmd{Command: "exit 0", Dir: dir}); err != nil || r.Exit != 0 {
		t.Fatalf("exit 0: %+v, %v", r, err)
	}
	t.Setenv("GIT_DIR", "/nowhere")
	if r, _ = shell.Run(shell.Cmd{Command: "echo ${GIT_DIR:-unset}", Dir: dir}); strings.TrimSpace(string(r.Output)) != "unset" {
		t.Fatalf("GIT_DIR reached the command: %q", r.Output)
	}
}

func TestRunKillsTheProcessGroupAtTheTimeout(t *testing.T) {
	// A floor past its timeout is UNCLEAR and the process is gone (R-PRB-022): the whole group,
	// so a background child of the sh dies too. The test waits on the process, never sleeps.
	// covers: R-PRB-022
	r, err := shell.Run(shell.Cmd{Command: "sleep 30 & echo $!; wait", Dir: t.TempDir(), Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if !r.TimedOut {
		t.Fatalf("Result = %+v; want TimedOut", r)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(r.Output)))
	if err != nil {
		t.Fatalf("no pid in output %q", r.Output)
	}
	if kerr := syscall.Kill(pid, 0); kerr != syscall.ESRCH {
		t.Fatalf("sleep %d is still alive (kill 0: %v)", pid, kerr)
	}
	if _, err = shell.Run(shell.Cmd{Command: "true", Dir: "/nonexistent/dir"}); err == nil {
		t.Fatal("a missing dir must be a start error")
	}
}
```
(The `ESRCH` check runs right after `Run` returns: `Run` waits for the group, so a surviving `sleep` would be a bug, not a race.)

`internal/atom/github/github_test.go`:
```go
package github_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/github"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestGitHubGoldenAndPayload(t *testing.T) {
	// GFM as is: every construct renders to its own source, tables native; the body is the
	// input; the limit is 65,536 characters (R-NTF-028, R-NTF-041).
	// covers: R-NTF-010, R-NTF-028, R-NTF-030, R-NTF-041
	c := github.Channel{Repo: "o/r"}
	for name, got := range testkit.RenderConstructs(t, c) {
		testkit.Golden(t, filepath.Join("testdata", "golden", name+".txt"), got)
	}
	md := "# T\n\n| a |\n|---|\n| 1 |\n"
	parts, err := notify.Split(c, notify.Parse(md))
	if err != nil || len(parts) != 1 || string(c.Payload(parts[0], "42")) != `{"body":"# T\n\n| a |\n|---|\n| 1 |"}` {
		t.Fatalf("payload = %s, %v", c.Payload(parts[0], "42"), err)
	}
	if parts, err = notify.Split(c, notify.Parse(strings.Repeat("é", 65536))); err != nil || len(parts) != 1 {
		t.Fatalf("65,536 code points: %d parts, %v", len(parts), err)
	}
	if !c.NeedsThread() {
		t.Fatal("github posts need an issue")
	}
}

func TestGitHubPostsACommentOnTheThreadIssue(t *testing.T) {
	// covers: R-NTF-028
	var path, auth, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":1}`)
	}))
	defer srv.Close()
	c := github.Channel{Token: "test-token", Repo: "o/r", URL: srv.URL, HTTP: srv.Client()}
	id, err := c.Post(notify.Part{Blocks: []string{"hi"}}, "42")
	if err != nil || id != "42" || path != "/repos/o/r/issues/42/comments" || auth != "Bearer test-token" || body != `{"body":"hi"}` {
		t.Fatalf("Post = %q, %v; path %q auth %q body %q", id, err, path, auth, body)
	}
	if _, err = c.Post(notify.Part{Blocks: []string{"hi"}}, ""); err == nil {
		t.Fatal("a post without a thread must fail")
	}
}
```

`internal/atom/command/command_test.go`:
```go
package command_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/command"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

func TestCommandGetsRawMarkdownOnStdinAndTheKeyInEnv(t *testing.T) {
	// covers: R-NTF-029
	dir := t.TempDir()
	c := command.Channel{Command: "cat > out; env > env", Dir: dir, Run: shell.Run}
	md := "# T\n\n" + strings.Repeat("x", 100000) + "\n\n| a |\n|---|\n| 1 |"
	parts, err := notify.Split(c, notify.Parse(md))
	if err != nil || len(parts) != 1 {
		t.Fatalf("no limit: %d parts, %v", len(parts), err)
	}
	if string(c.Payload(parts[0], "k")) != strings.TrimSuffix(md, "") {
		t.Fatalf("payload differs from the input")
	}
	id, err := c.Post(parts[0], "rel-1")
	if err != nil || id != "rel-1" {
		t.Fatalf("Post = %q, %v", id, err)
	}
	out, _ := os.ReadFile(filepath.Join(dir, "out"))
	env, _ := os.ReadFile(filepath.Join(dir, "env"))
	if string(out) != md || !strings.Contains(string(env), "NESS_THREAD_KEY=rel-1\n") {
		t.Fatalf("out %q env has key: %v", out, strings.Contains(string(env), "NESS_THREAD_KEY=rel-1"))
	}
	c.Command = "echo boom >&2; exit 2"
	if _, err = c.Post(parts[0], ""); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("a failing command: %v", err)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/shell/ ./internal/atom/github/ ./internal/atom/command/`
Expected: FAIL to compile.

- [ ] **Step 3: Implement `internal/shell/shell.go`**

```go
// Package shell runs one command the way a probe floor and the command channel need it: sh -c,
// stdin, added env, combined output, the exit code, a timeout that kills the whole process group.
// The Exec seam (stdout only, no timeout) cannot carry these; this is the second seam.
package shell

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Cmd is one command: run by sh -c in Dir with the inherited environment (git discovery
// variables stripped) plus Env, Stdin on its stdin, killed with its group after Timeout (0: never).
type Cmd struct {
	Command, Dir string
	Env          []string
	Stdin        string
	Timeout      time.Duration
}

// Result is what the command did: its combined output, exit code, and whether it was killed.
type Result struct {
	Output   []byte
	Exit     int
	TimedOut bool
}

// Runner is the seam: Run in production, a fake in the worker tests.
type Runner func(Cmd) (Result, error)

// Run runs c. The error is only "could not start"; a non-zero exit or a timeout is in Result.
func Run(c Cmd) (Result, error) {
	ctx, cancel := context.Background(), func() {}
	if c.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", c.Command) //nolint:gosec // the configured command
	cmd.Dir = c.Dir
	cmd.Env = append(store.CleanGitEnv(os.Environ()), c.Env...)
	cmd.Stdin = bytes.NewReader([]byte(c.Stdin))
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	r := Result{Output: out.Bytes(), TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded)}
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		r.Exit = ee.ExitCode()
		if r.Exit < 0 { // killed by a signal
			r.Exit = 128 + int(ee.Sys().(syscall.WaitStatus).Signal()) //nolint:forcetypeassert // Linux only (R-SESS-015)
		}
	case r.TimedOut:
		r.Exit = 137
	default:
		return Result{}, err
	}
	return r, nil
}
```

- [ ] **Step 4: Implement `internal/atom/github/github.go` and `internal/atom/command/command.go`**

```go
// Package github is the GitHub channel atom: GFM as is, one comment per part on the issue or
// pull request the thread key names (design §9).
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

// API is the REST base. URL overrides it in tests.
const API = "https://api.github.com"

// Channel comments on issues of one repository (owner/name).
type Channel struct {
	Token, Repo, URL string
	HTTP             *http.Client
}

// Kind implements notify.Channel.
func (Channel) Kind() string { return "github" }

// NeedsThread is true: a comment needs its issue or PR number (R-NTF-028).
func (Channel) NeedsThread() bool { return true }

// Limit is the comment body limit (R-NTF-028).
func (Channel) Limit() int { return 65536 }

// MaxBlocks implements notify.Renderer: none.
func (Channel) MaxBlocks() int { return 0 }

// Measure counts characters (R-NTF-041).
func (Channel) Measure(s string) int { return utf8.RuneCountInString(s) }

// Render is the identity: GFM as is, tables native (R-NTF-028).
func (Channel) Render(b notify.Block) string { return b.Raw }

// Payload is the exact comment body (R-NTF-003).
func (Channel) Payload(p notify.Part, _ string) []byte {
	out, _ := json.Marshal(map[string]string{"body": p.Text()}) //nolint:errcheck,errchkjson // strings
	return out
}

// Post comments on issue thread and returns thread: the issue is the thread (R-NTF-028).
func (c Channel) Post(p notify.Part, thread string) (string, error) {
	if thread == "" {
		return "", errors.New("github: a post is a comment on an issue or PR; pass --thread NUMBER")
	}
	base := c.URL
	if base == "" {
		base = API
	}
	url := fmt.Sprintf("%s/repos/%s/issues/%s/comments", base, c.Repo, thread)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewReader(c.Payload(p, thread)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }() //nolint:errcheck // read fully below
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("github: %s: %s", res.Status, strings.TrimSpace(string(body)))
	}
	return thread, nil
}
```

```go
// Package command is the command channel atom: raw Markdown on stdin, the thread key in
// NESS_THREAD_KEY, no length limit (design §9, R-NTF-029).
package command

import (
	"fmt"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

// Channel runs Command in Dir through Run for every post.
type Channel struct {
	Command, Dir string
	Run          shell.Runner
}

// Kind implements notify.Channel.
func (Channel) Kind() string { return "command" }

// NeedsThread implements notify.Channel.
func (Channel) NeedsThread() bool { return false }

// Limit is none: one part (R-NTF-029).
func (Channel) Limit() int { return 0 }

// MaxBlocks implements notify.Renderer: none.
func (Channel) MaxBlocks() int { return 0 }

// Measure implements notify.Renderer.
func (Channel) Measure(s string) int { return len(s) }

// Render is the identity: raw Markdown.
func (Channel) Render(b notify.Block) string { return b.Raw }

// Payload is the stdin text.
func (Channel) Payload(p notify.Part, _ string) []byte { return []byte(p.Text()) }

// Post runs the command with the text on stdin and the key in NESS_THREAD_KEY; the key is the
// thread id.
func (c Channel) Post(p notify.Part, thread string) (string, error) {
	r, err := c.Run(shell.Cmd{Command: c.Command, Dir: c.Dir, Env: []string{"NESS_THREAD_KEY=" + thread}, Stdin: p.Text()})
	if err != nil {
		return "", fmt.Errorf("command channel: %w", err)
	}
	if r.Exit != 0 {
		return "", fmt.Errorf("command channel exited %d: %s", r.Exit, strings.TrimSpace(string(r.Output)))
	}
	return thread, nil
}
```

`internal/cli/cli.go` `Deps` gains:
```go
	// Shell runs commands with stdin, a timeout and an exit code (the command channel, probe
	// floors); nil means shell.Run. Scripts keep the real one: the commands are their own.
	Shell shell.Runner
	// Transport carries every HTTP request (the channel atoms); nil means http.DefaultTransport.
	// Tests inject testkit.FakeHTTP, which records and never reaches the network.
	Transport http.RoundTripper
```
`internal/cli/exec.go`:
```go
// shell returns the process's command runner.
func (d Deps) shell() shell.Runner {
	if d.Shell == nil {
		return shell.Run
	}
	return d.Shell
}

// httpClient is the client the channel atoms post with: the injected transport, 30 s per request.
func (d Deps) httpClient() *http.Client {
	t := d.Transport
	if t == nil {
		t = http.DefaultTransport
	}
	return &http.Client{Transport: t, Timeout: 30 * time.Second}
}
```

- [ ] **Step 5: Write the GitHub golden files, run everything**

Run: `go test ./internal/atom/github/ -run Golden -update && go test ./internal/shell/ ./internal/atom/... && go build ./... && golangci-lint run ./internal/shell/ ./internal/atom/github/ ./internal/atom/command/ ./internal/cli/`
Expected: PASS; the GitHub golden files equal their construct sources minus the trailing newline.

- [ ] **Step 6: Commit**

```bash
git add internal/shell internal/atom/github internal/atom/command internal/cli/cli.go internal/cli/exec.go
git commit -m "feat(atom): github and command channels over the shell runner; Deps gains the shell and transport seams"
```

---

### Task 8: The poster, migration 4, the fake HTTP transport and `ness post`

**Files:**
- Modify: `internal/store/migrate.go` (migration 4), `internal/cli/atoms.go` (`buildChannel`, `buildPoster`), `internal/cli/features.go` (delete the `newPostCmd` stub), `internal/cli/script_test.go` (`Transport`), `internal/testkit/exec.go` (`fake http` verb), `internal/cli/testdata/script/send.txtar` (R-NTF-006 line)
- Create: `internal/store/notify.go`, `internal/store/notify_test.go`, `internal/notify/post.go`, `internal/notify/post_test.go`, `internal/testkit/http.go`, `internal/cli/post.go`, `internal/cli/testdata/script/post.txtar`, `internal/cli/testdata/script/post_command.txtar`

**Interfaces:**
- Consumes: `notify.Channel`, `notify.Split`, `notify.Parse` (Tasks 5–7); the atoms (Tasks 6–7); `requireFeature` (Task 1).
- Produces:
  - Migration 4: `threads (channel, key, id, PRIMARY KEY (channel, key))`, `posts (key PRIMARY KEY, channel, parts, sent, created_at, updated_at)`.
  - `(*store.Store).Thread(channel, key string) (string, error)` ("" when unknown), `SetThread(channel, key, id string) error`, `BeginPost(key, channel string, parts int, now time.Time) (sent int, err error)` (inserts or returns the recorded progress), `MarkSent(key string, sent int, now time.Time) error`, `Posts() ([]Post, error)` with `store.Post{Key, Channel string; Parts, Sent int}`.
  - `notify.Poster{Channel func(name string) (Channel, error); Store *store.Store; Now func() time.Time}`; `notify.Request{Channel, Markdown, Verbatim, Thread string; DryRun io.Writer}`; `(Poster).Post(r Request) (parts int, err error)` (the parts sent, or printed on a dry run); `notify.PostKey(channel, thread string, parts []Part) string`; `notify.PartialError{Sent, Parts int; Err error}` (`Error()`: `part 2 of 3 failed: …; part 1 was sent; run the same post again to resume at part 2`); `notify.ErrEmpty`.
  - `testkit.FakeHTTP{Dir string}` (`http.RoundTripper`): records every request in `<Dir>/http.json` (`{"requests":[{"method","url","body"}],"fail":N}`), answers Slack, Telegram and GitHub with increasing ids, fails the next `fail` requests with 500, and errors on any other host. Script verbs: `fake http requests N` (negatable), `fake http body N FILE` (the Nth body plus a newline), `fake http url N VAR`, `fake http fail N`.
  - `cli.buildChannel(d Deps, cfg *config.Config, root, name string) (notify.Channel, error)`: the atom for `[notify.channels.NAME]` with the token from `os.Getenv(TokenEnv)`; `ErrNoChannel` names the configured channels; a missing token is a refusal naming the variable.
  - `cli.buildPoster(d Deps, cfg *config.Config, root string, s *store.Store) notify.Poster`.
  - `ness post --channel NAME --file MD [--thread KEY] [--dry-run]`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/store/notify_test.go` (new file):
```go
package store_test

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestThreadsAndPostProgress(t *testing.T) {
	// The thread key → platform id mapping is per channel (R-NTF-040); a post's progress is
	// kept by key so a retry resumes at the first unsent part (R-NTF-007).
	// covers: R-NTF-040, R-NTF-007
	s := tree(t)
	if id, err := s.Thread("rel", "v1"); err != nil || id != "" {
		t.Fatalf("unknown thread = %q, %v", id, err)
	}
	if err := s.SetThread("rel", "v1", "1700.1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetThread("other", "v1", "9"); err != nil {
		t.Fatal(err)
	}
	if id, _ := s.Thread("rel", "v1"); id != "1700.1" {
		t.Fatalf("Thread = %q", id)
	}
	sent, err := s.BeginPost("k1", "rel", 3, now)
	if err != nil || sent != 0 {
		t.Fatalf("BeginPost = %d, %v", sent, err)
	}
	if err = s.MarkSent("k1", 1, now); err != nil {
		t.Fatal(err)
	}
	if sent, err = s.BeginPost("k1", "rel", 3, now); err != nil || sent != 1 {
		t.Fatalf("BeginPost again = %d, %v", sent, err)
	}
	posts, err := s.Posts()
	if err != nil || len(posts) != 1 || posts[0].Sent != 1 || posts[0].Parts != 3 || posts[0].Channel != "rel" {
		t.Fatalf("Posts = %+v, %v", posts, err)
	}
}
```
Also extend the existing migration test in `internal/store/store_test.go` that asserts `store.Version()`: it now expects 4 (find it with `grep -n 'Version()' internal/store/*_test.go`).

`internal/notify/post_test.go`:
```go
package notify_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// fakeChannel records posts; failAt makes that part (1-based) fail once.
type fakeChannel struct {
	plain
	posts   []string // "<thread>|<text>"
	failAt  int
	next    int
	thread  bool
}

func (f *fakeChannel) Kind() string       { return "fake" }
func (f *fakeChannel) NeedsThread() bool  { return f.thread }
func (f *fakeChannel) Payload(p notify.Part, thread string) []byte {
	return []byte("[" + thread + "]" + p.Text())
}

func (f *fakeChannel) Post(p notify.Part, thread string) (string, error) {
	if len(f.posts)+1 == f.failAt {
		f.failAt = 0
		return "", errors.New("boom")
	}
	f.posts = append(f.posts, thread+"|"+p.Text())
	f.next++
	return "id" + strings.Repeat("0", f.next), nil
}

func poster(t *testing.T, ch notify.Channel) (notify.Poster, *store.Store) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return notify.Poster{
		Channel: func(name string) (notify.Channel, error) {
			if name != "rel" {
				return nil, notify.ErrNoChannel
			}
			return ch, nil
		},
		Store: s, Now: func() time.Time { return time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC) },
	}, s
}

func TestPostThreadsByKeyAndRecordsTheMapping(t *testing.T) {
	// The first post with a new key starts the thread and records the platform id; the second
	// replies in it (R-NTF-005, R-NTF-040).
	// covers: R-NTF-005, R-NTF-040
	ch := &fakeChannel{plain: plain{limit: 100}}
	p, s := poster(t, ch)
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "one", Thread: "v1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "two", Thread: "v1"}); err != nil {
		t.Fatal(err)
	}
	if len(ch.posts) != 2 || ch.posts[0] != "|one" || ch.posts[1] != "id0|two" {
		t.Fatalf("posts = %v", ch.posts)
	}
	if id, _ := s.Thread("fake", "v1"); id != "id0" {
		t.Fatalf("recorded thread = %q", id)
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "three"}); err != nil || ch.posts[2] != "|three" {
		t.Fatalf("unthreaded post: %v %v", err, ch.posts)
	}
}

func TestPostResumesAtTheFirstUnsentPart(t *testing.T) {
	// A multi-part post records each sent part; a failure names the parts sent; the retry
	// sends only the rest and never reposts a sent part (R-NTF-007).
	// covers: R-NTF-007
	ch := &fakeChannel{plain: plain{limit: 5}, failAt: 2}
	p, _ := poster(t, ch)
	md := "aaaa\n\nbbbb\n\ncccc"
	_, err := p.Post(notify.Request{Channel: "rel", Markdown: md})
	var pe *notify.PartialError
	if !errors.As(err, &pe) || pe.Sent != 1 || pe.Parts != 3 || !strings.Contains(err.Error(), "part 1 was sent") {
		t.Fatalf("first attempt: %v", err)
	}
	if _, err = p.Post(notify.Request{Channel: "rel", Markdown: md}); err != nil {
		t.Fatal(err)
	}
	if len(ch.posts) != 3 || ch.posts[0] != "|aaaa" || ch.posts[1] != "|bbbb" || ch.posts[2] != "|cccc" {
		t.Fatalf("posts = %v; want each part once", ch.posts)
	}
	if _, err = p.Post(notify.Request{Channel: "rel", Markdown: md}); err != nil || len(ch.posts) != 3 {
		t.Fatalf("a completed post sends nothing again: %v, %d posts", err, len(ch.posts))
	}
}

func TestPostDryRunVerbatimAndRefusals(t *testing.T) {
	// --dry-run prints each part's payload and posts nothing (R-NTF-003); a verbatim note is one
	// part as is (R-NTF-004); an over-long block fails before anything is sent (R-NTF-012); an
	// unknown channel, an empty note and a thread-needing channel without a thread refuse.
	// covers: R-NTF-003, R-NTF-004, R-NTF-012, R-NTF-002
	ch := &fakeChannel{plain: plain{limit: 5}}
	p, _ := poster(t, ch)
	var dry bytes.Buffer
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "aaaa\n\nbbbb", Thread: "t", DryRun: &dry}); err != nil {
		t.Fatal(err)
	}
	if dry.String() != "[t]aaaa\n[t]bbbb\n" || len(ch.posts) != 0 {
		t.Fatalf("dry run printed %q, posts %v", dry.String(), ch.posts)
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "x", Verbatim: "**raw** stays"}); err != nil || ch.posts[0] != "|**raw** stays" {
		t.Fatalf("verbatim: %v %v", err, ch.posts)
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "toolongblock"}); !errors.Is(err, notify.ErrBlockTooLong) || len(ch.posts) != 1 {
		t.Fatalf("over-long: %v, %d posts", err, len(ch.posts))
	}
	if _, err := p.Post(notify.Request{Channel: "nope", Markdown: "x"}); !errors.Is(err, notify.ErrNoChannel) {
		t.Fatalf("unknown channel: %v", err)
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "\n\n"}); !errors.Is(err, notify.ErrEmpty) {
		t.Fatalf("empty note: %v", err)
	}
	ch.thread = true
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "x"}); err == nil || !strings.Contains(err.Error(), "--thread") {
		t.Fatalf("thread needed: %v", err)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/store/ ./internal/notify/ -run 'TestThreads|TestPost'`
Expected: FAIL to compile.

- [ ] **Step 3: Migration 4 and `internal/store/notify.go`**

Append to `migrations` in `internal/store/migrate.go`:
```go
	// Migration 4 (P5a): the thread key → platform id mapping per channel (R-NTF-040) and the
	// progress of multi-part posts, so a retry resumes at the first unsent part (R-NTF-007).
	`CREATE TABLE threads (
		channel TEXT NOT NULL,
		key     TEXT NOT NULL,
		id      TEXT NOT NULL,
		PRIMARY KEY (channel, key)
	);
	CREATE TABLE posts (
		key        TEXT PRIMARY KEY,
		channel    TEXT NOT NULL,
		parts      INTEGER NOT NULL,
		sent       INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`,
```
and update the comment above the slice: `P3 appended migration 2, P3b migration 3, P5a migration 4`.

`internal/store/notify.go`:
```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Post is the progress of one multi-part post.
type Post struct {
	Key, Channel string
	Parts, Sent  int
}

// Thread returns the platform thread id recorded for key on channel, "" when none (R-NTF-040).
func (s *Store) Thread(channel, key string) (string, error) {
	var id string
	err := s.db.QueryRowContext(context.Background(),
		`SELECT id FROM threads WHERE channel = ? AND key = ?`, channel, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// SetThread records the thread id of key on channel; a first post starts the thread.
func (s *Store) SetThread(channel, key, id string) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO threads (channel, key, id) VALUES (?, ?, ?)
		 ON CONFLICT (channel, key) DO UPDATE SET id = excluded.id`, channel, key, id)
	return err
}

// BeginPost records a post of parts parts under key, or returns how many parts an earlier
// attempt already sent (R-NTF-007).
func (s *Store) BeginPost(key, channel string, parts int, now time.Time) (sent int, err error) {
	err = s.inTx(func(tx *sql.Tx) error {
		qerr := tx.QueryRowContext(context.Background(), `SELECT sent FROM posts WHERE key = ?`, key).Scan(&sent)
		if qerr == nil {
			return nil
		}
		if !errors.Is(qerr, sql.ErrNoRows) {
			return qerr
		}
		_, qerr = tx.ExecContext(context.Background(),
			`INSERT INTO posts (key, channel, parts, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			key, channel, parts, stamp(now), stamp(now))
		return qerr
	})
	return sent, err
}

// MarkSent records that the first sent parts of the post went out.
func (s *Store) MarkSent(key string, sent int, now time.Time) error {
	return s.touch(errors.New("no such post"),
		`UPDATE posts SET sent = ?, updated_at = ? WHERE key = ?`, sent, stamp(now), key)
}

// Posts lists every recorded post, oldest first.
func (s *Store) Posts() (out []Post, err error) {
	rows, err := s.db.QueryContext(context.Background(), `SELECT key, channel, parts, sent FROM posts ORDER BY created_at, rowid`)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var p Post
		if serr := rows.Scan(&p.Key, &p.Channel, &p.Parts, &p.Sent); serr != nil {
			return nil, serr
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: `internal/notify/post.go`**

```go
package notify

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

var (
	// ErrNoChannel is returned for a channel name not in [notify.channels] (R-NTF-002).
	ErrNoChannel = errors.New("no such channel")
	// ErrEmpty is returned for a note with no blocks: nothing to send (Review Focus 5).
	ErrEmpty = errors.New("the note is empty")
)

// PartialError is a post that stopped at a part: the parts before it were sent and recorded,
// so the same post resumes at Sent+1 (R-NTF-007).
type PartialError struct {
	Sent, Parts int
	Err         error
}

func (e *PartialError) Error() string {
	sent := "nothing was sent"
	switch e.Sent {
	case 1:
		sent = "part 1 was sent"
	default:
		if e.Sent > 1 {
			sent = fmt.Sprintf("parts 1-%d were sent", e.Sent)
		}
	}
	return fmt.Sprintf("part %d of %d failed: %v; %s; run the same post again to resume at part %d",
		e.Sent+1, e.Parts, e.Err, sent, e.Sent+1)
}

func (e *PartialError) Unwrap() error { return e.Err }

// Poster posts one note: the channel by name (built by the caller, credentials included), the
// store for threads and progress, the clock.
type Poster struct {
	Channel func(name string) (Channel, error)
	Store   *store.Store
	Now     func() time.Time
}

// Request is one ness post: the note, the verbatim text that replaces its rendering when set
// (R-NTF-004), the thread key, and the writer a dry run prints payloads to (nil: a real post).
type Request struct {
	Channel, Markdown, Verbatim, Thread string
	DryRun                              io.Writer
}

// PostKey identifies a post for resumption: channel, thread key and every rendered part.
func PostKey(channel, thread string, parts []Part) string {
	h := sha256.New()
	for _, s := range append([]string{channel, thread}, texts(parts)...) {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func texts(parts []Part) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, p.Text())
	}
	return out
}

// Post renders, splits, prints (dry run) or sends each part in order, recording the thread
// after the first and the progress after every part (R-NTF-003, R-NTF-005, R-NTF-007). It
// returns how many parts the note has.
func (p Poster) Post(r Request) (int, error) {
	ch, err := p.Channel(r.Channel)
	if err != nil {
		return 0, err
	}
	parts, err := p.parts(ch, r)
	if err != nil {
		return 0, err
	}
	if ch.NeedsThread() && r.Thread == "" {
		return 0, fmt.Errorf("channel %s posts into a thread; pass --thread KEY", r.Channel)
	}
	thread, err := p.thread(ch, r)
	if err != nil {
		return 0, err
	}
	if r.DryRun != nil {
		for _, part := range parts {
			if _, werr := fmt.Fprintf(r.DryRun, "%s\n", ch.Payload(part, thread)); werr != nil {
				return 0, werr
			}
		}
		return len(parts), nil
	}
	return len(parts), p.send(ch, r, parts, thread)
}

// thread is the recorded platform thread of the key, "" for none; a channel that needs one
// (GitHub) takes the key itself as the thread.
func (p Poster) thread(ch Channel, r Request) (string, error) {
	if r.Thread == "" {
		return "", nil
	}
	thread, err := p.Store.Thread(r.Channel, r.Thread)
	if err != nil {
		return "", err
	}
	if thread == "" && ch.NeedsThread() {
		thread = r.Thread
	}
	return thread, nil
}

// parts is the verbatim text as one part, or the rendered and split note.
func (p Poster) parts(ch Channel, r Request) ([]Part, error) {
	if r.Verbatim != "" {
		return []Part{{Blocks: []string{r.Verbatim}}}, nil
	}
	blocks := Parse(r.Markdown)
	if len(blocks) == 0 {
		return nil, ErrEmpty
	}
	return Split(ch, blocks)
}

func (p Poster) send(ch Channel, r Request, parts []Part, thread string) error {
	key := PostKey(r.Channel, r.Thread, parts)
	sent, err := p.Store.BeginPost(key, r.Channel, len(parts), p.Now())
	if err != nil {
		return err
	}
	for i := sent; i < len(parts); i++ {
		id, perr := ch.Post(parts[i], thread)
		if perr != nil {
			return &PartialError{Sent: i, Parts: len(parts), Err: perr}
		}
		if merr := p.Store.MarkSent(key, i+1, p.Now()); merr != nil {
			return merr
		}
		if thread == "" && r.Thread != "" {
			thread = id
			if terr := p.Store.SetThread(r.Channel, r.Thread, id); terr != nil {
				return terr
			}
		}
	}
	return nil
}
```


- [ ] **Step 5: Run the unit tests**

Run: `go test ./internal/store/ ./internal/notify/ && golangci-lint run ./internal/store/ ./internal/notify/`
Expected: PASS.

- [ ] **Step 6: Commit the seam half**

```bash
git add internal/store/migrate.go internal/store/notify.go internal/store/notify_test.go internal/store/store_test.go internal/notify/post.go internal/notify/post_test.go
git commit -m "feat(notify): the poster threads by key, records every sent part and resumes; migration 4 adds threads and posts"
```

- [ ] **Step 7: Write the failing scripts**

`internal/cli/testdata/script/post.txtar`:
```
# ness post takes --channel and --file (both required), optionally --thread and --dry-run;
# refuses an unknown channel listing the configured ones; any session or the operator may post;
# the token comes from token_env and a missing variable refuses before any request; --dry-run
# prints exactly the bodies a real post sends and makes no request; note.<channel>.md is sent
# verbatim; a thread key is recorded in the store and reused by a second process; an empty note
# refuses; ness send never posts.
# covers: R-NTF-001, R-NTF-002, R-NTF-003, R-NTF-004, R-NTF-008, R-NTF-040, R-NTF-042, R-NTF-023, R-NTF-006
cd repo
exec ness init --gate true --base main --yes
exec sh -c 'cat ../channels.toml >> .ness/ness.toml'
exec git add -A
exec git commit -q -m 'ness init'
# Flags.
exits 2 ness post --file ../note.md
stderr 'channel is required'
exits 2 ness post --channel releases
stderr 'file is required'
exits 4 ness post --channel nope --file ../note.md
stderr 'no such channel nope; configured: announce, releases'
# Missing token: refused, no request.
exits 4 ness post --channel releases --file ../note.md
stderr 'SLACK_TOKEN is not set'
fake http requests 0
env SLACK_TOKEN=test-token
# Dry run prints the body and makes no request; the real post sends that body.
exec ness post --channel releases --file ../note.md --thread v1 --dry-run
cp stdout ../dry
fake http requests 0
exec ness post --channel releases --file ../note.md --thread v1
stdout '^posted 1 part to releases$'
fake http requests 1
fake http body 1 ../sent
cmp ../dry ../sent
grep '"type":"markdown"' ../sent
! grep 'thread_ts' ../sent
# The second post with the same key replies in the thread recorded by the first, from a new process.
exec ness post --channel releases --file ../note2.md --thread v1
fake http requests 2
fake http body 2 ../sent2
grep '"thread_ts":"1700000000.000001"' ../sent2
# A session posts too.
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness post --channel releases --file ../../../note.md'
fake http requests 3
# note.<channel>.md next to the note goes verbatim.
exec ness post --channel releases --file ../pkg/note.md
fake http body 4 ../sent4
grep 'hand-tuned for slack' ../sent4
! grep 'rendered' ../sent4
# An empty note refuses before any request.
exits 2 ness post --channel releases --file ../empty.md
stderr 'empty'
fake http requests 4
# send never posts.
exec ness send --to $R --body 'please post the note'
fake http requests 4
# Outside a project: refused.
cd $WORK
exits 4 ness post --channel releases --file note.md

-- channels.toml --
features = ["notify"]
[notify.channels.releases]
kind = "slack"
token_env = "SLACK_TOKEN"
channel_id = "C0TEST"
[notify.channels.announce]
kind = "telegram"
token_env = "TG_TOKEN"
chat_id = "1001"
-- note.md --
# Release 1.2

Shipped **today**.
-- note2.md --
Follow-up.
-- pkg/note.md --
rendered text
-- pkg/note.slack.md --
hand-tuned for slack
-- empty.md --


-- b.md --
b
```

`internal/cli/testdata/script/post_command.txtar`:
```
# The command channel gets raw Markdown on stdin and the thread key in NESS_THREAD_KEY, with
# no length limit.
# covers: R-NTF-029
cd repo
exec ness init --gate true --base main --yes
exec sh -c 'cat ../channels.toml >> .ness/ness.toml'
exec git add -A
exec git commit -q -m 'ness init'
exec sh -c 'yes "a long line of text" | head -5000 > ../big.md'
exec ness post --channel log --file ../big.md --thread k9
cmp out ../big.md
grep '^NESS_THREAD_KEY=k9$' env

-- channels.toml --
features = ["notify"]
[notify.channels.log]
kind = "command"
command = "cat > out; env > env"
```
(The resume after a failed part, R-NTF-007, is the `contract(fake)` unit `TestPostResumesAtTheFirstUnsentPart`; the command channel has no limit, so it never splits.)

Append to `internal/cli/testdata/script/send.txtar`, after its last `exec ness send` line: `fake http requests 0` with `# covers: R-NTF-006` added to its header marker line.

- [ ] **Step 8: Run them to verify they fail**

Run: `go test ./internal/cli/ -run 'TestScripts/(post|send)'`
Expected: FAIL (`fake http` unknown; `post` is the stub).

- [ ] **Step 9: `internal/testkit/http.go`**

```go
package testkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"
)

// httpState is http.json: every request made and how many of the next ones fail.
type httpState struct {
	Requests []Recorded `json:"requests"`
	Fail     int        `json:"fail"`
}

// Recorded is one request the fake saw.
type Recorded struct {
	Method string `json:"method"`
	URL    string `json:"url"`
	Body   string `json:"body"`
}

// FakeHTTP stands in for the network: it records every request in Dir/http.json, answers the
// three platforms with increasing ids, fails the next Fail requests with 500 and refuses any
// other host, so no test can reach a real service (R-QA-024).
type FakeHTTP struct{ Dir string }

func (f FakeHTTP) path() string { return filepath.Join(f.Dir, "http.json") }

// RoundTrip implements http.RoundTripper.
func (f FakeHTTP) RoundTrip(req *http.Request) (*http.Response, error) {
	var st httpState
	if err := loadJSON(f.path(), &st); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	st.Requests = append(st.Requests, Recorded{Method: req.Method, URL: req.URL.String(), Body: string(body)})
	n := len(st.Requests)
	status, reply := http.StatusOK, ""
	switch {
	case st.Fail > 0:
		st.Fail--
		status, reply = http.StatusInternalServerError, `{"ok":false,"error":"fake failure"}`
	case req.URL.Host == "slack.com":
		reply = fmt.Sprintf(`{"ok":true,"ts":"1700000000.%06d"}`, n)
	case req.URL.Host == "api.telegram.org":
		reply = fmt.Sprintf(`{"ok":true,"result":{"message_id":%d}}`, n)
	case req.URL.Host == "api.github.com":
		status, reply = http.StatusCreated, fmt.Sprintf(`{"id":%d}`, n)
	default:
		return nil, fmt.Errorf("fake http: no route to %s (tests never reach the network)", req.URL.Host)
	}
	if err = saveJSON(f.path(), st); err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: status, Status: strconv.Itoa(status) + " " + http.StatusText(status),
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:   io.NopCloser(bytes.NewReader([]byte(reply))), Request: req,
	}, nil
}

// Requests returns what was recorded so far.
func (f FakeHTTP) Requests() ([]Recorded, error) {
	var st httpState
	err := loadJSON(f.path(), &st)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return st.Requests, err
}

// httpCmd is `fake http`:
//
//	fake http requests N    fail unless N requests were made (negatable)
//	fake http body N FILE   write the Nth request's body plus a newline to FILE
//	fake http url N VAR     set $VAR to the Nth request's URL
//	fake http fail N        the next N requests answer 500
func httpCmd(ts *testscript.TestScript, neg bool, dir string, args []string) {
	f := FakeHTTP{Dir: dir}
	if len(args) < 2 {
		ts.Fatalf("usage: fake http requests N | body N FILE | url N VAR | fail N")
	}
	reqs, err := f.Requests()
	ts.Check(err)
	n, err := strconv.Atoi(args[1])
	ts.Check(err)
	switch args[0] {
	case "requests":
		if (len(reqs) == n) == neg {
			ts.Fatalf("%d requests recorded, want %d", len(reqs), n)
		}
	case "body", "url":
		if neg || len(args) != 3 || n < 1 || n > len(reqs) {
			ts.Fatalf("fake http %s N FILE|VAR with 1 <= N <= %d", args[0], len(reqs))
		}
		if args[0] == "url" {
			ts.Setenv(args[2], reqs[n-1].URL)
			return
		}
		ts.Check(os.WriteFile(ts.MkAbs(args[2]), []byte(reqs[n-1].Body+"\n"), 0o600))
	case "fail":
		var st httpState
		_ = loadJSON(f.path(), &st) //nolint:errcheck // a missing file is an empty state
		st.Fail = n
		ts.Check(saveJSON(f.path(), st))
	default:
		ts.Fatalf("fake http: unknown subcommand %s", args[0])
	}
}
```
`loadJSON` exists in `forge.go`. In `internal/testkit/exec.go` `fakeCmd` add `case "http": httpCmd(ts, neg, dir, args[1:])` and the line `fake http SUB ...  the fake network (see httpCmd)` to its doc comment. In `internal/cli/script_test.go`, next to `d.Forge = …`, add `d.Transport = testkit.FakeHTTP{Dir: dir}`.

- [ ] **Step 10: `buildChannel`, `buildPoster` and `internal/cli/post.go`**

Append to `internal/cli/atoms.go`:
```go
// buildChannel is the atom of [notify.channels.NAME] with its credential from the variable
// token_env names (R-NTF-042, R-CFG-046). An unknown name lists the configured ones (R-NTF-002).
func buildChannel(d Deps, cfg *config.Config, root, name string) (notify.Channel, error) {
	ch, ok := cfg.Notify.Channels[name]
	if !ok {
		return nil, fmt.Errorf("%w %s; configured: %s", notify.ErrNoChannel, name,
			strings.Join(slices.Sorted(maps.Keys(cfg.Notify.Channels)), ", "))
	}
	token := ""
	if ch.TokenEnv != "" {
		if token = os.Getenv(ch.TokenEnv); token == "" {
			return nil, refuse("channel %s: %s is not set (token_env); export it before posting", name, ch.TokenEnv)
		}
	}
	switch ch.Kind {
	case "slack":
		return slack.Channel{Token: token, ChannelID: ch.ChannelID, HTTP: d.httpClient()}, nil
	case "telegram":
		return telegram.Channel{Token: token, ChatID: ch.ChatID, HTTP: d.httpClient()}, nil
	case "github":
		return github.Channel{Token: token, Repo: ch.Repo, HTTP: d.httpClient()}, nil
	}
	return command.Channel{Command: ch.Command, Dir: root, Run: d.shell()}, nil
}

// buildPoster composes the poster over the project's channels and store.
func buildPoster(d Deps, cfg *config.Config, root string, s *store.Store) notify.Poster {
	return notify.Poster{
		Channel: func(name string) (notify.Channel, error) { return buildChannel(d, cfg, root, name) },
		Store:   s, Now: d.Now,
	}
}
```
(`config.validate` already guarantees the kind is one of the four.)

`internal/cli/post.go`:
```go
package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

type postOpts struct {
	channel, file, thread string
	dryRun                bool
}

// newPostCmd is ness post (R-NTF-001): separate from send because an external post is
// irreversible (R-NTF-006); any session and the operator may run it (R-NTF-008).
func newPostCmd(d Deps) *cobra.Command {
	var o postOpts
	cmd := &cobra.Command{
		Use:   "post",
		Short: "Post one Markdown note to a configured channel, rendered for its platform",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if o.channel == "" {
				return usage("--channel is required: a name from [notify.channels]")
			}
			if o.file == "" {
				return usage("--file is required: the Markdown note")
			}
			p, cfg, err := requireFeature(d, "notify")
			if err != nil {
				return err
			}
			req, err := readNote(o, cfg.Notify.Channels[o.channel].Kind)
			if err != nil {
				return err
			}
			return withStore(d, func(s *store.Store, _ caller) error { return runPost(d, p, cfg, s, o, req) })
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.channel, "channel", "", "channel `NAME` from [notify.channels]")
	f.StringVar(&o.file, "file", "", "the Markdown note `FILE`")
	f.StringVar(&o.thread, "thread", "", "thread `KEY`: the first post starts the thread, later ones reply in it")
	f.BoolVar(&o.dryRun, "dry-run", false, "print the exact payload of each part and send nothing")
	return cmd
}

// readNote reads the note and, when note.<channel>.md or note.<kind>.md sits next to it, that
// verbatim text (R-NTF-004).
func readNote(o postOpts, kind string) (notify.Request, error) {
	data, err := os.ReadFile(o.file) //nolint:gosec // the caller names its own file
	if err != nil {
		return notify.Request{}, usage("--file %s: %v", o.file, err)
	}
	req := notify.Request{Channel: o.channel, Markdown: string(data), Thread: o.thread}
	dir := filepath.Dir(o.file)
	for _, name := range []string{o.channel, kind} {
		if name == "" {
			continue
		}
		if v, verr := os.ReadFile(filepath.Join(dir, "note."+name+".md")); verr == nil { //nolint:gosec // beside the note
			req.Verbatim = string(v)
			break
		}
	}
	return req, nil
}

func runPost(d Deps, p *project, cfg *config.Config, s *store.Store, o postOpts, req notify.Request) error {
	if o.dryRun {
		req.DryRun = d.Stdout
	}
	n, err := buildPoster(d, cfg, p.root, s).Post(req)
	var pe *notify.PartialError
	switch {
	case errors.Is(err, notify.ErrNoChannel):
		return refuse("%v", err)
	case errors.Is(err, notify.ErrEmpty):
		return usage("%s: the note is empty; nothing to post", o.file)
	case errors.As(err, &pe):
		printErr(d.Stderr, err)
		return exitCode(ExitProblems)
	case err != nil:
		return err
	case o.dryRun:
		return nil
	}
	say(d.Stdout, "posted %s to %s", plural(n, "part"), o.channel)
	return nil
}

// plural is "1 part" or "3 parts".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
```
(`fmt` and `config` join the imports.) `readNote` reads the note before the store opens, so a missing file is a usage error that writes nothing.

- [ ] **Step 11: Run the scripts and the whole tree**

Run: `go test ./internal/cli/ ./internal/testkit/ ./internal/notify/ && golangci-lint run ./... && go test -race -count=1 ./...`
Expected: PASS. The `post.txtar` thread assertion expects `1700000000.000001`: the fake numbers requests from 1 across the process boundary because `http.json` persists in `$NESS_TEST_FAKES`.

- [ ] **Step 12: Commit**

```bash
git add internal/cli internal/testkit
git commit -m "feat(cli): ness post over the channel atoms: dry run, verbatim notes, threads by key, resume; the fake HTTP transport records and never reaches the network"
```

---

### Task 9: The channel contract suite, over the fake and (opt-in) the real platforms

**Files:**
- Modify: `internal/testkit/contracts.go` (`ChannelContract`), `internal/atom/slack/slack_test.go`, `internal/atom/telegram/telegram_test.go`, `internal/atom/github/github_test.go`, `internal/atom/command/command_test.go`
- Create: `internal/atom/slack/e2e_test.go`, `internal/atom/telegram/e2e_test.go`, `internal/atom/github/e2e_test.go` (build tag `e2e`)

**Interfaces:**
- Produces: `testkit.ChannelContract(t *testing.T, ch notify.Channel, thread string, inspect func(n int) (url, body string))`: posts two parts, the second in the thread the first returned (or `thread` for a channel that needs one), checks both ids are non-empty and that `inspect` (nil for a real run) shows the second request addressed to the thread: Slack `thread_ts`, Telegram `reply_to_message_id` or `message_thread_id`, GitHub `/issues/<thread>/`; the command channel is checked through its own stdin capture.

- [ ] **Step 1: Write the contract and the failing tests**

Append to `internal/testkit/contracts.go`:
```go
// ChannelContract is the channel seam's contract (R-QA-024, R-NTF-005): a first post starts a
// thread (or uses the given one) and a second post replies in it. inspect (nil on a real run)
// returns the nth request's url and body; the reply must address the thread.
func ChannelContract(t *testing.T, ch notify.Channel, thread string, inspect func(n int) (url, body string)) {
	t.Helper()
	first, err := ch.Post(notify.Part{Blocks: []string{"ness channel contract: first"}}, thread)
	if err != nil || first == "" {
		t.Fatalf("first Post = %q, %v", first, err)
	}
	second, err := ch.Post(notify.Part{Blocks: []string{"ness channel contract: reply"}}, first)
	if err != nil || second == "" {
		t.Fatalf("second Post = %q, %v", second, err)
	}
	if inspect == nil {
		return
	}
	url, body := inspect(2)
	addressed := strings.Contains(body, `"thread_ts":"`+first+`"`) ||
		strings.Contains(body, `"reply_to_message_id":`+first) || strings.Contains(body, `"message_thread_id":`+first) ||
		strings.Contains(url, "/issues/"+first+"/")
	if !addressed {
		t.Fatalf("the reply does not address thread %q: url %s body %s", first, url, body)
	}
}
```
(`notify` joins the imports.) The fake runs, one per atom test file:
```go
func TestSlackContractOverFake(t *testing.T) {
	// covers: R-NTF-005, R-NTF-023, R-QA-024
	f := testkit.FakeHTTP{Dir: t.TempDir()}
	c := slack.Channel{Token: "test-token", ChannelID: "C0TEST", HTTP: &http.Client{Transport: f}}
	testkit.ChannelContract(t, c, "", func(n int) (string, string) {
		reqs, err := f.Requests()
		if err != nil || len(reqs) < n {
			t.Fatalf("requests: %v, %d", err, len(reqs))
		}
		return reqs[n-1].URL, reqs[n-1].Body
	})
}
```
The same shape for telegram (`R-NTF-005, R-NTF-027`; `telegram.Channel{Token: "test-token", ChatID: "1001", …}`, thread `""`) and github (`R-NTF-005, R-NTF-028`; `github.Channel{Token: "test-token", Repo: "o/r", …}`, thread `"42"`). The command channel has no HTTP request to inspect; its contract test captures the keys itself:
```go
func TestCommandContract(t *testing.T) {
	// covers: R-NTF-005, R-NTF-029, R-QA-024
	dir := t.TempDir()
	c := command.Channel{Command: "cat >> posts; echo \"$NESS_THREAD_KEY\" >> keys", Dir: dir, Run: shell.Run}
	testkit.ChannelContract(t, c, "k1", func(int) (string, string) {
		keys, err := os.ReadFile(filepath.Join(dir, "keys")) //nolint:gosec // the test's own file
		if err != nil {
			t.Fatal(err)
		}
		// The second post's key is the first post's id (k1): shaped as the GitHub url form the
		// contract recognises.
		return "/issues/" + strings.Split(string(keys), "\n")[1] + "/", ""
	})
}
```

The real runs, `internal/atom/slack/e2e_test.go`:
```go
//go:build e2e

package slack_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/slack"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestSlackContractOverRealSlack(t *testing.T) {
	// The real half of contract(fake+real): opt-in, needs the operator's bot token and a test
	// channel in the environment; it posts two short messages there.
	// covers: R-NTF-023
	if os.Getenv("NESS_E2E") != "1" || os.Getenv("NESS_E2E_SLACK_TOKEN") == "" || os.Getenv("NESS_E2E_SLACK_CHANNEL") == "" {
		t.Skip("set NESS_E2E=1, NESS_E2E_SLACK_TOKEN and NESS_E2E_SLACK_CHANNEL to run")
	}
	c := slack.Channel{Token: os.Getenv("NESS_E2E_SLACK_TOKEN"), ChannelID: os.Getenv("NESS_E2E_SLACK_CHANNEL"), HTTP: http.DefaultClient}
	testkit.ChannelContract(t, c, "", nil)
}
```
Telegram: `NESS_E2E_TG_TOKEN`, `NESS_E2E_TG_CHAT`; GitHub: `NESS_E2E_GH_TOKEN`, `NESS_E2E_GH_REPO`, `NESS_E2E_GH_ISSUE` (the thread). Add `./internal/atom/...` to the `e2e` target's package list in the `Makefile` (`-run 'Real'` already matches).

- [ ] **Step 2: Run them**

Run: `go test ./internal/atom/... ./internal/testkit/ && go vet -tags e2e ./internal/atom/...`
Expected: PASS (the fake runs); the e2e files compile and skip.

- [ ] **Step 3: Commit**

```bash
git add internal/testkit/contracts.go internal/atom Makefile
git commit -m "test(notify): the channel contract over the fake transport for every atom, and opt-in real runs behind the e2e tag"
```

---

### Task 10: Coverage gate, status line, wrap-up

**Files:**
- Modify: `internal/testkit/coverage_repo_test.go`, `README.md`

- [ ] **Step 1: NTF and CTR join the gate**

Edit the two variables in `internal/testkit/coverage_repo_test.go` in place (never replace the file):
- `doneAreas`: append `"NTF", "CTR"` and update the comment to `(P1, P2, P3, P4, P5a)`.
- `deferred`: delete `"R-CFG-054": "P5", "R-PROF-015": "P5", "R-HOOK-010": "P5",` (covered now) and add `"R-NTF-030": "review",`.

The result:
```go
// doneAreas are the spec areas whose plans have landed (P1, P2, P3, P4, P5a).
var doneAreas = []string{"CFG", "SYNC", "PROF", "QA", "MSG", "HOOK", "MOD", "SESS", "DOC", "WRK", "NTF", "CTR"}

// deferred names ids of done areas that a later plan, a review entry or publication delivers.
var deferred = map[string]string{
	"R-CFG-030": "P5", "R-CFG-031": "P5",
	"R-SESS-070": "P5", "R-SESS-081": "P5", "R-SESS-083": "P5", "R-DOC-015": "P5",
	"R-WRK-060": "P5", "R-WRK-061": "P5",
	"R-NTF-030": "review",
	"R-CFG-032": "P6", "R-CFG-034": "P6", "R-CFG-035": "P6", "R-CFG-037": "P6", "R-CFG-051": "P6",
	"R-SYNC-032": "P6", "R-SYNC-033": "P6", "R-SYNC-089": "P6", "R-PROF-002": "P6",
	"R-QA-004": "P6", "R-QA-005": "P6", "R-QA-023": "P6", "R-QA-030": "P6", "R-QA-031": "P6", "R-QA-041": "P6",
	"R-MOD-002": "P6", "R-MOD-021": "P6",
	"R-QA-032": "publication", "R-QA-033": "publication",
	"R-QA-025": "withdrawn (D39)",
}
```

Run: `go test ./internal/testkit/ -run TestRequirementCoverage`
Expected: PASS. An uncovered NTF or CTR id is a real gap: find the task that delivers it, add the test with its marker, rerun. Where the ids live: NTF 001–004, 006, 008, 040, 042 (`post.txtar`), 005, 007 (`post_test.go`, the contracts), 010–013 (`markdown_test.go`, `split_test.go`, the goldens), 020–023 (slack tests), 024–027 (telegram tests), 028 (github tests), 029 (`shell_test.go`, `command_test.go`, `post_command.txtar`), 041 (slack and github limits tests); CTR 001–008 (`contract.txtar`, `contract_test.go`), 009–012 (`contract_guard.txtar`, `contract_test.go`, `edits_test.go`), 020 (`contract_rights.txtar`), 021 (`contract_guard.txtar`, `edits_test.go`).

- [ ] **Step 2: README status**

Replace the status line in `README.md` with:
`Status: P5a (notify, contracts) implemented: ness post over Slack, Telegram, GitHub and command channels with threads and resume; ness contract freeze/amend/check and the pre-tool contract guard; the feature switch with its skill text; on top of P4. P5b (probes) follows. Plans: docs/superpowers/specs/2026-10-08-ness-v1-scope.md.`

- [ ] **Step 3: The full gate and the dogfood check**

Run: `make validate && make fuzz FUZZTIME=10s`
Expected: PASS end to end (`FuzzParse` in `hook` and `notify`, `FuzzSplit` included).

Then the dogfood, in a throwaway repository and never in this checkout (this repository has live sessions and no feature on; `go run ./cmd/ness contract check` here must exit 4 naming `contracts`, and that is the only command to run here):
```bash
cd "$(mktemp -d)" && git init -q && git commit -q --allow-empty -m init
go run ~/src/ness/cmd/ness init --gate true --base main --yes
printf 'features = ["notify", "contracts"]\n[notify.channels.log]\nkind = "command"\ncommand = "cat"\n' >> .ness/ness.toml
printf '# Hello\n\nA **test** note.\n' > n.md
go run ~/src/ness/cmd/ness post --channel log --file n.md --dry-run
go run ~/src/ness/cmd/ness post --channel log --file n.md
go run ~/src/ness/cmd/ness contract check
```
Expected: the dry run prints the note, the post prints it (through `cat`) and `posted 1 part to log`, `contract check` prints `ok	0 frozen files`; `ls .agents/skills` shows `ness-contracts`, `ness-entry`, `ness-notify` after `go run … sync`.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "build: NTF and CTR join the requirement coverage gate; P5a status"
```

---

## End-of-plan review

After Task 10, launch one fresh reviewer on the most capable model with: `docs/spec/notify.md`, `docs/spec/contracts.md`, the carry-ins (R-CFG-054, R-PROF-015, R-HOOK-010), this plan, and `git diff <first P5a commit>^..HEAD`. The reviewer checks:
1. Each covered requirement's test would fail without its behaviour (spot-check: let `Split` cut inside a fence, make `Poster.send` skip `MarkSent`, let the guard read `CLAUDE_PROJECT_DIR`, make `FrozenFiles` prefer a sha over `amend`, drop the `ok:false` check in `slack.Post`).
2. The five Review Focus items hold, and the rulings: no code path posts from `integrate` or `send` (`grep -rn 'internal/notify' internal/session internal/cli/send.go internal/cli/integrate.go` is empty); no test fixture carries a real token, URL or e-mail (`grep -rniE 'xoxb-|hooks.slack.com|bot[0-9]+:|@gmail' internal` is empty); every HTTP request in tests goes through `FakeHTTP` or `httptest`.
3. R-NTF-030: the ten constructs exist in `internal/notify/testdata/constructs/` and each of slack, telegram and github has a golden per construct; write the `docs/spec/reviews.md` entry.
4. No wall-clock sleep in any test (`grep -rn 'time.Sleep' --include=*_test.go internal` is empty).

Then fix each finding, rerun `make validate`, and report to the operator: what shipped, the evidence (test counts, fuzz time, the dogfood run), and what P5b takes over (probes, with `notify.Poster` and `shell.Run` as its seams).
