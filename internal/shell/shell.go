// Package shell runs one command the way a probe floor and the command channel need it: sh -c,
// stdin, added env, combined output, the exit code, a timeout that kills the whole process group.
// The Exec seam (stdout only, no timeout) cannot carry these; this is the second seam.
package shell

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Cmd is one command: run by sh -c in Dir with the inherited environment (git discovery
// variables stripped) plus Env, Stdin on its stdin, killed with its group after Timeout (0: never).
type Cmd struct {
	Command, Dir string
	Env          []string
	Stdin        string
	Timeout      time.Duration
	// KeepTail keeps the last 2*OutputCap bytes of the output, dropping from the front, with no
	// truncation marker; the default keeps the head and marks the cut.
	KeepTail bool
	// SplitStderr keeps stderr out of Output, in Result.Stderr (a protocol read from stdout).
	SplitStderr bool
}

// Result is what the command did: its combined output, exit code, and whether it was killed.
type Result struct {
	Output   []byte
	Exit     int
	TimedOut bool
	// Stderr is the command's stderr when Cmd.SplitStderr asked for it, capped like Output.
	Stderr []byte
}

// Runner is the seam: Run in production, a fake in the worker tests.
type Runner func(Cmd) (Result, error)

// OutputCap is half of what Run keeps of the combined stdout and stderr; the rest is read and
// discarded so the child never blocks on a full pipe.
const OutputCap = 64 << 10

// Truncated ends an Output that hit OutputCap.
const Truncated = "…(truncated)"

// sink keeps stdout and stderr in one buffer, in the order the child wrote them, capped at
// 2*OutputCap. One writer for both streams makes os/exec give the child a single pipe: two
// writers would mean two pipes and no order between them.
type sink struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	trunc bool
	tail  bool
}

func (s *sink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tail {
		s.buf.Write(p)
		if over := s.buf.Len() - 2*OutputCap; over > 0 {
			s.buf.Next(over)
		}
		return len(p), nil
	}
	room := min(2*OutputCap-s.buf.Len(), len(p))
	s.buf.Write(p[:room])
	if room < len(p) {
		s.trunc = true
	}
	return len(p), nil
}

func (s *sink) bytes() []byte {
	if s.trunc {
		s.buf.WriteString(Truncated)
	}
	return s.buf.Bytes()
}

// exitCode is the shell-style exit code of a finished process: 128+signal when killed.
func exitCode(ps *os.ProcessState) int {
	if c := ps.ExitCode(); c >= 0 {
		return c
	}
	if ws, ok := ps.Sys().(syscall.WaitStatus); ok {
		return 128 + int(ws.Signal())
	}
	return 1
}

// Run runs c. The error is only "could not start"; a non-zero exit or a timeout is in Result.
// ponytail: only the process group is killed; a child that calls setsid escapes it. Containing
// that needs a cgroup or a pid namespace.
func Run(c Cmd) (Result, error) {
	ctx, cancel := context.Background(), func() {}
	if c.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
	}
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", c.Command) //nolint:gosec // the configured command
	cmd.Dir = c.Dir
	cmd.Env = append(store.CleanGitEnv(os.Environ()), c.Env...)
	cmd.Stdin = bytes.NewReader([]byte(c.Stdin))
	out := sink{tail: c.KeepTail}
	errOut := sink{}
	cmd.Stdout, cmd.Stderr = &out, &out
	if c.SplitStderr {
		cmd.Stderr = &errOut
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// The group, not just sh: a background child holding the output must die with it.
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	// A background child can keep the pipes open after sh exits: wait this long, then close them.
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	if cmd.Process != nil {
		// Whatever the outcome, no straggler of the group outlives the run (ESRCH when none is left).
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) //nolint:errcheck // gone is fine
	}
	r := Result{TimedOut: errors.Is(ctx.Err(), context.DeadlineExceeded)}
	var ee *exec.ExitError
	switch {
	case errors.Is(err, exec.ErrWaitDelay):
		// sh exited but stragglers held the pipes: its own status stands.
		r.Exit = exitCode(cmd.ProcessState)
	case err == nil:
	case errors.As(err, &ee):
		r.Exit = exitCode(ee.ProcessState)
	case r.TimedOut:
		r.Exit = 137
	default:
		return Result{}, err
	}
	r.Output = out.bytes()
	if c.SplitStderr {
		r.Stderr = errOut.bytes()
	}
	return r, nil
}
