> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# seams decisions

- **SM-1** N1 goes through the seam, not around it: cli declares two small interfaces at their
  consumers, `deliverer` (worker.go: ReadScreen, ResumeText, RenameText) and `authorizer`
  (close.go: OperatorSaid). The Claude atom implements them as methods; codex does not. The
  worker's client map and the close check are found by type assertion on `buildClient`, so
  adding worker support to Codex is adding three methods to its atom.
- **SM-2** `claude.Usage` stays a builder in atoms.go (`buildUsage`): it reads a per-user usage
  file, not a per-session client capability, and the worker takes one function.
- **SM-3** The doctor check `worker-codex` becomes `worker-clients` (R-WRK-091): it warns when the
  configured client, or the client of an open session, has no worker support, naming the client
  and the unwoken sessions. The check name changes in `ness doctor --json`.
- **SM-4** N3: the lists live in `config` (Clients, IntegrateModes, ModelPolicies, FeatureNames,
  Experiments); `enumKeys`, `validate`, `profile.atomSeams`, the R-PROF-009 feature check and
  `--client` validation read them. Hook events live in `sync.HookEvents` (event and name);
  `ness hook` reads the names from it. The atoms.go switches stay switches; a test proves each
  client name builds the atom of that name (R-CFG-070).
- **SM-5** Spec ids: R-WRK-090 (deliver through the seam), R-WRK-091 (doctor line), R-WRK-092
  (close refusal naming the client), R-CFG-070 (one list per set of names).
