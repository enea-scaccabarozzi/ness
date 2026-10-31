---
name: ness-example-release
description: An example of one release chain written as profile text: integrate, then a post-push CI probe, then a note to the releases channel, then promotion with integrate --from/--into.
---

# ness example-release

ness does not chain integrate, probes and notify; a profile says how a project chains them.
This is one way, with generic names: a `releases` channel in `[notify.channels]`, a `main`
branch for development and a `prod` branch for production.

1. **Integrate.** `ness integrate` folds your branch through the gate into your manager's
   branch and pushes it.
2. **Watch the push.** `ness probe start probes/ci-after-push.md --label <sha8>` registers the
   probe, labelled with the release (the first 8 characters of the pushed commit): every
   5 minutes for an hour its floor asks the forge for the latest run on your branch; three
   consecutive non-green runs wake you with the evidence; your verdict is posted to
   `releases` because the probe says `notify: judged`. After the hour the probe stops and wakes you once for a final verdict.
   A second release while the first is still watched is a second start of the same file under its own label: the
   windows overlap, each ends alone, and the floor reads `$NESS_PROBE_LABEL` to ask about its own
   commit (`gh run list --commit "$NESS_PROBE_LABEL"`). ness does not supersede a release: stop
   what a newer one replaces with `ness probe stop probes/ci-after-push.md --label <sha8> --note ...`.
3. **Tell people.** Copy `docs/release-notes/TEMPLATE.md` to
   `docs/release-notes/<version>.md`, fill it, and send it with
   `ness post --channel releases --file docs/release-notes/<version>.md --thread release-<version>`
   after a `--dry-run`.
4. **Promote.** When the operator says go, `ness integrate --from main --into prod` folds the
   development branch into production through the same gate; the same probe, started on
   `prod` under the production commit's label, watches that push.
