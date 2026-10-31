---
name: ness-git-conventional
description: Write commits in Conventional Commits form with trailers; the commit-msg hook rejects anything else.
---

# ness git-conventional

## Subject
`type(scope): subject` — the type is one of `build`, `chore`, `ci`, `docs`, `feat`, `fix`,
`perf`, `refactor`, `revert`, `sessions`, `style`, `test`; the scope is optional and names the
package or area; `!` before the colon marks a breaking change; the subject is imperative and
has no trailing period. `sessions` is what a running session must do after the change lands
(release notes put it under "Running sessions").

## Body
A blank line, then why the change is made, not what the diff shows.

## Trailers
After a blank line, one per line, `Key: value`: `Co-Authored-By: Name <email>`, `Refs: #123`,
`BREAKING CHANGE: what breaks and how to migrate`. Nothing follows the trailers.

## The hook
`.githooks/commit-msg` rejects a subject that is not in this form (merge, revert, fixup and
squash commits pass). Activate it once per clone:
`git config core.hooksPath .githooks && chmod +x .githooks/commit-msg`. With lefthook (the
go-strict profile) run it from `lefthook-local.yml` instead:

```yaml
commit-msg:
  commands:
    conventional:
      run: .githooks/commit-msg {1}
```
