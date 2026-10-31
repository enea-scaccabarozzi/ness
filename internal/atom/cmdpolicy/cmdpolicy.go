// Package cmdpolicy is the selector that hands the brief to a project command as JSON and reads its verdict back.
package cmdpolicy

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

// Policy runs Command in Dir through Run, bounded by Timeout, for every brief (R-MOD-045).
type Policy struct {
	Command, Dir string
	Timeout      time.Duration
	Run          shell.Runner
}

type input struct {
	Text      string `json:"text"`
	Role      string `json:"role"`
	Client    string `json:"client"`
	Kind      string `json:"kind"`
	AgentType string `json:"agent_type"`
}

type output struct {
	Offset     int     `json:"offset"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// Choose feeds b to the command on stdin and takes the one JSON object on its stdout as the
// verdict; a failed run, a timeout or an answer off the protocol is an error naming the command
// and carrying its stderr.
func (p Policy) Choose(b session.Brief) (session.Verdict, error) {
	var stderr []byte
	fail := func(format string, a ...any) (session.Verdict, error) {
		msg := fmt.Sprintf(format, a...)
		if e := strings.TrimSpace(string(stderr)); e != "" {
			msg += ": " + e
		}
		return session.Verdict{}, fmt.Errorf("selector %q: %s", p.Command, msg)
	}
	in, err := json.Marshal(input{b.Text, b.Role, b.Client, b.Kind, b.AgentType})
	if err != nil {
		return fail("%v", err)
	}
	c := shell.Cmd{Command: p.Command, Dir: p.Dir, Stdin: string(in), Timeout: p.Timeout, SplitStderr: true}
	res, err := p.Run(c)
	stderr = res.Stderr
	switch {
	case err != nil:
		return fail("%v", err)
	case res.TimedOut:
		return fail("timed out after %s", p.Timeout)
	case res.Exit != 0:
		return fail("exit %d", res.Exit)
	}
	var out output
	if err := json.Unmarshal(res.Output, &out); err != nil {
		return fail("not one JSON verdict: %v", err)
	}
	if out.Confidence < 0 || out.Confidence > 1 {
		return fail("confidence %v outside 0..1", out.Confidence)
	}
	if strings.ContainsAny(out.Reason, "\r\n") {
		return fail("reason is not one line")
	}
	return session.Verdict{Offset: out.Offset, Confidence: out.Confidence, Reason: out.Reason}, nil
}
