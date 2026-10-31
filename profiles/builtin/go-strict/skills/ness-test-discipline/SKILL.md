---
name: ness-test-discipline
description: Keep Go tests honest: no sleeps, no permissive mocks, marked helpers, fixtures in testdata, files under the ceiling.
---

# ness test discipline

## Rules
- No `time.Sleep`.
- No permissive mocks and no call-count assertions.
- Helpers call `t.Helper()`.
- Fixtures live in `testdata/`.
- Test files stay under the ceiling in `.ness/audit.toml` (400 lines).

## Enforcement
- The gate runs `-race` and `-count=4`.
- `.golangci.tests.yml` and `python3 .ness/audit.py` enforce these rules.
