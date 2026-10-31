> [!NOTE]
> Archive: the record of the session that prepared the public release. Names of private projects were replaced; the content is otherwise as written.

# Report: public-release

## Delivered
- Public repository `enea-scaccabarozzi/ness`, one fresh commit (`aa54d8e`) built from the tree minus `docs/discovery/`. The old repository was renamed `ness-private` and stays private until it is deleted (PR-D10).
- Docs: why, start, concepts (with guides and sensors, PR-D8), guides, reference with a generated CLI reference (R-QA-047), built-with-ness. Every command shown was run by hand; the runs are logged in `verification*.md`.
- Site: Fumadocs in `site/`, landing built from one captured round (PR-D7, PR-D9), deployed to GitHub Pages by `.github/workflows/site.yml`.
- Root files: README, CONTRIBUTING (with the writing style), LICENSE (Apache-2.0), SECURITY, CODE_OF_CONDUCT.
- Archive (design, spec, sessions, plans) anonymised, each file with a release banner. The leak scan covers the whole repository except `docs/discovery`, whitespace-normalised (R-QA-048).
- Private vulnerability reporting and Pages are enabled.

## Evidence
- `ness integrate`: gate passed (build, both lint configs 0 issues, `go test -race` all ok, audit, govulncheck 0 affecting, sync check).
- Pre-push leak scan on the exact pushed tree: denylist 0 hits; the only pattern hits were synthetic fixtures (`session_01A…`, `/tmp/claude-1000/-home-x-app/…`).
- Public repo CI on `aa54d8e`: `ci` (12m41s), `site` (1m8s), `release-please` all success.
- Live site: `/`, `/docs`, quickstart, guides-and-sensors, CLI reference, icon all HTTP 200; screenshots checked.
- `go install github.com/enea-scaccabarozzi/ness/cmd/ness@latest` from the public proxy in a clean GOPATH: installs, `ness --version` prints `ness version 0.1.0`.
- Local `main` now tracks the public history; `docs/discovery/` kept on disk through `.git/info/exclude`; the old history is in `.git/pre-public-history.bundle`.

## Open for the operator
- Delete `ness-private` after your check (needs `gh auth refresh -h github.com -s delete_repo`). Its issue #2 and v0.1.0 pre-release go with it unless moved.
- PR-D11 is PROVISIONAL: docs describe today's code, frictions to the backlog; documented escalation step; `waits` as labelled; SECURITY scope; archive links to GitHub; generalised probe example; no release cut.
- Friction findings 1 to 23 in `findings-capture-run.md` await triage for the next round.

## Lessons
- Write the denylist scan first and run it on the exact tree to be pushed: the in-repo scan missed a hit split across a Markdown line break until it normalised whitespace.
- Docs lanes that rerun every command catch drift the reviewers miss; budget a second rerun after review fixes, since the fixes change shown output.
- A fresh-history publish makes `@latest` safe: no stale tag follows the module into the new repository.
- Resetting a checkout to a history without a tracked folder deletes that folder; untrack it and back it up before the reset.
