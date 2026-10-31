> [!NOTE]
> Archive: altered for the public release. Names of private projects and local paths were replaced; the content is otherwise as written.

# Review: ness 13fd251 (branch sizes), parent main 01a18d9

Worktree: ~/src/ness/.claude/worktrees/agent-a2a438c5fd25197aa (not edited).

Verification:
- `mise exec -- go test ./...` passes, every package ok.
- Mutation check, run on a `git archive` copy in the scratchpad. Each of these seven reversions makes a test fail:
  1. drop the size clause in check.Run: TestRunFiltersOnTheRootsSize fails
  2. drop the quick-root child refusal: size_open:29 fails
  3. drop the Done skip in yourMove: TestYourMoveSkipsAnIntegratedQuickRoot fails
  4. drop the NESS_BASE export: TestIntegrateGivesGateAndSetupTheFoldBase fails
  5. drop the top-root size lookup in refuseOnChecks: size_program:22 fails
  6. drop the shrink refusal: size_open:43 fails
  7. change the root default away from full: open_here:33 fails
- The new tests and scripts contain no sleeps. The red-first claim in the build notes holds as far as a mutation check can show it.

Code matches the design in: migration 10, `Size` on Launch, Session and Check, `Resolved.Sizes` with its conflict refusal, `--size` and its refusals, `ness size`, status, JSON, the identity line, the unbound note, the program profile's `sizes` and `size` keys, and NESS_BASE. Size never reaches the gate: `runIn` adds only NESS_BASE, and `check/run.go` env carries no size.

---

## High

### H1. Deviation 2 is live in the research lab: its program checks stop firing, for old roots and for new ones
- **Where:** internal/store/migrate.go:189-191 (`UPDATE sessions SET size = 'full' WHERE parent IS NULL`), internal/cli/open.go:351-352 (default `full`), internal/cli/hook.go:30 (unbound note offers only `quick|full`).
- **Fact:** ~/src/research-lab/.ness/ness.toml has `profiles = ["program", "local"]`.
  - After upgrading, every open research-lab root is `full`, so the four program checks all skip without notice: autonomy-recorded, checkpoint-before-wave, recovery-line and no-child-decision-ids.
  - New roots are no better. The unbound note tells a fresh conversation `--size quick|full`. The program line in Work sizes (profiles/builtin/program/size.md) never says `--size program`. A promoted the research lab root therefore lands on `full` and runs with no program checks.
  - Before this commit those checks fired for every root in the project.
- **Repro:** start from a v9 DB with a root and its child in a project with program enabled. Run any ness command (it migrates), then from the root run `ness open --scope c ...` against a roadmap that records no autonomy. The open succeeds. On 01a18d9 it is refused with `program/autonomy-recorded`. size_program.txtar shows the same behaviour for root F before its resize.
- **Opens a hole:** yes. A gate the operator relies on turns off at upgrade with no warning.
- **Smallest fix (pick one):**
  - (a) No code, at rollout: run `ness size program --session <id>` for each open research-lab root, and put that step in the batched-rollout checklist. This does not cover new roots.
  - (b) Code. Default `--size` to the last entry of `Resolved.Sizes` when a profile adds sizes, otherwise `full`: open.go:351 becomes `o.size = env.sizes[len(env.sizes)-1]`. Backfill old rows the same way: migration 10 leaves roots at `''`, and in `withStore` after resolve, run `UPDATE sessions SET size=? WHERE parent IS NULL AND size=''` with the same default. Both lines key off the project's declared profiles, as the brief suggested.
  - With either fix, build the unbound note's `--size` choices from `Resolved.Sizes` (or say `--size quick|full|<profile size>`), and change size.md to `**program** (size `program`)`.

## Medium

### M1. A root can switch off its own profile checks with `ness size full`
- **Where:** internal/cli/size.go:50-58 (`sizeTarget` lets the root change its own size) and :98 (`resize` puts no condition on leaving a profile size).
- **Repro:** a program root runs `NESS_SESSION=$P ness size full`, then `ness open --scope c ...`. The child opens and autonomy-recorded never runs. Nothing records the change; the design chose to skip the report line.
- **Opens a hole:** yes, a soft one. The integrate gate is safe because size never reaches it, but the profile checks are enforced by code and an agent can now remove them by declaring a different size. That is the "weaken by a flag" pattern the design rejected for NESS_SIZE.
- **Smallest fix:** in `resize`, `if !c.operator() && root.Size != "quick" && root.Size != "full" { refuse("only the operator moves a root off size %s") }`. Pass `c` in. Growing from quick or full stays free for the root. A broader option is that only the operator may shrink, but that is wider than this hole needs.

### M2. The done state can hide a quick root that is actually stuck
- **Where:** internal/session/phases.go:43 (`Done` = quick and IntegratedAt set) and internal/worker/alert.go:148.
- **Repro:**
  1. A quick root integrates.
  2. The operator types a follow-up in the pane ("also fix X"). The agent works on it and ends its turn with a question in chat.
  3. `yourMove` never fires again for this root, because Done stays true once IntegratedAt is set. `ness status` shows a green `done`.
- The design's condition was "integrated and no open obligations". The code drops the "nothing happened since" part. Obligations and operator items are still covered by moveCovered and the stall reports, so this hole is limited to the chat-wait case. That case is exactly what yourMove exists for.
- **Smallest fix:** treat the root as done only while the integration is its last act. Add `busySince` to `pane`, set when the status leaves idle, and skip yourMove only when `session.Done(sess) && !sess.IntegratedAt.Before(p.busySince)`. Add one bench case: integrate, a busy pass, then idle for 40 minutes, expecting 1 alert. The cheaper heuristic `p.idleSince.Sub(sess.IntegratedAt) < w.Stall` also works.

### M3. A quick root now finishes without telling the operator
- **Where:** profiles/core/skills/ness-entry/SKILL.md, Quick root ("sends no checkpoint") plus the Done skip in alert.go:148.
- Before this commit, ness-entry said "A quick root sends one [checkpoint] at the end", which produced an AlertCheckpoint. Now there is no checkpoint, and yourMove is suppressed once integrated. The operator learns a quick task finished only from `ness status` or by looking at the pane.
- This matches the design table ("end-of-task checkpoint: full roots only"). The combination with option B is what is new, so the design missed it.
- **Smallest fix:** restore one line in Quick root: "send one checkpoint at the end (Done/Now/Next)". The checkpoint alert is the completion signal; yourMove was the noise.

### M4. Deviation 3: when the operator closes a root, size-gated checks are skipped; when the root closes itself, they run
- **Where:** internal/cli/profilechecks.go:40-51 (the size is filled only for `!c.operator()`) and internal/cli/close.go:163-166.
- **Repro:** add a local check `on="close" for="root" size=["full"]` that fails. `NESS_SESSION=$R ness close $R --authorized "..."` is refused. The same close from the operator's shell (`ness close $R`) succeeds, because ctx.Size is "" and the check is skipped. Closing roots is normally the operator's job, so the check misses its main path.
- **Integrate:** operator integrate sets `Child=false` (integrate.go:149), so `for="child"` checks already skipped there before this commit. With today's builtin checks, deviation 3 changes nothing. The hole is only open for size-gated close checks, and those come from project-local profiles.
- **Opens a hole:** latent today, real as soon as a project writes one.
- **Smallest fix:** in `preClose`, fill the size from the session being closed:
  `top, _ := topRoot(s, sess); r, _ := s.Session(top); ctx.Size = r.Size`, passed as `Size:` in the Context literal. refuseOnChecks already keeps a non-empty ctx.Size. For operator integrate, either look up the session by `from` branch and use its root's size, or document the skip.

### M5. A misspelled check size is never caught, so the check never fires
- **Where:** internal/profile/checks.go:124-145 checks the shape of `size` only. internal/profile/resolve.go never compares check sizes against `r.Sizes`.
- **Repro:** a local profile check with `size = ["progam"]` syncs cleanly and never runs.
- **Smallest fix:** at the end of `Resolve`, after all sizes are added, refuse any `c.Size` entry missing from `r.Sizes`, naming the check id. That is about five lines plus one bad testdata file.

## Low

### L1. ness-entry tells a quick root to skip sections it still needs
- Work sizes says "read the section of that name below and skip the others". But the rules a quick root still needs now sit under `## Full session`:
  - the root-close rule (`### Lifecycle`), which Quick root points to with "(Lifecycle, below)"
  - the `report.md` and `## Lessons` rule (`### Session record`)
- Rule pointers still resolve, because rule.HeadingLines matches any heading level. But RootClose and ReportMissing now send a quick root into the Full session section it was told to skip.
- **Fix:** move the root-close bullet and the report/Lessons sentence up to a shared section (Communicate, or a new `## Closing`), and repoint RootClose and ReportMissing in internal/rule/rule.go:78,81.

### L2. Text tests check the raw SKILL.md, not the rendered skill
- profiles/sizes_text_test.go checks headings and slot order in the source file. The design asked for a rendered check (program's size line under Work sizes, lifecycle slot text under Full session).
- The ordering is preserved through rendering, so the risk is small. A golden of `profiles sync` output with go-strict and program enabled would close it.

### L3. Untested branches in `ness size`
- No script covers:
  - "closed children are history" (shrink after the child closes)
  - "a root shows and changes only its own size" (size.go:54)
  - the operator resizing a closed root (size.go:68)
- One line each in size_cmd.txtar.

### L4. NESS_BASE quoting is safe, but only because of where the value comes from
- integrate.go:249 builds `"export NESS_BASE='"+base+"'\n"+command`. `base` comes from `git rev-parse --verify <target>^{commit}` (git.go:24), so it is always hex. No injection is possible today.
- Passing `Env` (if Git.Exec ever grows one), or asserting `^[0-9a-f]{40,64}$` before formatting, would make that independent of the caller.
- The extra first line shifts `sh -c` line numbers in gate error messages by one. This is cosmetic.
- Project gates should fall back when NESS_BASE is unset, for example in a lefthook pre-push run: `${NESS_BASE:-origin/main}`.

### L5. Migration safety: fine
- Migration 10 runs in the same transaction as the `user_version` bump (migrate.go:201-222). The ALTER uses a DEFAULT, so no row is rewritten unsafely. Roots are matched by `parent IS NULL`, which is consistent with `nullable(parent)`. The v9-fixture test covers this.
- Only operational note: rebuild the binary and restart the worker together, per the update-path memo, so an old worker never meets a v10 DB.

## Checked, no finding
- A quick root cannot open children. checkSizes refuses on `parent.Size == "quick"`, and `--root` is operator-only. The only legacy path is a child created before a shrink, which resize refuses while that child is open (Children is `state='open'`).
- `--size` on a child, unknown sizes, and program without its profile are all refused before any write (`store sessions 2` asserts this).
- The ness-entry restructure: every rule.Table pointer still resolves (TestTableIsFresh passes; HeadingLines matches any heading level). The goldens changed only for the command list, `standard full` and the unbound note.
