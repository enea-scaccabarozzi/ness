// Package check runs the checks a profile declares with [[checks]] at ness open, integrate and
// close (R-CHK-001..014). A check is a built-in kind or a shell command; core knows the kinds and
// never a profile. Run returns every failure, Refusal renders them for the refusal message.
package check

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

// Git is the part of git the kinds need: Out runs git in dir and returns its trimmed stdout.
type Git interface {
	Out(dir string, args ...string) (string, error)
}

// Facts is what the checkpoint kind reads from the store; the zero time means none.
type Facts interface {
	// LastLanding is the latest integration of any descendant of root.
	LastLanding(root string) (time.Time, error)
	// LastCheckpoint is the latest checkpoint root sent to the operator.
	LastCheckpoint(root string) (time.Time, error)
}

// Deps are the collaborators of Run, wired in the CLI composition root.
type Deps struct {
	Git   Git
	Facts Facts
	Shell shell.Runner
}

// Context is what a check is told about the command it gates (R-CHK-004).
type Context struct {
	// Event is "open", "integrate" or "close".
	Event string
	// Child reports that the session the command is about is a child (R-CHK-003).
	Child bool
	// Session is the acting session, empty for the operator; Root is its top ancestor.
	Session, Root string
	// Worktree is the absolute directory examined; Primary the primary checkout.
	Worktree, Primary string
	// Size is the work size of the acting root (the one being opened, or the top root of the actor);
	// empty for the operator (R-CHK-015).
	Size string
	// Scope is the new session's scope (open only).
	Scope string
	// Branch, Into and Record are the session branch, the branch it lands in and its record
	// directory (integrate and close).
	Branch, Into, Record string
}

// Failure is one check that did not pass (R-CHK-008).
type Failure struct {
	// ID is the check's id as resolved, "<profile>/<id>".
	ID string
	// Message is the declared message, empty when a run check declares none.
	Message string
	// Pointer is the skill and section the check enforces.
	Pointer rule.Pointer
	// Detail says what was found: the command's output, or what was missing.
	Detail string
}

// defaultTimeout bounds a run check that declares none (R-CHK-006).
const defaultTimeout = 30 * time.Second

// Run runs the checks whose event is ctx.Event and whose filter admits ctx, in order, and returns
// every failure. It returns nil when all pass.
func Run(ctx Context, checks []profile.Check, deps Deps) []Failure {
	var out []Failure
	for _, c := range checks {
		if c.On != ctx.Event || (c.For == "child" && !ctx.Child) || (c.For == "root" && ctx.Child) ||
			(len(c.Size) > 0 && !slices.Contains(c.Size, ctx.Size)) {
			continue
		}
		if detail, ok := decide(ctx, c, deps); !ok {
			out = append(out, Failure{
				ID: c.ID, Message: c.Message, Pointer: rule.Pointer{Skill: c.Skill, Section: c.Section}, Detail: detail,
			})
		}
	}
	return out
}

// decide runs one check and returns what it found when it did not pass.
func decide(ctx Context, c profile.Check, deps Deps) (detail string, ok bool) {
	if c.Run != "" {
		return runCommand(ctx, c, deps.Shell)
	}
	return runKind(ctx, c, deps)
}

// Refusal renders failures as the text of a refused command (R-CHK-009); empty for none.
func Refusal(event string, failures []Failure) string {
	var b strings.Builder
	for _, f := range failures {
		msg := f.Message
		if msg == "" {
			msg = f.ID + " failed"
		}
		fmt.Fprintf(&b, "ness %s refused: %s %s\n  %s: %s\n", event, msg, f.Pointer, f.ID, f.Detail)
	}
	return b.String()
}
