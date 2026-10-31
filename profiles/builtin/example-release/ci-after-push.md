---
every: 5m
window: 1h
floor: c=$(gh run list --branch "$(git rev-parse --abbrev-ref HEAD)" --limit 1 --json conclusion --jq '.[0].conclusion'); echo "conclusion=$c"; test "$c" = success
confirm: 3
wake: red
notify: judged
channel: releases
---
The CI run of the commit just pushed is green. When woken: open the failing job, decide
whether the failure is the change or the pipeline, and answer with `ness probe verdict`.
