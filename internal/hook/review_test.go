package hook_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
)

// Rejects a loop scanner that ends a loop at any word "done": a retry loop that prints "done"
// still polls (the old whole-command scan blocked all of these).
func TestLoopsPrintingDoneAreStillRefused(t *testing.T) {
	// covers: R-HOOK-025, R-HOOK-011
	blocked := []string{
		"while true; do echo done; sleep 5; ness integrate; done",
		"while true; do echo 'done'; sleep 5; ness integrate; done",
		"while true; do echo \"done\"; ness inbox; sleep 2; done",
		"while true; do printf 'all done\\n'; ness inbox; sleep 2; done",
		"while true; do echo all done; sleep 5; ness integrate; done",
		"while true; do echo done; ness status; done",
		"for i in 1 2; do echo done; done; while true; do sleep 5; ness integrate; done",
	}
	for _, c := range blocked {
		reason := hook.PreTool(bashCmd(c))
		if !strings.Contains(reason, "end your turn") && !strings.Contains(reason, "sleep") {
			t.Errorf("%q: allowed or no waiting rule: %q", c, reason)
		}
	}
	allowed := []string{
		"for i in 1 2; do echo done; done; ness status",
		"while read l; do echo 'done'; done < f; sleep 5; ness integrate",
	}
	for _, c := range allowed {
		if reason := hook.PreTool(bashCmd(c)); reason != "" {
			t.Errorf("%q: blocked: %q", c, reason)
		}
	}
}

// Rejects guards that fire on prose: a here-doc body or a quoted string that merely mentions a
// loop around a ness command is text, not a loop. (A quoted string handed to bash -c stays code;
// the existing tables pin that.)
func TestLoopGuardsIgnoreProse(t *testing.T) {
	// covers: R-HOOK-025, R-HOOK-011
	allowed := []string{
		"git commit -m \"$(cat <<'EOF'\nfix: retry while the gate is busy\nsleep is gone; ness integrate now\nEOF\n)\"",
		"cat > note.md <<'EOF'\nwhile true; do sleep 5; ness integrate; done\nEOF",
		"cat > note.md <<EOF\nfor i in 1 2; do ness status; done\nEOF",
		"git commit -m 'fix: while waiting, sleep then ness integrate'",
		"echo \"while true; do sleep 5; ness integrate; done\"",
		"echo 'for i in 1 2 3; do ness status; done' > note.txt",
		"git commit -m \"docs: for x in a; do ness inbox; done is refused\"",
	}
	for _, c := range allowed {
		if reason := hook.PreTool(bashCmd(c)); reason != "" {
			t.Errorf("%q: blocked: %q", c, reason)
		}
	}
	// A real loop after the prose is still a loop.
	blocked := []string{
		"git commit -m 'while x' && while true; do sleep 5; ness integrate; done",
		"cat <<'EOF'\nhello\nEOF\nwhile true; do ness inbox; sleep 2; done",
	}
	for _, c := range blocked {
		if reason := hook.PreTool(bashCmd(c)); reason == "" {
			t.Errorf("%q: allowed", c)
		}
	}
}
