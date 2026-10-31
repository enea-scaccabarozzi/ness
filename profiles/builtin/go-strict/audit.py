#!/usr/bin/env python3
"""go-strict compliance audit (ness, design §11, R-BLT-053, R-BLT-057, R-BLT-060, R-BLT-071).

Exit 0: clean. Exit 1: findings, one line each on stdout: RULE PATH: DETAIL. Exit 2: the
configuration, the exceptions file or the Go version is unusable (stderr).
Rules: T400 (a test file at or over the ceiling), SLEEP (time.Sleep in a test), FORTEST (a
ForTest identifier in production code), CLOCK (a Clock interface), PKGDOC (not exactly one
package or command comment per package), LIBS (a logger or validator other than the configured
one outside the composition roots), FEATURE (a godog scenario with more than one When, more than
10 steps or more than 5 example rows). The judgment rules of the skills stay with the agent.
An exception that matches no finding is itself a finding (EXCEPTION); an unknown rule name in
the exceptions file exits 2. The audit always scans the whole tree (v1 has one scope).
"""
import re
import sys
import tomllib
from pathlib import Path

CONFIG = Path(".ness/audit.toml")
MIN_GO = (1, 25)
LOGGERS = ["log/slog", "github.com/sirupsen/logrus", "go.uber.org/zap", "github.com/rs/zerolog"]
FORTEST_DECL = r"(?:func\s+(?:\([^)]*\)\s*)?|type\s+|var\s+|const\s+|)\w*ForTest(?:s|ing)?\b"
RULES = {"T400", "SLEEP", "FORTEST", "CLOCK", "PKGDOC", "LIBS", "FEATURE"}
# Each rule's skill and section; every printed finding ends with it.
POINTERS = {
    "T400": ("ness-test-discipline", "Rules"),
    "SLEEP": ("ness-test-discipline", "Rules"),
    "PKGDOC": ("ness-documentation", "Comments"),
    "FORTEST": ("ness-interface-design", "Time and test hooks"),
    "CLOCK": ("ness-interface-design", "Time and test hooks"),
    "LIBS": ("ness-interface-design", "Wiring"),
    "FEATURE": ("ness-behavior-specs", "Scenarios"),
}
VALIDATORS = ["github.com/go-playground/validator", "github.com/go-ozzo/ozzo-validation"]


def fail(msg):
    print(f"audit: {msg}", file=sys.stderr)
    sys.exit(2)


def load_config():
    try:
        cfg = tomllib.loads(CONFIG.read_text())
    except (OSError, tomllib.TOMLDecodeError) as e:
        fail(f"{CONFIG}: {e}")
    cfg.setdefault("package_roots", ["internal", "cmd"])
    cfg.setdefault("composition_roots", ["cmd"])
    cfg.setdefault("generated_dirs", [])
    cfg.setdefault("excluded_packages", [])
    cfg.setdefault("exceptions_file", ".ness/audit-exceptions.txt")
    cfg.setdefault("ceilings", {}).setdefault("test_file_lines", 400)
    cfg.setdefault("libraries", {}).setdefault("logger", "log/slog")
    cfg["libraries"].setdefault("validator", "")
    return cfg


def check_go_version():
    try:
        text = Path(".golangci.yml").read_text()
    except OSError as e:
        fail(f".golangci.yml: {e}")
    m = re.search(r'^\s*go:\s*"?(\d+)\.(\d+)', text, re.M)
    if m and (int(m[1]), int(m[2])) < MIN_GO:
        fail(f"go version {m[1]}.{m[2]} is below {MIN_GO[0]}.{MIN_GO[1]} (go-strict floor)")


def load_exceptions(path):
    out = {}
    if not Path(path).exists():
        return out
    for n, line in enumerate(Path(path).read_text().splitlines(), 1):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split(None, 2)
        if len(parts) < 3:
            fail(f"{path}:{n}: exception without a reason: RULE PATH REASON")
        if parts[0] not in RULES:
            fail(f"{path}:{n}: unknown rule {parts[0]}")
        out[(parts[0], parts[1])] = n
    return out


def skipped(path, cfg):
    s = str(path)
    return any(s == d.rstrip("/") or s.startswith(d.rstrip("/") + "/")
               for d in cfg["generated_dirs"] + cfg["excluded_packages"])


def ignored_by_go(rel):
    """True for a path the go tool ignores: testdata, vendor, and names starting with _ or ."""
    return any(p in ("testdata", "vendor") or p[0] in "_." for p in rel.parts)


def go_files(cfg):
    for root in cfg["package_roots"]:
        for p in sorted(Path(root).rglob("*.go")):
            if not ignored_by_go(p.relative_to(root)) and not skipped(p, cfg):
                yield p


def in_composition_root(path, cfg):
    return any(str(path).startswith(r.rstrip("/") + "/") for r in cfg["composition_roots"])


def check_file(path, text, cfg, out):
    lines = text.splitlines()
    if path.name.endswith("_test.go"):
        ceiling = cfg["ceilings"]["test_file_lines"]
        if len(lines) >= ceiling:
            out.append(("T400", str(path), f"{len(lines)} lines (ceiling {ceiling})"))
        if "time.Sleep(" in text:
            out.append(("SLEEP", str(path), "time.Sleep in a test"))
        return
    for n, line in enumerate(lines, 1):
        if re.match(FORTEST_DECL, line.strip()):
            out.append(("FORTEST", str(path), f"line {n}: test hook in production code"))
        if re.match(r"^type\s+Clock\s+interface", line):
            out.append(("CLOCK", str(path), f"line {n}: pass now as a value, use synctest for timers"))
    if not in_composition_root(path, cfg):
        check_libs(path, text, cfg, out)


def under(imp, lib):
    """imp is the module lib or a package below it (so a /v10 suffix still matches)."""
    return bool(lib) and (imp == lib or imp.startswith(lib + "/"))


def check_libs(path, text, cfg, out):
    libs = cfg["libraries"]
    for imp in re.findall(r'^\s*(?:\w+\s+)?"([^"]+)"', text, re.M):
        if any(under(imp, x) for x in LOGGERS) and not under(imp, libs["logger"]):
            out.append(("LIBS", str(path), f"{imp}: the logger is {libs['logger']}"))
        if any(under(imp, x) for x in VALIDATORS) and not under(imp, libs["validator"]):
            out.append(("LIBS", str(path), f"{imp}: the validator is {libs['validator'] or 'none'}"))


def check_packages(docs, out):
    for pkg, n in sorted(docs.items()):
        if n != 1:
            out.append(("PKGDOC", pkg, f"{n} package comments, want exactly one"))


def check_feature(path, text, out):
    name, whens, steps, rows, header = None, 0, 0, 0, False

    def flush():
        if name is None:
            return
        if whens > 1:
            out.append(("FEATURE", str(path), f'scenario "{name}" has {whens} When'))
        if steps > 10:
            out.append(("FEATURE", str(path), f'scenario "{name}" has {steps} steps (max 10)'))
        if rows > 5:
            out.append(("FEATURE", str(path), f'scenario "{name}" has {rows} example rows (max 5)'))

    for line in text.splitlines():
        s = line.strip()
        if s.startswith(("Scenario:", "Scenario Outline:", "Scenario Template:", "Example:")):
            flush()
            name, whens, steps, rows, header = s.split(":", 1)[1].strip(), 0, 0, 0, False
        elif s.startswith(("Feature:", "Background:", "Rule:")):
            flush()
            name = None
        elif s.startswith(("Examples:", "Scenarios:")):
            header = True
        elif s.startswith("|") and name is not None:
            if header:
                header = False
            else:
                rows += 1
        elif s.split(" ")[0] in ("Given", "When", "Then", "And", "But", "*") and name is not None:
            steps += 1
            whens += s.startswith("When")
    flush()


def has_package_comment(text):
    m = re.search(r"^((?://[^\n]*\n)+)package\s", text, re.M)
    return bool(m and re.match(r"// (Package|Command) \w", m[1]))


def main():
    cfg = load_config()
    check_go_version()
    exceptions = load_exceptions(cfg["exceptions_file"])
    out, docs = [], {}
    for p in go_files(cfg):
        text = p.read_text(errors="replace")
        check_file(p, text, cfg, out)
        if not p.name.endswith("_test.go"):
            docs[str(p.parent)] = docs.get(str(p.parent), 0) + has_package_comment(text)
    check_packages(docs, out)
    for p in sorted(Path(".").rglob("*.feature")):
        if not ignored_by_go(p) and not skipped(p, cfg):
            check_feature(p, p.read_text(errors="replace"), out)
    found = {(f[0], f[1]) for f in out}
    findings = [f for f in out if (f[0], f[1]) not in exceptions]
    for (rule, path), n in exceptions.items():
        if (rule, path) not in found:
            findings.append(("EXCEPTION", path, f"{cfg['exceptions_file']}:{n}: {rule} matches no finding"))
    for rule, path, detail in findings:
        ptr = f" (skill {POINTERS[rule][0]}, section {POINTERS[rule][1]})" if rule in POINTERS else ""
        print(f"{rule} {path}: {detail}{ptr}")
    sys.exit(1 if findings else 0)


if __name__ == "__main__":
    main()
