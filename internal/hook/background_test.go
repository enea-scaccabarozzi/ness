package hook_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
)

func bash(cmd string) hook.Payload {
	return payload("Bash", map[string]any{"command": cmd})
}

// Each blocked row rejects an implementation that only looks for a trailing "&" or only for
// "( ... ) &"; the pgrep rows reject one that ignores loops or the -f flag cluster.
func TestPreToolRefusesBackgroundingAndSelfMatchingWaits(t *testing.T) {
	// covers: R-HOOK-020, R-HOOK-021
	for _, cmd := range []string{
		"( git commit -F m > c.log 2>&1; echo done >> c.log ) &",
		"sleep 5 &",
		"nohup ./run.sh > o.log 2>&1 &",
		"make a & make b",
		"echo hi &\n",
		"until ! pgrep -f 'pilot.py 500'; do sleep 10; done",
		"while pgrep -af land.sh; do sleep 5; done",
		"until ! pgrep --full 'pilot.py'; do sleep 10; done", // F7: the long flag
		"while pgrep -u me -f land.sh; do sleep 5; done",     // F7: a flag with a value before -f
		"for i in $(seq 60); do pgrep -f x || break; sleep 5; done",
		"while [ -n \"$(pgrep -f pilot.py)\" ]; do sleep 5; done",
		"echo $(( x << 1 ))\nnohup srv &",              // F7: arithmetic is not a here-doc
		"git commit -m \"$(cat <<'EOF'\nx\nEOF\n)\" &", // F7: skipping the here-doc must not hide a real &
		"make a & make b & wait\nnohup srv &",          // F7: only an & followed by wait on its line passes
	} {
		reason := hook.PreTool(bash(cmd))
		if !strings.Contains(reason, "run_in_background") {
			t.Errorf("%q: want a refusal naming run_in_background, got %q", cmd, reason)
		}
	}
}

// Each allowed row rejects an implementation that treats any "&" (or any "pgrep") as a hit.
func TestPreToolAllowsNonBackgroundingAmpersands(t *testing.T) {
	// covers: R-HOOK-020, R-HOOK-021
	for _, cmd := range []string{
		"a && b",
		"a&&b",
		"cmd &> out.log",
		"cmd >& out.log",
		"cmd 2>&1 | tee x",
		"cmd |& tee x",
		`echo "a & b"`,
		"echo 'x &'",
		"bash -c 'sleep 1 &'",
		`echo a \& b`,
		"# cmd &",
		"cat <<'EOF'\nfoo & bar\nEOF\nls",
		"cat <<-EOF\n\tfoo & bar\n\tEOF\nls",
		"grep -f pat file",
		"pgrep -x foo",
		"until ! pgrep -x foo; do sleep 1; done",
		// F7: a here-doc inside a command substitution inside double quotes; the odd " in the
		// body must not flip the quote state (the review's false refusal of a commit).
		"git commit -m \"$(cat <<'EOF'\nfeat: the 6\" pipe & co\nEOF\n)\"",
		"gh pr create --title t --body \"$(cat <<'EOF'\n## Summary\nuses \"a & b\" and a lone \"\nEOF\n)\"",
		"echo $(( a & b ))",       // F7: arithmetic
		"echo $(( a & b )) && ls", // F7: arithmetic then a normal command
		"cmd & wait",              // F7: foreground in effect
		"a & b & wait",            // F7: foreground in effect
		"echo ${v#*&}",            // F7: inside a parameter expansion
		"pgrep -x foo -u me",      // F7: guard stays off without -f
		// field 3: one-shot lookups, not waits
		"pgrep -af land.sh",
		"pgrep -af 'ness worker' || echo none",
		"for p in $(pgrep -f 'ness worker'); do readlink /proc/$p/cwd; done",
		"for f in a b; do echo $f; done; pgrep -af land.sh",
	} {
		if reason := hook.PreTool(bash(cmd)); reason != "" {
			t.Errorf("%q: want allowed, got %q", cmd, reason)
		}
	}
}

// Rejects an implementation that ignores the run_in_background flag.
func TestPreToolAllowsBackgroundWhenTheCallIsBackground(t *testing.T) {
	// covers: R-HOOK-020
	p := payload("Bash", map[string]any{"command": "sleep 5 &", "run_in_background": true})
	if reason := hook.PreTool(p); reason != "" {
		t.Errorf("run_in_background call refused: %q", reason)
	}
}
