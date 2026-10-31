// Package command is the command channel atom: raw Markdown on stdin, the thread key in
// NESS_THREAD_KEY, no length limit (design §9, R-NTF-029).
package command

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

// Channel runs Command in Dir through Run for every post.
type Channel struct {
	Command, Dir string
	Run          shell.Runner
	// Timeout kills a hung command; 0 means DefaultTimeout.
	Timeout time.Duration
	// Secret is masked in the error text, if the command's output could echo one.
	Secret string
}

// DefaultTimeout bounds every post: a hung command is killed with its process group.
const DefaultTimeout = 30 * time.Second

// quoted is how much of a failing command's output its error quotes.
const quoted = 512

// Kind implements notify.Channel.
func (Channel) Kind() string { return "command" }

// NeedsThread implements notify.Channel.
func (Channel) NeedsThread() bool { return false }

// ThreadIsKey implements notify.Channel.
func (Channel) ThreadIsKey() bool { return true }

// Limit is none: one part (R-NTF-029).
func (Channel) Limit() int { return 0 }

// MaxBlocks implements notify.Renderer: none.
func (Channel) MaxBlocks() int { return 0 }

// Measure implements notify.Renderer.
func (Channel) Measure(s string) int { return len(s) }

// Render is the identity: raw Markdown.
func (Channel) Render(b notify.Block) string { return b.Raw }

// Payload is the stdin text.
func (Channel) Payload(p notify.Part, _ string) []byte { return []byte(p.Text()) }

// Post runs the command with the text on stdin and the key in NESS_THREAD_KEY; the key is the
// thread id. The text is never part of the command line or the environment.
func (c Channel) Post(p notify.Part, thread string) (string, error) {
	if c.Run == nil {
		return "", errors.New("command channel: no runner")
	}
	env := []string{"NESS_THREAD_KEY=" + thread}
	limit := c.Timeout
	if limit <= 0 {
		limit = DefaultTimeout
	}
	r, err := c.Run(shell.Cmd{Command: c.Command, Dir: c.Dir, Env: env, Stdin: p.Text(), Timeout: limit})
	if err != nil {
		return "", fmt.Errorf("command channel: %w", err)
	}
	if r.TimedOut {
		return "", fmt.Errorf("command channel: timed out after %s", limit)
	}
	if r.Exit != 0 {
		out := r.Output
		if len(out) > quoted {
			out = append(out[:quoted:quoted], "…"...)
		}
		return "", fmt.Errorf("command channel exited %d: %s", r.Exit,
			notify.Redact(strings.TrimSpace(string(out)), c.Secret))
	}
	return thread, nil
}
