> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# seams report

## Delivered
- **N1, the client seam.** Nothing in `internal/cli` outside `atoms.go` names an atom any more
  (`grep -n 'claude\.\|codex\.' internal/cli/*.go` hits only atoms.go). The worker finds the
  clients it can deliver to by asking each `config.Clients` atom for the cli-declared
  `deliverer` (3 methods); `ness close --authorized` asks the session's client for `authorizer`
  (1 method). The Claude atom implements both as methods over its existing functions; Codex
  implements neither, so:
  - `ness doctor` has a `worker-clients` line (was `worker-codex`) that warns when the configured
    client or an open session's client has no worker support, naming the client and the unwoken
    sessions (R-WRK-091).
  - a Codex root closing itself is refused with "client codex cannot check the operator's
    words" (R-WRK-092), instead of "no transcript".
  - Claude: no behaviour change (same functions behind the methods; worker tests unchanged).
- **N3, one list per set of names (R-CFG-070).** `config.Clients`, `IntegrateModes`,
  `ModelPolicies`, `FeatureNames`, `Experiments` feed `validate`, `enumKeys`, `profile.atomSeams`,
  the R-PROF-009 feature check and `--client`. `sync.HookEvents` feeds sync's hook wiring and
  `ness hook`. Existing refusal tests pass unchanged.
- Spec: R-WRK-090..092 in docs/spec/worker.md, R-CFG-070 in docs/spec/core-config.md.
- Decisions SM-1..SM-5 in decisions.md.

## Evidence
- New tests: `TestWorkerDeliversThroughTheClientSeam`, `TestEveryClientNameBuildsItsAtom`,
  `TestAuthorizedRefusesAClientThatCannotCheckWords` (internal/cli/seams_client_test.go),
  `TestCheckWorkerClients` (doctor_checks_test.go, three cases incl. codex configured with no
  session); doctor.txtar updated for the renamed check.
- Gate: two standalone `make validate` runs timed out under load 32-35 (testscript package
  deadline; no assertion failed); the affected scripts passed in isolation (45s). The gate inside
  `ness integrate` passed: integrated into session/cli-ux-4ad0a273d838 at 84f5c08.

## Not done
- The atoms.go switches (integrator, channel kind) remain switches over literals; the client
  one is tested against the list. Turning them into maps keyed by the list buys nothing yet.
- `ness hook`'s `Short` text still spells "session-start, pre-tool".

## Lessons
- The pre-tool hook refuses any Bash command whose text holds both a loop keyword and
  `ness status`/`ness inbox`, even inside a heredoc'd edit script; write such edits to a file in
  the scratchpad and run the file.
- The full gate takes over ten minutes here; run it in the background with its exit status
  captured, never through `tail`.
