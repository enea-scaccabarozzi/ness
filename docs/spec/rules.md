> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Rule pointers (RULE)

Package: `internal/rule`. Every refusal ness raises, and every check a profile ships, names the
skill and section that state the rule it enforces, in one format. A registry maps each built-in
refusal to its pointer; a Go test proves every pointer lands on a heading that exists.
Sources: field feedback batch E, `docs/sessions/field-feedback/e1-ness-inventory.md` §2.

## Pointer

- **R-RULE-001** A pointer MUST read `(skill NAME, section HEADING)` and be the only pointer
  format anywhere (Go messages, profile-shipped files, configuration); `rule.Ref(id)` MUST return
  the registry pointer of `id` preceded by one space, and an unknown id MUST panic.
  Verify: unit — `Pointer.String`, `Ref` for a registered and an unknown id.

## Refusals

- **R-RULE-002** Every refusal a built-in check raises (polling, background, output, child cap,
  checkpoint, handoff, subject, dirty worktree, root close, root open, report missing,
  contract guard, and the ones added by this batch) MUST end with the pointer of its registry entry.
  Verify: testscript per refusal — `stderr 'skill ness-[a-z-]+, section'`; unit — the registry
  holds an entry for each id of the list.
- **R-RULE-004** The child-cap refusal MUST NOT suggest raising `launch.max_children`.
  Verify: testscript `open_child_cap` — `! stderr 'raise launch'`.

## Resolution

- **R-RULE-003** Every registry pointer, and every pointer in a file a profile ships (scripts,
  `audit.py`, linter messages, Makefile, hooks, skill text), MUST name a skill that ships and a
  heading that exists in it; a heading inside a fenced code block MUST NOT count.
  Verify: unit — `profiles/pointers_test.go` over the embedded profiles, and a negative table
  (missing skill, missing section, heading in a code fence).

## Prose

- **R-RULE-005** Skill text MUST state every rule a hook refuses: no `&` backgrounding, no
  `pgrep -f` waits, no whole tool-result reads (ness-entry, Communicate).
  Verify: unit — the Communicate section of the embedded ness-entry contains `run_in_background`,
  `pgrep -f` and `tool result`.
