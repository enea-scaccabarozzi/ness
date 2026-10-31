> [!NOTE]
> Archive: published as written for the public release; only this note was added.

- **SE-1** Design first (plan-first autonomy): option A recommended, sent to manager as a decision. See design.md.
- **SE-2** Operator approved option A ("i authorize the secrets part", relayed m-5d45aafg); build, migrate both projects, then delete the hand-made env files and drop-ins.
- **SE-3** Probes do not auto-redact the secrets file's values (a design line dropped): a floor
  is a shell command that never receives those values (they are not in its environment), so
  there is nothing to mask; `redact_env` keeps covering the variables floors do inherit.
- **SE-4** Channel errors get every referenced value masked through one wrapper (`masked` in
  cli/atoms.go) around whatever atom the kind builds, so a new atom gets R-NTF-061 for free.
- **SE-5** Doctor's secrets check took id R-CFG-085 (not an R-DOC id) to stay inside this
  session's reserved range while siblings may add R-DOC ids.
- **SE-6** Secret values are read at each channel build, so `ness secret set` takes effect in a
  running worker without a restart.
