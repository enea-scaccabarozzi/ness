---
every: 30m
window: 48h            # optional; without it, runs until stopped
floor: ./probes/hard-wait.sh
expect: n>=1           # fewer samples = VACUOUS, never GREEN
confirm: 2             # consecutive non-GREEN runs before waking
wake: not-green        # never | change | red | not-green | always
sample: 0.1            # also wake on 10% of GREENs, at random
notify: judged         # none | auto | judged
channel: releases
---
Every hard send under the new config waited at least 50 s on the feed clock.

When woken: read the rows behind the evidence, look for alerts the query cannot see, and
decide whether a GREEN is real.
