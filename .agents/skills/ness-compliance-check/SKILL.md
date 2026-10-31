---
name: ness-compliance-check
description: Run the compliance audit at plan time and at the final review; fix or own every finding.
---

# ness compliance check

## When
- At plan time and at the final review.

## How
- Run `python3 .ness/audit.py` at the repository root. It scans the whole tree (every `package_roots` entry in `.ness/audit.toml`); v1 has no narrower scope.
- Every MUST is fixed or owned.
- An owned finding goes to `.ness/audit-exceptions.txt` as `RULE PATH REASON`.

## Scope
- The gate (`make validate`) runs the same whole-tree audit.
