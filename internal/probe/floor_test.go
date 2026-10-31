package probe_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

func floor(cmd string, expect string) probe.Probe {
	p, err := probe.Parse("probes/x.md", []byte("---\nevery: 1m\nfloor: "+cmd+"\ntimeout: 200ms\n"+expect+"\n---\n"))
	if err != nil {
		panic(err)
	}
	return p
}

func TestFloorStates(t *testing.T) {
	// exit 0 GREEN, 1 RED, other UNCLEAR, a timeout UNCLEAR; an unmet expect on the last line
	// is VACUOUS, never GREEN (R-PRB-020..023).
	// covers: R-PRB-020, R-PRB-021, R-PRB-022, R-PRB-023, R-PRB-025
	f := probe.Floor{Dir: t.TempDir(), Owner: "s-000000000001", Getenv: func(string) string { return "" }}
	for _, tc := range []struct{ cmd, expect, want string }{
		{"exit 0", "", probe.Green},
		{"exit 1", "", probe.Red},
		{"exit 2", "", probe.Unclear},
		{"sleep 30", "", probe.Unclear},
		{"echo n=0; exit 0", "expect: n>=1", probe.Vacuous},
		{"echo n=3; exit 0", "expect: n>=1", probe.Green},
		{"echo hello; exit 0", "expect: n>=1", probe.Vacuous},
		{"echo n=2 lag=10", "expect: n>=1, lag<50", probe.Green},
		{"echo n=2 lag=60", "expect: n>=1, lag<50", probe.Vacuous},
		{"echo n=0; exit 1", "expect: n>=1", probe.Red},
	} {
		r, err := probe.RunFloor(shell.Run, floor(tc.cmd, tc.expect), f)
		if err != nil || r.State != tc.want {
			t.Errorf("floor %q %q: state %s, %v; want %s", tc.cmd, tc.expect, r.State, err, tc.want)
		}
		if tc.cmd == "sleep 30" && !r.TimedOut {
			t.Error("the timeout was not reported")
		}
	}
}

// gone reports whether pid no longer runs: missing from /proc, or a zombie awaiting its reaper
// (kill(pid, 0) succeeds on a zombie, and an orphaned grandchild is one until init reaps it).
func gone(pid int) bool {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	s := string(b)
	i := strings.LastIndex(s, ")")
	return i >= 0 && len(s) > i+2 && s[i+2] == 'Z'
}

func TestFloorTimeoutKillsTheProcessGroup(t *testing.T) {
	// A timed-out floor's process is really gone (R-PRB-022), the whole group: the floor
	// backgrounds a grandchild and records its pid; after the timeout it is gone. Bounded poll,
	// SIGKILL is delivered asynchronously.
	// covers: R-PRB-022
	dir := t.TempDir()
	p := floor("sleep 30 & echo $! > pid; wait", "")
	p.Timeout = time.Second
	f := probe.Floor{Dir: dir, Getenv: func(string) string { return "" }}
	r, err := probe.RunFloor(shell.Run, p, f)
	if err != nil || r.State != probe.Unclear || !r.TimedOut {
		t.Fatalf("Run = %+v, %v", r, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	tick, stop := time.NewTicker(time.Millisecond), time.After(5*time.Second)
	defer tick.Stop()
	for !gone(pid) {
		select {
		case <-tick.C:
		case <-stop:
			_ = syscall.Kill(pid, syscall.SIGKILL) // only the pid this test recorded
			t.Fatalf("the floor's grandchild %d is still running", pid)
		}
	}
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("kill -0 %d = %v", pid, err)
	}
}

func TestFloorEvidenceKeepsTheTailOfARealPipe(t *testing.T) {
	// The runner keeps the end of a chatty floor, so the failure line is the evidence (R-PRB-039).
	// covers: R-PRB-039
	p := floor("yes hunter2 | head -c 1000000; echo done", "")
	p.Timeout = 10 * time.Second
	f := probe.Floor{Dir: t.TempDir(), Redact: []string{"S"}, Getenv: func(string) string { return "hunter2" }}
	r, err := probe.RunFloor(shell.Run, p, f)
	if err != nil || !strings.HasSuffix(r.Evidence, "done\n") || len(r.Evidence) > probe.EvidenceCap ||
		strings.Contains(r.Evidence, "hunter2") {
		t.Fatalf("Evidence (%d bytes, %v) ends %q", len(r.Evidence), err, r.Evidence[max(0, len(r.Evidence)-20):])
	}
}

func TestFloorCwdEnvAndEvidence(t *testing.T) {
	// The floor runs in the owner's worktree with NESS_PROBE and NESS_SESSION (R-PRB-083); the
	// evidence is the combined output with every redact_env value replaced, then the last 4 KB
	// (R-PRB-039).
	// covers: R-PRB-083, R-PRB-039
	dir := t.TempDir()
	env := map[string]string{"SECRET": "hunter2", "EMPTY": ""}
	f := probe.Floor{Dir: dir, Owner: "s-000000000001", Redact: []string{"SECRET", "EMPTY", "UNSET"},
		Getenv: func(k string) string { return env[k] }}
	r, err := probe.RunFloor(shell.Run, floor("pwd; echo $NESS_PROBE $NESS_SESSION; echo token=hunter2 >&2", ""), f)
	if err != nil {
		t.Fatal(err)
	}
	want := dir + "\nprobes/x.md s-000000000001\ntoken=[redacted]\n"
	if r.Evidence != want {
		t.Fatalf("Evidence = %q, want %q", r.Evidence, want)
	}
	big := strings.Repeat("hunter2 line\n", 100000) // 1.3 MB with the secret on every line
	ev := probe.Evidence([]byte(big), f)
	red := probe.Redact(big, []string{"hunter2"})
	if len(ev) > probe.EvidenceCap || strings.Contains(ev, "hunter2") || !strings.HasSuffix(ev, "[redacted] line\n") {
		t.Fatalf("Evidence of 1 MB: %d bytes, redacted %v", len(ev), !strings.Contains(ev, "hunter2"))
	}
	if ev != red[len(red)-probe.EvidenceCap:] {
		t.Fatal("the evidence is not the last 4,096 bytes of the redacted output")
	}
	// The cut lands inside the é: its tail byte is dropped, the rest kept.
	if got := probe.Evidence([]byte("é"+strings.Repeat("x", probe.EvidenceCap-1)), f); len(got) != probe.EvidenceCap-1 {
		t.Fatalf("cut at a rune boundary: %d bytes", len(got))
	}
	// An invalid byte inside the window costs nothing but itself.
	bad := "\xff" + strings.Repeat("y", 100) + "\xfe" + strings.Repeat("z", 100)
	if got := probe.Evidence([]byte(bad), f); got != bad {
		t.Fatalf("invalid bytes truncated the evidence: %d of %d bytes", len(got), len(bad))
	}
	if got := probe.Evidence([]byte(strings.Repeat("a", 5000)+"\xff"+strings.Repeat("b", 10)), f); len(got) != probe.EvidenceCap {
		t.Fatalf("an invalid byte in the window cut the evidence to %d bytes", len(got))
	}
}

func TestRunFloorStartError(t *testing.T) {
	// covers: R-PRB-022
	f := probe.Floor{Dir: "/nonexistent", Getenv: func(string) string { return "" }}
	if _, err := probe.RunFloor(shell.Run, floor("true", ""), f); err == nil {
		t.Fatal("a floor that cannot start must be an error, not a state")
	}
}

func TestRedactOverlappingSecrets(t *testing.T) {
	// covers: R-PRB-039
	for _, secrets := range [][]string{{"abc", "abcdef"}, {"abcdef", "abc"}} {
		got := probe.Redact("x abcdef y", secrets)
		if strings.Contains(got, "def") || strings.Contains(got, "abc") {
			t.Errorf("Redact with %v left a fragment: %q", secrets, got)
		}
	}
	if got := probe.Redact("abc", []string{"red", "abc"}); got != "[redacted]" {
		t.Errorf("a replacement was matched again: %q", got)
	}
}
