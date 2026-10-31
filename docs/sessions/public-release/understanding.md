> [!NOTE]
> Archive: the record of the session that prepared the public release. Names of private projects were replaced; the content is otherwise as written.

# Understanding of ness, round 2 (2026-10-31)

Round 1 put the hierarchy in the middle and turned some defaults into principles. Operator
correction: the thesis is that **no universal harness exists**. ness is the shared machinery
underneath the custom harnesses you build for yourself.

## Answers (operator, round 1)
- Q1: the name comes from har**ness**.
- Q2: tested solo and in small teams at most. Larger scale is unknown and is not claimed.
- Q3: field evidence is public, anonymised.
- Q4: same GitHub repo, fresh history (D30).
- Q5: the spec, session records and plans are public, as proof that ness builds ness. They are
  cleaned, and each one carries an explicit note that it was altered for the release.

## N. The narrative
1. **There is no universal harness.** No single setup fits every project, whether for code or
   for process. The only way that works: reason, per project, about what you need and what you
   do not, and build your own harness, tailor-made.
2. **I did that, more than once.** Custom harnesses for a few of my own projects. The process at
   the heart of each was different, as it should be.
3. **But the machinery underneath was the same, and it drifted.** Every harness rebuilt sessions,
   messaging, records and extension points. Each copy forked a little further from the others.
4. **So I pulled the machinery out: ness, from harness.** One CLI that ships the shared machinery
   and makes it available to your favourite coding agent (Claude Code today, Codex experimental).
   You build your harness on top of it.
5. **The payoff: you focus on the product, not the code.** You steer as a product manager. The
   harness carries the plumbing.
6. **Proof: ness builds ness.** After its first steps, ness has been developed with ness. The
   specs, session records and field rounds are public (anonymised), so the claims can be checked.

## M. The shared machinery (what ness ships)
1. **Hierarchical sessions with clear ownership.** Root, children and grandchildren, each owning a
   scope and a result over its own horizon of the product. Each session gets a worktree, a branch
   and a terminal tab. A plain conversation can promote itself to a root. Work comes in sizes
   (answer, quick, full), so a small job pays no program-size cost.
2. **Communication between sessions.** A store with send / reply / inbox, and a worker that wakes
   idle sessions, compacts them at boundaries, resumes them after usage limits, reports stalls,
   and notifies the operator.
3. **Persistent artifacts for explainability.** Every session leaves a committed record: brief,
   specs, plan, append-only decisions with ids, and a report with lessons. Why something was built
   stays readable after the agents are gone.
4. **An extensibility surface.** Profiles (text, files, config and checks, with slots in the core
   skills, declared requires and conflicts), the project's own local profile, atoms behind seams
   (runtime, client, integrator, channel, supervisor, model selector), and a safe sync that never
   overwrites what you wrote.
5. **Solid built-in defaults that grow over time.**
   - Features: probes (questions about the running world, owned by a session), notify (one
     Markdown note rendered for Slack, Telegram or GitHub), contracts (frozen acceptance files).
   - Profiles: program, go-strict, behavior-specs, git-conventional, example-release.
   - Atoms: herdr, Claude Code, Codex, merge / squash / rebase / pr, the Slack, Telegram, GitHub
     and command channels, systemd, and the model selectors.
6. **Guards and feedback.** Hooks that refuse polling and edits to frozen files and enforce the
   model ladder; `ness integrate` through the project's gate; `doctor` and `stats`.

7. **Guides and sensors (how ness steers agents).** Feedforward guides tell an agent what to do
   before it acts: the skills, the AGENTS.md blocks, profile text in slots, briefs and docs.
   Feedback sensors catch it when it drifts: pre-tool hooks (polling, frozen contracts, the model
   ladder, the gate run by hand), profile checks at open, integrate and close, the gate at
   integrate, `contract check`, checkpoints and handoffs, reviews, doctor, stats and probes. The
   loop is closed: every refusal names the guide it enforces as "(skill X, section Y)", and sync
   refuses a pointer to a section that does not exist. A local rule is a guide plus a sensor plus
   that pointer (docs/profiles/authoring.md).

## P. Principles (the rules ness itself is built by)
1. **No universal harness.** ness ships primitives and swappable defaults, never one "right"
   process.
2. **You steer the product, not the code.** The machinery exists so that your attention goes to
   ideas and decisions.
3. **Explicit and per project.** No global config. Everything lives in the repo, is committed and
   is reviewable as a diff.
4. **Ceremony must earn its place.** Every rule needs evidence behind it, and a rule that does not
   pay is removed.
5. **Context is the scarce resource.**
6. **Supported or refused.** It fails loudly and never degrades silently.
7. **Tested, not claimed.**

## D. Opinions in the shipped defaults (decisions, swappable, not principles)
- Every result received is questioned Socratically: a session challenges what it gets back
  (child results, measurements, operator remarks) instead of taking it as given. The program
  profile applies this to its map.
- The operator works only in the session chat. The phone only notifies and points at a session.
  Checkpoints say Done / Now / Next.
- A seven-rung model ladder, enforced by a hook.
- Reserved decisions stay the operator's. Autonomy is set per run, from the operator's words.

## Not claimed
Scale beyond solo and small teams. Platforms other than Linux, systemd and herdr. Codex as more
than experimental.
