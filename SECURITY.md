# Security policy

## Supported versions

Security fixes go to the latest stable release and to the previous minor line, because a `0.N`
binary still runs projects pinned to `0.N-1` (R-UPD-002). Upgrade with `ness self-update` before
you report, if you can.

| Version | Supported |
|---|---|
| The latest stable `0.N.x` | Yes |
| The previous minor, `0.(N-1).x` | Yes |
| Older, and release candidates | No |

## Report a vulnerability

Report it privately through a GitHub security advisory:
[open a draft advisory](https://github.com/enea-scaccabarozzi/ness/security/advisories/new).
Do not open a public issue, pull request or discussion for it.

Include:

- the ness version (`ness --version`) and your platform;
- what an attacker can do, and the steps or a proof of concept that show it;
- any fix or mitigation you already know.

The advisory stays private until a fix is released.
