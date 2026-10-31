package check

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

// detailCap bounds a run check's output in the refusal (R-CHK-008).
const detailCap = 1500

// env is the context a run check is handed; a variable that does not apply to the event is
// absent, one that applies but is empty is set empty (R-CHK-004).
func (c Context) env() []string {
	e := []string{
		"NESS_EVENT=" + c.Event, "NESS_SESSION=" + c.Session, "NESS_ROOT=" + c.Root,
		"NESS_WORKTREE=" + c.Worktree, "NESS_PRIMARY=" + c.Primary,
	}
	if c.Event == "open" {
		return append(e, "NESS_SCOPE="+c.Scope)
	}
	return append(e, "NESS_BRANCH="+c.Branch, "NESS_INTO="+c.Into, "NESS_RECORD="+c.Record)
}

// runCommand runs a script check through the runner and fails closed (R-CHK-006).
func runCommand(ctx Context, c profile.Check, run shell.Runner) (string, bool) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	res, err := run(shell.Cmd{Command: c.Run, Dir: ctx.Worktree, Env: ctx.env(), Timeout: timeout})
	if err != nil {
		return err.Error(), false
	}
	if res.Exit == 0 && !res.TimedOut {
		return "", true
	}
	out := capped(strings.TrimSpace(string(res.Output)))
	switch {
	case res.TimedOut && out != "":
		return fmt.Sprintf("timed out after %s: %s", timeout, out), false
	case res.TimedOut:
		return fmt.Sprintf("timed out after %s", timeout), false
	case out == "":
		return fmt.Sprintf("exit %d", res.Exit), false
	}
	return out, false
}

// capped cuts s to detailCap bytes without splitting a character.
func capped(s string) string {
	if len(s) <= detailCap {
		return s
	}
	s = s[:detailCap]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
