package shell_test

import (
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

func TestRunReportsExitCodesOutputCwdEnvAndStdin(t *testing.T) {
	// The runner is what floors (P5b) and the command channel need from a process: its exit
	// code, combined output, cwd, added env and stdin (R-NTF-029, R-PRB-083).
	// covers: R-NTF-029
	dir := t.TempDir()
	r, err := shell.Run(shell.Cmd{Command: "pwd; echo $NESS_THREAD_KEY; cat; echo err >&2; exit 3",
		Dir: dir, Env: []string{"NESS_THREAD_KEY=k1"}, Stdin: "in\n"})
	if err != nil {
		t.Fatal(err)
	}
	want := dir + "\nk1\nin\nerr\n"
	if r.Exit != 3 || r.TimedOut || string(r.Output) != want {
		t.Fatalf("Result = %+v (%q), want exit 3 and %q", r, r.Output, want)
	}
	if r, err = shell.Run(shell.Cmd{Command: "exit 0", Dir: dir}); err != nil || r.Exit != 0 {
		t.Fatalf("exit 0: %+v, %v", r, err)
	}
	t.Setenv("GIT_DIR", "/nowhere")
	got, _ := shell.Run(shell.Cmd{Command: "echo ${GIT_DIR:-unset}", Dir: dir})
	if strings.TrimSpace(string(got.Output)) != "unset" {
		t.Fatalf("GIT_DIR reached the command: %q", got.Output)
	}
}

// gone reports whether pid no longer runs: missing from /proc, or a zombie awaiting its reaper
// (kill(pid, 0) succeeds on a zombie, so it cannot tell).
func gone(pid int) bool {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	// "pid (comm) S ...": the state follows the last ")".
	s := string(b)
	i := strings.LastIndex(s, ")")
	return i >= 0 && len(s) > i+2 && s[i+2] == 'Z'
}

// waitGone waits until pid is gone or a zombie. SIGKILL is delivered asynchronously, so a bounded
// poll on the state (not a fixed delay) is the honest wait.
func waitGone(t *testing.T, pid int) {
	t.Helper()
	tick, stop := time.NewTicker(time.Millisecond), time.After(5*time.Second)
	defer tick.Stop()
	for !gone(pid) {
		select {
		case <-tick.C:
		case <-stop:
			_ = syscall.Kill(pid, syscall.SIGKILL) // do not leave it behind
			t.Fatalf("sleep %d is still running", pid)
		}
	}
}

func TestRunKillsTheProcessGroupAtTheTimeout(t *testing.T) {
	// A floor past its timeout is UNCLEAR and the process is gone (R-PRB-022): the whole group,
	// so a background child of the sh dies too. The test waits on the process, never sleeps.
	// covers: R-PRB-022
	r, err := shell.Run(shell.Cmd{Command: "sleep 30 & echo $!; wait", Dir: t.TempDir(), Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if !r.TimedOut {
		t.Fatalf("Result = %+v; want TimedOut", r)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(r.Output)))
	if err != nil {
		t.Fatalf("no pid in output %q", r.Output)
	}
	waitGone(t, pid)
	if _, err = shell.Run(shell.Cmd{Command: "true", Dir: "/nonexistent/dir"}); err == nil {
		t.Fatal("a missing dir must be a start error")
	}
}

func TestRunCapsOutputAndKeepsDraining(t *testing.T) {
	// A child writing 10 MB to each stream neither blocks nor grows the result: 64 KiB per stream
	// is kept and the cut is marked.
	// covers: R-NTF-029
	r, err := shell.Run(shell.Cmd{Command: "head -c 10000000 /dev/zero | tr '\\0' x; head -c 10000000 /dev/zero | tr '\\0' y >&2",
		Dir: t.TempDir(), Timeout: time.Minute})
	if err != nil || r.Exit != 0 || r.TimedOut {
		t.Fatalf("Result exit %d timedout %v err %v", r.Exit, r.TimedOut, err)
	}
	if len(r.Output) != 2*shell.OutputCap+len(shell.Truncated) || !strings.HasSuffix(string(r.Output), shell.Truncated) {
		t.Fatalf("len(Output) = %d, want %d and the truncation mark", len(r.Output), 2*shell.OutputCap+len(shell.Truncated))
	}
}

func TestRunSucceedsWhenABackgroundChildHoldsThePipes(t *testing.T) {
	// sh exits 0 but its background sleep keeps stdout open: the run ends after the wait delay
	// with the real status, not a start error, and the straggler is killed.
	// covers: R-NTF-029
	start := time.Now()
	r, err := shell.Run(shell.Cmd{Command: "sleep 30 & echo $!; exit 0", Dir: t.TempDir()})
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("Run waited %v for the background child", took)
	}
	if err != nil || r.Exit != 0 || r.TimedOut {
		t.Fatalf("Result = %+v, %v; want a clean exit 0", r, err)
	}
	pid, perr := strconv.Atoi(strings.TrimSpace(string(r.Output)))
	if perr != nil {
		t.Fatalf("no pid in %q", r.Output)
	}
	waitGone(t, pid)
	r, err = shell.Run(shell.Cmd{Command: "sleep 30 & echo $!; exit 5", Dir: t.TempDir()})
	if err != nil || r.Exit != 5 {
		t.Fatalf("exit 5 with a straggler: %+v, %v", r, err)
	}
	if pid, perr = strconv.Atoi(strings.TrimSpace(string(r.Output))); perr != nil {
		t.Fatalf("no pid in %q", r.Output)
	}
	waitGone(t, pid)
}

func TestRunKeepTailKeepsTheEndWithoutAMarker(t *testing.T) {
	// A floor's evidence is the end of its output (R-PRB-039): KeepTail drops from the front.
	// covers: R-PRB-039
	c := shell.Cmd{Command: "yes x | head -c 1000000; echo done", Dir: t.TempDir()}
	r, err := shell.Run(c)
	if err != nil || strings.HasSuffix(string(r.Output), "done\n") {
		t.Fatalf("default keeps the head: %v, tail %q", err, r.Output[len(r.Output)-20:])
	}
	c.KeepTail = true
	if r, err = shell.Run(c); err != nil {
		t.Fatal(err)
	}
	if len(r.Output) != 2*shell.OutputCap || !strings.HasSuffix(string(r.Output), "x\ndone\n") ||
		strings.Contains(string(r.Output), shell.Truncated) {
		t.Fatalf("KeepTail: %d bytes, tail %q", len(r.Output), r.Output[len(r.Output)-20:])
	}
}

func TestRunSplitStderrKeepsStdoutClean(t *testing.T) {
	// covers: R-MOD-045
	// Rejects: stderr noise mixed into the stdout a protocol is parsed from.
	r, err := shell.Run(shell.Cmd{Command: "echo warn >&2; echo '{}'", SplitStderr: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(r.Output) != "{}\n" || string(r.Stderr) != "warn\n" {
		t.Fatalf("Output = %q, Stderr = %q", r.Output, r.Stderr)
	}
}
